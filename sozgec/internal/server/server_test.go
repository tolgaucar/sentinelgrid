package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/otukenmatris/sozgec/internal/config"
	"github.com/otukenmatris/sozgec/internal/filter"
	"github.com/otukenmatris/sozgec/internal/spectrum"
)

func newTestServer() *Server {
	spec := spectrum.NewEngine(256, 100.0, -80.0, 64)
	filt := filter.NewEngine(-80, 6, 200, 0.05, 3600)
	cfg := config.Config{
		Filter: config.FilterConfig{ThreatDecayS: 3600},
	}
	return New(spec, filt, cfg)
}

func doReq(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	return rr
}

func TestHealth(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/health", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["module"] != "sozgec" {
		t.Fatalf("expected sozgec, got %v", resp["module"])
	}
}

func TestStatus(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/status", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestStats(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/stats", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestAnalyze(t *testing.T) {
	s := newTestServer()
	body := map[string]any{
		"signals": []map[string]any{
			{"id": "s1", "frequency_mhz": 30.0, "amplitude_db": -10, "bandwidth_mhz": 1.0},
		},
	}
	rr := doReq(t, s, "POST", "/analyze", body)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestAnalyzeMethodNotAllowed(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/analyze", nil)
	if rr.Code != 405 {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestBands(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/bands", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestBandByIDNotFound(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/bands/nonexistent", nil)
	if rr.Code != 404 {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestSignals(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/signals", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestThreats(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/threats", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestFriendly(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/friendly", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestFriendlyBandsGet(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/friendly-bands", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	count := resp["count"].(float64)
	if count < 8 {
		t.Fatalf("expected at least 8 default bands, got %.0f", count)
	}
}

func TestFriendlyBandsAdd(t *testing.T) {
	s := newTestServer()
	body := map[string]any{"id": "custom", "name": "Custom Band", "low_mhz": 500, "high_mhz": 600, "priority": 3}
	rr := doReq(t, s, "POST", "/friendly-bands", body)
	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestFriendlyBandsAddDuplicate(t *testing.T) {
	s := newTestServer()
	body := map[string]any{"id": "vhf-lo"}
	rr := doReq(t, s, "POST", "/friendly-bands", body)
	if rr.Code != 409 {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestFriendlyBandsDelete(t *testing.T) {
	s := newTestServer()
	body := map[string]any{"id": "vhf-lo"}
	rr := doReq(t, s, "DELETE", "/friendly-bands", body)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestProcess(t *testing.T) {
	s := newTestServer()
	body := map[string]any{
		"entities": []map[string]any{
			{"id": "e-1", "type": "tank", "faction": "friendly", "lat": 39.5, "lon": 32.5},
			{"id": "e-2", "type": "jammer", "faction": "enemy", "lat": 39.6, "lon": 32.6},
			{"id": "e-3", "type": "infantry", "faction": "enemy", "lat": 39.55, "lon": 32.55},
		},
	}
	rr := doReq(t, s, "POST", "/process", body)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["raw_signals"].(float64) != 3 {
		t.Fatalf("expected 3 raw signals, got %v", resp["raw_signals"])
	}
}

func TestProcessMethodNotAllowed(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/process", nil)
	if rr.Code != 405 {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestPrune(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/prune", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}
