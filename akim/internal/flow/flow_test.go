package flow

import (
	"testing"

	"github.com/otukenmatris/akim/internal/config"
)

func testManager() *Manager {
	cfg := config.DefaultConfig()
	cfg.FlowTTL = 0 // test için hemen expire
	return NewManager(cfg)
}

func TestCreateFlow(t *testing.T) {
	m := NewManager(config.DefaultConfig())
	f, err := m.CreateFlow("A", "B", config.QoSTelemetry, 100.0, "sensor", []string{"A->B"})
	if err != nil {
		t.Fatalf("CreateFlow hatası: %v", err)
	}
	if f.From != "A" || f.To != "B" {
		t.Fatalf("yanlış from/to: %s → %s", f.From, f.To)
	}
	if !f.Active {
		t.Fatal("yeni akış aktif olmalı")
	}
	if f.AllocatedBW != 100.0 {
		t.Fatalf("beklenen BW: 100, alınan: %.1f", f.AllocatedBW)
	}
}

func TestGetFlow(t *testing.T) {
	m := NewManager(config.DefaultConfig())
	f, _ := m.CreateFlow("A", "B", config.QoSTelemetry, 100, "sensor", nil)
	got, ok := m.GetFlow(f.ID)
	if !ok {
		t.Fatal("akış bulunamadı")
	}
	if got.ID != f.ID {
		t.Fatalf("yanlış ID: %s", got.ID)
	}
}

func TestGetFlowNotFound(t *testing.T) {
	m := NewManager(config.DefaultConfig())
	_, ok := m.GetFlow("missing")
	if ok {
		t.Fatal("olmayan akış bulunmamalı")
	}
}

func TestUpdateFlow(t *testing.T) {
	m := NewManager(config.DefaultConfig())
	f, _ := m.CreateFlow("A", "B", config.QoSTelemetry, 100, "sensor", nil)
	err := m.UpdateFlow(f.ID, 1024, 10)
	if err != nil {
		t.Fatalf("UpdateFlow hatası: %v", err)
	}
	got, _ := m.GetFlow(f.ID)
	if got.BytesSent != 1024 {
		t.Fatalf("beklenen bytes: 1024, alınan: %d", got.BytesSent)
	}
	if got.PacketsSent != 10 {
		t.Fatalf("beklenen packets: 10, alınan: %d", got.PacketsSent)
	}
}

func TestCloseFlow(t *testing.T) {
	m := NewManager(config.DefaultConfig())
	f, _ := m.CreateFlow("A", "B", config.QoSTelemetry, 100, "sensor", nil)
	err := m.CloseFlow(f.ID)
	if err != nil {
		t.Fatalf("CloseFlow hatası: %v", err)
	}
	got, _ := m.GetFlow(f.ID)
	if got.Active {
		t.Fatal("akış kapalı olmalı")
	}
}

func TestListFlowsActiveOnly(t *testing.T) {
	m := NewManager(config.DefaultConfig())
	f1, _ := m.CreateFlow("A", "B", config.QoSTelemetry, 100, "sensor", nil)
	m.CreateFlow("A", "C", config.QoSCommand, 50, "command", nil)
	m.CloseFlow(f1.ID)

	all := m.ListFlows(false)
	if len(all) != 2 {
		t.Fatalf("beklenen: 2 akış, alınan: %d", len(all))
	}
	active := m.ListFlows(true)
	if len(active) != 1 {
		t.Fatalf("beklenen: 1 aktif akış, alınan: %d", len(active))
	}
}

func TestFlowsByLink(t *testing.T) {
	m := NewManager(config.DefaultConfig())
	m.CreateFlow("A", "B", config.QoSTelemetry, 100, "sensor", []string{"link1", "link2"})
	m.CreateFlow("C", "D", config.QoSCommand, 50, "command", []string{"link2", "link3"})
	m.CreateFlow("E", "F", config.QoSBulk, 30, "bulk", []string{"link4"})

	flows := m.FlowsByLink("link2")
	if len(flows) != 2 {
		t.Fatalf("beklenen: 2 akış, alınan: %d", len(flows))
	}
}

func TestFlowsByQoS(t *testing.T) {
	m := NewManager(config.DefaultConfig())
	m.CreateFlow("A", "B", config.QoSTelemetry, 100, "sensor", nil)
	m.CreateFlow("C", "D", config.QoSTelemetry, 50, "sensor", nil)
	m.CreateFlow("E", "F", config.QoSCritical, 200, "alert", nil)

	flows := m.FlowsByQoS(config.QoSTelemetry)
	if len(flows) != 2 {
		t.Fatalf("beklenen: 2 telemetry akış, alınan: %d", len(flows))
	}
}

func TestAllocateBandwidth(t *testing.T) {
	m := NewManager(config.DefaultConfig())
	m.CreateFlow("A", "B", config.QoSCritical, 200, "alert", nil)
	m.CreateFlow("C", "D", config.QoSBulk, 100, "bulk", nil)

	allocs := m.AllocateBandwidth(1000)
	if len(allocs) != 2 {
		t.Fatalf("beklenen: 2 tahsis, alınan: %d", len(allocs))
	}
	// Kritik sınıf %30 pay = 300 Mbps, talep 200 → 200 almalı
	// Bulk sınıf %7 pay = 70 Mbps, talep 100 → orantılı
	for _, v := range allocs {
		if v <= 0 {
			t.Fatal("tahsis > 0 olmalı")
		}
	}
}

func TestExpireFlows(t *testing.T) {
	m := testManager() // TTL = 0
	m.CreateFlow("A", "B", config.QoSTelemetry, 100, "sensor", nil)
	m.CreateFlow("C", "D", config.QoSCommand, 50, "command", nil)

	expired := m.ExpireFlows()
	if expired != 2 {
		t.Fatalf("beklenen: 2 expire, alınan: %d", expired)
	}
	active := m.ListFlows(true)
	if len(active) != 0 {
		t.Fatalf("hiç aktif akış kalmamalı: %d", len(active))
	}
}

func TestFlowStats(t *testing.T) {
	m := NewManager(config.DefaultConfig())
	m.CreateFlow("A", "B", config.QoSTelemetry, 100, "sensor", nil)
	m.CreateFlow("C", "D", config.QoSCommand, 50, "command", nil)

	stats := m.GetStats()
	if stats.TotalFlows != 2 {
		t.Fatalf("beklenen: 2 total, alınan: %d", stats.TotalFlows)
	}
	if stats.ActiveFlows != 2 {
		t.Fatalf("beklenen: 2 active, alınan: %d", stats.ActiveFlows)
	}
}

func TestFlowSummary(t *testing.T) {
	m := NewManager(config.DefaultConfig())
	m.CreateFlow("A", "B", config.QoSTelemetry, 100, "sensor", nil)
	m.CreateFlow("C", "D", config.QoSCritical, 200, "alert", nil)

	summary := m.Summary()
	if summary["active_flows"].(int64) != 2 {
		t.Fatalf("beklenen: 2 active, alınan: %v", summary["active_flows"])
	}
	byType := summary["by_type"].(map[string]int)
	if byType["sensor"] != 1 || byType["alert"] != 1 {
		t.Fatalf("yanlış tip dağılımı: %v", byType)
	}
}

func BenchmarkCreateFlow(b *testing.B) {
	m := NewManager(config.DefaultConfig())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.CreateFlow("A", "B", config.QoSTelemetry, 100, "sensor", nil)
	}
}
