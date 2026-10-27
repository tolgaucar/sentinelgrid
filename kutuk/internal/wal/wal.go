// Package wal Write-Ahead Log - çökme kurtarma için disk kalıcılığı.
// Her yazma önce WAL'a kaydedilir, sonra belleğe uygulanır.
// Segment bazlı döndürme ile disk kullanımı kontrol altında.
package wal

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RecordType kayıt tipi.
type RecordType byte

const (
	RecordPut    RecordType = 1
	RecordDelete RecordType = 2
)

// Record WAL kaydı.
type Record struct {
	Type      RecordType
	Namespace string
	Key       string
	Value     []byte
	TTLNano   int64
	Timestamp int64
	Checksum  uint32
}

// WAL Write-Ahead Log.
type WAL struct {
	mu          sync.Mutex
	dir         string
	segmentSize int64
	syncWrite   bool

	currentFile   *os.File
	currentWriter *bufio.Writer
	currentSize   int64
	segmentIndex  int

	totalWrites atomic.Int64
	totalBytes  atomic.Int64
}

// New yeni WAL oluştur.
func New(dir string, segmentSize int64, syncWrite bool) (*WAL, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("wal dizini oluşturulamadı: %w", err)
	}

	w := &WAL{
		dir:         dir,
		segmentSize: segmentSize,
		syncWrite:   syncWrite,
	}

	// Mevcut segment sayısını bul
	segments, _ := w.listSegments()
	w.segmentIndex = len(segments)

	if err := w.rotateSegment(); err != nil {
		return nil, err
	}
	return w, nil
}

// Append kayıt ekle.
func (w *WAL) Append(rec Record) error {
	rec.Timestamp = time.Now().UnixNano()
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	rec.Checksum = crc32.ChecksumIEEE(data)

	// Uzunluk + veri + checksum formatında yaz
	w.mu.Lock()
	defer w.mu.Unlock()

	frame := encodeFrame(data, rec.Checksum)

	n, err := w.currentWriter.Write(frame)
	if err != nil {
		return fmt.Errorf("wal yazma hatası: %w", err)
	}
	w.currentSize += int64(n)
	w.totalWrites.Add(1)
	w.totalBytes.Add(int64(n))

	if w.syncWrite {
		if err := w.currentWriter.Flush(); err != nil {
			return err
		}
		if err := w.currentFile.Sync(); err != nil {
			return err
		}
	}

	// Segment dolu ise döndür
	if w.currentSize >= w.segmentSize {
		return w.rotateSegment()
	}
	return nil
}

// AppendPut put kaydı ekle.
func (w *WAL) AppendPut(ns, key string, value []byte, ttlNano int64) error {
	return w.Append(Record{
		Type:      RecordPut,
		Namespace: ns,
		Key:       key,
		Value:     value,
		TTLNano:   ttlNano,
	})
}

// AppendDelete delete kaydı ekle.
func (w *WAL) AppendDelete(ns, key string) error {
	return w.Append(Record{
		Type:      RecordDelete,
		Namespace: ns,
		Key:       key,
	})
}

// Replay tüm segment'leri oku ve uygula.
func (w *WAL) Replay(applyFn func(Record) error) (int, error) {
	segments, err := w.listSegments()
	if err != nil {
		return 0, err
	}

	total := 0
	for _, seg := range segments {
		count, err := w.replaySegment(seg, applyFn)
		if err != nil {
			return total, fmt.Errorf("segment %s replay hatası: %w", seg, err)
		}
		total += count
	}
	return total, nil
}

func (w *WAL) replaySegment(path string, applyFn func(Record) error) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	count := 0

	for {
		data, checksum, err := decodeFrame(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			// Kırık kayıt - durdurmadan devam
			break
		}

		// Checksum doğrula
		if crc32.ChecksumIEEE(data) != checksum {
			continue // Bozuk kayıt, atla
		}

		var rec Record
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}

		if err := applyFn(rec); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// rotateSegment yeni segment dosyası aç.
func (w *WAL) rotateSegment() error {
	if w.currentFile != nil {
		w.currentWriter.Flush()
		w.currentFile.Close()
	}

	w.segmentIndex++
	name := fmt.Sprintf("segment-%06d.wal", w.segmentIndex)
	path := filepath.Join(w.dir, name)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("segment açılamadı: %w", err)
	}

	w.currentFile = f
	w.currentWriter = bufio.NewWriterSize(f, 64*1024) // 64KB buffer
	w.currentSize = 0
	return nil
}

func (w *WAL) listSegments() ([]string, error) {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var segments []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".wal") {
			segments = append(segments, filepath.Join(w.dir, e.Name()))
		}
	}
	sort.Strings(segments)
	return segments, nil
}

// Truncate eski segment'leri sil (snapshot sonrası).
func (w *WAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	segments, err := w.listSegments()
	if err != nil {
		return err
	}

	currentPath := ""
	if w.currentFile != nil {
		currentPath = w.currentFile.Name()
	}

	for _, seg := range segments {
		if seg != currentPath {
			os.Remove(seg)
		}
	}
	return nil
}

// Close WAL'ı kapat.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.currentWriter != nil {
		w.currentWriter.Flush()
	}
	if w.currentFile != nil {
		return w.currentFile.Close()
	}
	return nil
}

// Stats istatistikler.
func (w *WAL) Stats() (int64, int64) {
	return w.totalWrites.Load(), w.totalBytes.Load()
}

// ── Frame Encoding ──
// Format: [4 byte uzunluk][data][4 byte checksum]

func encodeFrame(data []byte, checksum uint32) []byte {
	frame := make([]byte, 4+len(data)+4)
	binary.LittleEndian.PutUint32(frame[0:4], uint32(len(data)))
	copy(frame[4:4+len(data)], data)
	binary.LittleEndian.PutUint32(frame[4+len(data):], checksum)
	return frame
}

func decodeFrame(r *bufio.Reader) ([]byte, uint32, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, 0, err
	}
	dataLen := binary.LittleEndian.Uint32(lenBuf)
	if dataLen > 16*1024*1024 { // 16MB limit
		return nil, 0, fmt.Errorf("frame çok büyük: %d", dataLen)
	}

	data := make([]byte, dataLen)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, 0, err
	}

	csumBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, csumBuf); err != nil {
		return nil, 0, err
	}
	checksum := binary.LittleEndian.Uint32(csumBuf)

	return data, checksum, nil
}
