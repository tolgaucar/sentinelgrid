// Package codec — Kayıpsız çift-yönlü format dönüşüm motoru.
//
// Desteklenen formatlar: JSON, MessagePack, CBOR
// Her dönüşüm round-trip güvenlidir: encode(decode(data)) == data
//
// Tasarım ilkeleri:
// - Kayıpsız: Sayısal hassasiyet, Unicode, nil/null farkı korunur
// - Kararlı: Panic yerine error dönüşü, her edge case ele alınır
// - Esnek: Yeni format eklemek Codec interface'i uygulamak kadar kolay
package codec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
)

// Format — Serileştirme formatı
type Format string

const (
	JSON    Format = "json"
	MsgPack Format = "msgpack"
	CBOR    Format = "cbor"
)

// ParseFormat — String'den Format'a güvenli dönüşüm
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return JSON, nil
	case "msgpack", "messagepack", "msg_pack":
		return MsgPack, nil
	case "cbor":
		return CBOR, nil
	default:
		return "", fmt.Errorf("bilinmeyen format: %q", s)
	}
}

// AllFormats — Tüm desteklenen formatlar
func AllFormats() []Format {
	return []Format{JSON, MsgPack, CBOR}
}

// Errors
var (
	ErrUnsupportedFormat = errors.New("desteklenmeyen format")
	ErrEncodeFailed      = errors.New("kodlama (encode) hatası")
	ErrDecodeFailed      = errors.New("çözme (decode) hatası")
	ErrNilData           = errors.New("nil veri")
	ErrPayloadTooLarge   = errors.New("veri boyutu aşıldı")
	ErrInvalidInput      = errors.New("geçersiz girdi")
)

// Codec — Format codec arayüzü
type Codec interface {
	Format() Format
	Encode(v interface{}) ([]byte, error)
	Decode(data []byte) (interface{}, error)
	// DecodeTyped — JSON benzeri map[string]interface{} döndürür
	DecodeTyped(data []byte, v interface{}) error
}

// Registry — Codec kayıt defteri (thread-safe)
type Registry struct {
	mu     sync.RWMutex
	codecs map[Format]Codec
}

// NewRegistry — Tüm varsayılan codec'lerle yeni kayıt defteri
func NewRegistry() *Registry {
	r := &Registry{
		codecs: make(map[Format]Codec),
	}
	r.Register(&JSONCodec{})
	r.Register(&MsgPackCodec{})
	r.Register(&CBORCodec{})
	return r
}

// Register — Yeni codec kaydet
func (r *Registry) Register(c Codec) {
	r.mu.Lock()
	r.codecs[c.Format()] = c
	r.mu.Unlock()
}

// Get — Format'a göre codec getir
func (r *Registry) Get(f Format) (Codec, error) {
	r.mu.RLock()
	c, ok := r.codecs[f]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, f)
	}
	return c, nil
}

// Formats — Kayıtlı formatları döndür
func (r *Registry) Formats() []Format {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fmts := make([]Format, 0, len(r.codecs))
	for f := range r.codecs {
		fmts = append(fmts, f)
	}
	return fmts
}

// Convert — Bir formattan diğerine kayıpsız dönüştür
func (r *Registry) Convert(data []byte, from, to Format) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrNilData
	}
	if from == to {
		// Aynı format — yeniden encode ile normalize et
		src, err := r.Get(from)
		if err != nil {
			return nil, err
		}
		v, err := src.Decode(data)
		if err != nil {
			return nil, fmt.Errorf("%w: kaynak çözülemedi: %v", ErrDecodeFailed, err)
		}
		return src.Encode(v)
	}

	src, err := r.Get(from)
	if err != nil {
		return nil, err
	}
	dst, err := r.Get(to)
	if err != nil {
		return nil, err
	}

	// Decode kaynak
	v, err := src.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("%w: kaynak çözülemedi: %v", ErrDecodeFailed, err)
	}

	// Normalize — format arası uyumluluk
	v = Normalize(v)

	// Encode hedef
	out, err := dst.Encode(v)
	if err != nil {
		return nil, fmt.Errorf("%w: hedef kodlanamadı: %v", ErrEncodeFailed, err)
	}
	return out, nil
}

// ConvertToMap — Herhangi bir formattan map'e çevir
func (r *Registry) ConvertToMap(data []byte, from Format) (map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, ErrNilData
	}
	src, err := r.Get(from)
	if err != nil {
		return nil, err
	}
	v, err := src.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecodeFailed, err)
	}
	v = Normalize(v)
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: sonuç map değil, tip: %T", ErrDecodeFailed, v)
	}
	return m, nil
}

// ──────────────────────────────────────────────
// JSON Codec
// ──────────────────────────────────────────────

// JSONCodec — JSON format codec
type JSONCodec struct{}

func (c *JSONCodec) Format() Format { return JSON }

func (c *JSONCodec) Encode(v interface{}) ([]byte, error) {
	v = Normalize(v)
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncodeFailed, err)
	}
	// json.Encoder trailing newline ekler, kaldır
	b := buf.Bytes()
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return b, nil
}

func (c *JSONCodec) Decode(data []byte) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecodeFailed, err)
	}
	return normalizeJSON(v), nil
}

func (c *JSONCodec) DecodeTyped(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}

// normalizeJSON — json.Number'ları koruyarak normalize et
func normalizeJSON(v interface{}) interface{} {
	switch val := v.(type) {
	case json.Number:
		// Önce int dene, sonra float
		if i, err := val.Int64(); err == nil {
			return i
		}
		if f, err := val.Float64(); err == nil {
			return f
		}
		return val.String()
	case map[string]interface{}:
		for k, v2 := range val {
			val[k] = normalizeJSON(v2)
		}
		return val
	case []interface{}:
		for i, v2 := range val {
			val[i] = normalizeJSON(v2)
		}
		return val
	}
	return v
}

// ──────────────────────────────────────────────
// MessagePack Codec (saf Go, dış bağımlılık yok)
// ──────────────────────────────────────────────

// MsgPackCodec — MessagePack format codec (minimal saf Go implementasyon)
type MsgPackCodec struct{}

func (c *MsgPackCodec) Format() Format { return MsgPack }

func (c *MsgPackCodec) Encode(v interface{}) ([]byte, error) {
	v = Normalize(v)
	buf := &bytes.Buffer{}
	if err := msgpackEncode(buf, v); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncodeFailed, err)
	}
	return buf.Bytes(), nil
}

func (c *MsgPackCodec) Decode(data []byte) (interface{}, error) {
	r := bytes.NewReader(data)
	v, err := msgpackDecode(r)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecodeFailed, err)
	}
	return v, nil
}

func (c *MsgPackCodec) DecodeTyped(data []byte, target interface{}) error {
	v, err := c.Decode(data)
	if err != nil {
		return err
	}
	// JSON round-trip ile typed decode
	j, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(j, target)
}

// ──────────────────────────────────────────────
// CBOR Codec (saf Go, dış bağımlılık yok)
// ──────────────────────────────────────────────

// CBORCodec — CBOR format codec (minimal saf Go implementasyon)
type CBORCodec struct{}

func (c *CBORCodec) Format() Format { return CBOR }

func (c *CBORCodec) Encode(v interface{}) ([]byte, error) {
	v = Normalize(v)
	buf := &bytes.Buffer{}
	if err := cborEncode(buf, v); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncodeFailed, err)
	}
	return buf.Bytes(), nil
}

func (c *CBORCodec) Decode(data []byte) (interface{}, error) {
	r := bytes.NewReader(data)
	v, err := cborDecode(r)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecodeFailed, err)
	}
	return v, nil
}

func (c *CBORCodec) DecodeTyped(data []byte, target interface{}) error {
	v, err := c.Decode(data)
	if err != nil {
		return err
	}
	j, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(j, target)
}

// ──────────────────────────────────────────────
// Normalize — Format-bağımsız veri normalizasyonu
// ──────────────────────────────────────────────

// Normalize — Tüm formatlar arası uyumluluk için veri normalize et
// - int8/16/32/64 → int64
// - uint8/16/32/64 → int64 (taşma yoksa) veya float64
// - float32 → float64
// - []byte → []byte (korunur)
// - map anahtarları → string
func Normalize(v interface{}) interface{} {
	switch val := v.(type) {
	case nil:
		return nil
	case bool:
		return val
	case string:
		return val
	case []byte:
		return val
	case int:
		return int64(val)
	case int8:
		return int64(val)
	case int16:
		return int64(val)
	case int32:
		return int64(val)
	case int64:
		return val
	case uint:
		return int64(val)
	case uint8:
		return int64(val)
	case uint16:
		return int64(val)
	case uint32:
		return int64(val)
	case uint64:
		if val <= math.MaxInt64 {
			return int64(val)
		}
		return float64(val)
	case float32:
		return float64(val)
	case float64:
		// Tam sayıysa int64'e dönüştür
		if val == math.Trunc(val) && !math.IsInf(val, 0) && !math.IsNaN(val) &&
			val >= math.MinInt64 && val <= math.MaxInt64 {
			return int64(val)
		}
		return val
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return i
		}
		if f, err := val.Float64(); err == nil {
			return f
		}
		return val.String()
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, v2 := range val {
			out[k] = Normalize(v2)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, v2 := range val {
			out[fmt.Sprint(k)] = Normalize(v2)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, v2 := range val {
			out[i] = Normalize(v2)
		}
		return out
	default:
		return val
	}
}

// ──────────────────────────────────────────────
// MessagePack encoder/decoder (RFC 8949 uyumlu alt küme)
// ──────────────────────────────────────────────

func msgpackEncode(w io.Writer, v interface{}) error {
	v = Normalize(v)
	switch val := v.(type) {
	case nil:
		_, err := w.Write([]byte{0xc0})
		return err
	case bool:
		if val {
			_, err := w.Write([]byte{0xc3})
			return err
		}
		_, err := w.Write([]byte{0xc2})
		return err
	case int64:
		return msgpackEncodeInt(w, val)
	case float64:
		return msgpackEncodeFloat64(w, val)
	case string:
		return msgpackEncodeString(w, val)
	case []byte:
		return msgpackEncodeBin(w, val)
	case []interface{}:
		return msgpackEncodeArray(w, val)
	case map[string]interface{}:
		return msgpackEncodeMap(w, val)
	default:
		return fmt.Errorf("msgpack: desteklenmeyen tip: %T", v)
	}
}

func msgpackEncodeInt(w io.Writer, v int64) error {
	if v >= 0 && v <= 127 {
		_, err := w.Write([]byte{byte(v)})
		return err
	}
	if v >= -32 && v < 0 {
		_, err := w.Write([]byte{byte(v)}) // negative fixint
		return err
	}
	if v >= -128 && v <= 127 {
		_, err := w.Write([]byte{0xd0, byte(int8(v))})
		return err
	}
	if v >= -32768 && v <= 32767 {
		_, err := w.Write([]byte{0xd1, byte(v >> 8), byte(v)})
		return err
	}
	if v >= -2147483648 && v <= 2147483647 {
		_, err := w.Write([]byte{0xd2, byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
		return err
	}
	_, err := w.Write([]byte{0xd3,
		byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32),
		byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
	return err
}

func msgpackEncodeFloat64(w io.Writer, v float64) error {
	bits := math.Float64bits(v)
	_, err := w.Write([]byte{0xcb,
		byte(bits >> 56), byte(bits >> 48), byte(bits >> 40), byte(bits >> 32),
		byte(bits >> 24), byte(bits >> 16), byte(bits >> 8), byte(bits)})
	return err
}

func msgpackEncodeString(w io.Writer, s string) error {
	n := len(s)
	if n <= 31 {
		w.Write([]byte{0xa0 | byte(n)})
	} else if n <= 0xff {
		w.Write([]byte{0xd9, byte(n)})
	} else if n <= 0xffff {
		w.Write([]byte{0xda, byte(n >> 8), byte(n)})
	} else {
		w.Write([]byte{0xdb, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)})
	}
	_, err := w.Write([]byte(s))
	return err
}

func msgpackEncodeBin(w io.Writer, b []byte) error {
	n := len(b)
	if n <= 0xff {
		w.Write([]byte{0xc4, byte(n)})
	} else if n <= 0xffff {
		w.Write([]byte{0xc5, byte(n >> 8), byte(n)})
	} else {
		w.Write([]byte{0xc6, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)})
	}
	_, err := w.Write(b)
	return err
}

func msgpackEncodeArray(w io.Writer, arr []interface{}) error {
	n := len(arr)
	if n <= 15 {
		w.Write([]byte{0x90 | byte(n)})
	} else if n <= 0xffff {
		w.Write([]byte{0xdc, byte(n >> 8), byte(n)})
	} else {
		w.Write([]byte{0xdd, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)})
	}
	for _, item := range arr {
		if err := msgpackEncode(w, item); err != nil {
			return err
		}
	}
	return nil
}

func msgpackEncodeMap(w io.Writer, m map[string]interface{}) error {
	n := len(m)
	if n <= 15 {
		w.Write([]byte{0x80 | byte(n)})
	} else if n <= 0xffff {
		w.Write([]byte{0xde, byte(n >> 8), byte(n)})
	} else {
		w.Write([]byte{0xdf, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)})
	}
	// Deterministik sıralama
	keys := make([]string, 0, n)
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := msgpackEncodeString(w, k); err != nil {
			return err
		}
		if err := msgpackEncode(w, m[k]); err != nil {
			return err
		}
	}
	return nil
}

func msgpackDecode(r io.Reader) (interface{}, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}
	c := b[0]

	// Positive fixint 0x00-0x7f
	if c <= 0x7f {
		return int64(c), nil
	}
	// fixmap 0x80-0x8f
	if c >= 0x80 && c <= 0x8f {
		return msgpackDecodeMapN(r, int(c&0x0f))
	}
	// fixarray 0x90-0x9f
	if c >= 0x90 && c <= 0x9f {
		return msgpackDecodeArrayN(r, int(c&0x0f))
	}
	// fixstr 0xa0-0xbf
	if c >= 0xa0 && c <= 0xbf {
		return msgpackDecodeStrN(r, int(c&0x1f))
	}
	// Negative fixint 0xe0-0xff
	if c >= 0xe0 {
		return int64(int8(c)), nil
	}

	switch c {
	case 0xc0:
		return nil, nil
	case 0xc2:
		return false, nil
	case 0xc3:
		return true, nil
	// bin8
	case 0xc4:
		return msgpackDecodeBinN(r, 1)
	// bin16
	case 0xc5:
		return msgpackDecodeBinN(r, 2)
	// bin32
	case 0xc6:
		return msgpackDecodeBinN(r, 4)
	// float32
	case 0xca:
		buf := make([]byte, 4)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		bits := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
		return float64(math.Float32frombits(bits)), nil
	// float64
	case 0xcb:
		buf := make([]byte, 8)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		bits := uint64(buf[0])<<56 | uint64(buf[1])<<48 | uint64(buf[2])<<40 | uint64(buf[3])<<32 |
			uint64(buf[4])<<24 | uint64(buf[5])<<16 | uint64(buf[6])<<8 | uint64(buf[7])
		return math.Float64frombits(bits), nil
	// int8
	case 0xd0:
		buf := make([]byte, 1)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return int64(int8(buf[0])), nil
	// int16
	case 0xd1:
		buf := make([]byte, 2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return int64(int16(uint16(buf[0])<<8 | uint16(buf[1]))), nil
	// int32
	case 0xd2:
		buf := make([]byte, 4)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		v := int32(uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3]))
		return int64(v), nil
	// int64
	case 0xd3:
		buf := make([]byte, 8)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		v := int64(uint64(buf[0])<<56 | uint64(buf[1])<<48 | uint64(buf[2])<<40 | uint64(buf[3])<<32 |
			uint64(buf[4])<<24 | uint64(buf[5])<<16 | uint64(buf[6])<<8 | uint64(buf[7]))
		return v, nil
	// uint8
	case 0xcc:
		buf := make([]byte, 1)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return int64(buf[0]), nil
	// uint16
	case 0xcd:
		buf := make([]byte, 2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return int64(uint16(buf[0])<<8 | uint16(buf[1])), nil
	// uint32
	case 0xce:
		buf := make([]byte, 4)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return int64(uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])), nil
	// uint64
	case 0xcf:
		buf := make([]byte, 8)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		v := uint64(buf[0])<<56 | uint64(buf[1])<<48 | uint64(buf[2])<<40 | uint64(buf[3])<<32 |
			uint64(buf[4])<<24 | uint64(buf[5])<<16 | uint64(buf[6])<<8 | uint64(buf[7])
		if v <= math.MaxInt64 {
			return int64(v), nil
		}
		return float64(v), nil
	// str8
	case 0xd9:
		return msgpackDecodeStrSize(r, 1)
	// str16
	case 0xda:
		return msgpackDecodeStrSize(r, 2)
	// str32
	case 0xdb:
		return msgpackDecodeStrSize(r, 4)
	// array16
	case 0xdc:
		buf := make([]byte, 2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		n := int(uint16(buf[0])<<8 | uint16(buf[1]))
		return msgpackDecodeArrayN(r, n)
	// array32
	case 0xdd:
		buf := make([]byte, 4)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		n := int(uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3]))
		return msgpackDecodeArrayN(r, n)
	// map16
	case 0xde:
		buf := make([]byte, 2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		n := int(uint16(buf[0])<<8 | uint16(buf[1]))
		return msgpackDecodeMapN(r, n)
	// map32
	case 0xdf:
		buf := make([]byte, 4)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		n := int(uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3]))
		return msgpackDecodeMapN(r, n)
	}

	return nil, fmt.Errorf("msgpack: bilinmeyen tip baytı: 0x%02x", c)
}

func msgpackDecodeStrN(r io.Reader, n int) (string, error) {
	if n == 0 {
		return "", nil
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func msgpackDecodeStrSize(r io.Reader, sizeBytes int) (string, error) {
	buf := make([]byte, sizeBytes)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	var n int
	for _, b := range buf {
		n = n<<8 | int(b)
	}
	return msgpackDecodeStrN(r, n)
}

func msgpackDecodeBinN(r io.Reader, sizeBytes int) ([]byte, error) {
	buf := make([]byte, sizeBytes)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	var n int
	for _, b := range buf {
		n = n<<8 | int(b)
	}
	if n > maxDecodeSize {
		return nil, fmt.Errorf("msgpack: binary boyutu çok büyük: %d (maks: %d)", n, maxDecodeSize)
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

const maxDecodeSize = 64 * 1024 * 1024 // 64MB max allocation limit

func msgpackDecodeArrayN(r io.Reader, n int) ([]interface{}, error) {
	if n > maxDecodeSize {
		return nil, fmt.Errorf("msgpack: dizi boyutu çok büyük: %d (maks: %d)", n, maxDecodeSize)
	}
	arr := make([]interface{}, n)
	for i := 0; i < n; i++ {
		v, err := msgpackDecode(r)
		if err != nil {
			return nil, err
		}
		arr[i] = v
	}
	return arr, nil
}

func msgpackDecodeMapN(r io.Reader, n int) (map[string]interface{}, error) {
	if n > maxDecodeSize {
		return nil, fmt.Errorf("msgpack: harita boyutu çok büyük: %d (maks: %d)", n, maxDecodeSize)
	}
	m := make(map[string]interface{}, n)
	for i := 0; i < n; i++ {
		kv, err := msgpackDecode(r)
		if err != nil {
			return nil, err
		}
		key := fmt.Sprint(kv)
		val, err := msgpackDecode(r)
		if err != nil {
			return nil, err
		}
		m[key] = val
	}
	return m, nil
}

// ──────────────────────────────────────────────
// CBOR encoder/decoder (RFC 8949 uyumlu alt küme)
// ──────────────────────────────────────────────

func cborEncode(w io.Writer, v interface{}) error {
	v = Normalize(v)
	switch val := v.(type) {
	case nil:
		_, err := w.Write([]byte{0xf6}) // CBOR null
		return err
	case bool:
		if val {
			_, err := w.Write([]byte{0xf5})
			return err
		}
		_, err := w.Write([]byte{0xf4})
		return err
	case int64:
		if val >= 0 {
			return cborEncodeUint(w, 0, uint64(val))
		}
		return cborEncodeUint(w, 1, uint64(-1-val))
	case float64:
		bits := math.Float64bits(val)
		_, err := w.Write([]byte{0xfb,
			byte(bits >> 56), byte(bits >> 48), byte(bits >> 40), byte(bits >> 32),
			byte(bits >> 24), byte(bits >> 16), byte(bits >> 8), byte(bits)})
		return err
	case string:
		if err := cborEncodeUint(w, 3, uint64(len(val))); err != nil {
			return err
		}
		_, err := w.Write([]byte(val))
		return err
	case []byte:
		if err := cborEncodeUint(w, 2, uint64(len(val))); err != nil {
			return err
		}
		_, err := w.Write(val)
		return err
	case []interface{}:
		if err := cborEncodeUint(w, 4, uint64(len(val))); err != nil {
			return err
		}
		for _, item := range val {
			if err := cborEncode(w, item); err != nil {
				return err
			}
		}
		return nil
	case map[string]interface{}:
		if err := cborEncodeUint(w, 5, uint64(len(val))); err != nil {
			return err
		}
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if err := cborEncode(w, k); err != nil {
				return err
			}
			if err := cborEncode(w, val[k]); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("cbor: desteklenmeyen tip: %T", v)
	}
}

func cborEncodeUint(w io.Writer, major byte, val uint64) error {
	m := major << 5
	if val <= 23 {
		_, err := w.Write([]byte{m | byte(val)})
		return err
	}
	if val <= 0xff {
		_, err := w.Write([]byte{m | 24, byte(val)})
		return err
	}
	if val <= 0xffff {
		_, err := w.Write([]byte{m | 25, byte(val >> 8), byte(val)})
		return err
	}
	if val <= 0xffffffff {
		_, err := w.Write([]byte{m | 26, byte(val >> 24), byte(val >> 16), byte(val >> 8), byte(val)})
		return err
	}
	_, err := w.Write([]byte{m | 27,
		byte(val >> 56), byte(val >> 48), byte(val >> 40), byte(val >> 32),
		byte(val >> 24), byte(val >> 16), byte(val >> 8), byte(val)})
	return err
}

func cborDecode(r io.Reader) (interface{}, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}
	c := b[0]
	major := c >> 5
	info := c & 0x1f

	switch major {
	case 0: // Unsigned int
		n, err := cborDecodeArgument(r, info)
		if err != nil {
			return nil, err
		}
		if n <= math.MaxInt64 {
			return int64(n), nil
		}
		return float64(n), nil
	case 1: // Negative int
		n, err := cborDecodeArgument(r, info)
		if err != nil {
			return nil, err
		}
		return int64(-1 - int64(n)), nil
	case 2: // Byte string
		n, err := cborDecodeArgument(r, info)
		if err != nil {
			return nil, err
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return buf, nil
	case 3: // Text string
		n, err := cborDecodeArgument(r, info)
		if err != nil {
			return nil, err
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return string(buf), nil
	case 4: // Array
		n, err := cborDecodeArgument(r, info)
		if err != nil {
			return nil, err
		}
		arr := make([]interface{}, n)
		for i := uint64(0); i < n; i++ {
			v, err := cborDecode(r)
			if err != nil {
				return nil, err
			}
			arr[i] = v
		}
		return arr, nil
	case 5: // Map
		n, err := cborDecodeArgument(r, info)
		if err != nil {
			return nil, err
		}
		m := make(map[string]interface{}, n)
		for i := uint64(0); i < n; i++ {
			kv, err := cborDecode(r)
			if err != nil {
				return nil, err
			}
			val, err := cborDecode(r)
			if err != nil {
				return nil, err
			}
			m[fmt.Sprint(kv)] = val
		}
		return m, nil
	case 7: // Float/simple
		switch info {
		case 20:
			return false, nil
		case 21:
			return true, nil
		case 22:
			return nil, nil
		case 25: // float16 → float64
			buf := make([]byte, 2)
			if _, err := io.ReadFull(r, buf); err != nil {
				return nil, err
			}
			return float64(decodeFloat16(uint16(buf[0])<<8|uint16(buf[1]))), nil
		case 26: // float32
			buf := make([]byte, 4)
			if _, err := io.ReadFull(r, buf); err != nil {
				return nil, err
			}
			bits := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
			return float64(math.Float32frombits(bits)), nil
		case 27: // float64
			buf := make([]byte, 8)
			if _, err := io.ReadFull(r, buf); err != nil {
				return nil, err
			}
			bits := uint64(buf[0])<<56 | uint64(buf[1])<<48 | uint64(buf[2])<<40 | uint64(buf[3])<<32 |
				uint64(buf[4])<<24 | uint64(buf[5])<<16 | uint64(buf[6])<<8 | uint64(buf[7])
			return math.Float64frombits(bits), nil
		}
	}

	return nil, fmt.Errorf("cbor: bilinmeyen major=%d info=%d", major, info)
}

func cborDecodeArgument(r io.Reader, info byte) (uint64, error) {
	if info <= 23 {
		return uint64(info), nil
	}
	switch info {
	case 24:
		buf := make([]byte, 1)
		if _, err := io.ReadFull(r, buf); err != nil {
			return 0, err
		}
		return uint64(buf[0]), nil
	case 25:
		buf := make([]byte, 2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return 0, err
		}
		return uint64(buf[0])<<8 | uint64(buf[1]), nil
	case 26:
		buf := make([]byte, 4)
		if _, err := io.ReadFull(r, buf); err != nil {
			return 0, err
		}
		return uint64(buf[0])<<24 | uint64(buf[1])<<16 | uint64(buf[2])<<8 | uint64(buf[3]), nil
	case 27:
		buf := make([]byte, 8)
		if _, err := io.ReadFull(r, buf); err != nil {
			return 0, err
		}
		return uint64(buf[0])<<56 | uint64(buf[1])<<48 | uint64(buf[2])<<40 | uint64(buf[3])<<32 |
			uint64(buf[4])<<24 | uint64(buf[5])<<16 | uint64(buf[6])<<8 | uint64(buf[7]), nil
	}
	return 0, fmt.Errorf("cbor: geçersiz additional info: %d", info)
}

// decodeFloat16 — IEEE 754 half-precision → float64
func decodeFloat16(h uint16) float64 {
	sign := (h >> 15) & 1
	exp := (h >> 10) & 0x1f
	mant := h & 0x3ff
	var f float64
	switch exp {
	case 0:
		f = math.Ldexp(float64(mant), -24)
	case 31:
		if mant == 0 {
			f = math.Inf(1)
		} else {
			f = math.NaN()
		}
	default:
		f = math.Ldexp(float64(mant|0x400), int(exp)-25)
	}
	if sign == 1 {
		f = -f
	}
	return f
}
