// Package server — İM HTTP/REST API sunucusu.
//
// PKI sertifika işlemleri, JWT token yönetimi ve birim
// kimlik doğrulama endpoint'lerini sunar.
package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/im/internal/identity"
	"github.com/otukenmatris/im/internal/pki"
	"github.com/otukenmatris/im/internal/token"
)

// Server İM HTTP sunucusu.
type Server struct {
	ca       *pki.CA
	tokens   *token.Manager
	registry *identity.Registry
	startAt  time.Time
	reqCount int64
}

// NewServer yeni sunucu oluşturur.
func NewServer(ca *pki.CA, tokens *token.Manager, registry *identity.Registry) *Server {
	return &Server{
		ca:       ca,
		tokens:   tokens,
		registry: registry,
		startAt:  time.Now(),
	}
}

// Handler HTTP handler döndürür.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Sağlık ve durum
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)

	// PKI endpoint'leri
	mux.HandleFunc("/pki/ca", s.handleCACert)
	mux.HandleFunc("/pki/issue", s.handleIssueCert)
	mux.HandleFunc("/pki/verify", s.handleVerifyCert)
	mux.HandleFunc("/pki/revoke", s.handleRevokeCert)
	mux.HandleFunc("/pki/certs", s.handleListCerts)
	mux.HandleFunc("/pki/rotate", s.handleRotateCA)

	// Token endpoint'leri
	mux.HandleFunc("/token/issue", s.handleIssueToken)
	mux.HandleFunc("/token/verify", s.handleVerifyToken)
	mux.HandleFunc("/token/refresh", s.handleRefreshToken)
	mux.HandleFunc("/token/revoke", s.handleRevokeToken)

	// Kimlik endpoint'leri
	mux.HandleFunc("/identity/register", s.handleRegister)
	mux.HandleFunc("/identity/authorize", s.handleAuthorize)
	mux.HandleFunc("/identity/heartbeat", s.handleHeartbeat)
	mux.HandleFunc("/identity/units", s.handleListUnits)
	mux.HandleFunc("/identity/unit/", s.handleUnitByID)
	mux.HandleFunc("/identity/suspend", s.handleSuspend)
	mux.HandleFunc("/identity/reactivate", s.handleReactivate)
	mux.HandleFunc("/identity/revoke-unit", s.handleRevokeUnit)

	// Tam kayıt akışı (register + cert + token)
	mux.HandleFunc("/enroll", s.handleEnroll)

	return mux
}

// ── Sağlık & Durum ──

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	writeJSON(w, 200, map[string]interface{}{
		"healthy":    true,
		"module":     "İM",
		"status":     "operational",
		"uptime_sec": int64(time.Since(s.startAt).Seconds()),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	writeJSON(w, 200, map[string]interface{}{
		"module":         "İM — Dijital İmza ve Kimlik",
		"uptime_sec":     int64(time.Since(s.startAt).Seconds()),
		"total_requests": atomic.LoadInt64(&s.reqCount),
		"pki_stats":      s.ca.Stats(),
		"token_stats":    s.tokens.Stats(),
		"identity_stats": s.registry.Stats(),
	})
}

// ── PKI Endpoint'leri ──

func (s *Server) handleCACert(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Write(s.ca.CACertPEM())
}

func (s *Server) handleIssueCert(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		UnitID   string `json:"unit_id"`
		UnitType string `json:"unit_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	if body.UnitID == "" {
		writeJSON(w, 400, errMsg("unit_id gerekli"))
		return
	}

	cert, err := s.ca.IssueCert(body.UnitID, body.UnitType)
	if err != nil {
		writeJSON(w, 500, errMsg(err.Error()))
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"cert_pem": string(cert.CertPEM),
		"key_pem":  string(cert.KeyPEM),
		"info":     cert.Info,
	})
}

func (s *Server) handleVerifyCert(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		CertPEM string `json:"cert_pem"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}

	info, err := s.ca.VerifyCert([]byte(body.CertPEM))
	if err != nil {
		writeJSON(w, 401, map[string]interface{}{"valid": false, "error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"valid": true, "info": info})
}

func (s *Server) handleRevokeCert(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		SerialNumber string `json:"serial_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}

	if err := s.ca.RevokeCert(body.SerialNumber); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, map[string]string{"status": "revoked"})
}

func (s *Server) handleListCerts(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	certs := s.ca.ListCerts()
	writeJSON(w, 200, map[string]interface{}{"certs": certs, "count": len(certs)})
}

func (s *Server) handleRotateCA(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	if err := s.ca.RotateCA(); err != nil {
		writeJSON(w, 500, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, map[string]string{"status": "rotated"})
}

// ── Token Endpoint'leri ──

func (s *Server) handleIssueToken(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		UnitID string   `json:"unit_id"`
		Roles  []string `json:"roles"`
		Perms  []string `json:"perms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}

	pair, err := s.tokens.IssueTokenPair(body.UnitID, body.Roles, body.Perms)
	if err != nil {
		writeJSON(w, 500, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, pair)
}

func (s *Server) handleVerifyToken(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	tokenStr := extractBearerToken(r)
	if tokenStr == "" {
		var body struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Token == "" {
			writeJSON(w, 400, errMsg("token gerekli (Authorization header veya JSON body)"))
			return
		}
		tokenStr = body.Token
	}

	claims, err := s.tokens.Verify(tokenStr)
	if err != nil {
		writeJSON(w, 401, map[string]interface{}{"valid": false, "error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"valid": true, "claims": claims})
}

func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}

	newToken, err := s.tokens.RefreshAccess(body.RefreshToken)
	if err != nil {
		writeJSON(w, 401, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"access_token": newToken,
		"token_type":   "Bearer",
	})
}

func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	if err := s.tokens.RevokeToken(body.Token); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, map[string]string{"status": "revoked"})
}

// ── Kimlik Endpoint'leri ──

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		ID    string   `json:"id"`
		Name  string   `json:"name"`
		Type  string   `json:"type"`
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	if body.ID == "" || body.Type == "" {
		writeJSON(w, 400, errMsg("id ve type gerekli"))
		return
	}

	unit, err := s.registry.Register(body.ID, body.Name, body.Type, body.Roles)
	if err != nil {
		writeJSON(w, 409, errMsg(err.Error()))
		return
	}
	writeJSON(w, 201, unit)
}

func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	var body struct {
		UnitID     string `json:"unit_id"`
		Permission string `json:"permission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}

	if err := s.registry.Authorize(body.UnitID, body.Permission); err != nil {
		writeJSON(w, 403, map[string]interface{}{"authorized": false, "error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"authorized": true})
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		UnitID string `json:"unit_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	if err := s.registry.Heartbeat(body.UnitID); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleListUnits(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	unitType := r.URL.Query().Get("type")
	units := s.registry.ListByType(unitType)
	writeJSON(w, 200, map[string]interface{}{"units": units, "count": len(units)})
}

func (s *Server) handleUnitByID(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	id := strings.TrimPrefix(r.URL.Path, "/identity/unit/")
	if id == "" {
		writeJSON(w, 400, errMsg("birim ID gerekli"))
		return
	}
	unit, ok := s.registry.Get(id)
	if !ok {
		writeJSON(w, 404, errMsg("birim bulunamadı"))
		return
	}
	writeJSON(w, 200, unit)
}

func (s *Server) handleSuspend(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		UnitID string `json:"unit_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	if err := s.registry.Suspend(body.UnitID); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, map[string]string{"status": "suspended"})
}

func (s *Server) handleReactivate(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		UnitID string `json:"unit_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	if err := s.registry.Reactivate(body.UnitID); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, map[string]string{"status": "reactivated"})
}

func (s *Server) handleRevokeUnit(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		UnitID string `json:"unit_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}

	// Sertifika ve token'ları da iptal et
	if u, ok := s.registry.Get(body.UnitID); ok && u.CertSerial != "" {
		s.ca.RevokeCert(u.CertSerial)
	}
	s.tokens.RevokeAllForUnit(body.UnitID)

	if err := s.registry.Revoke(body.UnitID); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, map[string]string{"status": "revoked"})
}

// ── Tam Kayıt Akışı ──

func (s *Server) handleEnroll(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}

	var body struct {
		ID    string   `json:"id"`
		Name  string   `json:"name"`
		Type  string   `json:"type"`
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	if body.ID == "" || body.Type == "" {
		writeJSON(w, 400, errMsg("id ve type gerekli"))
		return
	}

	// 1. Kimlik kaydı
	unit, err := s.registry.Register(body.ID, body.Name, body.Type, body.Roles)
	if err != nil {
		writeJSON(w, 409, errMsg(err.Error()))
		return
	}

	// 2. Sertifika üret
	cert, err := s.ca.IssueCert(body.ID, body.Type)
	if err != nil {
		writeJSON(w, 500, errMsg("sertifika üretim hatası: "+err.Error()))
		return
	}
	s.registry.SetCertSerial(body.ID, cert.Info.SerialNumber)

	// 3. Token çifti oluştur
	pair, err := s.tokens.IssueTokenPair(body.ID, unit.Roles, unit.Permissions)
	if err != nil {
		writeJSON(w, 500, errMsg("token üretim hatası: "+err.Error()))
		return
	}

	writeJSON(w, 201, map[string]interface{}{
		"unit":         unit,
		"certificate":  cert.Info,
		"cert_pem":     string(cert.CertPEM),
		"key_pem":      string(cert.KeyPEM),
		"access_token": pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":   pair.ExpiresIn,
	})
}

// ── Yardımcılar ──

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func errMsg(msg string) map[string]string {
	return map[string]string{"error": msg}
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// Stats sunucu istatistikleri.
func (s *Server) Stats() map[string]interface{} {
	return map[string]interface{}{
		"pki":      s.ca.Stats(),
		"tokens":   s.tokens.Stats(),
		"identity": s.registry.Stats(),
		"requests": atomic.LoadInt64(&s.reqCount),
	}
}
