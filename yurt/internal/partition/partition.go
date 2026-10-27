// Package partition — YURT ağ bölünmesi (partition) yönetimi.
//
// Bağlantı kopma tespiti, graceful degradation, split-brain çözümü.
package partition

import (
	"fmt"
	"sync"
	"time"

	"github.com/otukenmatris/yurt/internal/config"
)

// PeerNode — ağdaki diğer düğüm
type PeerNode struct {
	ID          string    `json:"id"`
	Address     string    `json:"address"`
	LastSeen    time.Time `json:"last_seen"`
	Alive       bool      `json:"alive"`
	Latency     float64   `json:"latency_ms"`
	Role        string    `json:"role"` // edge, cloud, relay
}

// PartitionEvent — bölünme olayı
type PartitionEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // detected, resolved, split_brain
	Peers     []string  `json:"peers"`
	Timestamp time.Time `json:"timestamp"`
	Details   string    `json:"details"`
}

// Manager — bölünme yöneticisi
type Manager struct {
	mu        sync.Mutex
	peers     map[string]*PeerNode
	events    []PartitionEvent
	cfg       config.Config
	state     State
	stats     Stats
	eventCnt  int64
	stopCh    chan struct{}
	running   bool
}

// State — bölünme durumu
type State int

const (
	StateNormal      State = iota // tüm düğümler erişilebilir
	StatePartitioned              // en az bir düğüm erişilemez
	StateSplitBrain               // quorum sağlanamıyor
	StateIsolated                 // tamamen izole
)

func (s State) String() string {
	switch s {
	case StateNormal:
		return "NORMAL"
	case StatePartitioned:
		return "PARTITIONED"
	case StateSplitBrain:
		return "SPLIT_BRAIN"
	case StateIsolated:
		return "ISOLATED"
	default:
		return "UNKNOWN"
	}
}

// Stats — bölünme istatistikleri
type Stats struct {
	PartitionsDetected int64 `json:"partitions_detected"`
	PartitionsResolved int64 `json:"partitions_resolved"`
	SplitBrainEvents   int64 `json:"split_brain_events"`
	HeartbeatsSent     int64 `json:"heartbeats_sent"`
	HeartbeatsRecvd    int64 `json:"heartbeats_received"`
	PeerChecks         int64 `json:"peer_checks"`
}

// NewManager — yeni bölünme yöneticisi
func NewManager(cfg config.Config) *Manager {
	return &Manager{
		peers:  make(map[string]*PeerNode),
		events: make([]PartitionEvent, 0, 128),
		cfg:    cfg,
		state:  StateNormal,
		stopCh: make(chan struct{}),
	}
}

// AddPeer — düğüm ekle
func (m *Manager) AddPeer(id, address, role string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.peers[id] = &PeerNode{
		ID:       id,
		Address:  address,
		LastSeen: time.Now(),
		Alive:    true,
		Role:     role,
	}
}

// RemovePeer — düğüm kaldır
func (m *Manager) RemovePeer(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.peers, id)
}

// RecordHeartbeat — düğümden heartbeat al
func (m *Manager) RecordHeartbeat(peerID string, latencyMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.peers[peerID]; ok {
		p.LastSeen = time.Now()
		p.Alive = true
		p.Latency = latencyMs
		m.stats.HeartbeatsRecvd++
	}
}

// CheckPartition — bölünme kontrolü yap
func (m *Manager) CheckPartition() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats.PeerChecks++

	now := time.Now()
	aliveCount := 0
	deadPeers := make([]string, 0)

	for id, p := range m.peers {
		if now.Sub(p.LastSeen) > m.cfg.PartitionTimeout {
			if p.Alive {
				p.Alive = false
				deadPeers = append(deadPeers, id)
			}
		} else {
			aliveCount++
		}
	}

	oldState := m.state
	totalPeers := len(m.peers)

	if totalPeers == 0 || aliveCount == totalPeers {
		m.state = StateNormal
	} else if aliveCount == 0 {
		m.state = StateIsolated
	} else if aliveCount < m.cfg.QuorumSize {
		m.state = StateSplitBrain
	} else {
		m.state = StatePartitioned
	}

	// Durum değişikliği olayı
	if m.state != oldState {
		m.eventCnt++
		evt := PartitionEvent{
			ID:        fmt.Sprintf("part-%d", m.eventCnt),
			Timestamp: now,
			Peers:     deadPeers,
		}
		if m.state == StateNormal {
			evt.Type = "resolved"
			evt.Details = "tüm düğümler erişilebilir"
			m.stats.PartitionsResolved++
		} else if m.state == StateSplitBrain {
			evt.Type = "split_brain"
			evt.Details = fmt.Sprintf("quorum sağlanamadı (alive: %d, quorum: %d)", aliveCount, m.cfg.QuorumSize)
			m.stats.SplitBrainEvents++
		} else if m.state == StateIsolated {
			evt.Type = "detected"
			evt.Details = "tamamen izole"
			m.stats.PartitionsDetected++
		} else {
			evt.Type = "detected"
			evt.Details = fmt.Sprintf("erişilemeyen düğümler: %d", len(deadPeers))
			m.stats.PartitionsDetected++
		}
		m.events = append(m.events, evt)
		if len(m.events) > 500 {
			m.events = m.events[len(m.events)-250:]
		}
	}

	return m.state
}

// GetState — mevcut bölünme durumu
func (m *Manager) GetState() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// ListPeers — tüm düğümler
func (m *Manager) ListPeers() []PeerNode {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]PeerNode, 0, len(m.peers))
	for _, p := range m.peers {
		result = append(result, *p)
	}
	return result
}

// GetEvents — bölünme olayları
func (m *Manager) GetEvents(n int) []PartitionEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n <= 0 || n > len(m.events) {
		n = len(m.events)
	}
	result := make([]PartitionEvent, n)
	copy(result, m.events[len(m.events)-n:])
	return result
}

// GetStats — istatistikler
func (m *Manager) GetStats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stats
}

// AliveCount — erişilebilir düğüm sayısı
func (m *Manager) AliveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := 0
	for _, p := range m.peers {
		if p.Alive {
			c++
		}
	}
	return c
}

// Start — periyodik bölünme kontrolü
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go func() {
		ticker := time.NewTicker(m.cfg.HeartbeatFreq)
		defer ticker.Stop()
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.CheckPartition()
			}
		}
	}()
}

// Stop — periyodik kontrolü durdur
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		m.running = false
		close(m.stopCh)
	}
}

// Summary — durum özeti
func (m *Manager) Summary() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	alive := 0
	dead := 0
	for _, p := range m.peers {
		if p.Alive {
			alive++
		} else {
			dead++
		}
	}
	return map[string]interface{}{
		"state":       m.state.String(),
		"total_peers": len(m.peers),
		"alive":       alive,
		"dead":        dead,
		"quorum":      m.cfg.QuorumSize,
	}
}
