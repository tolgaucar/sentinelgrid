// Package flow — AKIM veri akış yönetimi.
//
// Ağ üzerindeki aktif veri akışlarını izler, QoS sınıfına göre
// bant genişliği tahsis eder ve akış ömür sürelerini yönetir.
package flow

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/akim/internal/config"
)

// Flow — aktif veri akışı
type Flow struct {
	ID          string        `json:"id"`
	From        string        `json:"from"`
	To          string        `json:"to"`
	QoS         config.QoSClass `json:"qos"`
	RequestedBW float64       `json:"requested_bw_mbps"`
	AllocatedBW float64       `json:"allocated_bw_mbps"`
	LinkIDs     []string      `json:"link_ids"`
	BytesSent   int64         `json:"bytes_sent"`
	PacketsSent int64         `json:"packets_sent"`
	CreatedAt   time.Time     `json:"created_at"`
	LastActive  time.Time     `json:"last_active"`
	Active      bool          `json:"active"`
	DataType    string        `json:"data_type"` // video, sensor, voice, command, bulk
}

// Manager — akış yöneticisi
type Manager struct {
	mu      sync.RWMutex
	flows   map[string]*Flow
	cfg     config.Config
	stats   Stats
	counter int64
}

// Stats — akış yönetimi istatistikleri
type Stats struct {
	TotalFlows    int64 `json:"total_flows"`
	ActiveFlows   int64 `json:"active_flows"`
	ExpiredFlows  int64 `json:"expired_flows"`
	RejectedFlows int64 `json:"rejected_flows"`
	TotalBytes    int64 `json:"total_bytes"`
	TotalPackets  int64 `json:"total_packets"`
	Reallocations int64 `json:"reallocations"`
}

// NewManager — yeni akış yöneticisi
func NewManager(cfg config.Config) *Manager {
	return &Manager{
		flows: make(map[string]*Flow),
		cfg:   cfg,
	}
}

// CreateFlow — yeni veri akışı oluştur
func (m *Manager) CreateFlow(from, to string, qos config.QoSClass, requestedBW float64, dataType string, linkIDs []string) (*Flow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("flow-%d", atomic.AddInt64(&m.counter, 1))
	now := time.Now()

	f := &Flow{
		ID:          id,
		From:        from,
		To:          to,
		QoS:         qos,
		RequestedBW: requestedBW,
		AllocatedBW: requestedBW, // başlangıçta istenen kadar
		LinkIDs:     linkIDs,
		CreatedAt:   now,
		LastActive:  now,
		Active:      true,
		DataType:    dataType,
	}

	m.flows[id] = f
	m.stats.TotalFlows++
	m.stats.ActiveFlows++
	return f, nil
}

// GetFlow — akış bilgisi
func (m *Manager) GetFlow(id string) (*Flow, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.flows[id]
	if !ok {
		return nil, false
	}
	cp := *f
	return &cp, true
}

// UpdateFlow — akış trafiğini güncelle
func (m *Manager) UpdateFlow(id string, bytes int64, packets int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.flows[id]
	if !ok {
		return fmt.Errorf("akış bulunamadı: %s", id)
	}
	f.BytesSent += bytes
	f.PacketsSent += packets
	f.LastActive = time.Now()
	m.stats.TotalBytes += bytes
	m.stats.TotalPackets += packets
	return nil
}

// CloseFlow — akışı kapat
func (m *Manager) CloseFlow(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.flows[id]
	if !ok {
		return fmt.Errorf("akış bulunamadı: %s", id)
	}
	if f.Active {
		f.Active = false
		m.stats.ActiveFlows--
	}
	return nil
}

// ListFlows — aktif akışları listele
func (m *Manager) ListFlows(activeOnly bool) []Flow {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Flow, 0)
	for _, f := range m.flows {
		if activeOnly && !f.Active {
			continue
		}
		result = append(result, *f)
	}
	return result
}

// FlowsByLink — belirli link üzerindeki akışlar
func (m *Manager) FlowsByLink(linkID string) []Flow {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Flow, 0)
	for _, f := range m.flows {
		if !f.Active {
			continue
		}
		for _, lid := range f.LinkIDs {
			if lid == linkID {
				result = append(result, *f)
				break
			}
		}
	}
	return result
}

// FlowsByQoS — QoS sınıfına göre akışları getir
func (m *Manager) FlowsByQoS(qos config.QoSClass) []Flow {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Flow, 0)
	for _, f := range m.flows {
		if f.Active && f.QoS == qos {
			result = append(result, *f)
		}
	}
	return result
}

// AllocateBandwidth — QoS tabanlı bant genişliği yeniden tahsisi
// Tüm akışlara QoS sınıflarına göre orantılı bant genişliği dağıtır
func (m *Manager) AllocateBandwidth(totalBW float64) map[string]float64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	// QoS sınıfı bazında toplam talep
	qosDemand := make(map[config.QoSClass]float64)
	qosFlows := make(map[config.QoSClass][]*Flow)

	for _, f := range m.flows {
		if !f.Active {
			continue
		}
		qosDemand[f.QoS] += f.RequestedBW
		qosFlows[f.QoS] = append(qosFlows[f.QoS], f)
	}

	allocations := make(map[string]float64)

	// Her QoS sınıfına ağırlığına göre pay ayır
	for qos, flows := range qosFlows {
		classBW := totalBW * qos.BandwidthWeight()
		totalDemand := qosDemand[qos]

		for _, f := range flows {
			var alloc float64
			if totalDemand <= classBW {
				// Yeterli bant genişliği var — istenen kadar ver
				alloc = f.RequestedBW
			} else {
				// Orantılı paylaştır
				alloc = (f.RequestedBW / totalDemand) * classBW
			}
			f.AllocatedBW = alloc
			allocations[f.ID] = alloc
		}
	}

	m.stats.Reallocations++
	return allocations
}

// ExpireFlows — TTL aşılmış akışları kapat
func (m *Manager) ExpireFlows() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	expired := 0
	now := time.Now()
	for _, f := range m.flows {
		if f.Active && now.Sub(f.LastActive) > m.cfg.FlowTTL {
			f.Active = false
			m.stats.ActiveFlows--
			m.stats.ExpiredFlows++
			expired++
		}
	}
	return expired
}

// GetStats — akış istatistikleri
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// Summary — akış özeti (QoS bazında)
func (m *Manager) Summary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	byQoS := make(map[string]int)
	byType := make(map[string]int)
	totalAllocated := 0.0

	for _, f := range m.flows {
		if !f.Active {
			continue
		}
		byQoS[f.QoS.String()]++
		byType[f.DataType]++
		totalAllocated += f.AllocatedBW
	}

	return map[string]interface{}{
		"active_flows":     m.stats.ActiveFlows,
		"total_flows":      m.stats.TotalFlows,
		"by_qos":           byQoS,
		"by_type":          byType,
		"total_allocated_bw": totalAllocated,
	}
}
