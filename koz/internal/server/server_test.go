package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/otukenmatris/koz/internal/analyzer"
	"github.com/otukenmatris/koz/internal/thermal"
)

func newTestServer() *Server {
	eng := thermal.NewEngine(22.0, 2.0, 300, 640, 24.0, 60)
	az := analyzer.NewAnalyzer(500.0, 0.5)
	return New(eng, az)
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
	if body["module"] != "KÖZ" {
		t.Error("module=KÖZ beklenirken farklı değer döndü")
	}
}

func TestStatus(t *testing.T) {
	s := newTestServer()
	w := doReq(t, s.Handler(), "GET", "/status", nil)
	if w.Code != 200 {
		t.Errorf("/status 200 beklenirken %d döndü", w.Code)
	}
}

func TestScan(t *testing.T) {
	s := newTestServer()
	ents := []thermal.EntityInput{
		{ID: "t1", Type: "tank", Faction: "enemy", Status: "active", Lat: 39, Lon: 32, Speed: 30, Health: 100},
	}
	w := doReq(t, s.Handler(), "POST", "/scan", ents)
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

func TestSignatures(t *testing.T) {
	s := newTestServer()
	// önce tarama yap
	doReq(t, s.Handler(), "POST", "/scan", []thermal.EntityInput{
		{ID: "t1", Type: "tank", Status: "active", Lat: 39, Lon: 32, Speed: 30, Health: 100},
	})
	w := doReq(t, s.Handler(), "GET", "/signatures", nil)
	if w.Code != 200 {
		t.Errorf("/signatures 200 beklenirken %d döndü", w.Code)
	}
}

func TestHotSignatures(t *testing.T) {
	s := newTestServer()
	doReq(t, s.Handler(), "POST", "/scan", []thermal.EntityInput{
		{ID: "j1", Type: "fighter_jet", Status: "active", Lat: 39, Lon: 32, Speed: 500, Health: 100},
	})
	w := doReq(t, s.Handler(), "GET", "/signatures/hot?min_delta=10", nil)
	if w.Code != 200 {
		t.Errorf("/signatures/hot 200 beklenirken %d döndü", w.Code)
	}
}

func TestSignatureByID(t *testing.T) {
	s := newTestServer()
	doReq(t, s.Handler(), "POST", "/scan", []thermal.EntityInput{
		{ID: "t1", Type: "tank", Status: "active", Lat: 39, Lon: 32, Speed: 30, Health: 100},
	})
	w := doReq(t, s.Handler(), "GET", "/signatures/t1", nil)
	if w.Code != 200 {
		t.Errorf("/signatures/t1 200 beklenirken %d döndü", w.Code)
	}
}

func TestSignatureByIDNotFound(t *testing.T) {
	s := newTestServer()
	w := doReq(t, s.Handler(), "GET", "/signatures/xxx", nil)
	if w.Code != 404 {
		t.Errorf("/signatures/xxx 404 beklenirken %d döndü", w.Code)
	}
}

func TestClusters(t *testing.T) {
	s := newTestServer()
	doReq(t, s.Handler(), "POST", "/scan", []thermal.EntityInput{
		{ID: "a", Type: "tank", Status: "active", Lat: 39.0000, Lon: 32.0000, Speed: 30, Health: 100},
		{ID: "b", Type: "apc", Status: "active", Lat: 39.0001, Lon: 32.0001, Speed: 20, Health: 100},
	})
	w := doReq(t, s.Handler(), "GET", "/analyze/clusters", nil)
	if w.Code != 200 {
		t.Errorf("/analyze/clusters 200 beklenirken %d döndü", w.Code)
	}
}

func TestCooling(t *testing.T) {
	s := newTestServer()
	doReq(t, s.Handler(), "POST", "/scan", []thermal.EntityInput{
		{ID: "t1", Type: "tank", Status: "active", Lat: 39, Lon: 32, Speed: 30, Health: 100},
	})
	w := doReq(t, s.Handler(), "GET", "/analyze/cooling", nil)
	if w.Code != 200 {
		t.Errorf("/analyze/cooling 200 beklenirken %d döndü", w.Code)
	}
}

func TestAnomaliesGetPost(t *testing.T) {
	s := newTestServer()
	doReq(t, s.Handler(), "POST", "/scan", []thermal.EntityInput{
		{ID: "t1", Type: "tank", Status: "active", Lat: 39, Lon: 32, Speed: 30, Health: 100},
	})
	// POST: anomali tespit
	w := doReq(t, s.Handler(), "POST", "/analyze/anomalies", nil)
	if w.Code != 200 {
		t.Errorf("POST /analyze/anomalies 200 beklenirken %d döndü", w.Code)
	}
	// GET: anomalileri listele
	w = doReq(t, s.Handler(), "GET", "/analyze/anomalies?n=10", nil)
	if w.Code != 200 {
		t.Errorf("GET /analyze/anomalies 200 beklenirken %d döndü", w.Code)
	}
}

func TestProcess(t *testing.T) {
	s := newTestServer()
	ents := []thermal.EntityInput{
		{ID: "a", Type: "tank", Status: "active", Lat: 39.0, Lon: 32.0, Speed: 30, Health: 100},
		{ID: "b", Type: "apc", Status: "active", Lat: 39.001, Lon: 32.001, Speed: 20, Health: 80},
		{ID: "c", Type: "fighter_jet", Status: "active", Lat: 39.5, Lon: 32.5, Speed: 500, Health: 100},
	}
	w := doReq(t, s.Handler(), "POST", "/process", ents)
	if w.Code != 200 {
		t.Errorf("/process 200 beklenirken %d döndü", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["pipeline"] == nil {
		t.Error("pipeline alanı eksik")
	}
}

func TestAmbientGetPut(t *testing.T) {
	s := newTestServer()
	// GET
	w := doReq(t, s.Handler(), "GET", "/ambient", nil)
	if w.Code != 200 {
		t.Errorf("GET /ambient 200 beklenirken %d döndü", w.Code)
	}
	// PUT
	w = doReq(t, s.Handler(), "PUT", "/ambient", map[string]float64{"temp_c": 35.0})
	if w.Code != 200 {
		t.Errorf("PUT /ambient 200 beklenirken %d döndü", w.Code)
	}
}

func TestStats(t *testing.T) {
	s := newTestServer()
	w := doReq(t, s.Handler(), "GET", "/stats", nil)
	if w.Code != 200 {
		t.Errorf("/stats 200 beklenirken %d döndü", w.Code)
	}
}
