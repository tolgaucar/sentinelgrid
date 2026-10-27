// Package probe — Servis sağlık kontrolü (health probe) sistemi.
// HTTP, TCP ve gRPC endpoint'lerine periyodik probe gönderir.
// Ardışık hata sayısına göre servisi "down" olarak işaretler.
package probe

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ProbeType — Probe protokolü
type ProbeType string

const (
	ProbeHTTP ProbeType = "http"
	ProbeTCP  ProbeType = "tcp"
	ProbeGRPC ProbeType = "grpc"
)

// ServiceStatus — Servis durumu
type ServiceStatus int

const (
	StatusUnknown  ServiceStatus = 0
	StatusHealthy  ServiceStatus = 1
	StatusDegraded ServiceStatus = 2
	StatusDown     ServiceStatus = 3
)

func (s ServiceStatus) String() string {
	switch s {
	case StatusHealthy:
		return "HEALTHY"
	case StatusDegraded:
		return "DEGRADED"
	case StatusDown:
		return "DOWN"
	default:
		return "UNKNOWN"
	}
}

// ProbeTarget — Probe hedefi tanımı
type ProbeTarget struct {
	ServiceID string    `json:"service_id"`
	Type      ProbeType `json:"type"`
	Endpoint  string    `json:"endpoint"`
	Interval  time.Duration `json:"interval"`
	Timeout   time.Duration `json:"timeout"`
	MaxFails  int       `json:"max_fails"`
}

// ProbeResult — Tek probe sonucu
type ProbeResult struct {
	ServiceID  string        `json:"service_id"`
	Status     ServiceStatus `json:"status"`
	LatencyMs  float64       `json:"latency_ms"`
	Error      string        `json:"error,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
	ConsecFail int           `json:"consecutive_failures"`
}

// ServiceState — Servis genel durumu
type ServiceState struct {
	ServiceID    string        `json:"service_id"`
	Status       ServiceStatus `json:"status"`
	LastCheck    time.Time     `json:"last_check"`
	LastSuccess  time.Time     `json:"last_success"`
	AvgLatencyMs float64      `json:"avg_latency_ms"`
	ConsecFails  int           `json:"consecutive_failures"`
	TotalChecks  int64         `json:"total_checks"`
	TotalFails   int64         `json:"total_failures"`
	Uptime       float64       `json:"uptime_percent"`
}

// Manager — Probe yöneticisi
type Manager struct {
	mu      sync.RWMutex
	targets map[string]*probeEntry
	client  *http.Client
	running int32
	stopCh  chan struct{}

	// Callbacks
	onStatusChange func(serviceID string, oldStatus, newStatus ServiceStatus)
	onProbeResult  func(ProbeResult)
}

type probeEntry struct {
	target     ProbeTarget
	state      ServiceState
	latencies  []float64
	maxLatHist int
	mu         sync.Mutex
}

// NewManager — Yeni probe yöneticisi
func NewManager() *Manager {
	return &Manager{
		targets: make(map[string]*probeEntry),
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:    20,
				IdleConnTimeout: 30 * time.Second,
			},
		},
		stopCh: make(chan struct{}),
	}
}

// OnStatusChange — Durum değişikliği callback'i
func (m *Manager) OnStatusChange(fn func(serviceID string, oldStatus, newStatus ServiceStatus)) {
	m.mu.Lock()
	m.onStatusChange = fn
	m.mu.Unlock()
}

// OnProbeResult — Probe sonuç callback'i
func (m *Manager) OnProbeResult(fn func(ProbeResult)) {
	m.mu.Lock()
	m.onProbeResult = fn
	m.mu.Unlock()
}

// AddTarget — Probe hedefi ekle
func (m *Manager) AddTarget(t ProbeTarget) {
	if t.Interval == 0 {
		t.Interval = 5 * time.Second
	}
	if t.Timeout == 0 {
		t.Timeout = 3 * time.Second
	}
	if t.MaxFails == 0 {
		t.MaxFails = 3
	}

	m.mu.Lock()
	m.targets[t.ServiceID] = &probeEntry{
		target: t,
		state: ServiceState{
			ServiceID: t.ServiceID,
			Status:    StatusUnknown,
		},
		maxLatHist: 100,
	}
	m.mu.Unlock()
}

// RemoveTarget — Probe hedefi kaldır
func (m *Manager) RemoveTarget(serviceID string) {
	m.mu.Lock()
	delete(m.targets, serviceID)
	m.mu.Unlock()
}

// GetState — Servis durumunu getir
func (m *Manager) GetState(serviceID string) (ServiceState, bool) {
	m.mu.RLock()
	entry, ok := m.targets[serviceID]
	m.mu.RUnlock()
	if !ok {
		return ServiceState{}, false
	}
	entry.mu.Lock()
	s := entry.state
	entry.mu.Unlock()
	return s, true
}

// AllStates — Tüm servis durumları
func (m *Manager) AllStates() []ServiceState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ServiceState, 0, len(m.targets))
	for _, e := range m.targets {
		e.mu.Lock()
		result = append(result, e.state)
		e.mu.Unlock()
	}
	return result
}

// DownServices — Çökmüş servisleri getir
func (m *Manager) DownServices() []ServiceState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []ServiceState
	for _, e := range m.targets {
		e.mu.Lock()
		if e.state.Status == StatusDown {
			result = append(result, e.state)
		}
		e.mu.Unlock()
	}
	return result
}

// TargetCount — Kayıtlı hedef sayısı
func (m *Manager) TargetCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.targets)
}

// Start — Periyodik probe'ları başlat
func (m *Manager) Start() {
	if !atomic.CompareAndSwapInt32(&m.running, 0, 1) {
		return
	}
	m.mu.RLock()
	for _, entry := range m.targets {
		go m.probeLoop(entry)
	}
	m.mu.RUnlock()
}

// Stop — Probe'ları durdur
func (m *Manager) Stop() {
	if atomic.CompareAndSwapInt32(&m.running, 1, 0) {
		close(m.stopCh)
	}
}

// ProbeOnce — Tek seferlik probe yap
func (m *Manager) ProbeOnce(serviceID string) (ProbeResult, error) {
	m.mu.RLock()
	entry, ok := m.targets[serviceID]
	m.mu.RUnlock()
	if !ok {
		return ProbeResult{}, fmt.Errorf("servis bulunamadı: %s", serviceID)
	}
	return m.doProbe(entry), nil
}

func (m *Manager) probeLoop(entry *probeEntry) {
	ticker := time.NewTicker(entry.target.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			result := m.doProbe(entry)
			m.mu.RLock()
			if m.onProbeResult != nil {
				m.onProbeResult(result)
			}
			m.mu.RUnlock()
		}
	}
}

func (m *Manager) doProbe(entry *probeEntry) ProbeResult {
	start := time.Now()
	var probeErr error

	switch entry.target.Type {
	case ProbeHTTP:
		probeErr = m.probeHTTP(entry.target)
	case ProbeTCP:
		probeErr = m.probeTCP(entry.target)
	case ProbeGRPC:
		probeErr = m.probeHTTP(entry.target) // gRPC health → HTTP fallback
	default:
		probeErr = fmt.Errorf("bilinmeyen probe tipi: %s", entry.target.Type)
	}

	latencyMs := float64(time.Since(start).Microseconds()) / 1000.0

	entry.mu.Lock()
	defer entry.mu.Unlock()

	entry.state.TotalChecks++
	entry.state.LastCheck = time.Now()

	result := ProbeResult{
		ServiceID: entry.target.ServiceID,
		LatencyMs: latencyMs,
		Timestamp: time.Now(),
	}

	if probeErr != nil {
		entry.state.ConsecFails++
		entry.state.TotalFails++
		result.Error = probeErr.Error()
		result.ConsecFail = entry.state.ConsecFails

		oldStatus := entry.state.Status
		if entry.state.ConsecFails >= entry.target.MaxFails {
			entry.state.Status = StatusDown
			result.Status = StatusDown
		} else {
			entry.state.Status = StatusDegraded
			result.Status = StatusDegraded
		}

		// Durum değişikliği bildirimi
		if oldStatus != entry.state.Status {
			m.mu.RLock()
			cb := m.onStatusChange
			m.mu.RUnlock()
			if cb != nil {
				cb(entry.target.ServiceID, oldStatus, entry.state.Status)
			}
		}
	} else {
		oldStatus := entry.state.Status
		entry.state.ConsecFails = 0
		entry.state.LastSuccess = time.Now()
		entry.state.Status = StatusHealthy
		result.Status = StatusHealthy

		// Latency geçmişi
		entry.latencies = append(entry.latencies, latencyMs)
		if len(entry.latencies) > entry.maxLatHist {
			entry.latencies = entry.latencies[1:]
		}
		sum := 0.0
		for _, l := range entry.latencies {
			sum += l
		}
		entry.state.AvgLatencyMs = sum / float64(len(entry.latencies))

		if oldStatus != StatusHealthy && oldStatus != StatusUnknown {
			m.mu.RLock()
			cb := m.onStatusChange
			m.mu.RUnlock()
			if cb != nil {
				cb(entry.target.ServiceID, oldStatus, StatusHealthy)
			}
		}
	}

	// Uptime hesapla
	if entry.state.TotalChecks > 0 {
		entry.state.Uptime = float64(entry.state.TotalChecks-entry.state.TotalFails) / float64(entry.state.TotalChecks) * 100
	}

	return result
}

func (m *Manager) probeHTTP(t ProbeTarget) error {
	ctx, cancel := context.WithTimeout(context.Background(), t.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", t.Endpoint, nil)
	if err != nil {
		return fmt.Errorf("istek oluşturulamadı: %w", err)
	}
	req.Header.Set("User-Agent", "DİRLİK-Probe/1.0")

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("bağlantı hatası: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP hata: %d", resp.StatusCode)
	}
	return nil
}

func (m *Manager) probeTCP(t ProbeTarget) error {
	conn, err := net.DialTimeout("tcp", t.Endpoint, t.Timeout)
	if err != nil {
		return fmt.Errorf("TCP bağlantı hatası: %w", err)
	}
	conn.Close()
	return nil
}
