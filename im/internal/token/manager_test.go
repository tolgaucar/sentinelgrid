package token

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m, err := NewManager("test-issuer", 15*time.Minute, 7*24*time.Hour, 5)
	if err != nil {
		t.Fatalf("NewManager hatası: %v", err)
	}
	if m == nil {
		t.Fatal("Manager nil")
	}
}

func TestIssueAndVerify(t *testing.T) {
	m, _ := NewManager("test", 15*time.Minute, 7*24*time.Hour, 5)

	pair, err := m.IssueTokenPair("drone-001", []string{"drone"}, []string{"read", "write"})
	if err != nil {
		t.Fatalf("IssueTokenPair hatası: %v", err)
	}
	if pair.AccessToken == "" {
		t.Fatal("AccessToken boş")
	}
	if pair.RefreshToken == "" {
		t.Fatal("RefreshToken boş")
	}
	if pair.TokenType != "Bearer" {
		t.Errorf("TokenType beklenen: Bearer, alınan: %s", pair.TokenType)
	}

	// Access token doğrula
	claims, err := m.Verify(pair.AccessToken)
	if err != nil {
		t.Fatalf("Verify hatası: %v", err)
	}
	if claims.Sub != "drone-001" {
		t.Errorf("Sub beklenen: drone-001, alınan: %s", claims.Sub)
	}
	if claims.Type != "access" {
		t.Errorf("Type beklenen: access, alınan: %s", claims.Type)
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != "drone" {
		t.Errorf("Roles beklenen: [drone], alınan: %v", claims.Roles)
	}
}

func TestExpiredToken(t *testing.T) {
	m, _ := NewManager("test", 1*time.Second, 7*24*time.Hour, 5)

	pair, _ := m.IssueTokenPair("unit-x", []string{"drone"}, []string{"read"})
	time.Sleep(2100 * time.Millisecond)

	_, err := m.Verify(pair.AccessToken)
	if err == nil {
		t.Fatal("Süresi dolmuş token için hata bekleniyor")
	}
}

func TestRevokeToken(t *testing.T) {
	m, _ := NewManager("test", 15*time.Minute, 7*24*time.Hour, 5)

	pair, _ := m.IssueTokenPair("unit-y", []string{"drone"}, []string{"read"})

	err := m.RevokeToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("RevokeToken hatası: %v", err)
	}

	_, err = m.Verify(pair.AccessToken)
	if err == nil {
		t.Fatal("İptal edilen token doğrulanmamalı")
	}
}

func TestRefreshAccess(t *testing.T) {
	m, _ := NewManager("test", 15*time.Minute, 7*24*time.Hour, 5)

	pair, _ := m.IssueTokenPair("unit-z", []string{"sensor"}, []string{"read", "telemetry.write"})

	newAccess, err := m.RefreshAccess(pair.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshAccess hatası: %v", err)
	}
	if newAccess == "" {
		t.Fatal("Yeni access token boş")
	}

	// Yeni token doğrulanmalı
	claims, err := m.Verify(newAccess)
	if err != nil {
		t.Fatalf("Yeni token doğrulama hatası: %v", err)
	}
	if claims.Sub != "unit-z" {
		t.Errorf("Sub beklenen: unit-z, alınan: %s", claims.Sub)
	}
}

func TestRefreshWithAccessToken(t *testing.T) {
	m, _ := NewManager("test", 15*time.Minute, 7*24*time.Hour, 5)
	pair, _ := m.IssueTokenPair("unit-a", []string{"drone"}, []string{"read"})

	_, err := m.RefreshAccess(pair.AccessToken)
	if err == nil {
		t.Fatal("Access token ile refresh yapılamamalı")
	}
}

func TestSessionLimit(t *testing.T) {
	m, _ := NewManager("test", 15*time.Minute, 7*24*time.Hour, 2) // Limit: 2

	m.IssueTokenPair("unit-b", []string{"drone"}, []string{"read"})
	m.IssueTokenPair("unit-b", []string{"drone"}, []string{"read"})

	// 3. oturum en eskiyi silmeli
	pair3, err := m.IssueTokenPair("unit-b", []string{"drone"}, []string{"read"})
	if err != nil {
		t.Fatalf("3. token hatası: %v", err)
	}

	claims, err := m.Verify(pair3.AccessToken)
	if err != nil {
		t.Fatalf("3. token doğrulama hatası: %v", err)
	}
	if claims.Sub != "unit-b" {
		t.Errorf("Sub beklenen: unit-b, alınan: %s", claims.Sub)
	}
}

func TestRevokeAllForUnit(t *testing.T) {
	m, _ := NewManager("test", 15*time.Minute, 7*24*time.Hour, 10)

	m.IssueTokenPair("unit-c", []string{"drone"}, []string{"read"})
	m.IssueTokenPair("unit-c", []string{"drone"}, []string{"read"})
	m.IssueTokenPair("unit-c", []string{"drone"}, []string{"read"})

	count := m.RevokeAllForUnit("unit-c")
	if count != 3 {
		t.Errorf("Beklenen 3 iptal, alınan: %d", count)
	}
}

func TestInvalidToken(t *testing.T) {
	m, _ := NewManager("test", 15*time.Minute, 7*24*time.Hour, 5)

	_, err := m.Verify("invalid.token.string")
	if err == nil {
		t.Fatal("Geçersiz token için hata bekleniyor")
	}

	_, err = m.Verify("only-two-parts")
	if err == nil {
		t.Fatal("Yanlış formatlı token için hata bekleniyor")
	}
}

func TestRotateKey(t *testing.T) {
	m, _ := NewManager("test", 15*time.Minute, 7*24*time.Hour, 5)
	pair, _ := m.IssueTokenPair("unit-d", []string{"drone"}, []string{"read"})

	// Rotasyon öncesi doğrulama
	_, err := m.Verify(pair.AccessToken)
	if err != nil {
		t.Fatalf("Rotasyon öncesi doğrulama hatası: %v", err)
	}

	// Anahtar rotasyonu
	err = m.RotateKey()
	if err != nil {
		t.Fatalf("RotateKey hatası: %v", err)
	}

	// Eski token artık doğrulanamamalı
	_, err = m.Verify(pair.AccessToken)
	if err == nil {
		t.Fatal("Rotasyon sonrası eski token doğrulanmamalı")
	}
}

func TestStats(t *testing.T) {
	m, _ := NewManager("test", 15*time.Minute, 7*24*time.Hour, 5)
	m.IssueTokenPair("u1", []string{"drone"}, []string{"read"})
	m.IssueTokenPair("u2", []string{"sensor"}, []string{"read"})

	stats := m.Stats()
	if stats.TotalIssued != 4 { // 2 çift = 4 token
		t.Errorf("TotalIssued beklenen: 4, alınan: %d", stats.TotalIssued)
	}
	if stats.ActiveSess != 2 {
		t.Errorf("ActiveSess beklenen: 2, alınan: %d", stats.ActiveSess)
	}
}

func TestCleanExpired(t *testing.T) {
	m, _ := NewManager("test", 15*time.Minute, 1*time.Millisecond, 5) // Çok kısa refresh

	pair, _ := m.IssueTokenPair("u-clean", []string{"drone"}, []string{"read"})
	m.RevokeToken(pair.AccessToken)
	time.Sleep(5 * time.Millisecond)

	cleaned := m.CleanExpired()
	if cleaned != 1 {
		t.Errorf("Beklenen 1 temizleme, alınan: %d", cleaned)
	}
}
