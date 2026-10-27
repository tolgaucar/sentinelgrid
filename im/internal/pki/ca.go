// Package pki — Açık Anahtar Altyapısı (PKI).
//
// Sıfır Güven mimarisinin kriptografik çekirdeğini oluşturur.
// CA sertifika üretimi, birim sertifika imzalama, anahtar rotasyonu
// ve sertifika doğrulama işlemlerini yönetir.
package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"
)

// CertInfo bir sertifikanın özet bilgisi.
type CertInfo struct {
	SerialNumber string    `json:"serial_number"`
	Subject      string    `json:"subject"`
	Issuer       string    `json:"issuer"`
	NotBefore    time.Time `json:"not_before"`
	NotAfter     time.Time `json:"not_after"`
	Fingerprint  string    `json:"fingerprint"` // SHA-256
	Revoked      bool      `json:"revoked"`
}

// IssuedCert imzalanmış bir sertifika ve anahtarı.
type IssuedCert struct {
	CertPEM    []byte   `json:"cert_pem"`
	KeyPEM     []byte   `json:"-"` // Özel anahtar asla JSON'a serileştirilmez
	Info       CertInfo `json:"info"`
}

// CA Sertifika Otoritesi.
type CA struct {
	mu            sync.RWMutex
	caCert        *x509.Certificate
	caKey         *ecdsa.PrivateKey
	caCertPEM     []byte
	issuedCerts   map[string]*IssuedCert // serial → cert
	revokedCerts  map[string]time.Time   // serial → revoke zamanı
	certLifetime  time.Duration
	maxCerts      int
	serialCounter int64
	stats         CAStats
}

// CAStats CA istatistikleri.
type CAStats struct {
	TotalIssued  int64 `json:"total_issued"`
	TotalRevoked int64 `json:"total_revoked"`
	ActiveCerts  int64 `json:"active_certs"`
	KeyRotations int64 `json:"key_rotations"`
}

// NewCA yeni bir Sertifika Otoritesi oluşturur.
func NewCA(certLifetime time.Duration, maxCerts int) (*CA, error) {
	// ECDSA P-256 anahtar çifti oluştur (askeri standart)
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("CA anahtar üretim hatası: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("seri numarası üretim hatası: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "ÖTÜKEN-MATRİS Kök CA",
			Organization: []string{"ÖTÜKEN-MATRİS"},
			Country:      []string{"TR"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 yıl
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, template, template, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("CA sertifika üretim hatası: %w", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, fmt.Errorf("CA sertifika ayrıştırma hatası: %w", err)
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})

	return &CA{
		caCert:       caCert,
		caKey:        caKey,
		caCertPEM:    caCertPEM,
		issuedCerts:  make(map[string]*IssuedCert),
		revokedCerts: make(map[string]time.Time),
		certLifetime: certLifetime,
		maxCerts:     maxCerts,
	}, nil
}

// CACertPEM CA sertifikasını PEM formatında döndürür.
func (ca *CA) CACertPEM() []byte {
	return ca.caCertPEM
}

// IssueCert bir birim için yeni sertifika imzalar.
func (ca *CA) IssueCert(unitID string, unitType string) (*IssuedCert, error) {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	active := int(atomic.LoadInt64(&ca.stats.ActiveCerts))
	if active >= ca.maxCerts {
		return nil, fmt.Errorf("maksimum sertifika limitine ulaşıldı: %d", ca.maxCerts)
	}

	// Birim anahtarı oluştur
	unitKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("birim anahtar üretim hatası: %w", err)
	}

	serial := atomic.AddInt64(&ca.serialCounter, 1)
	serialBig := big.NewInt(serial)

	template := &x509.Certificate{
		SerialNumber: serialBig,
		Subject: pkix.Name{
			CommonName:         unitID,
			Organization:       []string{"ÖTÜKEN-MATRİS"},
			OrganizationalUnit: []string{unitType},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(ca.certLifetime),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca.caCert, &unitKey.PublicKey, ca.caKey)
	if err != nil {
		return nil, fmt.Errorf("sertifika imzalama hatası: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(unitKey)
	if err != nil {
		return nil, fmt.Errorf("özel anahtar kodlama hatası: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	fingerprint := sha256.Sum256(certDER)
	serialStr := fmt.Sprintf("%d", serial)

	issued := &IssuedCert{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
		Info: CertInfo{
			SerialNumber: serialStr,
			Subject:      unitID,
			Issuer:       "ÖTÜKEN-MATRİS Kök CA",
			NotBefore:    template.NotBefore,
			NotAfter:     template.NotAfter,
			Fingerprint:  fmt.Sprintf("%x", fingerprint),
			Revoked:      false,
		},
	}

	ca.issuedCerts[serialStr] = issued
	atomic.AddInt64(&ca.stats.TotalIssued, 1)
	atomic.AddInt64(&ca.stats.ActiveCerts, 1)

	return issued, nil
}

// RevokeCert sertifikayı iptal eder.
func (ca *CA) RevokeCert(serialNumber string) error {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	cert, ok := ca.issuedCerts[serialNumber]
	if !ok {
		return fmt.Errorf("sertifika bulunamadı: %s", serialNumber)
	}
	if cert.Info.Revoked {
		return fmt.Errorf("sertifika zaten iptal edilmiş: %s", serialNumber)
	}

	cert.Info.Revoked = true
	ca.revokedCerts[serialNumber] = time.Now()
	atomic.AddInt64(&ca.stats.TotalRevoked, 1)
	atomic.AddInt64(&ca.stats.ActiveCerts, -1)

	return nil
}

// VerifyCert sertifikanın geçerliliğini doğrular.
func (ca *CA) VerifyCert(certPEM []byte) (*CertInfo, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("PEM ayrıştırma hatası")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("sertifika ayrıştırma hatası: %w", err)
	}

	// CA ile doğrula
	pool := x509.NewCertPool()
	pool.AddCert(ca.caCert)

	opts := x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	if _, err := cert.Verify(opts); err != nil {
		return nil, fmt.Errorf("sertifika doğrulama hatası: %w", err)
	}

	// İptal listesini kontrol et
	serialStr := cert.SerialNumber.String()
	if _, revoked := ca.revokedCerts[serialStr]; revoked {
		return nil, fmt.Errorf("sertifika iptal edilmiş: %s", serialStr)
	}

	fingerprint := sha256.Sum256(block.Bytes)
	info := &CertInfo{
		SerialNumber: serialStr,
		Subject:      cert.Subject.CommonName,
		Issuer:       cert.Issuer.CommonName,
		NotBefore:    cert.NotBefore,
		NotAfter:     cert.NotAfter,
		Fingerprint:  fmt.Sprintf("%x", fingerprint),
		Revoked:      false,
	}

	return info, nil
}

// RotateCA yeni bir CA anahtar çifti oluşturur (zero-downtime).
func (ca *CA) RotateCA() error {
	newKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("yeni CA anahtar üretim hatası: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("seri numarası üretim hatası: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "ÖTÜKEN-MATRİS Kök CA",
			Organization: []string{"ÖTÜKEN-MATRİS"},
			Country:      []string{"TR"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	// Eski CA ile yeni sertifikayı imzala (cross-sign)
	ca.mu.Lock()
	defer ca.mu.Unlock()

	caCertDER, err := x509.CreateCertificate(rand.Reader, template, ca.caCert, &newKey.PublicKey, ca.caKey)
	if err != nil {
		return fmt.Errorf("yeni CA sertifika üretim hatası: %w", err)
	}

	newCaCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return fmt.Errorf("yeni CA sertifika ayrıştırma hatası: %w", err)
	}

	ca.caCert = newCaCert
	ca.caKey = newKey
	ca.caCertPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	atomic.AddInt64(&ca.stats.KeyRotations, 1)

	return nil
}

// ListCerts tüm sertifikaları listeler.
func (ca *CA) ListCerts() []CertInfo {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	result := make([]CertInfo, 0, len(ca.issuedCerts))
	for _, c := range ca.issuedCerts {
		result = append(result, c.Info)
	}
	return result
}

// GetCert belirli bir sertifika bilgisini döndürür.
func (ca *CA) GetCert(serialNumber string) (*CertInfo, bool) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	c, ok := ca.issuedCerts[serialNumber]
	if !ok {
		return nil, false
	}
	info := c.Info
	return &info, true
}

// Stats istatistikleri döndürür.
func (ca *CA) Stats() CAStats {
	return CAStats{
		TotalIssued:  atomic.LoadInt64(&ca.stats.TotalIssued),
		TotalRevoked: atomic.LoadInt64(&ca.stats.TotalRevoked),
		ActiveCerts:  atomic.LoadInt64(&ca.stats.ActiveCerts),
		KeyRotations: atomic.LoadInt64(&ca.stats.KeyRotations),
	}
}
