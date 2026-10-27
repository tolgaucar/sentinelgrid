// Package proxy — YURT YurtHub benzeri sidecar proxy.
//
// Bağlantı koptuğunda istekleri buffer'a alır, bağlantı geldiğinde
// sıraya koyarak tekrar gönderir. Sağlık-farkında yönlendirme yapar.
package proxy

import (
	"fmt"
	"sync"
	"time"

	"github.com/otukenmatris/yurt/internal/config"
)

// Request — buffer'a alınmış istek
type Request struct {
	ID        string                 `json:"id"`
	Method    string                 `json:"method"`
	Path      string                 `json:"path"`
	Body      map[string]interface{} `json:"body,omitempty"`
	Headers   map[string]string      `json:"headers,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	Retries   int                    `json:"retries"`
	Priority  int                    `json:"priority"` // 0=kritik, 5=düşük
}

// HubProxy — YurtHub sidecar proxy
type HubProxy struct {
	mu          sync.Mutex
	buffer      []Request
	maxBuffer   int
	cloudURL    string
	connState   config.ConnState
	retryInt    time.Duration
	maxRetries  int
	stats       Stats
	reqCounter  int64
	stopCh      chan struct{}
	running     bool
	lastHealthy time.Time
}

// Stats — proxy istatistikleri
type Stats struct {
	Buffered      int64 `json:"buffered"`
	Replayed      int64 `json:"replayed"`
	Dropped       int64 `json:"dropped"`
	TotalReceived int64 `json:"total_received"`
	TotalForwarded int64 `json:"total_forwarded"`
	RetryAttempts int64 `json:"retry_attempts"`
	HealthChecks  int64 `json:"health_checks"`
}

// NewHub — yeni proxy oluştur
func NewHub(cfg config.Config) *HubProxy {
	return &HubProxy{
		buffer:      make([]Request, 0, cfg.BufferSize),
		maxBuffer:   cfg.BufferSize,
		cloudURL:    cfg.CloudURL,
		connState:   config.ConnOnline,
		retryInt:    cfg.RetryInterval,
		maxRetries:  cfg.MaxRetries,
		stopCh:      make(chan struct{}),
		lastHealthy: time.Now(),
	}
}

// HandleRequest — isteği işle: online ise ilet, offline ise buffer'a al
func (h *HubProxy) HandleRequest(method, path string, body map[string]interface{}, priority int) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.reqCounter++
	h.stats.TotalReceived++

	reqID := fmt.Sprintf("req-%d", h.reqCounter)
	req := Request{
		ID:        reqID,
		Method:    method,
		Path:      path,
		Body:      body,
		CreatedAt: time.Now(),
		Priority:  priority,
	}

	if h.connState == config.ConnOnline {
		// Bağlantı açık — doğrudan ilet (simülasyonda başarılı kabul)
		h.stats.TotalForwarded++
		return reqID, nil
	}

	// Offline — buffer'a al
	if len(h.buffer) >= h.maxBuffer {
		// Buffer dolu — en düşük öncelikli isteği çıkar
		worstIdx := 0
		for i, r := range h.buffer {
			if r.Priority > h.buffer[worstIdx].Priority {
				worstIdx = i
			}
		}
		if req.Priority < h.buffer[worstIdx].Priority {
			// Yeni istek daha öncelikli — eskisini çıkar
			h.buffer = append(h.buffer[:worstIdx], h.buffer[worstIdx+1:]...)
			h.stats.Dropped++
		} else {
			// Yeni istek daha düşük öncelikli — reddet
			h.stats.Dropped++
			return "", fmt.Errorf("buffer dolu, istek reddedildi")
		}
	}

	h.buffer = append(h.buffer, req)
	h.stats.Buffered++
	return reqID, nil
}

// Replay — buffer'daki istekleri tekrar gönder (bağlantı geldiğinde)
func (h *HubProxy) Replay() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.connState != config.ConnOnline || len(h.buffer) == 0 {
		return 0
	}

	// Öncelik sırasına göre gönder
	sortByPriority(h.buffer)

	replayed := 0
	remaining := make([]Request, 0)

	for i := range h.buffer {
		if h.buffer[i].Retries >= h.maxRetries {
			h.stats.Dropped++
			continue
		}
		// Simülasyonda başarılı kabul
		h.stats.Replayed++
		h.stats.TotalForwarded++
		replayed++
		// Başarısız olursa remaining'e ekle (şu an hep başarılı)
	}

	h.buffer = remaining
	return replayed
}

// SetConnectionState — bağlantı durumunu güncelle
func (h *HubProxy) SetConnectionState(state config.ConnState) {
	h.mu.Lock()
	defer h.mu.Unlock()
	old := h.connState
	h.connState = state
	if state == config.ConnOnline && old != config.ConnOnline {
		h.lastHealthy = time.Now()
	}
}

// GetConnectionState — bağlantı durumu
func (h *HubProxy) GetConnectionState() config.ConnState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.connState
}

// BufferSize — buffer'daki istek sayısı
func (h *HubProxy) BufferSize() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.buffer)
}

// GetBuffer — buffer içeriği
func (h *HubProxy) GetBuffer() []Request {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]Request, len(h.buffer))
	copy(result, h.buffer)
	return result
}

// ClearBuffer — buffer'ı temizle
func (h *HubProxy) ClearBuffer() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := len(h.buffer)
	h.buffer = h.buffer[:0]
	return n
}

// GetStats — proxy istatistikleri
func (h *HubProxy) GetStats() Stats {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stats
}

// HealthCheck — sağlık kontrolü simülasyonu
func (h *HubProxy) HealthCheck() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stats.HealthChecks++
	return h.connState == config.ConnOnline
}

// sortByPriority — öncelik sırasına göre sırala (düşük sayı = yüksek öncelik)
func sortByPriority(reqs []Request) {
	for i := 1; i < len(reqs); i++ {
		for j := i; j > 0 && reqs[j].Priority < reqs[j-1].Priority; j-- {
			reqs[j], reqs[j-1] = reqs[j-1], reqs[j]
		}
	}
}

// Summary — proxy durum özeti
func (h *HubProxy) Summary() map[string]interface{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	return map[string]interface{}{
		"connection":   h.connState.String(),
		"buffer_size":  len(h.buffer),
		"buffer_cap":   h.maxBuffer,
		"cloud_url":    h.cloudURL,
		"last_healthy": h.lastHealthy,
		"stats":        h.stats,
	}
}
