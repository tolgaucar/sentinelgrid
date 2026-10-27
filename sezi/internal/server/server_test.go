package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/otukenmatris/sezi/internal/detector"
	"github.com/otukenmatris/sezi/internal/texture"
)

func newTestServer() *Server {
	texEngine := texture.NewEngine(4, 6, 0.05, 0.40, 16, 0.35)
	det := detector.NewEngine(texEngine, 600, 0.3, 0.60, 0.55, 500)
	return New(texEngine, det)
}

func doReq(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestHealth(t *testing.T) {
	s := newTestServer()
	w := doReq(t, s.Handler(), "GET", "/health", nil)
	if w.Code != 200 {
		t.Errorf("/health 200 beklenirken %d döndü", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["module"] != "SEZİ" {
		t.Error("module=SEZİ beklenirken farklı değer döndü")
	}
}

func TestStatus(t *testing.T) {
	s := newTestServer()
	w := doReq(t, s.Handler(), "GET", "/status", nil)
	if w.Code != 200 {
		t.Errorf("/status 200 beklenirken %d döndü", w.Code)
	}
}

func TestAnalyze(t *testing.T) {
	s := newTestServer()
	regions := []texture.RegionInput{
		{ID: "r1", Lat: 39.0, Lon: 32.0, Terrain: texture.TerrainForest, Width: 256, Height: 256},
	}
	w := doReq(t, s.Handler(), "POST", "/analyze", regions)
	if w.Code != 200 {
		t.Errorf("/analyze 200 beklenirken %d döndü", w.Code)
	}
}

func TestAnalyzeBadJSON(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("POST", "/analyze", bytes.NewBufferString("{bad"))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("/analyze kötü JSON 400 beklenirken %d döndü", w.Code)
	}
}

func TestAnalyzeMethodNotAllowed(t *testing.T) {
	s := newTestServer()
	w := doReq(t, s.Handler(), "GET", "/analyze", nil)
	if w.Code != 405 {
		t.Errorf("/analyze GET 405 beklenirken %d döndü", w.Code)
	}
}

func TestScan(t *testing.T) {
	s := newTestServer()
	inputs := []detector.ScanInput{
		{ID: "area-1", Lat: 39.0, Lon: 32.0, Terrain: texture.TerrainForest, Width: 256, Height: 256},
	}
	w := doReq(t, s.Handler(), "POST", "/scan", inputs)
	if w.Code != 200 {
		t.Errorf("/scan 200 beklenirken %d döndü", w.Code)
	}
}

func TestScanBadJSON(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("POST", "/scan", bytes.NewBufferString("{bad"))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("/scan kötü JSON 400 beklenirken %d döndü", w.Code)
	}
}

func TestScanMethodNotAllowed(t *testing.T) {
	s := newTestServer()
	w := doReq(t, s.Handler(), "GET", "/scan", nil)
	if w.Code != 405 {
		t.Errorf("/scan GET 405 beklenirken %d döndü", w.Code)
	}
}

func TestDetections(t *testing.T) {
	s := newTestServer()
	// tarama yap
	doReq(t, s.Handler(), "POST", "/scan", []detector.ScanInput{
		{ID: "area-1", Lat: 39.0, Lon: 32.0, Terrain: texture.TerrainForest, Width: 256, Height: 256},
	})
	w := doReq(t, s.Handler(), "GET", "/detections", nil)
	if w.Code != 200 {
		t.Errorf("/detections 200 beklenirken %d döndü", w.Code)
	}
}

func TestThreats(t *testing.T) {
	s := newTestServer()
	w := doReq(t, s.Handler(), "GET", "/detections/threats", nil)
	if w.Code != 200 {
		t.Errorf("/detections/threats 200 beklenirken %d döndü", w.Code)
	}
	w = doReq(t, s.Handler(), "GET", "/detections/threats?level=high", nil)
	if w.Code != 200 {
		t.Errorf("/detections/threats?level=high 200 beklenirken %d döndü", w.Code)
	}
}

func TestDetectionByIDNotFound(t *testing.T) {
	s := newTestServer()
	w := doReq(t, s.Handler(), "GET", "/detections/xxx", nil)
	if w.Code != 404 {
		t.Errorf("/detections/xxx 404 beklenirken %d döndü", w.Code)
	}
}

func TestProcess(t *testing.T) {
	s := newTestServer()
	inputs := []detector.ScanInput{
		{ID: "z1", Lat: 39.0, Lon: 32.0, Terrain: texture.TerrainForest, Width: 256, Height: 256},
		{ID: "z2", Lat: 39.1, Lon: 32.1, Terrain: texture.TerrainRocky, Width: 256, Height: 256},
	}
	w := doReq(t, s.Handler(), "POST", "/process", inputs)
	if w.Code != 200 {
		t.Errorf("/process 200 beklenirken %d döndü", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["pipeline"] == nil {
		t.Error("pipeline alanı eksik")
	}
}

func TestProcessMethodNotAllowed(t *testing.T) {
	s := newTestServer()
	w := doReq(t, s.Handler(), "GET", "/process", nil)
	if w.Code != 405 {
		t.Errorf("/process GET 405 beklenirken %d döndü", w.Code)
	}
}

func TestTerrains(t *testing.T) {
	s := newTestServer()
	w := doReq(t, s.Handler(), "GET", "/terrains", nil)
	if w.Code != 200 {
		t.Errorf("/terrains 200 beklenirken %d döndü", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	count, ok := body["count"].(float64)
	if !ok || count < 1 {
		t.Error("terrains count en az 1 olmalı")
	}
}

func TestStats(t *testing.T) {
	s := newTestServer()
	w := doReq(t, s.Handler(), "GET", "/stats", nil)
	if w.Code != 200 {
		t.Errorf("/stats 200 beklenirken %d döndü", w.Code)
	}
}
