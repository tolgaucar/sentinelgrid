package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otukenmatris/tutum/internal/alert"
	"github.com/otukenmatris/tutum/internal/battery"
	"github.com/otukenmatris/tutum/internal/predictor"
	"github.com/otukenmatris/tutum/internal/resource"
)

func newTestServer() *Server {
	mon := battery.NewMonitor(100)
	trk := resource.NewTracker(100, nil)
	pred := predictor.NewEngine(3, 50, 15.0)
	alm := alert.NewManager(100, 24*time.Hour, 0, true)
	return NewServer(mon, trk, pred, alm, 10, 25, 40, 65, 80)
}

func doReq(handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
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
	w := doReq(s.Handler(), "GET", "/health", nil)
	if w.Code != 200 {
		t.Fatalf("health status = %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["module"] != "TUTUM" {
		t.Fatal("wrong module name")
	}
}

func TestStatus(t *testing.T) {
	s := newTestServer()
	w := doReq(s.Handler(), "GET", "/status", nil)
	if w.Code != 200 {
		t.Fatalf("status code = %d", w.Code)
	}
}

func TestPlatformCRUD(t *testing.T) {
	s := newTestServer()
	h := s.Handler()

	// Create
	w := doReq(h, "POST", "/platforms", registerReq{
		ID: "iha-01", Name: "Kartal-1", Type: "iha", CapacityWh: 500,
	})
	if w.Code != 201 {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}

	// List
	w = doReq(h, "GET", "/platforms", nil)
	if w.Code != 200 {
		t.Fatalf("list status = %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["count"].(float64) != 1 {
		t.Fatal("should have 1 platform")
	}

	// Get by ID
	w = doReq(h, "GET", "/platforms/iha-01", nil)
	if w.Code != 200 {
		t.Fatalf("get status = %d", w.Code)
	}

	// Delete
	w = doReq(h, "DELETE", "/platforms/iha-01", nil)
	if w.Code != 200 {
		t.Fatalf("delete status = %d", w.Code)
	}

	// Get not found
	w = doReq(h, "GET", "/platforms/iha-01", nil)
	if w.Code != 404 {
		t.Fatal("should be 404 after delete")
	}
}

func TestBatterySample(t *testing.T) {
	s := newTestServer()
	h := s.Handler()

	// Register platform first
	doReq(h, "POST", "/platforms", registerReq{
		ID: "iha-01", Name: "Kartal", Type: "iha", CapacityWh: 100,
	})

	// Sample
	w := doReq(h, "POST", "/battery/sample", sampleReq{
		PlatformID: "iha-01", VoltageV: 11.1, CurrentA: -3.0, TempC: 30,
	})
	if w.Code != 200 {
		t.Fatalf("sample status = %d, body = %s", w.Code, w.Body.String())
	}

	// Unknown platform
	w = doReq(h, "POST", "/battery/sample", sampleReq{
		PlatformID: "nonexist", VoltageV: 11, CurrentA: -1, TempC: 25,
	})
	if w.Code != 404 {
		t.Fatal("should be 404")
	}
}

func TestBatteryStats(t *testing.T) {
	s := newTestServer()
	w := doReq(s.Handler(), "GET", "/battery/stats", nil)
	if w.Code != 200 {
		t.Fatal("stats failed")
	}
}

func TestBatteryCritical(t *testing.T) {
	s := newTestServer()
	w := doReq(s.Handler(), "GET", "/battery/critical", nil)
	if w.Code != 200 {
		t.Fatal("critical failed")
	}
}

func TestResourceMetric(t *testing.T) {
	s := newTestServer()
	h := s.Handler()
	cpu := 75.0
	mem := 60.0
	w := doReq(h, "POST", "/resource/metric", resourceReq{
		PlatformID: "iha-01", CPU: &cpu, Memory: &mem,
	})
	if w.Code != 200 {
		t.Fatalf("metric status = %d", w.Code)
	}
}

func TestResourceStats(t *testing.T) {
	s := newTestServer()
	w := doReq(s.Handler(), "GET", "/resource/stats", nil)
	if w.Code != 200 {
		t.Fatal("resource stats failed")
	}
}

func TestPredict_InsufficientData(t *testing.T) {
	s := newTestServer()
	h := s.Handler()
	w := doReq(h, "POST", "/predict", predictReq{PlatformID: "iha-01"})
	if w.Code != 422 {
		t.Fatalf("should be 422, got %d", w.Code)
	}
}

func TestMissionFeasibility(t *testing.T) {
	s := newTestServer()
	h := s.Handler()
	// Not enough samples
	w := doReq(h, "POST", "/predict/mission", missionReq{
		PlatformID: "iha-01", MissionMinutes: 30,
	})
	if w.Code != 422 {
		t.Fatalf("should be 422, got %d", w.Code)
	}
}

func TestAlerts(t *testing.T) {
	s := newTestServer()
	h := s.Handler()
	w := doReq(h, "GET", "/alerts", nil)
	if w.Code != 200 {
		t.Fatal("alerts failed")
	}
}

func TestAlertAckResolve(t *testing.T) {
	s := newTestServer()
	h := s.Handler()

	// Register and send critical sample
	doReq(h, "POST", "/platforms", registerReq{
		ID: "low", Name: "Low", Type: "iha", CapacityWh: 100,
	})
	p, _ := s.monitor.Get("low")
	p.RemainingWh = 5
	p.SoCPercent = 5
	// This should trigger a critical alert via battery sample
	doReq(h, "POST", "/battery/sample", sampleReq{
		PlatformID: "low", VoltageV: 3.0, CurrentA: -0.1, TempC: 25,
	})

	// Get alerts
	w := doReq(h, "GET", "/alerts", nil)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	alerts := resp["alerts"].([]any)
	if len(alerts) == 0 {
		t.Fatal("should have alerts")
	}

	alertID := alerts[0].(map[string]any)["id"].(string)

	// Acknowledge
	w = doReq(h, "POST", "/alerts/ack", ackReq{AlertID: alertID})
	if w.Code != 200 {
		t.Fatalf("ack status = %d", w.Code)
	}

	// Resolve
	w = doReq(h, "POST", "/alerts/resolve", ackReq{AlertID: alertID})
	if w.Code != 200 {
		t.Fatalf("resolve status = %d", w.Code)
	}
}

func TestAlertStats(t *testing.T) {
	s := newTestServer()
	w := doReq(s.Handler(), "GET", "/alerts/stats", nil)
	if w.Code != 200 {
		t.Fatal("alert stats failed")
	}
}

func TestRTB(t *testing.T) {
	s := newTestServer()
	w := doReq(s.Handler(), "GET", "/rtb", nil)
	if w.Code != 200 {
		t.Fatal("rtb failed")
	}
}

func TestHandoff(t *testing.T) {
	s := newTestServer()
	w := doReq(s.Handler(), "GET", "/handoff", nil)
	if w.Code != 200 {
		t.Fatal("handoff failed")
	}
}

func TestInvalidMethod(t *testing.T) {
	s := newTestServer()
	h := s.Handler()
	w := doReq(h, "PUT", "/platforms", nil)
	if w.Code != 405 {
		t.Fatalf("should be 405, got %d", w.Code)
	}
}
