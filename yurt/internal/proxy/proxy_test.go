package proxy

import (
	"testing"

	"github.com/otukenmatris/yurt/internal/config"
)

func testProxy() *HubProxy {
	cfg := config.DefaultConfig()
	cfg.BufferSize = 10
	cfg.MaxRetries = 3
	return NewHub(cfg)
}

func TestHandleRequestOnline(t *testing.T) {
	h := testProxy()
	id, err := h.HandleRequest("GET", "/api/data", nil, 3)
	if err != nil {
		t.Fatalf("online request hatası: %v", err)
	}
	if id == "" {
		t.Fatal("request ID boş")
	}
	if h.BufferSize() != 0 {
		t.Fatal("online modda buffer'a eklenmemeli")
	}
}

func TestHandleRequestOffline(t *testing.T) {
	h := testProxy()
	h.SetConnectionState(config.ConnOffline)

	id, err := h.HandleRequest("POST", "/api/sensor", map[string]interface{}{"x": 1}, 2)
	if err != nil {
		t.Fatalf("offline request hatası: %v", err)
	}
	if id == "" {
		t.Fatal("request ID boş")
	}
	if h.BufferSize() != 1 {
		t.Fatalf("beklenen: 1 buffered, alınan: %d", h.BufferSize())
	}
}

func TestBufferFull(t *testing.T) {
	h := testProxy() // maxBuffer=10
	h.SetConnectionState(config.ConnOffline)

	for i := 0; i < 10; i++ {
		h.HandleRequest("GET", "/api", nil, 5) // düşük öncelik
	}

	// Yüksek öncelikli istek — düşük önceliklisini çıkarır
	_, err := h.HandleRequest("GET", "/api/critical", nil, 0)
	if err != nil {
		t.Fatalf("yüksek öncelik reddedilmemeli: %v", err)
	}
	if h.BufferSize() != 10 {
		t.Fatalf("buffer boyutu 10 kalmalı: %d", h.BufferSize())
	}
}

func TestBufferFullLowPriorityRejected(t *testing.T) {
	h := testProxy()
	h.SetConnectionState(config.ConnOffline)

	for i := 0; i < 10; i++ {
		h.HandleRequest("GET", "/api", nil, 0) // yüksek öncelik
	}

	// Düşük öncelikli istek — reddedilmeli
	_, err := h.HandleRequest("GET", "/api/low", nil, 5)
	if err == nil {
		t.Fatal("düşük öncelik reddedilmeli")
	}
}

func TestReplay(t *testing.T) {
	h := testProxy()
	h.SetConnectionState(config.ConnOffline)

	h.HandleRequest("POST", "/a", nil, 1)
	h.HandleRequest("POST", "/b", nil, 2)
	h.HandleRequest("POST", "/c", nil, 0)

	// Bağlantı gel
	h.SetConnectionState(config.ConnOnline)
	replayed := h.Replay()
	if replayed != 3 {
		t.Fatalf("beklenen: 3 replay, alınan: %d", replayed)
	}
}

func TestReplayOffline(t *testing.T) {
	h := testProxy()
	h.SetConnectionState(config.ConnOffline)
	h.HandleRequest("POST", "/a", nil, 1)

	// Hala offline — replay yapılmamalı
	replayed := h.Replay()
	if replayed != 0 {
		t.Fatalf("offline iken replay 0 olmalı: %d", replayed)
	}
}

func TestSetConnectionState(t *testing.T) {
	h := testProxy()
	if h.GetConnectionState() != config.ConnOnline {
		t.Fatal("başlangıç online olmalı")
	}
	h.SetConnectionState(config.ConnOffline)
	if h.GetConnectionState() != config.ConnOffline {
		t.Fatal("offline olmalı")
	}
	h.SetConnectionState(config.ConnDegraded)
	if h.GetConnectionState() != config.ConnDegraded {
		t.Fatal("degraded olmalı")
	}
}

func TestGetBuffer(t *testing.T) {
	h := testProxy()
	h.SetConnectionState(config.ConnOffline)
	h.HandleRequest("GET", "/a", nil, 1)
	h.HandleRequest("GET", "/b", nil, 2)

	buf := h.GetBuffer()
	if len(buf) != 2 {
		t.Fatalf("beklenen: 2, alınan: %d", len(buf))
	}
}

func TestClearBuffer(t *testing.T) {
	h := testProxy()
	h.SetConnectionState(config.ConnOffline)
	h.HandleRequest("GET", "/a", nil, 1)
	n := h.ClearBuffer()
	if n != 1 {
		t.Fatalf("beklenen: 1 temizlendi, alınan: %d", n)
	}
	if h.BufferSize() != 0 {
		t.Fatal("buffer boş olmalı")
	}
}

func TestHealthCheck(t *testing.T) {
	h := testProxy()
	if !h.HealthCheck() {
		t.Fatal("online iken healthy olmalı")
	}
	h.SetConnectionState(config.ConnOffline)
	if h.HealthCheck() {
		t.Fatal("offline iken unhealthy olmalı")
	}
}

func TestProxyStats(t *testing.T) {
	h := testProxy()
	h.HandleRequest("GET", "/a", nil, 1)
	h.HandleRequest("GET", "/b", nil, 2)

	stats := h.GetStats()
	if stats.TotalReceived != 2 {
		t.Fatalf("total_received: %d", stats.TotalReceived)
	}
	if stats.TotalForwarded != 2 {
		t.Fatalf("total_forwarded: %d", stats.TotalForwarded)
	}
}

func TestProxySummary(t *testing.T) {
	h := testProxy()
	summary := h.Summary()
	if summary["connection"] != "ONLINE" {
		t.Fatalf("connection: %v", summary["connection"])
	}
}

func TestSortByPriority(t *testing.T) {
	reqs := []Request{
		{Priority: 5}, {Priority: 0}, {Priority: 3}, {Priority: 1},
	}
	sortByPriority(reqs)
	if reqs[0].Priority != 0 || reqs[1].Priority != 1 || reqs[2].Priority != 3 || reqs[3].Priority != 5 {
		t.Fatalf("sıralama yanlış: %v", reqs)
	}
}
