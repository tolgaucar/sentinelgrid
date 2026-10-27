package pki

import (
	"testing"
	"time"
)

func TestNewCA(t *testing.T) {
	ca, err := NewCA(24*time.Hour, 1000)
	if err != nil {
		t.Fatalf("NewCA hatası: %v", err)
	}
	if len(ca.CACertPEM()) == 0 {
		t.Fatal("CA sertifika PEM boş")
	}
}

func TestIssueCert(t *testing.T) {
	ca, _ := NewCA(24*time.Hour, 1000)
	cert, err := ca.IssueCert("drone-001", "drone")
	if err != nil {
		t.Fatalf("IssueCert hatası: %v", err)
	}
	if cert.Info.Subject != "drone-001" {
		t.Errorf("Subject beklenen: drone-001, alınan: %s", cert.Info.Subject)
	}
	if len(cert.CertPEM) == 0 {
		t.Error("CertPEM boş")
	}
	if len(cert.KeyPEM) == 0 {
		t.Error("KeyPEM boş")
	}
	if cert.Info.Fingerprint == "" {
		t.Error("Fingerprint boş")
	}

	stats := ca.Stats()
	if stats.TotalIssued != 1 {
		t.Errorf("TotalIssued beklenen: 1, alınan: %d", stats.TotalIssued)
	}
	if stats.ActiveCerts != 1 {
		t.Errorf("ActiveCerts beklenen: 1, alınan: %d", stats.ActiveCerts)
	}
}

func TestVerifyCert(t *testing.T) {
	ca, _ := NewCA(24*time.Hour, 1000)
	cert, _ := ca.IssueCert("tank-001", "tank")

	info, err := ca.VerifyCert(cert.CertPEM)
	if err != nil {
		t.Fatalf("VerifyCert hatası: %v", err)
	}
	if info.Subject != "tank-001" {
		t.Errorf("Subject beklenen: tank-001, alınan: %s", info.Subject)
	}
}

func TestVerifyInvalidCert(t *testing.T) {
	ca, _ := NewCA(24*time.Hour, 1000)
	_, err := ca.VerifyCert([]byte("geçersiz PEM"))
	if err == nil {
		t.Fatal("Geçersiz PEM için hata bekleniyor")
	}
}

func TestVerifyCrossCA(t *testing.T) {
	ca1, _ := NewCA(24*time.Hour, 1000)
	ca2, _ := NewCA(24*time.Hour, 1000)
	cert, _ := ca1.IssueCert("unit-001", "drone")

	_, err := ca2.VerifyCert(cert.CertPEM)
	if err == nil {
		t.Fatal("Farklı CA ile doğrulama başarısız olmalı")
	}
}

func TestRevokeCert(t *testing.T) {
	ca, _ := NewCA(24*time.Hour, 1000)
	cert, _ := ca.IssueCert("spy-001", "infantry")

	err := ca.RevokeCert(cert.Info.SerialNumber)
	if err != nil {
		t.Fatalf("RevokeCert hatası: %v", err)
	}

	// İptal edilen sertifikayı doğrulama başarısız olmalı
	_, err = ca.VerifyCert(cert.CertPEM)
	if err == nil {
		t.Fatal("İptal edilen sertifika doğrulanmamalı")
	}

	stats := ca.Stats()
	if stats.TotalRevoked != 1 {
		t.Errorf("TotalRevoked beklenen: 1, alınan: %d", stats.TotalRevoked)
	}
	if stats.ActiveCerts != 0 {
		t.Errorf("ActiveCerts beklenen: 0, alınan: %d", stats.ActiveCerts)
	}
}

func TestDoubleRevoke(t *testing.T) {
	ca, _ := NewCA(24*time.Hour, 1000)
	cert, _ := ca.IssueCert("unit-x", "drone")
	ca.RevokeCert(cert.Info.SerialNumber)

	err := ca.RevokeCert(cert.Info.SerialNumber)
	if err == nil {
		t.Fatal("Çift iptal hatası bekleniyor")
	}
}

func TestMaxCertsLimit(t *testing.T) {
	ca, _ := NewCA(24*time.Hour, 3) // Limit: 3
	ca.IssueCert("u1", "drone")
	ca.IssueCert("u2", "drone")
	ca.IssueCert("u3", "drone")

	_, err := ca.IssueCert("u4", "drone")
	if err == nil {
		t.Fatal("Limit aşımı hatası bekleniyor")
	}
}

func TestRotateCA(t *testing.T) {
	ca, _ := NewCA(24*time.Hour, 1000)
	oldPEM := string(ca.CACertPEM())

	err := ca.RotateCA()
	if err != nil {
		t.Fatalf("RotateCA hatası: %v", err)
	}

	newPEM := string(ca.CACertPEM())
	if oldPEM == newPEM {
		t.Error("CA sertifikası rotasyondan sonra değişmeli")
	}

	stats := ca.Stats()
	if stats.KeyRotations != 1 {
		t.Errorf("KeyRotations beklenen: 1, alınan: %d", stats.KeyRotations)
	}
}

func TestListCerts(t *testing.T) {
	ca, _ := NewCA(24*time.Hour, 1000)
	ca.IssueCert("a", "drone")
	ca.IssueCert("b", "tank")
	ca.IssueCert("c", "infantry")

	certs := ca.ListCerts()
	if len(certs) != 3 {
		t.Errorf("Beklenen 3 sertifika, alınan: %d", len(certs))
	}
}

func TestGetCert(t *testing.T) {
	ca, _ := NewCA(24*time.Hour, 1000)
	cert, _ := ca.IssueCert("test-unit", "sensor")

	info, ok := ca.GetCert(cert.Info.SerialNumber)
	if !ok {
		t.Fatal("Sertifika bulunamadı")
	}
	if info.Subject != "test-unit" {
		t.Errorf("Subject beklenen: test-unit, alınan: %s", info.Subject)
	}

	_, ok = ca.GetCert("nonexistent")
	if ok {
		t.Fatal("Olmayan sertifika bulunamazdı")
	}
}
