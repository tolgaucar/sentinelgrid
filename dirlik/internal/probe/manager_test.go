package probe

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("manager nil")
	}
	if m.TargetCount() != 0 {
		t.Error("boş başlamalı")
	}
}

func TestAddRemoveTarget(t *testing.T) {
	m := NewManager()
	m.AddTarget(ProbeTarget{
		ServiceID: "test-svc",
		Type:      ProbeHTTP,
		Endpoint:  "http://localhost:9999/health",
	})

	if m.TargetCount() != 1 {
		t.Errorf("1 hedef beklendi, %d var", m.TargetCount())
	}

	m.RemoveTarget("test-svc")
	if m.TargetCount() != 0 {
		t.Error("hedef kaldırılmadı")
	}
}

func TestProbeHTTPHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	m := NewManager()
	m.AddTarget(ProbeTarget{
		ServiceID: "healthy",
		Type:      ProbeHTTP,
		Endpoint:  srv.URL + "/health",
		Timeout:   2 * time.Second,
		MaxFails:  3,
	})

	result, err := m.ProbeOnce("healthy")
	if err != nil {
		t.Fatalf("probe hatası: %v", err)
	}
	if result.Status != StatusHealthy {
		t.Errorf("HEALTHY beklendi, %s geldi", result.Status)
	}
	if result.LatencyMs <= 0 {
		t.Error("latency sıfır")
	}
	if result.Error != "" {
		t.Errorf("hata beklenmedi: %s", result.Error)
	}
}

func TestProbeHTTPDown(t *testing.T) {
	// Kapalı sunucu
	m := NewManager()
	m.AddTarget(ProbeTarget{
		ServiceID: "down",
		Type:      ProbeHTTP,
		Endpoint:  "http://127.0.0.1:1/health",
		Timeout:   500 * time.Millisecond,
		MaxFails:  2,
	})

	// İlk hata → DEGRADED
	r1, _ := m.ProbeOnce("down")
	if r1.Status != StatusDegraded {
		t.Errorf("1. hata: DEGRADED beklendi, %s geldi", r1.Status)
	}
	if r1.ConsecFail != 1 {
		t.Errorf("consec fail beklenen 1, aldık %d", r1.ConsecFail)
	}

	// İkinci hata → DOWN
	r2, _ := m.ProbeOnce("down")
	if r2.Status != StatusDown {
		t.Errorf("2. hata: DOWN beklendi, %s geldi", r2.Status)
	}
}

func TestProbeHTTPServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	m := NewManager()
	m.AddTarget(ProbeTarget{
		ServiceID: "errorsvc",
		Type:      ProbeHTTP,
		Endpoint:  srv.URL,
		Timeout:   2 * time.Second,
		MaxFails:  3,
	})

	result, _ := m.ProbeOnce("errorsvc")
	if result.Status == StatusHealthy {
		t.Error("500 hatası sonrası HEALTHY olmamalı")
	}
}

func TestProbeNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.ProbeOnce("nonexistent")
	if err == nil {
		t.Error("olmayan servis için hata beklendi")
	}
}

func TestGetState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	m := NewManager()
	m.AddTarget(ProbeTarget{
		ServiceID: "svc1",
		Type:      ProbeHTTP,
		Endpoint:  srv.URL,
		Timeout:   2 * time.Second,
	})

	m.ProbeOnce("svc1")

	state, ok := m.GetState("svc1")
	if !ok {
		t.Fatal("state bulunamadı")
	}
	if state.TotalChecks != 1 {
		t.Errorf("total checks beklenen 1, aldık %d", state.TotalChecks)
	}
	if state.Status != StatusHealthy {
		t.Errorf("status HEALTHY beklendi, %s geldi", state.Status)
	}
}

func TestAllStates(t *testing.T) {
	m := NewManager()
	m.AddTarget(ProbeTarget{ServiceID: "a", Type: ProbeHTTP, Endpoint: "http://x"})
	m.AddTarget(ProbeTarget{ServiceID: "b", Type: ProbeHTTP, Endpoint: "http://y"})

	states := m.AllStates()
	if len(states) != 2 {
		t.Errorf("2 state beklendi, %d geldi", len(states))
	}
}

func TestDownServices(t *testing.T) {
	m := NewManager()
	m.AddTarget(ProbeTarget{
		ServiceID: "crashsvc",
		Type:      ProbeHTTP,
		Endpoint:  "http://127.0.0.1:1",
		Timeout:   200 * time.Millisecond,
		MaxFails:  1,
	})

	m.ProbeOnce("crashsvc")

	down := m.DownServices()
	if len(down) != 1 {
		t.Errorf("1 çökmüş servis beklendi, %d geldi", len(down))
	}
	if down[0].ServiceID != "crashsvc" {
		t.Errorf("crashsvc beklendi, %s geldi", down[0].ServiceID)
	}
}

func TestRecovery(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			w.WriteHeader(500)
		} else {
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()

	m := NewManager()
	m.AddTarget(ProbeTarget{
		ServiceID: "recover",
		Type:      ProbeHTTP,
		Endpoint:  srv.URL,
		Timeout:   2 * time.Second,
		MaxFails:  3,
	})

	m.ProbeOnce("recover") // fail 1
	m.ProbeOnce("recover") // fail 2
	r3, _ := m.ProbeOnce("recover") // success → recovery

	if r3.Status != StatusHealthy {
		t.Errorf("recovery sonrası HEALTHY beklendi, %s geldi", r3.Status)
	}

	state, _ := m.GetState("recover")
	if state.ConsecFails != 0 {
		t.Errorf("recovery sonrası consec fails 0 beklendi, %d geldi", state.ConsecFails)
	}
}

func TestOnStatusChangeCallback(t *testing.T) {
	var changes []string
	m := NewManager()
	m.OnStatusChange(func(serviceID string, oldS, newS ServiceStatus) {
		changes = append(changes, serviceID+":"+newS.String())
	})

	m.AddTarget(ProbeTarget{
		ServiceID: "cb-svc",
		Type:      ProbeHTTP,
		Endpoint:  "http://127.0.0.1:1",
		Timeout:   200 * time.Millisecond,
		MaxFails:  1,
	})

	m.ProbeOnce("cb-svc")

	if len(changes) == 0 {
		t.Error("status change callback çağrılmadı")
	}
}

func TestStatusString(t *testing.T) {
	tests := []struct {
		s    ServiceStatus
		want string
	}{
		{StatusHealthy, "HEALTHY"},
		{StatusDegraded, "DEGRADED"},
		{StatusDown, "DOWN"},
		{StatusUnknown, "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Status(%d) = %s, beklenen %s", tt.s, got, tt.want)
		}
	}
}

func TestProbeTCP(t *testing.T) {
	// TCP test — HTTP server TCP olarak da erişilebilir
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	addr := srv.Listener.Addr().String()

	m := NewManager()
	m.AddTarget(ProbeTarget{
		ServiceID: "tcpsvc",
		Type:      ProbeTCP,
		Endpoint:  addr,
		Timeout:   2 * time.Second,
	})

	result, err := m.ProbeOnce("tcpsvc")
	if err != nil {
		t.Fatalf("TCP probe hatası: %v", err)
	}
	if result.Status != StatusHealthy {
		t.Errorf("TCP probe HEALTHY beklendi, %s geldi", result.Status)
	}
}

func TestUptimeCalculation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	m := NewManager()
	m.AddTarget(ProbeTarget{
		ServiceID: "uptime-svc",
		Type:      ProbeHTTP,
		Endpoint:  srv.URL,
		Timeout:   2 * time.Second,
	})

	for i := 0; i < 5; i++ {
		m.ProbeOnce("uptime-svc")
	}

	state, _ := m.GetState("uptime-svc")
	if state.Uptime != 100.0 {
		t.Errorf("uptime 100%% beklendi, %.1f%% geldi", state.Uptime)
	}
	if state.TotalChecks != 5 {
		t.Errorf("total checks 5 beklendi, %d geldi", state.TotalChecks)
	}
}
