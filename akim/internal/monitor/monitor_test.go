package monitor

import (
	"testing"
	"time"

	"github.com/otukenmatris/akim/internal/config"
	"github.com/otukenmatris/akim/internal/router"
)

func testMonitor() (*Monitor, *router.Engine) {
	cfg := config.DefaultConfig()
	cfg.MonitorInterval = 10 * time.Millisecond
	eng := router.New(cfg)

	eng.AddNode(router.Node{ID: "A", Capacity: 10000})
	eng.AddNode(router.Node{ID: "B", Capacity: 5000})
	eng.AddNode(router.Node{ID: "C", Capacity: 3000})
	eng.AddLink(router.Link{From: "A", To: "B", Bandwidth: 1000, Latency: 2.0})
	eng.AddLink(router.Link{From: "B", To: "C", Bandwidth: 500, Latency: 3.0})

	mon := New(eng, cfg)
	return mon, eng
}

func TestMonitorScan(t *testing.T) {
	mon, _ := testMonitor()
	mon.scan()

	snaps := mon.AllSnapshots()
	if len(snaps) == 0 {
		t.Fatal("snapshot boş")
	}
	for _, s := range snaps {
		if s.Status != "normal" {
			t.Fatalf("boş link normal olmalı: %s = %s", s.LinkID, s.Status)
		}
	}
}

func TestMonitorCongestionWarning(t *testing.T) {
	mon, eng := testMonitor()
	eng.UpdateLinkTraffic("A->B", 800) // 800/1000 = %80 → warning
	mon.scan()

	snap, ok := mon.Snapshot("A->B")
	if !ok {
		t.Fatal("snapshot bulunamadı")
	}
	if snap.Status != "warning" {
		t.Fatalf("beklenen: warning, alınan: %s (util: %.1f%%)", snap.Status, snap.Utilization)
	}
}

func TestMonitorCongestionCritical(t *testing.T) {
	mon, eng := testMonitor()
	eng.UpdateLinkTraffic("A->B", 920) // %92 → critical
	mon.scan()

	snap, ok := mon.Snapshot("A->B")
	if !ok {
		t.Fatal("snapshot bulunamadı")
	}
	if snap.Status != "critical" {
		t.Fatalf("beklenen: critical, alınan: %s", snap.Status)
	}
}

func TestMonitorCongestionBlocked(t *testing.T) {
	mon, eng := testMonitor()
	eng.UpdateLinkTraffic("A->B", 990) // %99 → blocked
	mon.scan()

	snap, ok := mon.Snapshot("A->B")
	if !ok {
		t.Fatal("snapshot bulunamadı")
	}
	if snap.Status != "blocked" {
		t.Fatalf("beklenen: blocked, alınan: %s", snap.Status)
	}
}

func TestMonitorAlerts(t *testing.T) {
	mon, eng := testMonitor()
	eng.UpdateLinkTraffic("A->B", 950) // critical
	mon.scan()

	alerts := mon.GetAlerts(10)
	if len(alerts) == 0 {
		t.Fatal("alarm üretilmemiş")
	}
	if alerts[0].Level != "critical" {
		t.Fatalf("beklenen: critical alarm, alınan: %s", alerts[0].Level)
	}
}

func TestMonitorStartStop(t *testing.T) {
	mon, _ := testMonitor()
	mon.Start()
	time.Sleep(50 * time.Millisecond)
	mon.Stop()
	time.Sleep(20 * time.Millisecond)

	stats := mon.GetStats()
	if stats.ScanCycles == 0 {
		t.Fatal("scan döngüsü çalışmamış")
	}
}

func TestMonitorLinkHistory(t *testing.T) {
	mon, _ := testMonitor()
	mon.scan()
	mon.scan()
	mon.scan()

	hist := mon.LinkHistory("A->B", 10)
	if len(hist) != 3 {
		t.Fatalf("beklenen: 3 snapshot, alınan: %d", len(hist))
	}
}

func TestMonitorCongestionReport(t *testing.T) {
	mon, eng := testMonitor()
	eng.UpdateLinkTraffic("A->B", 850) // warning
	mon.scan()

	report := mon.CongestionReport()
	congested := report["congested_links"].(int)
	if congested == 0 {
		t.Fatal("tıkanık link tespit edilmemiş")
	}
}

func TestMonitorStats(t *testing.T) {
	mon, eng := testMonitor()
	eng.UpdateLinkTraffic("A->B", 950)
	mon.scan()

	stats := mon.GetStats()
	if stats.SnapshotsCollected == 0 {
		t.Fatal("snapshot toplanmamış")
	}
	if stats.AlertsGenerated == 0 {
		t.Fatal("alarm üretilmemiş")
	}
}

func TestMonitorSnapshotNotFound(t *testing.T) {
	mon, _ := testMonitor()
	_, ok := mon.Snapshot("nonexistent")
	if ok {
		t.Fatal("olmayan link snapshot döndürmemeli")
	}
}

func TestMonitorEmptyHistory(t *testing.T) {
	mon, _ := testMonitor()
	hist := mon.LinkHistory("nonexistent", 10)
	if hist != nil {
		t.Fatal("olmayan link geçmişi nil olmalı")
	}
}
