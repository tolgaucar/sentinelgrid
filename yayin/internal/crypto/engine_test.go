package crypto

import (
	"bytes"
	"testing"
)

func TestNewEngine(t *testing.T) {
	e, err := NewEngine(3)
	if err != nil {
		t.Fatalf("NewEngine hatası: %v", err)
	}
	if e.ActiveKeyID() != 1 {
		t.Errorf("Aktif anahtar ID beklenen: 1, alınan: %d", e.ActiveKeyID())
	}
	if e.KeyCount() != 1 {
		t.Errorf("Anahtar sayısı beklenen: 1, alınan: %d", e.KeyCount())
	}
}

func TestEncryptDecrypt(t *testing.T) {
	e, _ := NewEngine(3)
	plain := []byte("ALFA TİMİ: Koordinat 39.9334,32.8597 hedefe ilerle")

	cipher, err := e.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt hatası: %v", err)
	}
	if bytes.Equal(cipher, plain) {
		t.Fatal("Şifreli veri orijinal ile aynı!")
	}

	decrypted, err := e.Decrypt(cipher)
	if err != nil {
		t.Fatalf("Decrypt hatası: %v", err)
	}
	if !bytes.Equal(decrypted, plain) {
		t.Errorf("Çözümlenen veri eşleşmiyor: %q != %q", decrypted, plain)
	}
}

func TestEncryptEmpty(t *testing.T) {
	e, _ := NewEngine(3)
	cipher, err := e.Encrypt([]byte{})
	if err != nil {
		t.Fatalf("Boş veri şifreleme hatası: %v", err)
	}
	decrypted, err := e.Decrypt(cipher)
	if err != nil {
		t.Fatalf("Boş veri çözümleme hatası: %v", err)
	}
	if len(decrypted) != 0 {
		t.Errorf("Boş veri sonucu beklenen: 0, alınan: %d byte", len(decrypted))
	}
}

func TestDecryptTampered(t *testing.T) {
	e, _ := NewEngine(3)
	cipher, _ := e.Encrypt([]byte("gizli emir"))

	// Veriyi boz
	cipher[len(cipher)-1] ^= 0xFF

	_, err := e.Decrypt(cipher)
	if err == nil {
		t.Fatal("Bozulmuş veri için hata bekleniyor")
	}
}

func TestDecryptShortData(t *testing.T) {
	e, _ := NewEngine(3)
	_, err := e.Decrypt([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("Kısa veri için hata bekleniyor")
	}
}

func TestRotateKey(t *testing.T) {
	e, _ := NewEngine(3)
	if err := e.RotateKey(); err != nil {
		t.Fatalf("RotateKey hatası: %v", err)
	}
	if e.ActiveKeyID() != 2 {
		t.Errorf("Rotasyon sonrası aktif ID beklenen: 2, alınan: %d", e.ActiveKeyID())
	}
	if e.KeyCount() != 2 {
		t.Errorf("Anahtar sayısı beklenen: 2, alınan: %d", e.KeyCount())
	}
}

func TestDecryptWithOldKey(t *testing.T) {
	e, _ := NewEngine(3)
	plain := []byte("eski anahtar ile şifrelenmiş veri")

	cipher, _ := e.Encrypt(plain)
	e.RotateKey()

	// Eski anahtar ile şifrelenmiş veri hâlâ çözülebilmeli
	decrypted, err := e.Decrypt(cipher)
	if err != nil {
		t.Fatalf("Eski anahtar ile çözümleme hatası: %v", err)
	}
	if !bytes.Equal(decrypted, plain) {
		t.Errorf("Çözümlenen veri eşleşmiyor")
	}
}

func TestMaxOldKeys(t *testing.T) {
	e, _ := NewEngine(2) // Maks 2 eski anahtar

	for i := 0; i < 5; i++ {
		e.RotateKey()
	}

	// Aktif (6) + eski (2) = 3
	if e.KeyCount() != 3 {
		t.Errorf("Max eski anahtar limiti: beklenen 3, alınan: %d", e.KeyCount())
	}
}

func TestDecryptUnknownKey(t *testing.T) {
	e1, _ := NewEngine(3)
	e2, _ := NewEngine(3)

	cipher, _ := e1.Encrypt([]byte("farklı motor"))
	_, err := e2.Decrypt(cipher)
	// keyID=1 her ikisinde de var ama farklı anahtarlar, bütünlük kontrolü başarısız olmalı
	if err == nil {
		t.Fatal("Farklı anahtar ile çözümleme başarılı olmamalı")
	}
}

func TestStats(t *testing.T) {
	e, _ := NewEngine(3)
	e.Encrypt([]byte("test"))
	e.RotateKey()

	s := e.Stats()
	if s.TotalEncrypted != 1 {
		t.Errorf("TotalEncrypted beklenen: 1, alınan: %d", s.TotalEncrypted)
	}
	if s.KeyRotations != 1 {
		t.Errorf("KeyRotations beklenen: 1, alınan: %d", s.KeyRotations)
	}
}

func TestKeys(t *testing.T) {
	e, _ := NewEngine(3)
	e.RotateKey()
	keys := e.Keys()
	if len(keys) != 2 {
		t.Errorf("Keys beklenen: 2, alınan: %d", len(keys))
	}
	if keys[0].Active {
		t.Error("İlk anahtar aktif olmamalı")
	}
	if !keys[1].Active {
		t.Error("İkinci anahtar aktif olmalı")
	}
}
