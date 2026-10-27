package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/otukenmatris/izlek/internal/predictor"
	"github.com/otukenmatris/izlek/internal/tracker"
)

func newTestServer() *Server {
	tm := tracker.NewManager(1000, 100, 120.0, 3.0)
	pe := predictor.NewEngine(0.1, 0.5, 60, 600, 5, 15, 3)
	return New(tm, pe)
}

func TestHealth(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("health status = %d", w.Code)
	}
	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["module"] != "İZLEK" {
		t.Errorf("module = %v", body["module"])
	}
}

func TestTracksUpdateAndList(t *testing.T) {
	s := newTestServer()

	// Varlık güncelle
	entities := []tracker.EntityUpdate{
		{ID: "e1", Type: "tank", Faction: "enemy", Lat: 39.0, Lon: 32.0, Speed: 15},
		{ID: "e2", Type: "apc", Faction: "friendly", Lat: 39.1, Lon: 32.1, Speed: 20},
	}
	body, _ := json.Marshal(entities)
	req := httptest.NewRequest("POST", "/tracks/update", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("update status = %d, body = %s", w.Code, w.Body.String())
	}

	// Liste
	req = httptest.NewRequest("GET", "/tracks", nil)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("list status = %d", w.Code)
	}
	var listResp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&listResp)
	if listResp["count"].(float64) != 2 {
		t.Errorf("count = %v, beklenen 2", listResp["count"])
	}
}

func TestTrackByID(t *testing.T) {
	s := newTestServer()

	entities := []tracker.EntityUpdate{
		{ID: "tank-1", Type: "tank", Faction: "enemy", Lat: 39.0, Lon: 32.0},
	}
	body, _ := json.Marshal(entities)
	req := httptest.NewRequest("POST", "/tracks/update", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	// Belirli iz
	req = httptest.NewRequest("GET", "/tracks/tank-1", nil)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("track by id status = %d", w.Code)
	}

	// Olmayan iz
	req = httptest.NewRequest("GET", "/tracks/nonexist", nil)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("olmayan iz status = %d, beklenen 404", w.Code)
	}
}

func TestPredict(t *testing.T) {
	s := newTestServer()

	// En az 2 güncelleme gerekli
	for i := 0; i < 3; i++ {
		entities := []tracker.EntityUpdate{
			{ID: "p1", Type: "tank", Faction: "enemy", Lat: 39.0 + float64(i)*0.001, Lon: 32.0, Speed: 15, Heading: 0},
		}
		body, _ := json.Marshal(entities)
		req := httptest.NewRequest("POST", "/tracks/update", bytes.NewReader(body))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
	}

	// Tahmin
	predReq := map[string]interface{}{"entity_id": "p1", "horizon_sec": 30}
	body, _ := json.Marshal(predReq)
	req := httptest.NewRequest("POST", "/predict", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("predict status = %d, body = %s", w.Code, w.Body.String())
	}

	var pred predictor.Prediction
	json.NewDecoder(w.Body).Decode(&pred)
	if pred.EntityID != "p1" {
		t.Errorf("entityID = %s", pred.EntityID)
	}
	if len(pred.Points) < 1 {
		t.Error("tahmin noktası yok")
	}
}

func TestIntercept(t *testing.T) {
	s := newTestServer()

	for i := 0; i < 3; i++ {
		entities := []tracker.EntityUpdate{
			{ID: "i1", Type: "tank", Faction: "enemy", Lat: 39.1 + float64(i)*0.001, Lon: 32.0, Speed: 15},
		}
		body, _ := json.Marshal(entities)
		req := httptest.NewRequest("POST", "/tracks/update", bytes.NewReader(body))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
	}

	interceptReq := map[string]interface{}{
		"entity_id":        "i1",
		"projectile_speed": 800.0,
		"launcher_lat":     39.0,
		"launcher_lon":     32.0,
	}
	body, _ := json.Marshal(interceptReq)
	req := httptest.NewRequest("POST", "/intercept", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("intercept status = %d, body = %s", w.Code, w.Body.String())
	}

	var result predictor.InterceptResult
	json.NewDecoder(w.Body).Decode(&result)
	if result.EntityID != "i1" {
		t.Errorf("entityID = %s", result.EntityID)
	}
	if result.TimeSec <= 0 {
		t.Errorf("süre pozitif olmalı: %f", result.TimeSec)
	}
}

func TestStats(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("stats status = %d", w.Code)
	}
}

func TestDeleteTracks(t *testing.T) {
	s := newTestServer()

	entities := []tracker.EntityUpdate{
		{ID: "d1", Type: "tank", Faction: "enemy", Lat: 39.0, Lon: 32.0},
	}
	body, _ := json.Marshal(entities)
	req := httptest.NewRequest("POST", "/tracks/update", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	req = httptest.NewRequest("DELETE", "/tracks", nil)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("delete status = %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["cleared"].(float64) != 1 {
		t.Errorf("cleared = %v", resp["cleared"])
	}
}
