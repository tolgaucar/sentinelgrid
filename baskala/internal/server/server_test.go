package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/otukenmatris/baskala/internal/change"
	"github.com/otukenmatris/baskala/internal/config"
	"github.com/otukenmatris/baskala/internal/fusion"
	"github.com/otukenmatris/baskala/internal/imagery"
)

func newTestServer() *Server {
	img := imagery.NewEngine(500, 16, 0.15)
	chg := change.NewEngine(1000, 0.1, 0.15, 2.0, 3600)
	fus := fusion.NewEngine(0.5, 0.5, 0.3, 0.4)
	cfg := config.Config{
		Imagery: config.ImageryConfig{RetentionHours: 48},
	}
	return New(img, chg, fus, cfg)
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
	if resp["module"] != "baskala" {
		t.Fatalf("expected baskala, got %v", resp["module"])
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

func TestImagesEmpty(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/images", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestImageIngest(t *testing.T) {
	s := newTestServer()
	body := map[string]any{
		"id": "img-test-1", "type": "optical",
		"center_lat": 39.9, "center_lon": 32.8,
		"width": 8, "height": 8, "resolution_m": 0.5,
	}
	rr := doReq(t, s, "POST", "/images", body)
	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestImageIngestDuplicate(t *testing.T) {
	s := newTestServer()
	body := map[string]any{"id": "dup", "type": "sar"}
	doReq(t, s, "POST", "/images", body)
	rr := doReq(t, s, "POST", "/images", body)
	if rr.Code != 409 {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestImageGetByID(t *testing.T) {
	s := newTestServer()
	doReq(t, s, "POST", "/images", map[string]any{"id": "img-1", "type": "sar"})
	rr := doReq(t, s, "GET", "/images/img-1", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestImageGetByIDNotFound(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/images/nonexistent", nil)
	if rr.Code != 404 {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestImageDelete(t *testing.T) {
	s := newTestServer()
	doReq(t, s, "POST", "/images", map[string]any{"id": "del-1", "type": "sar"})
	rr := doReq(t, s, "DELETE", "/images/del-1", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestChangesEmpty(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/changes", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestChangeByIDNotFound(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/changes/nonexistent", nil)
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

func TestTimeline(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/timeline", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestCompareMethodNotAllowed(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/compare", nil)
	if rr.Code != 405 {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestCompareMissingImage(t *testing.T) {
	s := newTestServer()
	body := map[string]any{"image_a_id": "a", "image_b_id": "b"}
	rr := doReq(t, s, "POST", "/compare", body)
	if rr.Code != 400 {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestProcess(t *testing.T) {
	s := newTestServer()
	body := map[string]any{
		"entities": []map[string]any{
			{"id": "e-1", "type": "tank", "faction": "enemy", "lat": 39.5, "lon": 32.5, "speed": 30.0},
			{"id": "e-2", "type": "infantry", "faction": "friendly", "lat": 39.51, "lon": 32.51, "speed": 5.0},
		},
	}
	rr := doReq(t, s, "POST", "/process", body)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["images_generated"].(float64) != 2 {
		t.Fatal("expected 2 images generated")
	}
}

func TestProcessTwice(t *testing.T) {
	s := newTestServer()
	entities := map[string]any{
		"entities": []map[string]any{
			{"id": "e-1", "type": "tank", "faction": "enemy", "lat": 39.5, "lon": 32.5},
		},
	}
	// ilk çağrı — referans görüntü oluşturur
	doReq(t, s, "POST", "/process", entities)
	// ikinci çağrı — önceki ile karşılaştırma yapabilir
	rr := doReq(t, s, "POST", "/process", entities)
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

func TestAnalyze(t *testing.T) {
	s := newTestServer()
	body := map[string]any{"lat": 39.925, "lon": 32.837, "radius_km": 50.0}
	rr := doReq(t, s, "POST", "/analyze", body)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}
