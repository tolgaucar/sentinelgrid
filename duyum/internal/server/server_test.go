package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/otukenmatris/duyum/internal/analyzer"
	"github.com/otukenmatris/duyum/internal/collector"
	"github.com/otukenmatris/duyum/internal/config"
)

func newTestServer() *Server {
	coll := collector.NewEngine(50, 1000, 0.2, 3600)
	anal := analyzer.NewEngine(100, 0.1, 2, 1800)
	cfg := config.Config{}
	return New(coll, anal, cfg)
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
	if resp["status"] != "ok" {
		t.Fatalf("expected ok, got %v", resp["status"])
	}
	if resp["module"] != "duyum" {
		t.Fatalf("expected duyum, got %v", resp["module"])
	}
}

func TestStatus(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/status", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["module"] != "duyum" {
		t.Fatal("expected duyum")
	}
}

func TestStats(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/stats", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestSourcesList(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/sources", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	count := resp["count"].(float64)
	if count < 5 {
		t.Fatalf("expected at least 5 default sources, got %.0f", count)
	}
}

func TestSourcesAdd(t *testing.T) {
	s := newTestServer()
	body := map[string]any{
		"id":          "test-new-src",
		"type":        "news_feed",
		"name":        "Test News",
		"url":         "https://test.example/rss",
		"credibility": 0.7,
		"enabled":     true,
	}
	rr := doReq(t, s, "POST", "/sources", body)
	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestSourcesAddDuplicate(t *testing.T) {
	s := newTestServer()
	body := map[string]any{"id": "src-sm-01", "type": "social_media", "name": "Dup", "credibility": 0.5}
	rr := doReq(t, s, "POST", "/sources", body)
	if rr.Code != 409 {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestSourceGetByID(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/sources/src-sm-01", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestSourceGetByIDNotFound(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/sources/nonexistent", nil)
	if rr.Code != 404 {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestSourceUpdate(t *testing.T) {
	s := newTestServer()
	body := map[string]any{"name": "Updated Name", "enabled": true}
	rr := doReq(t, s, "PUT", "/sources/src-sm-01", body)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestSourceDelete(t *testing.T) {
	s := newTestServer()
	// önce ekle
	doReq(t, s, "POST", "/sources", map[string]any{"id": "to-delete", "type": "forum", "name": "Del", "credibility": 0.5})
	rr := doReq(t, s, "DELETE", "/sources/to-delete", nil)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCollect(t *testing.T) {
	s := newTestServer()
	body := map[string]any{"source_id": "src-sm-01"}
	rr := doReq(t, s, "POST", "/collect", body)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestCollectNotFound(t *testing.T) {
	s := newTestServer()
	body := map[string]any{"source_id": "nonexistent"}
	rr := doReq(t, s, "POST", "/collect", body)
	if rr.Code != 400 {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCollectMethodNotAllowed(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/collect", nil)
	if rr.Code != 405 {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestCollectAll(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/collect/all", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestAnalyze(t *testing.T) {
	s := newTestServer()
	body := map[string]any{
		"items": []map[string]any{
			{"id": "i1", "source_id": "s1", "title": "Birlik hareket konvoy", "content": "Tank hareket", "lat": 39.0, "lon": 33.0, "credibility": 0.7},
		},
	}
	rr := doReq(t, s, "POST", "/analyze", body)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestReports(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/reports", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestReportByIDNotFound(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/reports/nonexistent", nil)
	if rr.Code != 404 {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestThreats(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/threats", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestTrends(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/trends", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestProcess(t *testing.T) {
	s := newTestServer()
	body := map[string]any{
		"entities": []map[string]any{
			{"id": "e-1", "type": "tank", "faction": "enemy", "lat": 39.5, "lon": 33.0, "speed": 30.0},
			{"id": "e-2", "type": "infantry", "faction": "enemy", "lat": 39.501, "lon": 33.001, "speed": 5.0},
		},
	}
	rr := doReq(t, s, "POST", "/process", body)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
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
