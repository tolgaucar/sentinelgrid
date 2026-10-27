package compress

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewCompressor(t *testing.T) {
	c := NewCompressor(6)
	if c.Level() != 6 {
		t.Errorf("Level beklenen: 6, alınan: %d", c.Level())
	}
}

func TestCompressorLevelClamp(t *testing.T) {
	c1 := NewCompressor(0)
	if c1.Level() != 1 {
		t.Errorf("Alt limit beklenen: 1, alınan: %d", c1.Level())
	}
	c2 := NewCompressor(99)
	if c2.Level() != 9 {
		t.Errorf("Üst limit beklenen: 9, alınan: %d", c2.Level())
	}
}

func TestCompressDecompress(t *testing.T) {
	c := NewCompressor(6)
	original := []byte("KOMUTA: Tüm birimler mevziye geçsin. Koordinat 39.9334N 32.8597E. Tekrar ediyorum, mevziye geçsin.")

	compressed, err := c.Compress(original)
	if err != nil {
		t.Fatalf("Compress hatası: %v", err)
	}

	if len(compressed) >= len(original) {
		t.Logf("Uyarı: Sıkıştırma kazanç sağlamadı (%d → %d)", len(original), len(compressed))
	}

	decompressed, err := c.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress hatası: %v", err)
	}

	if !bytes.Equal(decompressed, original) {
		t.Errorf("Açılan veri eşleşmiyor: %q != %q", decompressed, original)
	}
}

func TestCompressEmpty(t *testing.T) {
	c := NewCompressor(6)
	compressed, err := c.Compress([]byte{})
	if err != nil {
		t.Fatalf("Boş veri hatası: %v", err)
	}
	if len(compressed) != 0 {
		t.Errorf("Boş veri sonucu beklenen: 0, alınan: %d", len(compressed))
	}
}

func TestDecompressEmpty(t *testing.T) {
	c := NewCompressor(6)
	result, err := c.Decompress([]byte{})
	if err != nil {
		t.Fatalf("Boş decompress hatası: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Boş decompress sonucu beklenen: 0, alınan: %d", len(result))
	}
}

func TestCompressLargeData(t *testing.T) {
	c := NewCompressor(9)
	// Tekrarlı veri — iyi sıkışır
	original := []byte(strings.Repeat("ALFA TİMİ koordinat raporu. ", 1000))

	compressed, err := c.Compress(original)
	if err != nil {
		t.Fatalf("Büyük veri compress hatası: %v", err)
	}

	ratio := float64(len(compressed)) / float64(len(original)) * 100
	if ratio > 20 {
		t.Errorf("Tekrarlı veri sıkıştırma oranı çok yüksek: %.1f%%", ratio)
	}

	decompressed, err := c.Decompress(compressed)
	if err != nil {
		t.Fatalf("Büyük veri decompress hatası: %v", err)
	}
	if !bytes.Equal(decompressed, original) {
		t.Error("Büyük veri açma eşleşmiyor")
	}
}

func TestDecompressInvalid(t *testing.T) {
	c := NewCompressor(6)
	_, err := c.Decompress([]byte{0xFF, 0xFE, 0xFD})
	if err == nil {
		t.Fatal("Geçersiz veri için hata bekleniyor")
	}
}

func TestStats(t *testing.T) {
	c := NewCompressor(6)
	data := []byte("test verisi sıkıştırma")
	c.Compress(data)
	c.Compress(data)

	s := c.Stats()
	if s.TotalCompressed != 2 {
		t.Errorf("TotalCompressed beklenen: 2, alınan: %d", s.TotalCompressed)
	}
	if s.BytesIn != int64(len(data)*2) {
		t.Errorf("BytesIn beklenen: %d, alınan: %d", len(data)*2, s.BytesIn)
	}
}

func TestCompressionRatio(t *testing.T) {
	c := NewCompressor(6)
	c.Compress([]byte(strings.Repeat("A", 1000)))
	ratio := c.CompressionRatio()
	if ratio <= 0 || ratio >= 100 {
		t.Errorf("Oran mantıksız: %.1f%%", ratio)
	}
}
