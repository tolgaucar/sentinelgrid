package sync

import (
	"testing"
	"time"

	"github.com/otukenmatris/yurt/internal/config"
	"github.com/otukenmatris/yurt/internal/store"
)

func testEngine() (*Engine, *store.Store, *store.Store) {
	cfg := config.DefaultConfig()
	cfg.NodeID = "edge-test"
	cfg.SyncBatchSize = 100
	local := store.New("edge-test", 10000, time.Hour)
	eng := New(local, cfg)
	return eng, local, eng.remoteStore
}

func TestPushEmpty(t *testing.T) {
	eng, _, _ := testEngine()
	n, err := eng.Push()
	if err != nil {
		t.Fatalf("Push hatası: %v", err)
	}
	if n != 0 {
		t.Fatalf("boş push 0 olmalı: %d", n)
	}
}

func TestPushAndPull(t *testing.T) {
	eng, local, remote := testEngine()

	// Yerel depoya yaz
	local.Put("sensor/radar", map[string]interface{}{"range": 100})
	local.Put("entity/tank", map[string]interface{}{"speed": 60})

	// Push
	pushed, err := eng.Push()
	if err != nil {
		t.Fatalf("Push hatası: %v", err)
	}
	if pushed != 2 {
		t.Fatalf("beklenen: 2 push, alınan: %d", pushed)
	}

	// Dirty temizlendi mi?
	dirty := local.GetDirty(10)
	if len(dirty) != 0 {
		t.Fatalf("push sonrası dirty kalmamalı: %d", len(dirty))
	}

	// Remote'da var mı?
	_, ok := remote.Get("sensor/radar")
	if !ok {
		t.Fatal("remote'da sensor/radar bulunamadı")
	}
}

func TestPullNewEntries(t *testing.T) {
	eng, local, remote := testEngine()

	// Remote'a doğrudan yaz
	remote.PutWithVersion("cloud/data", map[string]interface{}{"source": "cloud"}, store.VectorClock{"cloud": 1}, "cloud", time.Now())

	// Pull
	pulled, err := eng.Pull()
	if err != nil {
		t.Fatalf("Pull hatası: %v", err)
	}
	if pulled != 1 {
		t.Fatalf("beklenen: 1 pull, alınan: %d", pulled)
	}

	// Local'da var mı?
	entry, ok := local.Get("cloud/data")
	if !ok {
		t.Fatal("pull sonrası local'da bulunamadı")
	}
	if entry.Value["source"] != "cloud" {
		t.Fatalf("yanlış value: %v", entry.Value)
	}
}

func TestFullSync(t *testing.T) {
	eng, local, remote := testEngine()

	local.Put("local/a", map[string]interface{}{"from": "edge"})
	remote.PutWithVersion("remote/b", map[string]interface{}{"from": "cloud"}, store.VectorClock{"cloud": 1}, "cloud", time.Now())

	pushed, pulled, err := eng.Sync()
	if err != nil {
		t.Fatalf("Sync hatası: %v", err)
	}
	if pushed != 1 {
		t.Fatalf("push: %d", pushed)
	}
	if pulled != 1 {
		t.Fatalf("pull: %d", pulled)
	}
}

func TestConflictLWW(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ConflictPolicy = config.LWW
	cfg.NodeID = "edge-test"
	local := store.New("edge-test", 10000, time.Hour)
	eng := New(local, cfg)

	// Local yaz (daha yeni)
	local.Put("conflict-key", map[string]interface{}{"winner": "local"})
	time.Sleep(time.Millisecond)

	// Remote'a eski versiyon yaz (eşzamanlı - farklı düğüm)
	eng.remoteStore.PutWithVersion("conflict-key",
		map[string]interface{}{"winner": "remote"},
		store.VectorClock{"cloud": 1},
		"cloud",
		time.Now().Add(-time.Second), // daha eski
	)

	eng.Push()

	// LWW: local daha yeni, remote'da local'ın verisi olmalı
	entry, ok := eng.remoteStore.Get("conflict-key")
	if !ok {
		t.Fatal("conflict-key remote'da bulunamadı")
	}
	if entry.Value["winner"] != "local" {
		t.Fatalf("LWW: local kazanmalı, alınan: %v", entry.Value["winner"])
	}
}

func TestConflictCloudWins(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ConflictPolicy = config.CloudWins
	cfg.NodeID = "edge-test"
	local := store.New("edge-test", 10000, time.Hour)
	eng := New(local, cfg)

	local.Put("ck", map[string]interface{}{"v": "local"})
	eng.remoteStore.PutWithVersion("ck",
		map[string]interface{}{"v": "cloud"},
		store.VectorClock{"cloud": 1},
		"cloud", time.Now(),
	)

	eng.Push()

	entry, _ := eng.remoteStore.Get("ck")
	if entry.Value["v"] != "cloud" {
		t.Fatalf("CloudWins: cloud kazanmalı, alınan: %v", entry.Value["v"])
	}
}

func TestConflictEdgeWins(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ConflictPolicy = config.EdgeWins
	cfg.NodeID = "edge-test"
	local := store.New("edge-test", 10000, time.Hour)
	eng := New(local, cfg)

	local.Put("ek", map[string]interface{}{"v": "edge"})
	eng.remoteStore.PutWithVersion("ek",
		map[string]interface{}{"v": "cloud"},
		store.VectorClock{"cloud": 1},
		"cloud", time.Now(),
	)

	eng.Push()

	entry, _ := eng.remoteStore.Get("ek")
	if entry.Value["v"] != "edge" {
		t.Fatalf("EdgeWins: edge kazanmalı, alınan: %v", entry.Value["v"])
	}
}

func TestConflictMergeAll(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ConflictPolicy = config.MergeAll
	cfg.NodeID = "edge-test"
	local := store.New("edge-test", 10000, time.Hour)
	eng := New(local, cfg)

	local.Put("mk", map[string]interface{}{"local_field": "L", "shared": "local"})
	eng.remoteStore.PutWithVersion("mk",
		map[string]interface{}{"remote_field": "R", "shared": "remote"},
		store.VectorClock{"cloud": 1},
		"cloud", time.Now(),
	)

	eng.Push()

	entry, _ := eng.remoteStore.Get("mk")
	// MergeAll: remote önce, local üzerine yazar
	if entry.Value["local_field"] != "L" {
		t.Fatal("local_field merge'de kaybolmuş")
	}
	if entry.Value["remote_field"] != "R" {
		t.Fatal("remote_field merge'de kaybolmuş")
	}
	if entry.Value["shared"] != "local" {
		t.Fatalf("shared field local ile override olmalı: %v", entry.Value["shared"])
	}
}

func TestSyncStats(t *testing.T) {
	eng, local, _ := testEngine()
	local.Put("a", map[string]interface{}{})
	eng.Sync()

	stats := eng.GetStats()
	if stats.PushCount != 1 {
		t.Fatalf("push_count: %d", stats.PushCount)
	}
	if stats.SyncCycles != 1 {
		t.Fatalf("sync_cycles: %d", stats.SyncCycles)
	}
}

func TestSyncState(t *testing.T) {
	eng, _, _ := testEngine()
	if eng.GetState() != config.SyncIdle {
		t.Fatalf("başlangıç durumu IDLE olmalı: %s", eng.GetState())
	}
}

func TestGetConflicts(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ConflictPolicy = config.LWW
	cfg.NodeID = "edge"
	local := store.New("edge", 10000, time.Hour)
	eng := New(local, cfg)

	local.Put("c1", map[string]interface{}{"x": 1})
	eng.remoteStore.PutWithVersion("c1", map[string]interface{}{"x": 2}, store.VectorClock{"cloud": 1}, "cloud", time.Now())
	eng.Push()

	conflicts := eng.GetConflicts(10)
	if len(conflicts) != 1 {
		t.Fatalf("beklenen: 1 conflict, alınan: %d", len(conflicts))
	}
}

func TestGetLastSync(t *testing.T) {
	eng, _, _ := testEngine()
	before := time.Now()
	eng.Sync()
	after := time.Now()
	ls := eng.GetLastSync()
	if ls.Before(before) || ls.After(after) {
		t.Fatal("last_sync zamanı yanlış")
	}
}

func TestPullSkipsOlderRemote(t *testing.T) {
	eng, local, remote := testEngine()

	// Local'a yeni versiyon yaz
	local.Put("k", map[string]interface{}{"v": "new"})
	local.Put("k", map[string]interface{}{"v": "newer"})
	local.MarkClean([]string{"k"})

	// Remote'a eski versiyon
	remote.PutWithVersion("k", map[string]interface{}{"v": "old"}, store.VectorClock{"edge-test": 1}, "cloud", time.Now())

	pulled, _ := eng.Pull()
	if pulled != 0 {
		t.Fatalf("eski versiyon pull edilmemeli: %d", pulled)
	}
	entry, _ := local.Get("k")
	if entry.Value["v"] != "newer" {
		t.Fatal("local üzerine yazılmamalı")
	}
}
