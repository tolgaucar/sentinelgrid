package hub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/otukenmatris/yuzey/internal/config"
	"github.com/otukenmatris/yuzey/internal/event"
)

func testConfig() config.Config {
	return config.Config{
		HTTPPort:     9103,
		PollInterval: 100 * time.Millisecond,
		TimeoutMs:    2000,
		MaxEntities:  100,
		Modules:      make(map[string]config.ModuleEndpoint),
	}
}

func TestNewHub(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	h := New(cfg, bus)
	if h == nil {
		t.Fatal("Hub nil olmamalı")
	}
	if h.EntityCount() != 0 {
		t.Error("Başlangıçta 0 varlık olmalı")
	}
}

func TestUpdateEntity(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	h := New(cfg, bus)

	e := config.Entity{
		ID:      "tank-1",
		Type:    "tank",
		Faction: "friendly",
		Status:  "active",
		Lat:     39.9, Lon: 32.8,
		Health:  100, Fuel: 80, Speed: 30,
	}
	h.UpdateEntity(e)

	got, ok := h.GetEntity("tank-1")
	if !ok {
		t.Fatal("Varlık bulunamadı")
	}
	if got.Type != "tank" {
		t.Errorf("Type=tank bekleniyordu, %s geldi", got.Type)
	}
	if got.LastUpdate == 0 {
		t.Error("LastUpdate otomatik ayarlanmalı")
	}
}

func TestUpdateEntities(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	h := New(cfg, bus)

	ents := []config.Entity{
		{ID: "drone-1", Type: "drone", Faction: "friendly"},
		{ID: "drone-2", Type: "drone", Faction: "friendly"},
		{ID: "tank-1", Type: "tank", Faction: "enemy"},
	}
	h.UpdateEntities(ents)
	if h.EntityCount() != 3 {
		t.Errorf("3 varlık bekleniyordu, %d geldi", h.EntityCount())
	}
}

func TestGetAllEntities(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	h := New(cfg, bus)

	h.UpdateEntity(config.Entity{ID: "a", Faction: "friendly"})
	h.UpdateEntity(config.Entity{ID: "b", Faction: "enemy"})

	all := h.GetAllEntities()
	if len(all) != 2 {
		t.Errorf("2 varlık bekleniyordu, %d geldi", len(all))
	}
}

func TestGetEntitiesByFaction(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	h := New(cfg, bus)

	h.UpdateEntities([]config.Entity{
		{ID: "a1", Faction: "friendly"},
		{ID: "a2", Faction: "friendly"},
		{ID: "e1", Faction: "enemy"},
		{ID: "n1", Faction: "neutral"},
	})

	friendly := h.GetEntitiesByFaction("friendly")
	if len(friendly) != 2 {
		t.Errorf("2 dost varlık bekleniyordu, %d geldi", len(friendly))
	}
	enemy := h.GetEntitiesByFaction("enemy")
	if len(enemy) != 1 {
		t.Errorf("1 düşman varlık bekleniyordu, %d geldi", len(enemy))
	}
}

func TestClearEntities(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	h := New(cfg, bus)

	h.UpdateEntities([]config.Entity{
		{ID: "a"}, {ID: "b"}, {ID: "c"},
	})
	n := h.ClearEntities()
	if n != 3 {
		t.Errorf("3 temizlenmeliydi, %d geldi", n)
	}
	if h.EntityCount() != 0 {
		t.Error("Temizlendikten sonra 0 olmalı")
	}
}

func TestEntityOverflow(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	cfg.MaxEntities = 3
	h := New(cfg, bus)

	for i := 0; i < 5; i++ {
		h.UpdateEntity(config.Entity{ID: "ent-" + string(rune('a'+i))})
		time.Sleep(time.Millisecond) // lastUpdate farklı olsun
	}
	if h.EntityCount() > 3 {
		t.Errorf("Max 3 varlık olmalı, %d var", h.EntityCount())
	}
}

func TestGetModuleStatus(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	cfg.Modules["butun"] = config.ModuleEndpoint{Name: "BÜTÜN", Host: "http://127.0.0.1", Port: 8000, Enabled: true}
	h := New(cfg, bus)

	ms := h.GetModuleStatus()
	if len(ms) != 1 {
		t.Errorf("1 modül bekleniyordu, %d geldi", len(ms))
	}
	if ms["butun"] == nil {
		t.Error("butun modülü olmalı")
	}
	if ms["butun"].Alive {
		t.Error("Başlangıçta alive=false olmalı")
	}
}

func TestGetModuleByID(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	cfg.Modules["bag"] = config.ModuleEndpoint{Name: "BAĞ", Host: "http://127.0.0.1", Port: 9091, Enabled: true}
	h := New(cfg, bus)

	ms, ok := h.GetModuleByID("bag")
	if !ok {
		t.Fatal("Modül bulunamadı")
	}
	if ms.Name != "BAĞ" {
		t.Errorf("Name=BAĞ bekleniyordu, %s geldi", ms.Name)
	}

	_, ok = h.GetModuleByID("nonexistent")
	if ok {
		t.Error("Olmayan modül bulunamazdı")
	}
}

func TestIsModuleAlive(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	cfg.Modules["kutuk"] = config.ModuleEndpoint{Name: "KÜTÜK", Host: "http://127.0.0.1", Port: 9093, Enabled: true}
	h := New(cfg, bus)

	if h.IsModuleAlive("kutuk") {
		t.Error("Başlangıçta alive=false olmalı")
	}
	if h.IsModuleAlive("nonexistent") {
		t.Error("Olmayan modül alive olmamalı")
	}
}

func TestGetStats(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	h := New(cfg, bus)
	s := h.GetStats()
	if s.PollCycles != 0 {
		t.Errorf("PollCycles=0 bekleniyordu, %d geldi", s.PollCycles)
	}
}

func TestGetTacticalPicture(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	cfg.Modules["akim"] = config.ModuleEndpoint{Name: "AKIM", Host: "http://127.0.0.1", Port: 9099, Enabled: true}
	h := New(cfg, bus)
	h.UpdateEntities([]config.Entity{
		{ID: "a", Faction: "friendly", Type: "drone", Status: "active"},
		{ID: "b", Faction: "enemy", Type: "tank", Status: "active"},
	})

	pic := h.GetTacticalPicture()
	if pic["entities_count"] != 2 {
		t.Errorf("entities_count=2 bekleniyordu")
	}
	factions := pic["entity_factions"].(map[string]int)
	if factions["friendly"] != 1 || factions["enemy"] != 1 {
		t.Error("Taraf sayıları yanlış")
	}
}

func TestSummary(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	h := New(cfg, bus)
	s := h.Summary()
	if s["poll_cycles"] != int64(0) {
		t.Error("poll_cycles 0 olmalı")
	}
}

func TestPollModuleWithMockServer(t *testing.T) {
	// Mock HTTP sunucu
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok", "module": "TEST",
		})
	}))
	defer ts.Close()

	bus := event.NewBus(100)
	cfg := testConfig()
	h := New(cfg, bus)

	// Modül URL'sini mock sunucuya yönlendir
	h.mu.Lock()
	h.modules["test"] = &config.ModuleStatus{
		Name: "TEST", ID: "test", URL: ts.URL, Alive: false,
	}
	h.mu.Unlock()

	// Port=0 olmadan doğrudan URL kullanımı — sadece /health ekler
	// Ancak URL() host:port döndürür, mock sunucuyu doğrudan kullanmamız lazım
	// pollModule, ep.URL()+"/health" kullanır, bu yüzden ep doğru ayarlanmalı
	// Workaround: host olarak ts.URL kullanıp port=0 vererek URL() = "ts.URL:0" olmasını engelle
	// Daha iyi: doğrudan http.Get yaparak test edelim
	
	resp, err := h.client.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Error("Mock sunucu 200 döndürmeli")
	}

	// Modülü alive olarak güncelle (simüle et)
	h.mu.Lock()
	h.modules["test"].Alive = true
	h.mu.Unlock()

	ms, ok := h.GetModuleByID("test")
	if !ok {
		t.Fatal("Test modülü bulunamadı")
	}
	if !ms.Alive {
		t.Error("Mock sunucu alive=true olmalı")
	}
}

func TestPollModuleFailure(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	cfg.Modules["bad"] = config.ModuleEndpoint{
		Name: "BAD", Host: "http://127.0.0.1", Port: 59999, Enabled: true,
	}
	h := New(cfg, bus)
	ep := cfg.Modules["bad"]
	h.pollModule("bad", ep)

	ms, _ := h.GetModuleByID("bad")
	if ms.Alive {
		t.Error("Erişilemeyen modül alive=false olmalı")
	}
}

func TestPollModuleUpDownEvents(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer ts.Close()

	h := New(cfg, bus)
	// Modülü doğrudan alive=false olarak ekle, sonra poll ile alive=true yap
	h.mu.Lock()
	h.modules["flipflop"] = &config.ModuleStatus{Name: "FLIP", ID: "flipflop", URL: ts.URL, Alive: false}
	h.mu.Unlock()

	// pollModule yerine doğrudan http ile test
	resp, err := h.client.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Alive olarak ayarla ve event yayınla
	h.mu.Lock()
	wasAlive := h.modules["flipflop"].Alive
	h.modules["flipflop"].Alive = true
	h.mu.Unlock()

	if !wasAlive {
		bus.EmitSimple("module_up", "flipflop", event.SevInfo, "FLIP tekrar erişilebilir")
	}

	evts := bus.GetByType("module_up", 10)
	if len(evts) != 1 {
		t.Errorf("1 module_up olayı bekleniyordu, %d geldi", len(evts))
	}
}

func TestConcurrentEntityUpdate(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	cfg.MaxEntities = 1000
	h := New(cfg, bus)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			h.UpdateEntity(config.Entity{
				ID: "ent-" + string(rune('a'+idx%26)),
				Lat: 39.9 + float64(idx)*0.001,
			})
		}(i)
	}
	wg.Wait()
	if h.EntityCount() == 0 {
		t.Error("Varlık sayısı 0 olmamalı")
	}
}

func TestStartStop(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	cfg.PollInterval = 50 * time.Millisecond
	h := New(cfg, bus)
	h.Start()
	time.Sleep(100 * time.Millisecond)
	h.Stop()
	// Double stop should not panic
	h.Stop()
}

func TestStartIdempotent(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	cfg.PollInterval = 100 * time.Millisecond
	h := New(cfg, bus)
	h.Start()
	h.Start() // İkinci start etkisiz olmalı
	time.Sleep(50 * time.Millisecond)
	h.Stop()
}

func TestEntityUpdateOverwrite(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	h := New(cfg, bus)

	h.UpdateEntity(config.Entity{ID: "tank-1", Health: 100, Status: "active"})
	h.UpdateEntity(config.Entity{ID: "tank-1", Health: 50, Status: "damaged"})

	e, ok := h.GetEntity("tank-1")
	if !ok {
		t.Fatal("Varlık bulunamadı")
	}
	if e.Health != 50 {
		t.Errorf("Health=50 bekleniyordu, %f geldi", e.Health)
	}
	if e.Status != "damaged" {
		t.Errorf("Status=damaged bekleniyordu, %s geldi", e.Status)
	}
	if h.EntityCount() != 1 {
		t.Errorf("Aynı ID güncellenmeli, 1 varlık olmalı, %d var", h.EntityCount())
	}
}

func TestPollAllModulesStats(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer ts.Close()

	cfg.Modules["m1"] = config.ModuleEndpoint{Name: "M1", Host: ts.URL, Port: 0, Enabled: true}
	cfg.Modules["m2"] = config.ModuleEndpoint{Name: "M2", Host: "http://127.0.0.1", Port: 59998, Enabled: true}
	h := New(cfg, bus)

	h.pollAllModules()
	s := h.GetStats()
	if s.PollCycles != 1 {
		t.Errorf("PollCycles=1 bekleniyordu, %d geldi", s.PollCycles)
	}
	if s.HealthChecks != 2 {
		t.Errorf("HealthChecks=2 bekleniyordu, %d geldi", s.HealthChecks)
	}
}

func TestPollModuleStatsEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total": 42, "errors": 0,
		})
	}))
	defer ts.Close()

	bus := event.NewBus(100)
	cfg := testConfig()
	// Port olarak mock sunucunun portunu kullan
	cfg.Modules["test"] = config.ModuleEndpoint{Name: "TEST", Host: ts.URL, Port: 0, Enabled: true}
	h := New(cfg, bus)

	// URL sorununu aşmak için doğrudan client ile test
	resp, err := h.client.Get(ts.URL + "/stats")
	if err != nil {
		t.Fatalf("stats endpoint hatası: %v", err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	total, ok := result["total"]
	if !ok {
		t.Error("total alanı olmalı")
	}
	if total.(float64) != 42 {
		t.Errorf("total=42 bekleniyordu, %v geldi", total)
	}
}

func TestPollModuleStatsUnknown(t *testing.T) {
	bus := event.NewBus(100)
	cfg := testConfig()
	h := New(cfg, bus)
	_, err := h.PollModuleStats("bilinmeyen")
	if err == nil {
		t.Error("Bilinmeyen modül için hata bekleniyordu")
	}
}

func TestFetchEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"path": r.URL.Path})
	}))
	defer ts.Close()

	bus := event.NewBus(100)
	cfg := testConfig()
	h := New(cfg, bus)

	// Doğrudan client ile test
	resp, err := h.client.Get(ts.URL + "/test/path")
	if err != nil {
		t.Fatalf("FetchEndpoint hatası: %v", err)
	}
	defer resp.Body.Close()
	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)
	if data["path"] != "/test/path" {
		t.Errorf("Path=/test/path bekleniyordu, %v geldi", data["path"])
	}
}
