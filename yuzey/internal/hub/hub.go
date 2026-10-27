// Package hub — YÜZEY merkezi veri toplayıcı.
//
// Tüm 8 ÖTÜKEN-MATRİS modülünü periyodik olarak yoklar,
// sağlık durumlarını izler, taktiksel resmi oluşturur.
package hub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/yuzey/internal/config"
	"github.com/otukenmatris/yuzey/internal/event"
)

// Hub — merkezi veri toplama motoru
type Hub struct {
	mu            sync.RWMutex
	cfg           config.Config
	client        *http.Client
	bus           *event.Bus
	modules       map[string]*config.ModuleStatus
	entities      map[string]config.Entity
	stats         Stats
	running       bool
	stopCh        chan struct{}
	startTime     time.Time
	startupDone   bool // ilk yoklama döngüsü tamamlandı mı
}

// Stats — hub istatistikleri
type Stats struct {
	PollCycles     int64 `json:"poll_cycles"`
	HealthChecks   int64 `json:"health_checks"`
	ModulesAlive   int   `json:"modules_alive"`
	ModulesTotal   int   `json:"modules_total"`
	EntitiesCount  int   `json:"entities_count"`
	SuccessPolls   int64 `json:"success_polls"`
	FailedPolls    int64 `json:"failed_polls"`
	TotalLatencyMs int64 `json:"total_latency_ms"`
}

// New — yeni hub
func New(cfg config.Config, bus *event.Bus) *Hub {
	h := &Hub{
		cfg:       cfg,
		bus:       bus,
		client:    &http.Client{Timeout: time.Duration(cfg.TimeoutMs) * time.Millisecond},
		modules:   make(map[string]*config.ModuleStatus),
		entities:  make(map[string]config.Entity),
		stopCh:    make(chan struct{}),
		startTime: time.Now(),
	}
	// Modülleri başlangıç durumuna getir
	for id, ep := range cfg.Modules {
		h.modules[id] = &config.ModuleStatus{
			Name:  ep.Name,
			ID:    id,
			URL:   ep.URL(),
			Alive: false,
		}
	}
	return h
}

// Start — periyodik yoklamayı başlat
func (h *Hub) Start() {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return
	}
	h.running = true
	h.mu.Unlock()

	go h.pollLoop()
}

// Stop — yoklamayı durdur
func (h *Hub) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running {
		h.running = false
		close(h.stopCh)
	}
}

func (h *Hub) pollLoop() {
	ticker := time.NewTicker(h.cfg.PollInterval)
	defer ticker.Stop()

	// İlk yoklama hemen
	h.pollAllModules()
	// İlk yoklama sonrası özet olay yayınla
	h.mu.Lock()
	if !h.startupDone {
		h.startupDone = true
		alive := 0
		for _, ms := range h.modules {
			if ms.Alive {
				alive++
			}
		}
		if alive > 0 {
			h.bus.EmitSimple("system_ready", "yuzey", event.SevInfo,
				fmt.Sprintf("%d/%d mod\u00fcl ba\u011fland\u0131", alive, len(h.modules)))
		}
	}
	h.mu.Unlock()

	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.pollAllModules()
		}
	}
}

// pollAllModules — tüm modülleri yokla
func (h *Hub) pollAllModules() {
	atomic.AddInt64(&h.stats.PollCycles, 1)

	var wg sync.WaitGroup
	for id, ep := range h.cfg.Modules {
		if !ep.Enabled {
			continue
		}
		wg.Add(1)
		go func(modID string, endpoint config.ModuleEndpoint) {
			defer wg.Done()
			h.pollModule(modID, endpoint)
		}(id, ep)
	}
	wg.Wait()

	// Canlı modül sayısını güncelle
	h.mu.RLock()
	alive := 0
	for _, ms := range h.modules {
		if ms.Alive {
			alive++
		}
	}
	h.mu.RUnlock()
	h.stats.ModulesAlive = alive
	h.stats.ModulesTotal = len(h.cfg.Modules)
}

// pollModule — tek modülü yokla
func (h *Hub) pollModule(id string, ep config.ModuleEndpoint) {
	atomic.AddInt64(&h.stats.HealthChecks, 1)
	url := ep.URL() + "/health"

	start := time.Now()
	resp, err := h.client.Get(url)
	latency := time.Since(start).Milliseconds()

	h.mu.Lock()
	defer h.mu.Unlock()

	ms, ok := h.modules[id]
	if !ok {
		return
	}

	ms.LastCheck = time.Now().UnixMilli()
	ms.LatencyMs = float64(latency)
	atomic.AddInt64(&h.stats.TotalLatencyMs, latency)

	if err != nil {
		wasAlive := ms.Alive
		ms.Alive = false
		ms.Details = map[string]interface{}{"error": err.Error()}
		atomic.AddInt64(&h.stats.FailedPolls, 1)
		if wasAlive {
			h.bus.EmitSimple("module_down", id, event.SevCritical, ep.Name+" bağlantısı kesildi")
		}
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ms.Details = map[string]interface{}{"error": "response okuma hatası: " + err.Error()}
		atomic.AddInt64(&h.stats.FailedPolls, 1)
		return
	}
	var details map[string]interface{}
	if err := json.Unmarshal(body, &details); err != nil {
		details = map[string]interface{}{"raw": string(body), "parse_error": err.Error()}
	}

	wasAlive := ms.Alive
	ms.Alive = resp.StatusCode == 200
	ms.Details = details
	atomic.AddInt64(&h.stats.SuccessPolls, 1)

	if !wasAlive && ms.Alive && h.startupDone {
		h.bus.EmitSimple("module_up", id, event.SevInfo, ep.Name+" tekrar erişilebilir")
	}
}

// PollModuleStats — belirli modülün /stats endpoint'ini çek
func (h *Hub) PollModuleStats(id string) (map[string]interface{}, error) {
	ep, ok := h.cfg.Modules[id]
	if !ok {
		return nil, fmt.Errorf("bilinmeyen modül: %s", id)
	}
	url := ep.URL() + "/stats"
	resp, err := h.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// FetchEndpoint — modülün herhangi bir endpoint'ini çek
func (h *Hub) FetchEndpoint(id, path string) (map[string]interface{}, error) {
	ep, ok := h.cfg.Modules[id]
	if !ok {
		return nil, fmt.Errorf("bilinmeyen modül: %s", id)
	}
	url := ep.URL() + path
	resp, err := h.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("yanıt parse hatası: %w", err)
	}
	return result, nil
}

// ── Varlık Yönetimi ──

// UpdateEntity — varlık bilgisini güncelle
func (h *Hub) UpdateEntity(e config.Entity) {
	h.mu.Lock()
	defer h.mu.Unlock()
	e.LastUpdate = time.Now().UnixMilli()
	h.entities[e.ID] = e
	for len(h.entities) > h.cfg.MaxEntities {
		// En eski varlığı temizle
		oldest := ""
		oldestTime := int64(1<<62)
		for id, ent := range h.entities {
			if ent.LastUpdate < oldestTime {
				oldest = id
				oldestTime = ent.LastUpdate
			}
		}
		if oldest != "" {
			delete(h.entities, oldest)
		} else {
			break
		}
	}
}

// EntityChanges — varlık güncellemesinde tespit edilen değişiklikler
type EntityChanges struct {
	NewEnemies    []config.Entity
	NewFriendlies []config.Entity
	Destroyed     []config.Entity
	Critical      []config.Entity // health < 20
	DamageEvents  []config.Entity // health düştü
}

// UpdateEntities — toplu varlık güncellemesi + değişiklik tespiti
func (h *Hub) UpdateEntities(entities []config.Entity) EntityChanges {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now().UnixMilli()

	var changes EntityChanges

	for _, e := range entities {
		e.LastUpdate = now
		old, existed := h.entities[e.ID]

		if !existed {
			// Yeni varlık tespiti
			if e.Faction == "enemy" {
				changes.NewEnemies = append(changes.NewEnemies, e)
			} else if e.Faction == "friendly" {
				changes.NewFriendlies = append(changes.NewFriendlies, e)
			}
		} else {
			// Durum değişikliği tespiti
			wasAlive := old.Status != "destroyed" && (old.Health > 0 || old.Health == 0 && old.Status != "destroyed")
			nowDead := e.Status == "destroyed" || e.Health <= 0
			if wasAlive && nowDead {
				changes.Destroyed = append(changes.Destroyed, e)
			}

			// Hasar tespiti (health düştü)
			if old.Health > 20 && e.Health <= 20 && e.Health > 0 {
				changes.Critical = append(changes.Critical, e)
			}
			if old.Health > e.Health && e.Health > 0 && (old.Health-e.Health) >= 5 {
				changes.DamageEvents = append(changes.DamageEvents, e)
			}
		}

		h.entities[e.ID] = e
	}
	// MaxEntities limiti uygula
	for len(h.entities) > h.cfg.MaxEntities {
		oldest := ""
		oldestTime := int64(1 << 62)
		for id, ent := range h.entities {
			if ent.LastUpdate < oldestTime {
				oldest = id
				oldestTime = ent.LastUpdate
			}
		}
		if oldest != "" {
			delete(h.entities, oldest)
		} else {
			break
		}
	}

	return changes
}

// GetEntity — varlık bilgisi
func (h *Hub) GetEntity(id string) (config.Entity, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	e, ok := h.entities[id]
	return e, ok
}

// GetAllEntities — tüm varlıklar
func (h *Hub) GetAllEntities() []config.Entity {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]config.Entity, 0, len(h.entities))
	for _, e := range h.entities {
		result = append(result, e)
	}
	return result
}

// GetEntitiesByFaction — taraf bazlı varlıklar
func (h *Hub) GetEntitiesByFaction(faction string) []config.Entity {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]config.Entity, 0)
	for _, e := range h.entities {
		if e.Faction == faction {
			result = append(result, e)
		}
	}
	return result
}

// EntityCount — varlık sayısı
func (h *Hub) EntityCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.entities)
}

// ClearEntities — varlıkları temizle
func (h *Hub) ClearEntities() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := len(h.entities)
	h.entities = make(map[string]config.Entity)
	return n
}

// ── Durum Sorguları ──

// GetModuleStatus — tüm modül durumları
func (h *Hub) GetModuleStatus() map[string]*config.ModuleStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make(map[string]*config.ModuleStatus)
	for id, ms := range h.modules {
		cp := *ms
		result[id] = &cp
	}
	return result
}

// GetModuleByID — tek modül durumu
func (h *Hub) GetModuleByID(id string) (*config.ModuleStatus, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ms, ok := h.modules[id]
	if !ok {
		return nil, false
	}
	cp := *ms
	return &cp, true
}

// IsModuleAlive — modül yaşıyor mu
func (h *Hub) IsModuleAlive(id string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ms, ok := h.modules[id]
	return ok && ms.Alive
}

// GetStats — hub istatistikleri
func (h *Hub) GetStats() Stats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s := Stats{
		PollCycles:     atomic.LoadInt64(&h.stats.PollCycles),
		HealthChecks:   atomic.LoadInt64(&h.stats.HealthChecks),
		ModulesAlive:   h.stats.ModulesAlive,
		ModulesTotal:   h.stats.ModulesTotal,
		EntitiesCount:  len(h.entities),
		SuccessPolls:   atomic.LoadInt64(&h.stats.SuccessPolls),
		FailedPolls:    atomic.LoadInt64(&h.stats.FailedPolls),
		TotalLatencyMs: atomic.LoadInt64(&h.stats.TotalLatencyMs),
	}
	return s
}

// GetTacticalPicture — tam taktiksel resim
func (h *Hub) GetTacticalPicture() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	mods := make(map[string]interface{})
	aliveCount := 0
	for id, ms := range h.modules {
		mods[id] = map[string]interface{}{
			"name":       ms.Name,
			"alive":      ms.Alive,
			"latency_ms": ms.LatencyMs,
			"last_check": ms.LastCheck,
		}
		if ms.Alive {
			aliveCount++
		}
	}

	factions := make(map[string]int)
	statuses := make(map[string]int)
	types := make(map[string]int)
	for _, e := range h.entities {
		factions[e.Faction]++
		statuses[e.Status]++
		types[e.Type]++
	}

	return map[string]interface{}{
		"timestamp":       time.Now().UnixMilli(),
		"modules":         mods,
		"modules_alive":   aliveCount,
		"modules_total":   len(h.modules),
		"entities_count":  len(h.entities),
		"entity_factions": factions,
		"entity_statuses": statuses,
		"entity_types":    types,
		"uptime_sec":      time.Since(h.startTime).Seconds(),
	}
}

// Summary — kısa özet
func (h *Hub) Summary() map[string]interface{} {
	s := h.GetStats()
	return map[string]interface{}{
		"poll_cycles":     s.PollCycles,
		"health_checks":  s.HealthChecks,
		"modules_alive":  s.ModulesAlive,
		"modules_total":  s.ModulesTotal,
		"entities_count": s.EntitiesCount,
		"success_polls":  s.SuccessPolls,
		"failed_polls":   s.FailedPolls,
		"uptime_sec":     time.Since(h.startTime).Seconds(),
	}
}
