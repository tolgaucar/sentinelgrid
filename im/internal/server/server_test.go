package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otukenmatris/im/internal/identity"
	"github.com/otukenmatris/im/internal/pki"
	"github.com/otukenmatris/im/internal/token"
)

func setupServer(t *testing.T) *Server {
	t.Helper()
	ca, err := pki.NewCA(24*time.Hour, 1000)
	if err != nil {
		t.Fatalf("CA hatası: %v", err)
	}
	tokens, err := token.NewManager("test", 15*time.Minute, 7*24*time.Hour, 5)
	if err != nil {
		t.Fatalf("Token hatası: %v", err)
	}
	registry := identity.NewRegistry()
	return NewServer(ca, tokens, registry)
}

func doReq(handler http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestHealth(t *testing.T) {
	s := setupServer(t)
	rr := doReq(s.Handler(), "GET", "/health", nil)
	if rr.Code != 200 {
		t.Fatalf("Health status beklenen: 200, alınan: %d", rr.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["healthy"] != true {
		t.Error("healthy beklenen: true")
	}
	if resp["module"] != "İM" {
		t.Errorf("module beklenen: İM, alınan: %v", resp["module"])
	}
}

func TestStatus(t *testing.T) {
	s := setupServer(t)
	rr := doReq(s.Handler(), "GET", "/status", nil)
	if rr.Code != 200 {
		t.Fatalf("Status code beklenen: 200, alınan: %d", rr.Code)
	}
}

func TestEnrollFlow(t *testing.T) {
	s := setupServer(t)
	handler := s.Handler()

	// Tam kayıt
	rr := doReq(handler, "POST", "/enroll", map[string]interface{}{
		"id": "drone-001", "name": "Keşif Dronu", "type": "drone", "roles": []string{"drone"},
	})
	if rr.Code != 201 {
		t.Fatalf("Enroll status beklenen: 201, alınan: %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["access_token"] == nil || resp["access_token"] == "" {
		t.Error("access_token boş")
	}
	if resp["cert_pem"] == nil || resp["cert_pem"] == "" {
		t.Error("cert_pem boş")
	}

	// Token doğrulama
	accessToken := resp["access_token"].(string)
	rr = doReq(handler, "POST", "/token/verify", map[string]string{"token": accessToken})
	if rr.Code != 200 {
		t.Fatalf("Token verify status beklenen: 200, alınan: %d", rr.Code)
	}

	// Birim sorgulama
	rr = doReq(handler, "GET", "/identity/unit/drone-001", nil)
	if rr.Code != 200 {
		t.Fatalf("Unit get status beklenen: 200, alınan: %d", rr.Code)
	}
}

func TestEnrollDuplicate(t *testing.T) {
	s := setupServer(t)
	handler := s.Handler()
	body := map[string]interface{}{"id": "u1", "name": "Birim", "type": "drone"}
	doReq(handler, "POST", "/enroll", body)
	rr := doReq(handler, "POST", "/enroll", body)
	if rr.Code != 409 {
		t.Fatalf("Çift kayıt status beklenen: 409, alınan: %d", rr.Code)
	}
}

func TestPKIIssueCert(t *testing.T) {
	s := setupServer(t)
	handler := s.Handler()

	rr := doReq(handler, "POST", "/pki/issue", map[string]string{
		"unit_id": "tank-001", "unit_type": "vehicle",
	})
	if rr.Code != 200 {
		t.Fatalf("Issue cert status beklenen: 200, alınan: %d, body: %s", rr.Code, rr.Body.String())
	}

	// CA sertifikası al
	rr = doReq(handler, "GET", "/pki/ca", nil)
	if rr.Code != 200 {
		t.Fatalf("CA cert status beklenen: 200, alınan: %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "application/x-pem-file" {
		t.Errorf("Content-Type beklenen: application/x-pem-file")
	}
}

func TestTokenIssueAndVerify(t *testing.T) {
	s := setupServer(t)
	handler := s.Handler()

	rr := doReq(handler, "POST", "/token/issue", map[string]interface{}{
		"unit_id": "sensor-001", "roles": []string{"sensor"}, "perms": []string{"read"},
	})
	if rr.Code != 200 {
		t.Fatalf("Issue token status beklenen: 200, alınan: %d", rr.Code)
	}

	var tokenResp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&tokenResp)

	// Verify
	rr = doReq(handler, "POST", "/token/verify", map[string]string{
		"token": tokenResp["access_token"].(string),
	})
	if rr.Code != 200 {
		t.Fatalf("Verify status beklenen: 200, alınan: %d", rr.Code)
	}
}

func TestTokenRefresh(t *testing.T) {
	s := setupServer(t)
	handler := s.Handler()

	rr := doReq(handler, "POST", "/token/issue", map[string]interface{}{
		"unit_id": "u-ref", "roles": []string{"drone"}, "perms": []string{"read"},
	})
	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	rr = doReq(handler, "POST", "/token/refresh", map[string]string{
		"refresh_token": resp["refresh_token"].(string),
	})
	if rr.Code != 200 {
		t.Fatalf("Refresh status beklenen: 200, alınan: %d, body: %s", rr.Code, rr.Body.String())
	}
}

func TestIdentityAuthorize(t *testing.T) {
	s := setupServer(t)
	handler := s.Handler()

	doReq(handler, "POST", "/identity/register", map[string]interface{}{
		"id": "cmd-001", "name": "Komutan", "type": "commander", "roles": []string{"commander"},
	})

	rr := doReq(handler, "POST", "/identity/authorize", map[string]string{
		"unit_id": "cmd-001", "permission": "admin",
	})
	if rr.Code != 200 {
		t.Fatalf("Authorize status beklenen: 200, alınan: %d", rr.Code)
	}

	rr = doReq(handler, "POST", "/identity/authorize", map[string]string{
		"unit_id": "cmd-001", "permission": "nonexistent_perm",
	})
	// commander has "admin" which acts as wildcard, so it should still be 200
	if rr.Code != 200 {
		t.Fatalf("Admin wildcard authorize beklenen: 200, alınan: %d", rr.Code)
	}
}

func TestSuspendReactivateRevoke(t *testing.T) {
	s := setupServer(t)
	handler := s.Handler()

	doReq(handler, "POST", "/identity/register", map[string]interface{}{
		"id": "u-sr", "name": "Test", "type": "drone",
	})

	// Suspend
	rr := doReq(handler, "POST", "/identity/suspend", map[string]string{"unit_id": "u-sr"})
	if rr.Code != 200 {
		t.Fatalf("Suspend status beklenen: 200, alınan: %d", rr.Code)
	}

	// Reactivate
	rr = doReq(handler, "POST", "/identity/reactivate", map[string]string{"unit_id": "u-sr"})
	if rr.Code != 200 {
		t.Fatalf("Reactivate status beklenen: 200, alınan: %d", rr.Code)
	}

	// Revoke
	rr = doReq(handler, "POST", "/identity/revoke-unit", map[string]string{"unit_id": "u-sr"})
	if rr.Code != 200 {
		t.Fatalf("Revoke status beklenen: 200, alınan: %d", rr.Code)
	}
}

func TestListUnits(t *testing.T) {
	s := setupServer(t)
	handler := s.Handler()

	doReq(handler, "POST", "/enroll", map[string]interface{}{"id": "a", "name": "A", "type": "drone"})
	doReq(handler, "POST", "/enroll", map[string]interface{}{"id": "b", "name": "B", "type": "sensor"})

	rr := doReq(handler, "GET", "/identity/units", nil)
	if rr.Code != 200 {
		t.Fatalf("List units status beklenen: 200, alınan: %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	count := int(resp["count"].(float64))
	if count != 2 {
		t.Errorf("Beklenen 2 birim, alınan: %d", count)
	}

	// Type filtresi
	rr = doReq(handler, "GET", "/identity/units?type=drone", nil)
	json.NewDecoder(rr.Body).Decode(&resp)
	count = int(resp["count"].(float64))
	if count != 1 {
		t.Errorf("Beklenen 1 drone, alınan: %d", count)
	}
}
