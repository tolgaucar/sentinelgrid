package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/otukenmatris/yanki/internal/acoustic"
	"github.com/otukenmatris/yanki/internal/config"
)

func newTestServer() *Server {
	engine := acoustic.NewEngine(30, 10000, 20, 343, 3, 2, 0.1, 600, 500)
	cfg := config.Config{}
	return New(engine, cfg)
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
	if resp["module"] != "yanki" {
		t.Fatalf("expected yanki, got %v", resp["module"])
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
	if resp["module"] != "yanki" {
		t.Fatalf("expected yanki")
	}
}

func TestDetectEndpoint(t *testing.T) {
	s := newTestServer()
	body := map[string]any{
		"readings": []acoustic.SensorReading{
			{SensorID: "s1", Lat: 39.0, Lon: 35.0, FrequencyHz: 1500, AmplitudeDB: 145, DirectionDeg: 45, SignatureType: "gunshot", Confidence: 0.9},
		},
	}
	rr := doReq(t, s, "POST", "/detect", body)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestDetectMethodNotAllowed(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/detect", nil)
	if rr.Code != 405 {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestLocateEndpoint(t *testing.T) {
	s := newTestServer()
	body := map[string]any{
		"toas": []acoustic.TOAMeasurement{
			{SensorID: "s1", SensorLat: 39.0, SensorLon: 35.0, ArrivalNs: 1000, AmplitudeDB: 120, DirectionDeg: 45},
			{SensorID: "s2", SensorLat: 39.001, SensorLon: 35.001, ArrivalNs: 4000, AmplitudeDB: 115, DirectionDeg: 315},
			{SensorID: "s3", SensorLat: 39.002, SensorLon: 35.0, ArrivalNs: 7000, AmplitudeDB: 110, DirectionDeg: 0},
		},
	}
	rr := doReq(t, s, "POST", "/locate", body)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestLocateInsufficientSensors(t *testing.T) {
	s := newTestServer()
	body := map[string]any{
		"toas": []acoustic.TOAMeasurement{
			{SensorID: "s1", SensorLat: 39.0, SensorLon: 35.0, ArrivalNs: 1000, AmplitudeDB: 100, DirectionDeg: 0},
		},
	}
	rr := doReq(t, s, "POST", "/locate", body)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["status"] != "insufficient_sensors" {
		t.Fatalf("expected insufficient_sensors, got %v", resp["status"])
	}
}

func TestDetections(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/detections", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	count := resp["count"].(float64)
	if count != 0 {
		t.Fatalf("expected 0, got %v", count)
	}
}

func TestDetectionByIDNotFound(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/detections/nonexistent", nil)
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
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["count"].(float64) != 0 {
		t.Fatalf("expected 0 threats")
	}
}

func TestGunshots(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/gunshots", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestProcess(t *testing.T) {
	s := newTestServer()
	body := map[string]any{
		"entities": []map[string]any{
			{"id": "sensor-1", "lat": 39.0, "lon": 35.0, "frequency_hz": 200.0, "amplitude_db": 80.0, "direction_deg": 45.0, "signature_type": "tracked_vehicle", "confidence": 0.8},
		},
	}
	rr := doReq(t, s, "POST", "/process", body)
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
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["active_detections"].(float64) != 0 {
		t.Fatalf("expected 0")
	}
}

func TestPrune(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, "GET", "/prune", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}
