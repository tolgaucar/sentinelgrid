// Package monitor — AKIM ağ tıkanıklık izleyicisi.
//
// Link kullanım oranlarını, gecikme değerlerini ve paket kayıplarını
// düzenli aralıklarla örnekleyerek tıkanıklık tespiti yapar.
// Eşik aşıldığında alarm üretir.
package monitor

import (
	"fmt"
	"sync"
	"time"

	"github.com/otukenmatris/akim/internal/config"
	"github.com/otukenmatris/akim/internal/router"
)

// LinkSnapshot — anlık link durumu
type LinkSnapshot struct {
	LinkID      string    `json:"link_id"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Bandwidth   float64   `json:"bandwidth_mbps"`
	UsedBW      float64   `json:"used_bw_mbps"`
	Utilization float64   `json:"utilization"`
	Latency     float64   `json:"latency_ms"`
	PacketLoss  float64   `json:"packet_loss"`
	Status      string    `json:"status"` // normal, warning, critical, blocked
	Timestamp   time.Time `json:"timestamp"`
}

// Alert — tıkanıklık alarmı
type Alert struct {
	ID        string    `json:"id"`
	LinkID    string    `json:"link_id"`
	Level     string    `json:"level"` // warning, critical, blocked
	Message   string    `json:"message"`
	Util      float64   `json:"utilization"`
	Timestamp time.Time `json:"timestamp"`
}

// Monitor — ağ izleyicisi
type Monitor struct {
	mu       sync.RWMutex
	engine   *router.Engine
	cfg      config.Config
	history  map[string][]LinkSnapshot // linkID → snapshot geçmişi
	alerts   []Alert
	stats    Stats
	stopCh   chan struct{}
	running  bool
	alertCnt int64
	lastAlertLevel map[string]string // linkID → son alert seviyesi (dedup için)
}

// Stats — izleme istatistikleri
type Stats struct {
	SnapshotsCollected int64 `json:"snapshots_collected"`
	AlertsGenerated    int64 `json:"alerts_generated"`
	LinksMonitored     int64 `json:"links_monitored"`
	CongestionEvents   int64 `json:"congestion_events"`
	ScanCycles         int64 `json:"scan_cycles"`
}

// New — yeni ağ izleyicisi
func New(engine *router.Engine, cfg config.Config) *Monitor {
	return &Monitor{
		engine:         engine,
		cfg:            cfg,
		history:        make(map[string][]LinkSnapshot),
		alerts:         make([]Alert, 0, 256),
		stopCh:         make(chan struct{}),
		lastAlertLevel: make(map[string]string),
	}
}

// Start — periyodik izlemeyi başlat
func (m *Monitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go m.loop()
}

// Stop — izlemeyi durdur
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		m.running = false
		close(m.stopCh)
	}
}

func (m *Monitor) loop() {
	ticker := time.NewTicker(m.cfg.MonitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.scan()
		}
	}
}

// scan — tüm linkleri tara ve durum belirle
func (m *Monitor) scan() {
	links := m.engine.ListLinks()
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.ScanCycles++
	m.stats.LinksMonitored = int64(len(links))

	now := time.Now()
	for _, l := range links {
		if !l.Active {
			continue
		}
		util := l.Utilization()
		status := "normal"
		if util >= m.cfg.CongestionBlock {
			status = "blocked"
		} else if util >= m.cfg.CongestionCritical {
			status = "critical"
		} else if util >= m.cfg.CongestionWarn {
			status = "warning"
		}

		snap := LinkSnapshot{
			LinkID:      l.ID,
			From:        l.From,
			To:          l.To,
			Bandwidth:   l.Bandwidth,
			UsedBW:      l.UsedBW,
			Utilization: util,
			Latency:     l.Latency,
			PacketLoss:  l.PacketLoss,
			Status:      status,
			Timestamp:   now,
		}

		hist := m.history[l.ID]
		hist = append(hist, snap)
		if len(hist) > m.cfg.HistorySize {
			hist = hist[len(hist)-m.cfg.HistorySize:]
		}
		m.history[l.ID] = hist
		m.stats.SnapshotsCollected++

		// Alarm üret (seviye değişmişse — deduplication)
		if status != "normal" {
			if lastLevel, ok := m.lastAlertLevel[l.ID]; !ok || lastLevel != status {
				m.alertCnt++
				alert := Alert{
					ID:        fmt.Sprintf("alert-%d", m.alertCnt),
					LinkID:    l.ID,
					Level:     status,
					Message:   fmt.Sprintf("Link %s tıkanıklık: %.1f%% (%s)", l.ID, util, status),
					Util:      util,
					Timestamp: now,
				}
				m.alerts = append(m.alerts, alert)
				if len(m.alerts) > 1000 {
					m.alerts = m.alerts[len(m.alerts)-500:]
				}
				m.stats.AlertsGenerated++
				m.stats.CongestionEvents++
				m.lastAlertLevel[l.ID] = status
			}
		} else {
			delete(m.lastAlertLevel, l.ID)
		}
	}
}

// Snapshot — bir linkin son snapshot'ı
func (m *Monitor) Snapshot(linkID string) (*LinkSnapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hist, ok := m.history[linkID]
	if !ok || len(hist) == 0 {
		return nil, false
	}
	cp := hist[len(hist)-1]
	return &cp, true
}

// AllSnapshots — tüm linklerin son snapshot'ları
func (m *Monitor) AllSnapshots() []LinkSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]LinkSnapshot, 0, len(m.history))
	for _, hist := range m.history {
		if len(hist) > 0 {
			result = append(result, hist[len(hist)-1])
		}
	}
	return result
}

// LinkHistory — bir linkin snapshot geçmişi
func (m *Monitor) LinkHistory(linkID string, n int) []LinkSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hist, ok := m.history[linkID]
	if !ok {
		return nil
	}
	if n <= 0 || n > len(hist) {
		n = len(hist)
	}
	result := make([]LinkSnapshot, n)
	copy(result, hist[len(hist)-n:])
	return result
}

// GetAlerts — alarm geçmişi
func (m *Monitor) GetAlerts(n int) []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if n <= 0 || n > len(m.alerts) {
		n = len(m.alerts)
	}
	result := make([]Alert, n)
	copy(result, m.alerts[len(m.alerts)-n:])
	return result
}

// CongestionReport — tıkanıklık raporu
func (m *Monitor) CongestionReport() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	congested := make([]map[string]interface{}, 0)
	for linkID, hist := range m.history {
		if len(hist) == 0 {
			continue
		}
		last := hist[len(hist)-1]
		if last.Status != "normal" {
			congested = append(congested, map[string]interface{}{
				"link_id":     linkID,
				"utilization": last.Utilization,
				"status":      last.Status,
				"latency_ms":  last.Latency,
			})
		}
	}

	return map[string]interface{}{
		"total_links":    m.stats.LinksMonitored,
		"congested_links": len(congested),
		"details":        congested,
		"scan_cycles":    m.stats.ScanCycles,
	}
}

// GetStats — izleme istatistikleri
func (m *Monitor) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}


