package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/otukenmatris/dirlik/internal/alerter"
	"github.com/otukenmatris/dirlik/internal/anomaly"
	"github.com/otukenmatris/dirlik/internal/collector"
	"github.com/otukenmatris/dirlik/internal/config"
	"github.com/otukenmatris/dirlik/internal/probe"
)

func setup() *Server {
	cfg := config.DefaultConfig()
	cfg.WindowSize = 10
	c := collector.New(time.Second, 100)
	d := anomaly.NewDetector(cfg)
	p := probe.NewManager()
	a := alerter.New("http://localhost:8000", "http://localhost:9091")
	return New(c, d, p, a)
}

func TestHealthEndpoint(t *testing.T) {
	s := setup()
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("health status 200 beklendi, %d geldi", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "healthy" {
		t.Errorf("status healthy beklendi, %v geldi", body["status"])
	}
	if body["service"] != "dirlik" {
		t.Errorf("service dirlik beklendi, %v geldi", body["service"])
	}
}

func TestStatusEndpoint(t *testing.T) {
	s := setup()
	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status code 200 beklendi, %d geldi", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["service"] != "dirlik" {
		t.Error("service name yanlış")
	}
	if _, ok := body["alerts"]; !ok {
		t.Error("alerts alanı eksik")
	}
}

func TestMetricsCurrentEndpoint(t *testing.T) {
	s := setup()
	req := httptest.NewRequest("GET", "/metrics/current", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("200 beklendi, %d geldi", w.Code)
	}

	var snap collector.SystemSnapshot
	json.Unmarshal(w.Body.Bytes(), &snap)
	if snap.Goroutines <= 0 {
		t.Error("goroutines > 0 beklendi")
	}
}

func TestMetricsHistoryEndpoint(t *testing.T) {
	s := setup()
	// Birkaç snapshot topla
	s.collector.Start()
	time.Sleep(200 * time.Millisecond)
	s.collector.Stop()

	req := httptest.NewRequest("GET", "/metrics/history?n=5", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("200 beklendi, %d geldi", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if _, ok := body["metrics"]; !ok {
		t.Error("metrics alanı eksik")
	}
}

func TestProbesEndpoints(t *testing.T) {
	s := setup()

	// POST probe ekle
	body := `{"service_id":"test","type":"http","endpoint":"http://localhost:1234/health"}`
	req := httptest.NewRequest("POST", "/probes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 201 {
		t.Errorf("POST /probes 201 beklendi, %d geldi", w.Code)
	}

	// GET tüm probe'lar
	req2 := httptest.NewRequest("GET", "/probes", nil)
	w2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(w2, req2)

	var probeList map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &probeList)
	if probeList["count"].(float64) != 1 {
		t.Errorf("1 probe beklendi, %v geldi", probeList["count"])
	}

	// GET tek probe
	req3 := httptest.NewRequest("GET", "/probes/test", nil)
	w3 := httptest.NewRecorder()
	s.Handler().ServeHTTP(w3, req3)

	if w3.Code != 200 {
		t.Errorf("GET /probes/test 200 beklendi, %d geldi", w3.Code)
	}

	// DELETE probe
	req4 := httptest.NewRequest("DELETE", "/probes/test", nil)
	w4 := httptest.NewRecorder()
	s.Handler().ServeHTTP(w4, req4)

	if w4.Code != 200 {
		t.Errorf("DELETE 200 beklendi, %d geldi", w4.Code)
	}
}

func TestProbeInvalidPost(t *testing.T) {
	s := setup()
	req := httptest.NewRequest("POST", "/probes", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("400 beklendi, %d geldi", w.Code)
	}
}

func TestAlertsEndpoint(t *testing.T) {
	s := setup()
	// Alarm üret
	s.detector.Analyze(collector.SystemSnapshot{
		CPUPercent: 95, MemPercent: 40, DiskPercent: 50, Timestamp: time.Now(),
	})

	req := httptest.NewRequest("GET", "/alerts?n=10", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("200 beklendi, %d geldi", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["count"].(float64) == 0 {
		t.Error("alarm beklendi")
	}
}

func TestAlertsSummaryEndpoint(t *testing.T) {
	s := setup()
	req := httptest.NewRequest("GET", "/alerts/summary", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("200 beklendi, %d geldi", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if _, ok := body["by_severity"]; !ok {
		t.Error("by_severity alanı eksik")
	}
}

func TestMetricsServicesEndpoint(t *testing.T) {
	s := setup()
	s.collector.RecordRequest("bag", 5.0, false)

	req := httptest.NewRequest("GET", "/metrics/services", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("200 beklendi, %d geldi", w.Code)
	}
}

func TestXServiceHeader(t *testing.T) {
	s := setup()
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Header().Get("X-Service") != "dirlik" {
		t.Error("X-Service header dirlik beklendi")
	}
}

func TestAlertRecordsEndpoint(t *testing.T) {
	s := setup()
	req := httptest.NewRequest("GET", "/alerts/records", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("200 beklendi, %d geldi", w.Code)
	}
}

func TestProbeNotFoundGet(t *testing.T) {
	s := setup()
	req := httptest.NewRequest("GET", "/probes/nonexistent", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("404 beklendi, %d geldi", w.Code)
	}
}
