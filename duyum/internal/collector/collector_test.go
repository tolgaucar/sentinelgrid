package collector

import (
	"testing"
)

func newTestEngine() *Engine {
	return NewEngine(50, 1000, 0.2, 3600)
}

func TestNewEngine(t *testing.T) {
	e := newTestEngine()
	if e == nil {
		t.Fatal("Engine nil")
	}
	sources := e.GetAllSources()
	if len(sources) < 5 {
		t.Fatalf("expected at least 5 default sources, got %d", len(sources))
	}
}

func TestAddSource(t *testing.T) {
	e := newTestEngine()
	initial := len(e.GetAllSources())
	err := e.AddSource(Source{
		ID:          "test-src-1",
		Type:        SourceNewsFeed,
		Name:        "Test Kaynak",
		URL:         "https://test.example/rss",
		Language:    "en",
		Credibility: 0.7,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if len(e.GetAllSources()) != initial+1 {
		t.Fatalf("expected %d sources, got %d", initial+1, len(e.GetAllSources()))
	}
}

func TestAddSourceDuplicate(t *testing.T) {
	e := newTestEngine()
	_ = e.AddSource(Source{ID: "dup-1", Type: SourceForum, Name: "Dup", Credibility: 0.5})
	err := e.AddSource(Source{ID: "dup-1", Type: SourceForum, Name: "Dup2", Credibility: 0.5})
	if err == nil {
		t.Fatal("expected error for duplicate source")
	}
}

func TestUpdateSource(t *testing.T) {
	e := newTestEngine()
	_ = e.AddSource(Source{ID: "upd-1", Type: SourceForum, Name: "Old Name", Credibility: 0.5, Enabled: true})
	err := e.UpdateSource("upd-1", Source{Name: "New Name", Enabled: false})
	if err != nil {
		t.Fatalf("UpdateSource: %v", err)
	}
	s, ok := e.GetSource("upd-1")
	if !ok {
		t.Fatal("source not found after update")
	}
	if s.Name != "New Name" {
		t.Fatalf("expected 'New Name', got %s", s.Name)
	}
	if s.Enabled != false {
		t.Fatal("expected disabled")
	}
}

func TestUpdateSourceNotFound(t *testing.T) {
	e := newTestEngine()
	err := e.UpdateSource("nonexistent", Source{Name: "X"})
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestDeleteSource(t *testing.T) {
	e := newTestEngine()
	_ = e.AddSource(Source{ID: "del-1", Type: SourceForum, Name: "To Delete", Credibility: 0.5})
	err := e.DeleteSource("del-1")
	if err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}
	_, ok := e.GetSource("del-1")
	if ok {
		t.Fatal("source should be deleted")
	}
}

func TestDeleteSourceNotFound(t *testing.T) {
	e := newTestEngine()
	err := e.DeleteSource("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestGetSource(t *testing.T) {
	e := newTestEngine()
	s, ok := e.GetSource("src-sm-01")
	if !ok {
		t.Fatal("expected default source")
	}
	if s.Type != SourceSocialMedia {
		t.Fatalf("expected social_media, got %s", s.Type)
	}
}

func TestGetEnabledSources(t *testing.T) {
	e := newTestEngine()
	enabled := e.GetEnabledSources()
	if len(enabled) == 0 {
		t.Fatal("expected some enabled sources")
	}
	for _, s := range enabled {
		if !s.Enabled {
			t.Fatalf("disabled source in enabled list: %s", s.ID)
		}
	}
}

func TestCollect(t *testing.T) {
	e := newTestEngine()
	result, err := e.Collect("src-sm-01")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if result.ItemCount == 0 {
		t.Fatal("expected some items")
	}
	if result.SourceID != "src-sm-01" {
		t.Fatalf("expected src-sm-01, got %s", result.SourceID)
	}
}

func TestCollectNotFound(t *testing.T) {
	e := newTestEngine()
	_, err := e.Collect("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestCollectDisabled(t *testing.T) {
	e := newTestEngine()
	_ = e.AddSource(Source{ID: "dis-1", Type: SourceForum, Name: "Disabled", Credibility: 0.5, Enabled: false})
	_, err := e.Collect("dis-1")
	if err == nil {
		t.Fatal("expected error for disabled source")
	}
}

func TestCollectAll(t *testing.T) {
	e := newTestEngine()
	results := e.CollectAll()
	if len(results) == 0 {
		t.Fatal("expected some results from CollectAll")
	}
}

func TestCollectFromEntities(t *testing.T) {
	e := newTestEngine()
	entities := []map[string]any{
		{"id": "e-1", "type": "tank", "faction": "enemy", "lat": 39.5, "lon": 33.0, "speed": 30.0},
		{"id": "e-2", "type": "infantry", "faction": "friendly", "lat": 39.6, "lon": 33.1, "speed": 5.0},
	}
	items := e.CollectFromEntities(entities)
	if len(items) == 0 {
		t.Fatal("expected some items from entities")
	}
}

func TestGetItems(t *testing.T) {
	e := newTestEngine()
	e.Collect("src-sm-01")
	items := e.GetItems(10)
	if len(items) == 0 {
		t.Fatal("expected items after collect")
	}
}

func TestPrune(t *testing.T) {
	e := NewEngine(50, 1000, 0.2, 1)
	e.Collect("src-sm-01")
	// items are fresh, nothing to prune yet
	removed := e.Prune()
	if removed != 0 {
		t.Fatalf("expected 0 removed (items are fresh), got %d", removed)
	}
}

func TestStats(t *testing.T) {
	e := newTestEngine()
	stats := e.Stats()
	if stats["total_sources"].(int) < 5 {
		t.Fatalf("expected at least 5 sources, got %v", stats["total_sources"])
	}
	if stats["enabled_sources"].(int) == 0 {
		t.Fatal("expected some enabled sources")
	}
}

func TestEntityTypeLabel(t *testing.T) {
	if entityTypeLabel("tank") != "Zırhlı" {
		t.Fatalf("expected 'Zırhlı', got %s", entityTypeLabel("tank"))
	}
	if entityTypeLabel("helicopter") != "Helikopter" {
		t.Fatalf("expected 'Helikopter', got %s", entityTypeLabel("helicopter"))
	}
}
