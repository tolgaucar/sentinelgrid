// Package router — AKIM tıkanıklık-farkında yönlendirme motoru.
//
// Ağ topolojisini graf olarak modelleyerek, Dijkstra ve tıkanıklık-farkında
// maliyet fonksiyonlarıyla en uygun rotaları hesaplar.
package router

import (
	"container/heap"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/otukenmatris/akim/internal/config"
)

// ── Ağ Düğümü ──

// Node — ağ topolojisindeki düğüm (edge node, röle, komuta merkezi vb.)
type Node struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"` // edge, relay, command, sensor
	Capacity float64           `json:"capacity_mbps"`
	Load     float64           `json:"load_mbps"`
	Lat      float64           `json:"lat"`
	Lon      float64           `json:"lon"`
	Meta     map[string]string `json:"meta,omitempty"`
	Active   bool              `json:"active"`
}

// Utilization — düğüm kullanım yüzdesi
func (n *Node) Utilization() float64 {
	if n.Capacity <= 0 {
		return 100.0
	}
	return (n.Load / n.Capacity) * 100.0
}

// ── Ağ Bağlantısı ──

// Link — iki düğüm arasındaki bağlantı
type Link struct {
	ID        string  `json:"id"`
	From      string  `json:"from"`
	To        string  `json:"to"`
	Bandwidth float64 `json:"bandwidth_mbps"` // maks bant genişliği
	UsedBW    float64 `json:"used_bw_mbps"`   // kullanılan bant genişliği
	Latency   float64 `json:"latency_ms"`     // gecikme
	PacketLoss float64 `json:"packet_loss"`   // 0.0 - 1.0
	Active    bool    `json:"active"`
	Weight    float64 `json:"weight"`         // statik ağırlık
}

// Utilization — link kullanım yüzdesi
func (l *Link) Utilization() float64 {
	if l.Bandwidth <= 0 {
		return 100.0
	}
	return (l.UsedBW / l.Bandwidth) * 100.0
}

// AvailableBW — kalan bant genişliği
func (l *Link) AvailableBW() float64 {
	avail := l.Bandwidth - l.UsedBW
	if avail < 0 {
		return 0
	}
	return avail
}

// ── Yol ──

// Route — bir kaynak düğümden hedef düğüme giden yol
type Route struct {
	From      string   `json:"from"`
	To        string   `json:"to"`
	Path      []string `json:"path"`       // düğüm ID'leri sırasıyla
	Links     []string `json:"links"`      // link ID'leri
	TotalCost float64  `json:"total_cost"` // toplam maliyet
	TotalBW   float64  `json:"total_bw_mbps"`
	TotalLat  float64  `json:"total_latency_ms"`
	HopCount  int      `json:"hop_count"`
	ComputedAt time.Time `json:"computed_at"`
}

// ── Yönlendirme Motoru ──

// Engine — tıkanıklık-farkında yönlendirme motoru
type Engine struct {
	mu     sync.RWMutex
	nodes  map[string]*Node
	links  map[string]*Link
	adj    map[string][]string // düğüm → bağlı link ID'leri
	cfg    config.Config
	stats  Stats
}

// Stats — yönlendirme istatistikleri
type Stats struct {
	RoutesComputed  int64 `json:"routes_computed"`
	RoutesSucceeded int64 `json:"routes_succeeded"`
	RoutesFailed    int64 `json:"routes_failed"`
	NodesAdded      int64 `json:"nodes_added"`
	LinksAdded      int64 `json:"links_added"`
	Reroutes        int64 `json:"reroutes"`
}

// New — yeni yönlendirme motoru
func New(cfg config.Config) *Engine {
	return &Engine{
		nodes: make(map[string]*Node),
		links: make(map[string]*Link),
		adj:   make(map[string][]string),
		cfg:   cfg,
	}
}

// AddNode — topolojiye düğüm ekle
func (e *Engine) AddNode(n Node) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.nodes) >= e.cfg.MaxNodes {
		return fmt.Errorf("maksimum düğüm sayısına ulaşıldı: %d", e.cfg.MaxNodes)
	}
	if n.ID == "" {
		return fmt.Errorf("düğüm ID boş olamaz")
	}
	n.Active = true
	e.nodes[n.ID] = &n
	if _, ok := e.adj[n.ID]; !ok {
		e.adj[n.ID] = nil
	}
	e.stats.NodesAdded++
	return nil
}

// RemoveNode — düğümü kaldır
func (e *Engine) RemoveNode(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.nodes[id]; !ok {
		return fmt.Errorf("düğüm bulunamadı: %s", id)
	}
	// Bu düğüme bağlı linkleri kaldır
	for _, linkID := range e.adj[id] {
		delete(e.links, linkID)
	}
	// Diğer düğümlerin adj listesinden bu linkları temizle
	for nodeID, linkIDs := range e.adj {
		filtered := linkIDs[:0]
		for _, lid := range linkIDs {
			link, ok := e.links[lid]
			if ok && link.From != id && link.To != id {
				filtered = append(filtered, lid)
			}
		}
		e.adj[nodeID] = filtered
	}
	delete(e.adj, id)
	delete(e.nodes, id)
	return nil
}

// GetNode — düğüm bilgisi
func (e *Engine) GetNode(id string) (*Node, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	n, ok := e.nodes[id]
	if !ok {
		return nil, false
	}
	cp := *n
	return &cp, true
}

// ListNodes — tüm düğümler
func (e *Engine) ListNodes() []Node {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]Node, 0, len(e.nodes))
	for _, n := range e.nodes {
		result = append(result, *n)
	}
	return result
}

// AddLink — topolojiye bağlantı ekle (çift yönlü)
func (e *Engine) AddLink(l Link) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.links) >= e.cfg.MaxLinks {
		return fmt.Errorf("maksimum bağlantı sayısına ulaşıldı: %d", e.cfg.MaxLinks)
	}
	if l.From == "" || l.To == "" {
		return fmt.Errorf("from/to boş olamaz")
	}
	if _, ok := e.nodes[l.From]; !ok {
		return fmt.Errorf("kaynak düğüm bulunamadı: %s", l.From)
	}
	if _, ok := e.nodes[l.To]; !ok {
		return fmt.Errorf("hedef düğüm bulunamadı: %s", l.To)
	}
	if l.ID == "" {
		l.ID = fmt.Sprintf("%s->%s", l.From, l.To)
	}
	if l.Bandwidth <= 0 {
		l.Bandwidth = e.cfg.DefaultBW
	}
	if l.Latency <= 0 {
		l.Latency = e.cfg.DefaultLat
	}
	l.Active = true
	e.links[l.ID] = &l
	e.adj[l.From] = append(e.adj[l.From], l.ID)
	// Ters yön
	revID := fmt.Sprintf("%s->%s", l.To, l.From)
	rev := Link{
		ID: revID, From: l.To, To: l.From,
		Bandwidth: l.Bandwidth, Latency: l.Latency,
		Active: true, Weight: l.Weight,
	}
	e.links[revID] = &rev
	e.adj[l.To] = append(e.adj[l.To], revID)
	e.stats.LinksAdded++
	return nil
}

// GetLink — link bilgisi
func (e *Engine) GetLink(id string) (*Link, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	l, ok := e.links[id]
	if !ok {
		return nil, false
	}
	cp := *l
	return &cp, true
}

// ListLinks — tüm bağlantılar
func (e *Engine) ListLinks() []Link {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]Link, 0, len(e.links))
	for _, l := range e.links {
		result = append(result, *l)
	}
	return result
}

// UpdateLinkTraffic — link üzerindeki trafiği güncelle
func (e *Engine) UpdateLinkTraffic(linkID string, usedBW float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	l, ok := e.links[linkID]
	if !ok {
		return fmt.Errorf("link bulunamadı: %s", linkID)
	}
	l.UsedBW = usedBW
	return nil
}

// SetLinkActive — linki aktif/pasif yap
func (e *Engine) SetLinkActive(linkID string, active bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	l, ok := e.links[linkID]
	if !ok {
		return fmt.Errorf("link bulunamadı: %s", linkID)
	}
	l.Active = active
	return nil
}

// ── Dijkstra ile Tıkanıklık-Farkında Yönlendirme ──

// costFn — link maliyet fonksiyonu (düşük = daha iyi)
func (e *Engine) costFn(l *Link, qos config.QoSClass) float64 {
	if !l.Active {
		return math.Inf(1)
	}
	util := l.Utilization()
	// Tıkanıklık ağırlığı: kullanım arttıkça maliyet üstel artar
	congestionPenalty := 1.0
	if util >= e.cfg.CongestionBlock {
		return math.Inf(1) // tamamen dolu — geçilemez
	} else if util >= e.cfg.CongestionCritical {
		congestionPenalty = 10.0 + (util-e.cfg.CongestionCritical)*2.0
	} else if util >= e.cfg.CongestionWarn {
		congestionPenalty = 2.0 + (util-e.cfg.CongestionWarn)*0.1
	}

	// Gecikme maliyeti
	latCost := l.Latency / 10.0

	// Paket kaybı cezası
	lossPenalty := 1.0 + l.PacketLoss*50.0

	// Düşük öncelikli trafik tıkanıklığa daha duyarlı, kritik trafik her yoldan geçer
	qosMultiplier := 1.0
	if qos <= config.QoSRealtime {
		qosMultiplier = 1.0 // kritik/realtime: tıkanıklıktan etkilenmez
	} else if qos <= config.QoSCommand {
		qosMultiplier = math.Sqrt(congestionPenalty)
	} else {
		qosMultiplier = congestionPenalty // telemetry/bulk/besteffort: tam ceza
	}

	base := (1.0 + l.Weight) * latCost * lossPenalty
	return base * qosMultiplier
}

// pqItem — Dijkstra öncelik kuyruğu elemanı
type pqItem struct {
	nodeID string
	cost   float64
	index  int
}

type priorityQueue []*pqItem

func (pq priorityQueue) Len() int            { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool   { return pq[i].cost < pq[j].cost }
func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}
func (pq *priorityQueue) Push(x interface{}) {
	item := x.(*pqItem)
	item.index = len(*pq)
	*pq = append(*pq, item)
}
func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*pq = old[:n-1]
	return item
}

// FindRoute — Dijkstra ile tıkanıklık-farkında en kısa yol
func (e *Engine) FindRoute(from, to string, qos config.QoSClass) (*Route, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.stats.RoutesComputed++

	if _, ok := e.nodes[from]; !ok {
		e.stats.RoutesFailed++
		return nil, fmt.Errorf("kaynak düğüm bulunamadı: %s", from)
	}
	if _, ok := e.nodes[to]; !ok {
		e.stats.RoutesFailed++
		return nil, fmt.Errorf("hedef düğüm bulunamadı: %s", to)
	}
	if from == to {
		return &Route{From: from, To: to, Path: []string{from}, TotalCost: 0, ComputedAt: time.Now()}, nil
	}

	dist := make(map[string]float64)
	prev := make(map[string]string)     // önceki düğüm
	prevLink := make(map[string]string)  // önceki link ID
	dist[from] = 0

	pq := &priorityQueue{{nodeID: from, cost: 0}}
	heap.Init(pq)

	for pq.Len() > 0 {
		cur := heap.Pop(pq).(*pqItem)
		if cur.cost > dist[cur.nodeID] {
			continue
		}
		if cur.nodeID == to {
			break
		}

		for _, linkID := range e.adj[cur.nodeID] {
			link := e.links[linkID]
			if link == nil || !link.Active {
				continue
			}
			neighbor := link.To
			// Hedef düğüm aktif mi?
			if n, ok := e.nodes[neighbor]; !ok || !n.Active {
				continue
			}
			cost := e.costFn(link, qos)
			if math.IsInf(cost, 1) {
				continue
			}
			newDist := cur.cost + cost
			if d, ok := dist[neighbor]; !ok || newDist < d {
				dist[neighbor] = newDist
				prev[neighbor] = cur.nodeID
				prevLink[neighbor] = linkID
				heap.Push(pq, &pqItem{nodeID: neighbor, cost: newDist})
			}
		}
	}

	if _, ok := dist[to]; !ok {
		e.stats.RoutesFailed++
		return nil, fmt.Errorf("yol bulunamadı: %s → %s", from, to)
	}

	// Yolu geri oluştur
	var path []string
	var links []string
	totalLat := 0.0
	minBW := math.Inf(1)

	for cur := to; cur != ""; cur = prev[cur] {
		path = append(path, cur)
		if lid, ok := prevLink[cur]; ok {
			links = append(links, lid)
			if l, ok := e.links[lid]; ok {
				totalLat += l.Latency
				if l.AvailableBW() < minBW {
					minBW = l.AvailableBW()
				}
			}
		}
	}
	// Ters çevir
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	for i, j := 0, len(links)-1; i < j; i, j = i+1, j-1 {
		links[i], links[j] = links[j], links[i]
	}
	if math.IsInf(minBW, 1) {
		minBW = 0
	}

	e.stats.RoutesSucceeded++
	return &Route{
		From:       from,
		To:         to,
		Path:       path,
		Links:      links,
		TotalCost:  dist[to],
		TotalBW:    minBW,
		TotalLat:   totalLat,
		HopCount:   len(path) - 1,
		ComputedAt: time.Now(),
	}, nil
}

// FindAlternateRoutes — birden fazla alternatif rota bul
func (e *Engine) FindAlternateRoutes(from, to string, qos config.QoSClass, count int) ([]Route, error) {
	routes := make([]Route, 0, count)

	// İlk en iyi rota
	best, err := e.FindRoute(from, to, qos)
	if err != nil {
		return nil, err
	}
	routes = append(routes, *best)

	// Yen'in K-en-kısa-yol benzeri: her link'i geçici devre dışı bırakarak alternatif bul
	for i := 0; i < len(best.Links) && len(routes) < count; i++ {
		lid := best.Links[i]
		e.mu.Lock()
		if l, ok := e.links[lid]; ok {
			l.Active = false
		}
		e.mu.Unlock()

		alt, altErr := e.FindRoute(from, to, qos)

		e.mu.Lock()
		if l, ok := e.links[lid]; ok {
			l.Active = true
		}
		e.mu.Unlock()

		if altErr == nil && alt.TotalCost < best.TotalCost*3.0 {
			routes = append(routes, *alt)
		}
	}
	return routes, nil
}

// GetStats — yönlendirme istatistikleri
func (e *Engine) GetStats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// Topology — topoloji özeti
func (e *Engine) Topology() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	activeNodes := 0
	for _, n := range e.nodes {
		if n.Active {
			activeNodes++
		}
	}
	activeLinks := 0
	for _, l := range e.links {
		if l.Active {
			activeLinks++
		}
	}
	return map[string]interface{}{
		"total_nodes":  len(e.nodes),
		"active_nodes": activeNodes,
		"total_links":  len(e.links),
		"active_links": activeLinks,
	}
}
