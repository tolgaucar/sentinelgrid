// Package token — JWT tabanlı token yönetimi.
//
// Sıfır Güven mimarisinde her birime özel, sürekli dönen
// (rotating) erişim ve yenileme token'ları üretir/doğrular.
// HMAC-SHA256 ile imzalanır. Token'lar claim bazlı yetki
// içerir: birim kimliği, roller, izinler.
package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Claims JWT payload.
type Claims struct {
	Sub     string   `json:"sub"`      // Birim ID
	Iss     string   `json:"iss"`      // Yayıncı
	Iat     int64    `json:"iat"`      // Oluşturma zamanı
	Exp     int64    `json:"exp"`      // Son kullanma zamanı
	Roles   []string `json:"roles"`    // Roller (commander, sensor, relay)
	Perms   []string `json:"perms"`    // İzinler (read, write, admin)
	TokenID string   `json:"jti"`      // Benzersiz token kimliği
	Type    string   `json:"typ"`      // access veya refresh
}

// TokenPair erişim ve yenileme token'ı çifti.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`    // Saniye
	TokenType    string `json:"token_type"`
}

// Manager JWT token yöneticisi.
type Manager struct {
	mu              sync.RWMutex
	signingKey      []byte
	issuer          string
	accessLifetime  time.Duration
	refreshLifetime time.Duration
	maxActiveSess   int

	// Aktif oturumlar: unitID → []tokenID
	activeSessions map[string][]string
	// Token iptal listesi (blacklist)
	revokedTokens map[string]time.Time

	stats ManagerStats
}

// ManagerStats token istatistikleri.
type ManagerStats struct {
	TotalIssued   int64 `json:"total_issued"`
	TotalVerified int64 `json:"total_verified"`
	TotalRevoked  int64 `json:"total_revoked"`
	TotalRejected int64 `json:"total_rejected"`
	ActiveSess    int64 `json:"active_sessions"`
}

// NewManager yeni token yöneticisi oluşturur.
func NewManager(issuer string, accessLifetime, refreshLifetime time.Duration, maxActiveSess int) (*Manager, error) {
	key := make([]byte, 32) // 256-bit anahtar
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("imzalama anahtarı üretim hatası: %w", err)
	}

	return &Manager{
		signingKey:      key,
		issuer:          issuer,
		accessLifetime:  accessLifetime,
		refreshLifetime: refreshLifetime,
		maxActiveSess:   maxActiveSess,
		activeSessions:  make(map[string][]string),
		revokedTokens:   make(map[string]time.Time),
	}, nil
}

// IssueTokenPair bir birime erişim + yenileme token çifti verir.
func (m *Manager) IssueTokenPair(unitID string, roles []string, perms []string) (*TokenPair, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Oturum limiti kontrol
	sessions := m.activeSessions[unitID]
	if len(sessions) >= m.maxActiveSess {
		// En eski oturumu sil
		oldest := sessions[0]
		m.revokedTokens[oldest] = time.Now()
		m.activeSessions[unitID] = sessions[1:]
		atomic.AddInt64(&m.stats.TotalRevoked, 1)
	}

	now := time.Now()

	accessTokenID := generateTokenID()
	accessClaims := &Claims{
		Sub:     unitID,
		Iss:     m.issuer,
		Iat:     now.Unix(),
		Exp:     now.Add(m.accessLifetime).Unix(),
		Roles:   roles,
		Perms:   perms,
		TokenID: accessTokenID,
		Type:    "access",
	}

	refreshTokenID := generateTokenID()
	refreshClaims := &Claims{
		Sub:     unitID,
		Iss:     m.issuer,
		Iat:     now.Unix(),
		Exp:     now.Add(m.refreshLifetime).Unix(),
		Roles:   roles,
		Perms:   perms,
		TokenID: refreshTokenID,
		Type:    "refresh",
	}

	accessToken, err := m.signWithKey(accessClaims, m.signingKey)
	if err != nil {
		return nil, err
	}
	refreshToken, err := m.signWithKey(refreshClaims, m.signingKey)
	if err != nil {
		return nil, err
	}

	m.activeSessions[unitID] = append(m.activeSessions[unitID], accessTokenID)
	atomic.AddInt64(&m.stats.TotalIssued, 2)
	atomic.AddInt64(&m.stats.ActiveSess, 1)

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(m.accessLifetime.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

// Verify token'ı doğrular ve claim'leri döndürür.
func (m *Manager) Verify(tokenStr string) (*Claims, error) {
	claims, err := m.parseAndVerify(tokenStr)
	if err != nil {
		atomic.AddInt64(&m.stats.TotalRejected, 1)
		return nil, err
	}

	// Süre kontrolü
	if time.Now().Unix() > claims.Exp {
		atomic.AddInt64(&m.stats.TotalRejected, 1)
		return nil, fmt.Errorf("token süresi dolmuş")
	}

	// İptal listesi kontrolü
	m.mu.RLock()
	_, revoked := m.revokedTokens[claims.TokenID]
	m.mu.RUnlock()

	if revoked {
		atomic.AddInt64(&m.stats.TotalRejected, 1)
		return nil, fmt.Errorf("token iptal edilmiş")
	}

	atomic.AddInt64(&m.stats.TotalVerified, 1)
	return claims, nil
}

// RefreshAccess yenileme token'ı ile yeni erişim token'ı üretir.
func (m *Manager) RefreshAccess(refreshTokenStr string) (string, error) {
	claims, err := m.Verify(refreshTokenStr)
	if err != nil {
		return "", fmt.Errorf("yenileme token doğrulama hatası: %w", err)
	}
	if claims.Type != "refresh" {
		return "", fmt.Errorf("geçersiz token tipi: %s (refresh bekleniyor)", claims.Type)
	}

	now := time.Now()
	newClaims := &Claims{
		Sub:     claims.Sub,
		Iss:     m.issuer,
		Iat:     now.Unix(),
		Exp:     now.Add(m.accessLifetime).Unix(),
		Roles:   claims.Roles,
		Perms:   claims.Perms,
		TokenID: generateTokenID(),
		Type:    "access",
	}

	token, err := m.sign(newClaims)
	if err != nil {
		return "", err
	}

	atomic.AddInt64(&m.stats.TotalIssued, 1)
	return token, nil
}

// RevokeToken token'ı iptal eder.
func (m *Manager) RevokeToken(tokenStr string) error {
	claims, err := m.parseAndVerify(tokenStr)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.revokedTokens[claims.TokenID] = time.Now()
	// Aktif oturumlardan sil
	sessions := m.activeSessions[claims.Sub]
	for i, sid := range sessions {
		if sid == claims.TokenID {
			m.activeSessions[claims.Sub] = append(sessions[:i], sessions[i+1:]...)
			atomic.AddInt64(&m.stats.ActiveSess, -1)
			break
		}
	}
	m.mu.Unlock()

	atomic.AddInt64(&m.stats.TotalRevoked, 1)
	return nil
}

// RevokeAllForUnit bir birime ait tüm token'ları iptal eder.
func (m *Manager) RevokeAllForUnit(unitID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessions := m.activeSessions[unitID]
	count := len(sessions)
	for _, tid := range sessions {
		m.revokedTokens[tid] = time.Now()
	}
	delete(m.activeSessions, unitID)
	atomic.AddInt64(&m.stats.TotalRevoked, int64(count))
	atomic.AddInt64(&m.stats.ActiveSess, -int64(count))
	return count
}

// RotateKey imzalama anahtarını döndürür.
func (m *Manager) RotateKey() error {
	newKey := make([]byte, 32)
	if _, err := rand.Read(newKey); err != nil {
		return fmt.Errorf("yeni imzalama anahtarı üretim hatası: %w", err)
	}

	m.mu.Lock()
	m.signingKey = newKey
	m.mu.Unlock()

	return nil
}

// CleanExpired süresi dolmuş token'ları iptal listesinden temizler.
func (m *Manager) CleanExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	cleaned := 0
	for tid, revokedAt := range m.revokedTokens {
		if now.Sub(revokedAt) > m.refreshLifetime*2 {
			delete(m.revokedTokens, tid)
			cleaned++
		}
	}
	return cleaned
}

// Stats istatistikleri döndürür.
func (m *Manager) Stats() ManagerStats {
	return ManagerStats{
		TotalIssued:   atomic.LoadInt64(&m.stats.TotalIssued),
		TotalVerified: atomic.LoadInt64(&m.stats.TotalVerified),
		TotalRevoked:  atomic.LoadInt64(&m.stats.TotalRevoked),
		TotalRejected: atomic.LoadInt64(&m.stats.TotalRejected),
		ActiveSess:    atomic.LoadInt64(&m.stats.ActiveSess),
	}
}

// ── İç fonksiyonlar ──

func (m *Manager) sign(claims *Claims) (string, error) {
	m.mu.RLock()
	key := m.signingKey
	m.mu.RUnlock()
	return m.signWithKey(claims, key)
}

func (m *Manager) signWithKey(claims *Claims, key []byte) (string, error) {
	headerJSON := []byte(`{"alg":"HS256","typ":"JWT"}`)
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("claims serileştirme hatası: %w", err)
	}

	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)

	signingInput := header + "." + payload

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature, nil
}

func (m *Manager) parseAndVerify(tokenStr string) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("geçersiz token formatı")
	}

	signingInput := parts[0] + "." + parts[1]

	m.mu.RLock()
	key := m.signingKey
	m.mu.RUnlock()

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(signingInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, fmt.Errorf("geçersiz token imzası")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("payload decode hatası: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("claims ayrıştırma hatası: %w", err)
	}

	return &claims, nil
}

func generateTokenID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
