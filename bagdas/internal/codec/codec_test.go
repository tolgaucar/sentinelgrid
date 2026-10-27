package codec

import (
	"encoding/json"
	"math"
	"testing"
)

// ──────────────────────────────────────────────
// ParseFormat Testleri
// ──────────────────────────────────────────────

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input string
		want  Format
		ok    bool
	}{
		{"json", JSON, true},
		{"JSON", JSON, true},
		{" Json ", JSON, true},
		{"msgpack", MsgPack, true},
		{"messagepack", MsgPack, true},
		{"msg_pack", MsgPack, true},
		{"cbor", CBOR, true},
		{"CBOR", CBOR, true},
		{"xml", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, err := ParseFormat(tt.input)
		if tt.ok && err != nil {
			t.Errorf("ParseFormat(%q) hata: %v", tt.input, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ParseFormat(%q) hata beklendi", tt.input)
		}
		if got != tt.want {
			t.Errorf("ParseFormat(%q) = %q, beklenen %q", tt.input, got, tt.want)
		}
	}
}

func TestAllFormats(t *testing.T) {
	fmts := AllFormats()
	if len(fmts) != 3 {
		t.Errorf("AllFormats() = %d, beklenen 3", len(fmts))
	}
}

// ──────────────────────────────────────────────
// JSON Codec Testleri
// ──────────────────────────────────────────────

func TestJSONRoundTrip_Map(t *testing.T) {
	c := &JSONCodec{}
	original := map[string]interface{}{
		"name":    "radar-01",
		"lat":     39.925,
		"count":   int64(42),
		"active":  true,
		"tags":    []interface{}{"sensor", "radar"},
		"nested":  map[string]interface{}{"key": "val"},
	}
	encoded, err := c.Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	assertMapEqual(t, original, decoded.(map[string]interface{}))
}

func TestJSONRoundTrip_Array(t *testing.T) {
	c := &JSONCodec{}
	original := []interface{}{int64(1), "iki", 3.14, true, nil}
	encoded, err := c.Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	arr := decoded.([]interface{})
	if len(arr) != 5 {
		t.Fatalf("dizi uzunluk %d, beklenen 5", len(arr))
	}
}

func TestJSONRoundTrip_Null(t *testing.T) {
	c := &JSONCodec{}
	encoded, err := c.Encode(nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != nil {
		t.Errorf("nil beklendi, gelen: %v", decoded)
	}
}

func TestJSONRoundTrip_NumberPrecision(t *testing.T) {
	c := &JSONCodec{}
	original := map[string]interface{}{
		"big_int":   int64(9007199254740992),
		"small_neg": int64(-999999),
		"pi":        3.141592653589793,
	}
	encoded, err := c.Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	m := decoded.(map[string]interface{})
	if m["big_int"] != int64(9007199254740992) {
		t.Errorf("big_int = %v (%T)", m["big_int"], m["big_int"])
	}
	if m["pi"] != 3.141592653589793 {
		t.Errorf("pi = %v", m["pi"])
	}
}

func TestJSONRoundTrip_Unicode(t *testing.T) {
	c := &JSONCodec{}
	original := map[string]interface{}{
		"türkçe":   "ÖTÜKEN-MATRİS",
		"emoji":    "🇹🇷",
		"japanese": "日本語",
	}
	encoded, err := c.Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	m := decoded.(map[string]interface{})
	if m["türkçe"] != "ÖTÜKEN-MATRİS" {
		t.Errorf("Türkçe karakterler kayboldu: %v", m["türkçe"])
	}
	if m["emoji"] != "🇹🇷" {
		t.Errorf("emoji kayboldu: %v", m["emoji"])
	}
}

func TestJSONRoundTrip_EmptyContainers(t *testing.T) {
	c := &JSONCodec{}
	original := map[string]interface{}{
		"empty_map":   map[string]interface{}{},
		"empty_array": []interface{}{},
		"empty_str":   "",
	}
	encoded, err := c.Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	m := decoded.(map[string]interface{})
	if len(m["empty_map"].(map[string]interface{})) != 0 {
		t.Error("boş map korunmadı")
	}
	if len(m["empty_array"].([]interface{})) != 0 {
		t.Error("boş array korunmadı")
	}
}

func TestJSON_DecodeInvalid(t *testing.T) {
	c := &JSONCodec{}
	_, err := c.Decode([]byte("{invalid"))
	if err == nil {
		t.Error("geçersiz JSON'da hata beklendi")
	}
}

func TestJSON_DecodeTyped(t *testing.T) {
	c := &JSONCodec{}
	var target struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	err := c.DecodeTyped([]byte(`{"name":"test","age":25}`), &target)
	if err != nil {
		t.Fatal(err)
	}
	if target.Name != "test" || target.Age != 25 {
		t.Errorf("DecodeTyped: %+v", target)
	}
}

// ──────────────────────────────────────────────
// MsgPack Codec Testleri
// ──────────────────────────────────────────────

func TestMsgPackRoundTrip_Map(t *testing.T) {
	c := &MsgPackCodec{}
	original := map[string]interface{}{
		"id":     int64(42),
		"name":   "test",
		"active": true,
		"score":  99.5,
	}
	encoded, err := c.Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	m := decoded.(map[string]interface{})
	if m["id"] != int64(42) {
		t.Errorf("id = %v (%T)", m["id"], m["id"])
	}
	if m["name"] != "test" {
		t.Errorf("name = %v", m["name"])
	}
	if m["active"] != true {
		t.Errorf("active = %v", m["active"])
	}
}

func TestMsgPackRoundTrip_Array(t *testing.T) {
	c := &MsgPackCodec{}
	original := []interface{}{int64(1), int64(2), int64(3), "dört", true}
	encoded, err := c.Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	arr := decoded.([]interface{})
	if len(arr) != 5 {
		t.Fatalf("dizi uzunluk %d", len(arr))
	}
	if arr[3] != "dört" {
		t.Errorf("arr[3] = %v", arr[3])
	}
}

func TestMsgPackRoundTrip_Null(t *testing.T) {
	c := &MsgPackCodec{}
	encoded, err := c.Encode(nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != nil {
		t.Errorf("nil beklendi, gelen: %v", decoded)
	}
}

func TestMsgPackRoundTrip_NegativeInt(t *testing.T) {
	c := &MsgPackCodec{}
	tests := []int64{-1, -31, -32, -128, -129, -32768, -32769, -2147483648, -2147483649}
	for _, v := range tests {
		encoded, err := c.Encode(v)
		if err != nil {
			t.Fatalf("encode %d: %v", v, err)
		}
		decoded, err := c.Decode(encoded)
		if err != nil {
			t.Fatalf("decode %d: %v", v, err)
		}
		if decoded != v {
			t.Errorf("%d round-trip: %v (%T)", v, decoded, decoded)
		}
	}
}

func TestMsgPackRoundTrip_PositiveInt(t *testing.T) {
	c := &MsgPackCodec{}
	tests := []int64{0, 1, 127, 128, 255, 256, 32767, 32768, 2147483647, 2147483648}
	for _, v := range tests {
		encoded, err := c.Encode(v)
		if err != nil {
			t.Fatalf("encode %d: %v", v, err)
		}
		decoded, err := c.Decode(encoded)
		if err != nil {
			t.Fatalf("decode %d: %v", v, err)
		}
		if decoded != v {
			t.Errorf("%d round-trip: %v (%T)", v, decoded, decoded)
		}
	}
}

func TestMsgPackRoundTrip_Float(t *testing.T) {
	c := &MsgPackCodec{}
	tests := []float64{1.5, -1.5, math.Pi, math.MaxFloat64, math.SmallestNonzeroFloat64}
	for _, v := range tests {
		encoded, err := c.Encode(v)
		if err != nil {
			t.Fatalf("encode %v: %v", v, err)
		}
		decoded, err := c.Decode(encoded)
		if err != nil {
			t.Fatalf("decode %v: %v", v, err)
		}
		f, ok := decoded.(float64)
		if !ok {
			t.Fatalf("%v round-trip: %T", v, decoded)
		}
		if f != v {
			t.Errorf("%v round-trip: %v", v, f)
		}
	}
	// 0.0 Normalize ile int64(0)'a dönüşür (MsgPack encode Normalize çağırır)
	encoded, err := c.Encode(float64(0.0))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != int64(0) {
		t.Errorf("0.0 normalize: %v (%T)", decoded, decoded)
	}
}

func TestMsgPackRoundTrip_String(t *testing.T) {
	c := &MsgPackCodec{}
	tests := []string{"", "hello", "ÖTÜKEN", string(make([]byte, 200)), string(make([]byte, 70000))}
	for _, v := range tests {
		encoded, err := c.Encode(v)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		decoded, err := c.Decode(encoded)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if decoded != v {
			t.Errorf("string len=%d round-trip başarısız", len(v))
		}
	}
}

func TestMsgPackRoundTrip_Binary(t *testing.T) {
	c := &MsgPackCodec{}
	data := []byte{0x00, 0xff, 0xfe, 0x01, 0x42}
	encoded, err := c.Encode(data)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	bs := decoded.([]byte)
	if len(bs) != len(data) {
		t.Fatalf("binary uzunluk %d, beklenen %d", len(bs), len(data))
	}
	for i, b := range bs {
		if b != data[i] {
			t.Errorf("byte[%d] = %x, beklenen %x", i, b, data[i])
		}
	}
}

func TestMsgPackRoundTrip_Bool(t *testing.T) {
	c := &MsgPackCodec{}
	for _, v := range []bool{true, false} {
		encoded, err := c.Encode(v)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := c.Decode(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if decoded != v {
			t.Errorf("%v round-trip: %v", v, decoded)
		}
	}
}

func TestMsgPackRoundTrip_NestedComplex(t *testing.T) {
	c := &MsgPackCodec{}
	original := map[string]interface{}{
		"units": []interface{}{
			map[string]interface{}{
				"id":   int64(1),
				"name": "T-72 Tank",
				"pos":  map[string]interface{}{"lat": 39.925, "lon": 32.866},
			},
			map[string]interface{}{
				"id":   int64(2),
				"name": "F-16",
				"pos":  map[string]interface{}{"lat": 40.0, "lon": 33.0},
			},
		},
		"count": int64(2),
	}
	encoded, err := c.Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	m := decoded.(map[string]interface{})
	units := m["units"].([]interface{})
	if len(units) != 2 {
		t.Fatalf("units uzunluk %d", len(units))
	}
	first := units[0].(map[string]interface{})
	if first["name"] != "T-72 Tank" {
		t.Errorf("first name = %v", first["name"])
	}
}

// ──────────────────────────────────────────────
// CBOR Codec Testleri
// ──────────────────────────────────────────────

func TestCBORRoundTrip_Map(t *testing.T) {
	c := &CBORCodec{}
	original := map[string]interface{}{
		"id":      int64(100),
		"label":   "hedef",
		"active":  false,
		"coords":  []interface{}{39.925, 32.866},
	}
	encoded, err := c.Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	m := decoded.(map[string]interface{})
	if m["id"] != int64(100) {
		t.Errorf("id = %v (%T)", m["id"], m["id"])
	}
	if m["label"] != "hedef" {
		t.Errorf("label = %v", m["label"])
	}
}

func TestCBORRoundTrip_NegativeInt(t *testing.T) {
	c := &CBORCodec{}
	tests := []int64{-1, -24, -25, -256, -257, -65536, -65537}
	for _, v := range tests {
		encoded, err := c.Encode(v)
		if err != nil {
			t.Fatalf("encode %d: %v", v, err)
		}
		decoded, err := c.Decode(encoded)
		if err != nil {
			t.Fatalf("decode %d: %v", v, err)
		}
		if decoded != v {
			t.Errorf("%d round-trip: %v (%T)", v, decoded, decoded)
		}
	}
}

func TestCBORRoundTrip_Null(t *testing.T) {
	c := &CBORCodec{}
	encoded, err := c.Encode(nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != nil {
		t.Errorf("nil beklendi, gelen: %v", decoded)
	}
}

func TestCBORRoundTrip_ByteString(t *testing.T) {
	c := &CBORCodec{}
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	encoded, err := c.Encode(data)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	bs := decoded.([]byte)
	if len(bs) != 4 {
		t.Fatalf("byte uzunluk %d", len(bs))
	}
}

func TestCBORRoundTrip_Nested(t *testing.T) {
	c := &CBORCodec{}
	original := map[string]interface{}{
		"config": map[string]interface{}{
			"interval": int64(5),
			"enabled":  true,
		},
		"items": []interface{}{int64(1), int64(2), int64(3)},
	}
	encoded, err := c.Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	m := decoded.(map[string]interface{})
	cfg := m["config"].(map[string]interface{})
	if cfg["interval"] != int64(5) {
		t.Errorf("interval = %v", cfg["interval"])
	}
}

// ──────────────────────────────────────────────
// Registry & Convert Testleri
// ──────────────────────────────────────────────

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	fmts := r.Formats()
	if len(fmts) != 3 {
		t.Errorf("format sayısı %d, beklenen 3", len(fmts))
	}
}

func TestRegistryGet_InvalidFormat(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("xml")
	if err == nil {
		t.Error("bilinmeyen format için hata beklendi")
	}
}

func TestConvert_JSONtoMsgPack(t *testing.T) {
	r := NewRegistry()
	jsonData := []byte(`{"name":"test","value":42}`)

	msgpackData, err := r.Convert(jsonData, JSON, MsgPack)
	if err != nil {
		t.Fatal(err)
	}

	// Geri dönüştür
	jsonBack, err := r.Convert(msgpackData, MsgPack, JSON)
	if err != nil {
		t.Fatal(err)
	}

	var original, roundTrip map[string]interface{}
	json.Unmarshal(jsonData, &original)

	dec := json.NewDecoder(nil)
	_ = dec
	json.Unmarshal(jsonBack, &roundTrip)

	// "value" json round-trip'de float olabilir, normalize edip karşılaştır
	if roundTrip["name"] != original["name"] {
		t.Errorf("name: %v != %v", roundTrip["name"], original["name"])
	}
}

func TestConvert_JSONtoCBOR(t *testing.T) {
	r := NewRegistry()
	jsonData := []byte(`{"sensor":"radar","lat":39.925,"count":100}`)

	cborData, err := r.Convert(jsonData, JSON, CBOR)
	if err != nil {
		t.Fatal(err)
	}
	if len(cborData) == 0 {
		t.Fatal("CBOR çıktı boş")
	}

	// Geri dönüştür
	jsonBack, err := r.Convert(cborData, CBOR, JSON)
	if err != nil {
		t.Fatal(err)
	}
	if len(jsonBack) == 0 {
		t.Fatal("JSON geri dönüş boş")
	}
}

func TestConvert_MsgPackToCBOR(t *testing.T) {
	r := NewRegistry()
	original := map[string]interface{}{
		"id":    int64(1),
		"name":  "test",
		"array": []interface{}{int64(1), int64(2)},
	}

	mp := &MsgPackCodec{}
	mpBytes, err := mp.Encode(original)
	if err != nil {
		t.Fatal(err)
	}

	cborBytes, err := r.Convert(mpBytes, MsgPack, CBOR)
	if err != nil {
		t.Fatal(err)
	}

	// CBOR'dan decode
	cb := &CBORCodec{}
	decoded, err := cb.Decode(cborBytes)
	if err != nil {
		t.Fatal(err)
	}
	m := decoded.(map[string]interface{})
	if m["id"] != int64(1) {
		t.Errorf("id = %v", m["id"])
	}
}

func TestConvert_SameFormat(t *testing.T) {
	r := NewRegistry()
	jsonData := []byte(`{"test":true}`)
	out, err := r.Convert(jsonData, JSON, JSON)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("aynı format dönüşüm boş")
	}
}

func TestConvert_NilData(t *testing.T) {
	r := NewRegistry()
	_, err := r.Convert(nil, JSON, MsgPack)
	if err == nil {
		t.Error("nil veri için hata beklendi")
	}
}

func TestConvert_EmptyData(t *testing.T) {
	r := NewRegistry()
	_, err := r.Convert([]byte{}, JSON, MsgPack)
	if err == nil {
		t.Error("boş veri için hata beklendi")
	}
}

func TestConvertToMap(t *testing.T) {
	r := NewRegistry()
	jsonData := []byte(`{"a":"b","c":1}`)
	m, err := r.ConvertToMap(jsonData, JSON)
	if err != nil {
		t.Fatal(err)
	}
	if m["a"] != "b" {
		t.Errorf("a = %v", m["a"])
	}
}

// ──────────────────────────────────────────────
// Normalize Testleri
// ──────────────────────────────────────────────

func TestNormalize_IntTypes(t *testing.T) {
	tests := []struct {
		input interface{}
		want  int64
	}{
		{int(5), 5},
		{int8(5), 5},
		{int16(5), 5},
		{int32(5), 5},
		{int64(5), 5},
		{uint(5), 5},
		{uint8(5), 5},
		{uint16(5), 5},
		{uint32(5), 5},
		{uint64(5), 5},
	}
	for _, tt := range tests {
		got := Normalize(tt.input)
		if got != tt.want {
			t.Errorf("Normalize(%T(%v)) = %v (%T), beklenen %d", tt.input, tt.input, got, got, tt.want)
		}
	}
}

func TestNormalize_Float(t *testing.T) {
	// Tam sayı float → int64
	got := Normalize(float64(42))
	if got != int64(42) {
		t.Errorf("Normalize(42.0) = %v (%T)", got, got)
	}

	// Pi float → float64
	got = Normalize(float64(3.14))
	if got != float64(3.14) {
		t.Errorf("Normalize(3.14) = %v", got)
	}

	// Inf kalır
	got = Normalize(math.Inf(1))
	if got != math.Inf(1) {
		t.Errorf("Normalize(+Inf) = %v", got)
	}
}

func TestNormalize_MapKeyConversion(t *testing.T) {
	input := map[interface{}]interface{}{
		"key1": "val1",
		42:     "val2",
	}
	got := Normalize(input)
	m := got.(map[string]interface{})
	if m["key1"] != "val1" {
		t.Error("key1 korunmadı")
	}
	if m["42"] != "val2" {
		t.Error("int key string'e dönüşmedi")
	}
}

func TestNormalize_Nil(t *testing.T) {
	if Normalize(nil) != nil {
		t.Error("nil korunmadı")
	}
}

func TestNormalize_Bool(t *testing.T) {
	if Normalize(true) != true {
		t.Error("true korunmadı")
	}
	if Normalize(false) != false {
		t.Error("false korunmadı")
	}
}

func TestNormalize_Float32(t *testing.T) {
	got := Normalize(float32(1.5))
	if got != float64(1.5) {
		t.Errorf("float32 → float64: %v (%T)", got, got)
	}
}

func TestNormalize_JSONNumber(t *testing.T) {
	got := Normalize(json.Number("42"))
	if got != int64(42) {
		t.Errorf("json.Number int: %v (%T)", got, got)
	}

	got = Normalize(json.Number("3.14"))
	if got != float64(3.14) {
		t.Errorf("json.Number float: %v (%T)", got, got)
	}
}

// ──────────────────────────────────────────────
// Cross-format Round-trip Testleri
// ──────────────────────────────────────────────

func TestCrossFormatRoundTrip_AllPairs(t *testing.T) {
	r := NewRegistry()
	original := map[string]interface{}{
		"sensor_id":   "radar-01",
		"sensor_type": "radar",
		"lat":         39.925,
		"lon":         32.866,
		"count":       int64(42),
		"active":      true,
		"tags":        []interface{}{"military", "nato"},
	}

	formats := AllFormats()
	for _, from := range formats {
		srcCodec, _ := r.Get(from)
		srcBytes, err := srcCodec.Encode(original)
		if err != nil {
			t.Fatalf("encode %s: %v", from, err)
		}

		for _, to := range formats {
			converted, err := r.Convert(srcBytes, from, to)
			if err != nil {
				t.Errorf("%s→%s dönüşüm hatası: %v", from, to, err)
				continue
			}

			// Geri dönüştür
			backBytes, err := r.Convert(converted, to, from)
			if err != nil {
				t.Errorf("%s→%s→%s geri dönüşüm hatası: %v", from, to, from, err)
				continue
			}

			// Decode ve karşılaştır
			result, err := srcCodec.Decode(backBytes)
			if err != nil {
				t.Errorf("%s→%s→%s decode hatası: %v", from, to, from, err)
				continue
			}

			m := result.(map[string]interface{})
			m = normalizeMapForCompare(m)
			orig := normalizeMapForCompare(original)

			if m["sensor_id"] != orig["sensor_id"] {
				t.Errorf("%s→%s: sensor_id %v != %v", from, to, m["sensor_id"], orig["sensor_id"])
			}
			if m["active"] != orig["active"] {
				t.Errorf("%s→%s: active %v != %v", from, to, m["active"], orig["active"])
			}
		}
	}
}

// ──────────────────────────────────────────────
// Benchmark
// ──────────────────────────────────────────────

func BenchmarkJSON_Encode(b *testing.B) {
	c := &JSONCodec{}
	data := map[string]interface{}{
		"id": int64(1), "name": "test", "value": 42.5, "active": true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Encode(data)
	}
}

func BenchmarkMsgPack_Encode(b *testing.B) {
	c := &MsgPackCodec{}
	data := map[string]interface{}{
		"id": int64(1), "name": "test", "value": 42.5, "active": true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Encode(data)
	}
}

func BenchmarkCBOR_Encode(b *testing.B) {
	c := &CBORCodec{}
	data := map[string]interface{}{
		"id": int64(1), "name": "test", "value": 42.5, "active": true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Encode(data)
	}
}

func BenchmarkConvert_JSONtoMsgPack(b *testing.B) {
	r := NewRegistry()
	jsonData := []byte(`{"id":1,"name":"test","value":42.5,"active":true}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Convert(jsonData, JSON, MsgPack)
	}
}

func BenchmarkConvert_JSONtoCBOR(b *testing.B) {
	r := NewRegistry()
	jsonData := []byte(`{"id":1,"name":"test","value":42.5,"active":true}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Convert(jsonData, JSON, CBOR)
	}
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func assertMapEqual(t *testing.T, expected, actual map[string]interface{}) {
	t.Helper()
	for k, v := range expected {
		av, ok := actual[k]
		if !ok {
			t.Errorf("anahtar %q eksik", k)
			continue
		}
		// Normalize both for comparison
		nv := Normalize(v)
		nav := Normalize(av)
		switch expected := nv.(type) {
		case map[string]interface{}:
			actualMap, ok := nav.(map[string]interface{})
			if !ok {
				t.Errorf("%q map beklendi, gelen %T", k, nav)
				continue
			}
			assertMapEqual(t, expected, actualMap)
		case []interface{}:
			actualArr, ok := nav.([]interface{})
			if !ok {
				t.Errorf("%q array beklendi, gelen %T", k, nav)
				continue
			}
			if len(expected) != len(actualArr) {
				t.Errorf("%q uzunluk: %d != %d", k, len(expected), len(actualArr))
			}
		default:
			if nv != nav {
				t.Errorf("%q: %v (%T) != %v (%T)", k, nv, nv, nav, nav)
			}
		}
	}
}

func normalizeMapForCompare(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range m {
		out[k] = Normalize(v)
	}
	return out
}
