// Package balancer — AKIM çok-stratejili yük dengeleyici.
//
// Round-robin, en-az-bağlantı, ağırlıklı ve tıkanıklık-farkında
// stratejileri destekler. Arka uç düğüm havuzunu yöneterek trafiği
// en uygun hedefe yönlendirir.
package balancer

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Strategy — yük dengeleme stratejisi
type Strategy string

const (
	RoundRobin      Strategy = "round_robin"
	LeastConn       Strategy = "least_conn"
	Weighted        Strategy = "weighted"
	CongestionAware Strategy = "congestion_aware"
	Random          Strategy = "random"
)

// Backend — hedef düğüm
type Backend struct {
	ID          string  `json:"id"`
	Address     string  `json:"address"`
	Weight      int     `json:"weight"`      // ağırlıklı strateji için (1-100)
	ActiveConns int64   `json:"active_conns"`
	TotalConns  int64   `json:"total_conns"`
	Utilization float64 `json:"utilization"` // %0-100
	Healthy     bool    `json:"healthy"`
	LastCheck   time.Time `json:"last_check"`
}

// Balancer — yük dengeleyici
type Balancer struct {
	mu       sync.RWMutex
	backends map[string]*Backend
	order    []string // round-robin sırası
	idx      uint64   // atomik round-robin indeksi
	strategy Strategy
	stats    Stats
	rng      *rand.Rand
}

// Stats — yük dengeleme istatistikleri
type Stats struct {
	TotalSelections int64            `json:"total_selections"`
	PerBackend      map[string]int64 `json:"per_backend"`
	Failovers       int64            `json:"failovers"`
	NoHealthy       int64            `json:"no_healthy"`
}

// New — yeni yük dengeleyici
func New(strategy Strategy) *Balancer {
	return &Balancer{
		backends: make(map[string]*Backend),
		order:    make([]string, 0),
		strategy: strategy,
		stats:    Stats{PerBackend: make(map[string]int64)},
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// AddBackend — arka uç ekle
func (b *Balancer) AddBackend(id, address string, weight int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if id == "" {
		return fmt.Errorf("backend ID boş olamaz")
	}
	if weight <= 0 {
		weight = 1
	}
	b.backends[id] = &Backend{
		ID:      id,
		Address: address,
		Weight:  weight,
		Healthy: true,
		LastCheck: time.Now(),
	}
	b.order = append(b.order, id)
	b.stats.PerBackend[id] = 0
	return nil
}

// RemoveBackend — arka uç kaldır
func (b *Balancer) RemoveBackend(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.backends[id]; !ok {
		return fmt.Errorf("backend bulunamadı: %s", id)
	}
	delete(b.backends, id)
	newOrder := make([]string, 0, len(b.order))
	for _, bid := range b.order {
		if bid != id {
			newOrder = append(newOrder, bid)
		}
	}
	b.order = newOrder
	return nil
}

// SetHealthy — sağlık durumu güncelle
func (b *Balancer) SetHealthy(id string, healthy bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	be, ok := b.backends[id]
	if !ok {
		return fmt.Errorf("backend bulunamadı: %s", id)
	}
	be.Healthy = healthy
	be.LastCheck = time.Now()
	return nil
}

// UpdateUtilization — kullanım oranı güncelle
func (b *Balancer) UpdateUtilization(id string, util float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	be, ok := b.backends[id]
	if !ok {
		return fmt.Errorf("backend bulunamadı: %s", id)
	}
	be.Utilization = util
	return nil
}

// Select — strateji'ye göre en uygun backend'i seç
func (b *Balancer) Select() (*Backend, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	healthy := b.healthyBackends()
	if len(healthy) == 0 {
		b.stats.NoHealthy++
		return nil, fmt.Errorf("sağlıklı backend yok")
	}

	var selected *Backend
	switch b.strategy {
	case RoundRobin:
		selected = b.selectRoundRobin(healthy)
	case LeastConn:
		selected = b.selectLeastConn(healthy)
	case Weighted:
		selected = b.selectWeighted(healthy)
	case CongestionAware:
		selected = b.selectCongestionAware(healthy)
	case Random:
		selected = b.selectRandom(healthy)
	default:
		selected = b.selectRoundRobin(healthy)
	}

	selected.ActiveConns++
	selected.TotalConns++
	b.stats.TotalSelections++
	b.stats.PerBackend[selected.ID]++
	cp := *selected
	return &cp, nil
}

// Release — bağlantı serbest bırak
func (b *Balancer) Release(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if be, ok := b.backends[id]; ok {
		if be.ActiveConns > 0 {
			be.ActiveConns--
		}
	}
}

// ListBackends — tüm backend'ler
func (b *Balancer) ListBackends() []Backend {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]Backend, 0, len(b.backends))
	for _, be := range b.backends {
		result = append(result, *be)
	}
	return result
}

// GetStrategy — mevcut strateji
func (b *Balancer) GetStrategy() Strategy {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.strategy
}

// SetStrategy — strateji değiştir
func (b *Balancer) SetStrategy(s Strategy) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.strategy = s
}

// GetStats — istatistikler
func (b *Balancer) GetStats() Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	cp := Stats{
		TotalSelections: b.stats.TotalSelections,
		Failovers:       b.stats.Failovers,
		NoHealthy:       b.stats.NoHealthy,
		PerBackend:      make(map[string]int64),
	}
	for k, v := range b.stats.PerBackend {
		cp.PerBackend[k] = v
	}
	return cp
}

// ── Strateji implementasyonları ──

func (b *Balancer) healthyBackends() []*Backend {
	result := make([]*Backend, 0)
	for _, id := range b.order {
		if be, ok := b.backends[id]; ok && be.Healthy {
			result = append(result, be)
		}
	}
	return result
}

func (b *Balancer) selectRoundRobin(healthy []*Backend) *Backend {
	idx := atomic.AddUint64(&b.idx, 1) - 1
	return healthy[idx%uint64(len(healthy))]
}

func (b *Balancer) selectLeastConn(healthy []*Backend) *Backend {
	best := healthy[0]
	for _, be := range healthy[1:] {
		if be.ActiveConns < best.ActiveConns {
			best = be
		}
	}
	return best
}

func (b *Balancer) selectWeighted(healthy []*Backend) *Backend {
	totalWeight := 0
	for _, be := range healthy {
		totalWeight += be.Weight
	}
	if totalWeight <= 0 {
		return healthy[0]
	}
	r := b.rng.Intn(totalWeight)
	for _, be := range healthy {
		r -= be.Weight
		if r < 0 {
			return be
		}
	}
	return healthy[0]
}

func (b *Balancer) selectCongestionAware(healthy []*Backend) *Backend {
	best := healthy[0]
	bestScore := math.MaxFloat64
	for _, be := range healthy {
		// Skor = kullanım oranı + aktif bağlantı ağırlığı
		score := be.Utilization + float64(be.ActiveConns)*0.5
		// Ağırlık bonusu (yüksek ağırlık = düşük skor)
		score -= float64(be.Weight) * 0.1
		if score < bestScore {
			bestScore = score
			best = be
		}
	}
	return best
}

func (b *Balancer) selectRandom(healthy []*Backend) *Backend {
	return healthy[b.rng.Intn(len(healthy))]
}
