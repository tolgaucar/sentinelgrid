package tracker

import (
	"testing"
	"time"
)

func TestManager_UpdateAndGet(t *testing.T) {
	m := NewManager(100, 50, 120.0, 3.0)

	entities := []EntityUpdate{
		{ID: "tank-1", Type: "tank", Faction: "enemy", Lat: 39.9, Lon: 32.8, Speed: 15, Heading: 45},
		{ID: "apc-1", Type: "apc", Faction: "enemy", Lat: 39.91, Lon: 32.81, Speed: 20, Heading: 90},
	}

	updated, newTracks := m.Update(entities)
	if updated != 2 {
		t.Errorf("updated = %d, beklenen 2", updated)
	}
	if newTracks != 2 {
		t.Errorf("newTracks = %d, beklenen 2", newTracks)
	}

	track, ok := m.Get("tank-1")
	if !ok {
		t.Fatal("tank-1 bulunamadı")
	}
	if track.Type != "tank" {
		t.Errorf("type = %s, beklenen tank", track.Type)
	}
	if len(track.Points) != 1 {
		t.Errorf("points = %d, beklenen 1", len(track.Points))
	}
}

func TestManager_VelocityComputation(t *testing.T) {
	m := NewManager(100, 50, 120.0, 3.0)

	// İlk güncelleme
	m.Update([]EntityUpdate{
		{ID: "t1", Type: "tank", Faction: "enemy", Lat: 39.0, Lon: 32.0, Speed: 10, Heading: 0},
	})

	// Zaman farkı oluşması için izin eklenen noktanın timestamp'ini geri al
	m.mu.Lock()
	tr := m.tracks["t1"]
	tr.Points[0].Timestamp = tr.Points[0].Timestamp.Add(-2 * time.Second)
	m.mu.Unlock()

	// İkinci güncelleme — kuzeye konum değişikliği
	m.Update([]EntityUpdate{
		{ID: "t1", Type: "tank", Faction: "enemy", Lat: 39.001, Lon: 32.0, Speed: 10, Heading: 0},
	})

	track, _ := m.Get("t1")
	if track.Updates != 2 {
		t.Errorf("updates = %d, beklenen 2", track.Updates)
	}
	if len(track.Points) != 2 {
		t.Errorf("points = %d, beklenen 2", len(track.Points))
	}
	// Kuzeye hareket ettiği için VelNorth > 0 olmalı
	if track.VelNorth <= 0 {
		t.Errorf("VelNorth = %f, pozitif bekleniyor (kuzeye hareket)", track.VelNorth)
	}
}

func TestManager_List(t *testing.T) {
	m := NewManager(100, 50, 120.0, 3.0)
	m.Update([]EntityUpdate{
		{ID: "a", Type: "tank", Faction: "enemy", Lat: 39.0, Lon: 32.0},
		{ID: "b", Type: "apc", Faction: "friendly", Lat: 39.1, Lon: 32.1},
		{ID: "c", Type: "drone", Faction: "enemy", Lat: 39.2, Lon: 32.2},
	})

	list := m.List()
	if len(list) != 3 {
		t.Errorf("list len = %d, beklenen 3", len(list))
	}
	// Points list'te nil olmalı (hafif cevap)
	for _, tr := range list {
		if tr.Points != nil {
			t.Errorf("list'te points nil olmalı, track %s", tr.ID)
		}
	}
}

func TestManager_ClearAndPrune(t *testing.T) {
	m := NewManager(100, 50, 0.001, 3.0) // çok kısa stale süresi

	m.Update([]EntityUpdate{
		{ID: "x", Type: "tank", Faction: "enemy", Lat: 39.0, Lon: 32.0},
	})

	cleared := m.Clear()
	if cleared != 1 {
		t.Errorf("cleared = %d, beklenen 1", cleared)
	}

	// Tekrar ekle ve prune test et
	m.Update([]EntityUpdate{
		{ID: "y", Type: "apc", Faction: "enemy", Lat: 39.0, Lon: 32.0},
	})

	// LastSeen'i geçmişe çek
	m.mu.Lock()
	m.tracks["y"].LastSeen = m.tracks["y"].LastSeen.Add(-time.Second)
	m.mu.Unlock()

	pruned := m.Prune()
	if pruned != 1 {
		t.Errorf("pruned = %d, beklenen 1 (stale süre çok kısa)", pruned)
	}
}

func TestManager_MaxTracks(t *testing.T) {
	m := NewManager(2, 50, 120.0, 3.0)

	m.Update([]EntityUpdate{
		{ID: "a", Type: "tank", Faction: "enemy", Lat: 39.0, Lon: 32.0},
		{ID: "b", Type: "apc", Faction: "enemy", Lat: 39.1, Lon: 32.1},
		{ID: "c", Type: "drone", Faction: "enemy", Lat: 39.2, Lon: 32.2},
	})

	list := m.List()
	if len(list) != 2 {
		t.Errorf("max 2 iz olmalı, %d var", len(list))
	}
}

func TestManager_Stats(t *testing.T) {
	m := NewManager(100, 50, 120.0, 3.0)
	m.Update([]EntityUpdate{
		{ID: "t1", Type: "tank", Faction: "enemy", Lat: 39.0, Lon: 32.0},
		{ID: "t2", Type: "apc", Faction: "friendly", Lat: 39.1, Lon: 32.1},
	})

	stats := m.Stats()
	if stats["active_tracks"] != 2 {
		t.Errorf("active_tracks = %v, beklenen 2", stats["active_tracks"])
	}
}
