package alerter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otukenmatris/dirlik/internal/anomaly"
	"github.com/otukenmatris/dirlik/internal/probe"
)

func TestNew(t *testing.T) {
	a := New("http://localhost:8000", "http://localhost:9091")
	if a == nil {
		t.Fatal("alerter nil")
	}
}

func TestHandleAnomaly(t *testing.T) {
	a := New("http://localhost:8000", "http://localhost:9091")
	alert := anomaly.Alert{
		ID:        "TEST-001",
		Metric:    "cpu_percent",
		Value:     95,
		Threshold: 85,
		Severity:  anomaly.SeverityWarning,
		Message:   "CPU yüksek",
		Timestamp: time.Now(),
	}

	a.HandleAnomaly(alert)
	time.Sleep(100 * time.Millisecond)

	records := a.Records(10)
	if len(records) == 0 {
		t.Error("alarm kaydı beklendi")
	}
	if records[0].Alert.ID != "TEST-001" {
		t.Errorf("alert ID TEST-001 beklendi, %s geldi", records[0].Alert.ID)
	}
}

func TestHandleAnomalyCooldown(t *testing.T) {
	a := New("http://localhost:8000", "http://localhost:9091")
	a.cooldownD = 1 * time.Second

	alert := anomaly.Alert{
		ID:       "CD-001",
		Metric:   "cpu_percent",
		Severity: anomaly.SeverityWarning,
	}

	a.HandleAnomaly(alert)
	a.HandleAnomaly(alert) // cooldown içinde → atlanmalı

	time.Sleep(100 * time.Millisecond)

	records := a.Records(10)
	if len(records) != 1 {
		t.Errorf("cooldown ile 1 kayıt beklendi, %d geldi", len(records))
	}
}

func TestHandleServiceDown(t *testing.T) {
	received := false
	butunSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(200)
	}))
	defer butunSrv.Close()

	a := New(butunSrv.URL, "http://localhost:9091")
	a.HandleServiceDown("bag", probe.StatusHealthy, probe.StatusDown)
	time.Sleep(200 * time.Millisecond)

	if !received {
		t.Error("BÜTÜN'e bildirim gönderilmedi")
	}

	records := a.Records(10)
	if len(records) == 0 {
		t.Error("service down kaydı beklendi")
	}
	if records[0].Action != ActionRestart {
		t.Errorf("RESTART aksiyonu beklendi, %s geldi", records[0].Action)
	}
}

func TestHandleServiceNotDown(t *testing.T) {
	a := New("http://localhost:8000", "http://localhost:9091")
	a.HandleServiceDown("bag", probe.StatusUnknown, probe.StatusHealthy)
	time.Sleep(50 * time.Millisecond)

	records := a.Records(10)
	if len(records) != 0 {
		t.Error("healthy durumda kayıt beklenmedi")
	}
}

func TestStats(t *testing.T) {
	a := New("http://localhost:8000", "http://localhost:9091")
	a.HandleAnomaly(anomaly.Alert{
		ID:       "ST-001",
		Metric:   "test",
		Severity: anomaly.SeverityInfo,
	})
	time.Sleep(100 * time.Millisecond)

	stats := a.Stats()
	if stats["sent"] != 1 {
		t.Errorf("sent 1 beklendi, %d geldi", stats["sent"])
	}
}

func TestCriticalAction(t *testing.T) {
	a := New("http://localhost:8000", "http://localhost:9091")
	alert := anomaly.Alert{
		ID:        "CRIT-001",
		Metric:    "service_down",
		Severity:  anomaly.SeverityCritical,
		ServiceID: "kutuk",
	}

	a.HandleAnomaly(alert)
	time.Sleep(100 * time.Millisecond)

	records := a.Records(10)
	if len(records) == 0 {
		t.Fatal("kayıt beklendi")
	}
	if records[0].Action != ActionRestart {
		t.Errorf("critical + service_id → RESTART beklendi, %s geldi", records[0].Action)
	}
}
