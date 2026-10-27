// Package compress — Kayıpsız veri sıkıştırma.
//
// Dar bant genişliğine sahip cephe koşullarında veri boyutunu
// küçültmek için DEFLATE (flate) tabanlı kayıpsız sıkıştırma
// uygular. Sıkıştırma seviyesi ayarlanabilir (1=hız, 9=oran).
package compress

import (
	"bytes"
	"compress/flate"
	"fmt"
	"io"
	"sync/atomic"
)

// Compressor veri sıkıştırma motoru.
type Compressor struct {
	level int
	stats CompressorStats
}

// CompressorStats sıkıştırma istatistikleri.
type CompressorStats struct {
	TotalCompressed   int64 `json:"total_compressed"`
	TotalDecompressed int64 `json:"total_decompressed"`
	BytesIn           int64 `json:"bytes_in"`
	BytesOut          int64 `json:"bytes_out"`
	TotalErrors       int64 `json:"total_errors"`
	BestRatio         int64 `json:"best_ratio_pct"`  // En iyi oran (0-100)
	WorstRatio        int64 `json:"worst_ratio_pct"` // En kötü oran (0-100)
}

// NewCompressor yeni sıkıştırıcı oluşturur. level: 1-9.
func NewCompressor(level int) *Compressor {
	if level < 1 {
		level = 1
	}
	if level > 9 {
		level = 9
	}
	return &Compressor{
		level: level,
		stats: CompressorStats{BestRatio: 100, WorstRatio: 0},
	}
}

// Compress veriyi sıkıştırır.
func (c *Compressor) Compress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, c.level)
	if err != nil {
		atomic.AddInt64(&c.stats.TotalErrors, 1)
		return nil, fmt.Errorf("flate writer hatası: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		atomic.AddInt64(&c.stats.TotalErrors, 1)
		return nil, fmt.Errorf("sıkıştırma yazma hatası: %w", err)
	}

	if err := w.Close(); err != nil {
		atomic.AddInt64(&c.stats.TotalErrors, 1)
		return nil, fmt.Errorf("sıkıştırma kapatma hatası: %w", err)
	}

	result := buf.Bytes()
	atomic.AddInt64(&c.stats.TotalCompressed, 1)
	atomic.AddInt64(&c.stats.BytesIn, int64(len(data)))
	atomic.AddInt64(&c.stats.BytesOut, int64(len(result)))

	// Oran takibi
	if len(data) > 0 {
		ratio := int64(len(result) * 100 / len(data))
		if ratio < atomic.LoadInt64(&c.stats.BestRatio) {
			atomic.StoreInt64(&c.stats.BestRatio, ratio)
		}
		if ratio > atomic.LoadInt64(&c.stats.WorstRatio) {
			atomic.StoreInt64(&c.stats.WorstRatio, ratio)
		}
	}

	return result, nil
}

// Decompress sıkıştırılmış veriyi açar.
func (c *Compressor) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	r := flate.NewReader(bytes.NewReader(data))
	defer r.Close()

	result, err := io.ReadAll(r)
	if err != nil {
		atomic.AddInt64(&c.stats.TotalErrors, 1)
		return nil, fmt.Errorf("açma hatası: %w", err)
	}

	atomic.AddInt64(&c.stats.TotalDecompressed, 1)
	return result, nil
}

// Level sıkıştırma seviyesini döndürür.
func (c *Compressor) Level() int {
	return c.level
}

// Stats istatistikleri döndürür.
func (c *Compressor) Stats() CompressorStats {
	return CompressorStats{
		TotalCompressed:   atomic.LoadInt64(&c.stats.TotalCompressed),
		TotalDecompressed: atomic.LoadInt64(&c.stats.TotalDecompressed),
		BytesIn:           atomic.LoadInt64(&c.stats.BytesIn),
		BytesOut:          atomic.LoadInt64(&c.stats.BytesOut),
		TotalErrors:       atomic.LoadInt64(&c.stats.TotalErrors),
		BestRatio:         atomic.LoadInt64(&c.stats.BestRatio),
		WorstRatio:        atomic.LoadInt64(&c.stats.WorstRatio),
	}
}

// CompressionRatio son sıkıştırma oranını yüzde olarak döndürür (küçük = daha iyi).
func (c *Compressor) CompressionRatio() float64 {
	in := atomic.LoadInt64(&c.stats.BytesIn)
	out := atomic.LoadInt64(&c.stats.BytesOut)
	if in == 0 {
		return 0
	}
	return float64(out) / float64(in) * 100.0
}
