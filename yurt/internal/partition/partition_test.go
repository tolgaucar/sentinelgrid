package partition

import (
	"testing"
	"time"

	"github.com/otukenmatris/yurt/internal/config"
)

func testManager() *Manager {
	cfg := config.DefaultConfig()
	cfg.PartitionTimeout = 50 * time.Millisecond
	cfg.HeartbeatFreq = 10 * time.Millisecond
	cfg.QuorumSize = 2
	return NewManager(cfg)
}

func TestAddRemovePeer(t *testing.T) {
	m := testManager()
	m.AddPeer("cloud", "http://cloud:8000", "cloud")
	m.AddPeer("relay", "http://relay:9000", "relay")

	peers := m.ListPeers()
	if len(peers) != 2 {
		t.Fatalf("beklenen: 2 peer, alınan: %d", len(peers))
	}

	m.RemovePeer("relay")
	peers = m.ListPeers()
	if len(peers) != 1 {
		t.Fatalf("remove sonrası: 1, alınan: %d", len(peers))
	}
}

func TestRecordHeartbeat(t *testing.T) {
	m := testManager()
	m.AddPeer("cloud", "http://cloud:8000", "cloud")
	m.RecordHeartbeat("cloud", 5.0)

	peers := m.ListPeers()
	if !peers[0].Alive {
		t.Fatal("heartbeat sonrası alive olmalı")
	}
	if peers[0].Latency != 5.0 {
		t.Fatalf("latency: %f", peers[0].Latency)
	}
}

func TestCheckPartitionNormal(t *testing.T) {
	m := testManager()
	m.AddPeer("a", "addr", "cloud")
	m.AddPeer("b", "addr", "relay")
	m.RecordHeartbeat("a", 1)
	m.RecordHeartbeat("b", 1)

	state := m.CheckPartition()
	if state != StateNormal {
		t.Fatalf("beklenen: NORMAL, alınan: %s", state)
	}
}

func TestCheckPartitionPartitioned(t *testing.T) {
	m := testManager()
	m.AddPeer("a", "addr", "cloud")
	m.AddPeer("b", "addr", "relay")
	m.AddPeer("c", "addr", "edge")

	time.Sleep(60 * time.Millisecond) // timeout aşılsın (c ölsün)
	// a ve b'ye taze heartbeat gönder
	m.RecordHeartbeat("a", 1)
	m.RecordHeartbeat("b", 1)

	state := m.CheckPartition()
	// a ve b alive (2 >= quorum), c dead → PARTITIONED
	if state != StatePartitioned {
		t.Fatalf("beklenen: PARTITIONED, alınan: %s", state)
	}
}

func TestCheckPartitionSplitBrain(t *testing.T) {
	m := testManager()
	m.AddPeer("a", "addr", "cloud")
	m.AddPeer("b", "addr", "relay")
	m.AddPeer("c", "addr", "edge")
	// Hiçbirine güncel heartbeat yok ama a alive
	m.RecordHeartbeat("a", 1)

	time.Sleep(60 * time.Millisecond)
	m.RecordHeartbeat("a", 1) // sadece a güncel

	state := m.CheckPartition()
	// alive=1 < quorum=2 → SPLIT_BRAIN
	if state != StateSplitBrain {
		t.Fatalf("beklenen: SPLIT_BRAIN, alınan: %s", state)
	}
}

func TestCheckPartitionIsolated(t *testing.T) {
	m := testManager()
	m.AddPeer("a", "addr", "cloud")
	m.AddPeer("b", "addr", "relay")
	// Hiçbirine heartbeat yok

	time.Sleep(60 * time.Millisecond)
	state := m.CheckPartition()
	if state != StateIsolated {
		t.Fatalf("beklenen: ISOLATED, alınan: %s", state)
	}
}

func TestPartitionEvents(t *testing.T) {
	m := testManager()
	m.AddPeer("a", "addr", "cloud")
	time.Sleep(60 * time.Millisecond)
	m.CheckPartition()

	events := m.GetEvents(10)
	if len(events) == 0 {
		t.Fatal("olay üretilmemiş")
	}
}

func TestPartitionResolved(t *testing.T) {
	m := testManager()
	m.AddPeer("a", "addr", "cloud")
	time.Sleep(60 * time.Millisecond)
	m.CheckPartition() // ISOLATED

	// Heartbeat gelsin
	m.RecordHeartbeat("a", 1)
	state := m.CheckPartition() // NORMAL
	if state != StateNormal {
		t.Fatalf("beklenen: NORMAL (resolved), alınan: %s", state)
	}

	stats := m.GetStats()
	if stats.PartitionsResolved == 0 {
		t.Fatal("resolved sayılmamış")
	}
}

func TestAliveCount(t *testing.T) {
	m := testManager()
	m.AddPeer("a", "addr", "cloud")
	m.AddPeer("b", "addr", "relay")
	m.RecordHeartbeat("a", 1)
	// b'ye heartbeat yok, ama henüz timeout olmadı
	if m.AliveCount() != 2 {
		t.Fatalf("beklenen: 2, alınan: %d", m.AliveCount())
	}
}

func TestPartitionStartStop(t *testing.T) {
	m := testManager()
	m.AddPeer("a", "addr", "cloud")
	m.Start()
	time.Sleep(30 * time.Millisecond)
	m.Stop()

	stats := m.GetStats()
	if stats.PeerChecks == 0 {
		t.Fatal("periyodik check çalışmamış")
	}
}

func TestGetState(t *testing.T) {
	m := testManager()
	if m.GetState() != StateNormal {
		t.Fatal("başlangıç NORMAL olmalı")
	}
}

func TestSummary(t *testing.T) {
	m := testManager()
	m.AddPeer("a", "addr", "cloud")
	summary := m.Summary()
	if summary["total_peers"].(int) != 1 {
		t.Fatalf("total_peers: %v", summary["total_peers"])
	}
}

func TestStateStrings(t *testing.T) {
	tests := []struct {
		s State
		e string
	}{
		{StateNormal, "NORMAL"},
		{StatePartitioned, "PARTITIONED"},
		{StateSplitBrain, "SPLIT_BRAIN"},
		{StateIsolated, "ISOLATED"},
	}
	for _, tc := range tests {
		if tc.s.String() != tc.e {
			t.Fatalf("beklenen: %s, alınan: %s", tc.e, tc.s.String())
		}
	}
}

func TestNoPeersNormal(t *testing.T) {
	m := testManager()
	state := m.CheckPartition()
	if state != StateNormal {
		t.Fatalf("düğüm yokken NORMAL olmalı: %s", state)
	}
}
