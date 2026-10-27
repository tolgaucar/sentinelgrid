package fec

import (
	"bytes"
	"testing"
)

func TestNewEncoder(t *testing.T) {
	e, err := NewEncoder(4, 1024)
	if err != nil {
		t.Fatalf("NewEncoder hatası: %v", err)
	}
	if e.DataShards() != 4 {
		t.Errorf("DataShards beklenen: 4, alınan: %d", e.DataShards())
	}
	if e.BlockSize() != 1024 {
		t.Errorf("BlockSize beklenen: 1024, alınan: %d", e.BlockSize())
	}
}

func TestNewEncoderInvalid(t *testing.T) {
	_, err := NewEncoder(1, 1024)
	if err == nil {
		t.Fatal("dataShards=1 için hata bekleniyor")
	}
	_, err = NewEncoder(4, 0)
	if err == nil {
		t.Fatal("blockSize=0 için hata bekleniyor")
	}
}

func TestEncodeNoLoss(t *testing.T) {
	e, _ := NewEncoder(4, 8)
	data := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ012345") // 32 byte = 4 blok

	blocks := e.Encode(data)
	if len(blocks) == 0 {
		t.Fatal("Encode sonucu boş")
	}

	// Parite blokları var mı?
	hasParity := false
	for _, b := range blocks {
		if b.IsParity {
			hasParity = true
			break
		}
	}
	if !hasParity {
		t.Fatal("Parite bloğu yok")
	}

	// Kayıpsız decode
	result, err := e.Decode(blocks, len(data))
	if err != nil {
		t.Fatalf("Decode hatası: %v", err)
	}
	if !bytes.Equal(result, data) {
		t.Errorf("Decode sonucu eşleşmiyor: %q != %q", result, data)
	}
}

func TestEncodeSmallData(t *testing.T) {
	e, _ := NewEncoder(4, 8)
	data := []byte("ABC") // 3 byte < 1 blok

	blocks := e.Encode(data)
	result, err := e.Decode(blocks, len(data))
	if err != nil {
		t.Fatalf("Küçük veri decode hatası: %v", err)
	}
	if !bytes.Equal(result, data) {
		t.Errorf("Küçük veri eşleşmiyor: %q != %q", result, data)
	}
}

func TestEncodeEmpty(t *testing.T) {
	e, _ := NewEncoder(4, 8)
	blocks := e.Encode([]byte{})
	if blocks != nil {
		t.Errorf("Boş veri sonucu nil bekleniyor, alınan: %d blok", len(blocks))
	}
}

func TestRecoverOneBlock(t *testing.T) {
	e, _ := NewEncoder(4, 8)
	data := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ012345") // 32 byte = 4 blok + 1 parite

	blocks := e.Encode(data)

	// İlk gruptaki 2. veri bloğunu sil (index=1)
	var damaged []Block
	for _, b := range blocks {
		if b.GroupID == 0 && b.Index == 1 && !b.IsParity {
			continue // Kayıp simüle et
		}
		damaged = append(damaged, b)
	}

	result, err := e.Decode(damaged, len(data))
	if err != nil {
		t.Fatalf("Kurtarma hatası: %v", err)
	}
	if !bytes.Equal(result, data) {
		t.Errorf("Kurtarılan veri eşleşmiyor")
	}

	s := e.Stats()
	if s.TotalRecovered < 1 {
		t.Errorf("TotalRecovered beklenen: >=1, alınan: %d", s.TotalRecovered)
	}
}

func TestTwoBlockLoss(t *testing.T) {
	e, _ := NewEncoder(4, 8)
	data := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ012345")

	blocks := e.Encode(data)

	// Aynı gruptan 2 blok sil → kurtarılamaz
	var damaged []Block
	removed := 0
	for _, b := range blocks {
		if b.GroupID == 0 && !b.IsParity && removed < 2 {
			removed++
			continue
		}
		damaged = append(damaged, b)
	}

	_, err := e.Decode(damaged, len(data))
	if err == nil {
		t.Fatal("2 blok kaybı için hata bekleniyor")
	}
}

func TestStats(t *testing.T) {
	e, _ := NewEncoder(4, 8)
	data := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ012345")

	e.Encode(data)
	s := e.Stats()
	if s.TotalEncoded != 1 {
		t.Errorf("TotalEncoded beklenen: 1, alınan: %d", s.TotalEncoded)
	}
	if s.BlocksCreated < 4 {
		t.Errorf("BlocksCreated beklenen: >=4, alınan: %d", s.BlocksCreated)
	}
	if s.ParityCreated < 1 {
		t.Errorf("ParityCreated beklenen: >=1, alınan: %d", s.ParityCreated)
	}
}

func TestLargeData(t *testing.T) {
	e, _ := NewEncoder(4, 256)
	data := make([]byte, 10000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	blocks := e.Encode(data)
	result, err := e.Decode(blocks, len(data))
	if err != nil {
		t.Fatalf("Büyük veri decode hatası: %v", err)
	}
	if !bytes.Equal(result, data) {
		t.Error("Büyük veri eşleşmiyor")
	}
}

func TestXorBytes(t *testing.T) {
	a := []byte{0xFF, 0x00, 0xAA}
	b := []byte{0x0F, 0xF0, 0x55}
	xorBytes(a, b)
	expected := []byte{0xF0, 0xF0, 0xFF}
	if !bytes.Equal(a, expected) {
		t.Errorf("XOR sonucu: %x, beklenen: %x", a, expected)
	}
}
