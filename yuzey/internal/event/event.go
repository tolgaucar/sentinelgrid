// Package event — YÜZEY olay yolu.
//
// Modüller arası gerçek zamanlı olay dağıtımı, WebSocket broadcast
// ve olay geçmişi tutma.
package event

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/yuzey/internal/config"
)

// Severity seviyeleri
const (
	SevInfo     = "info"
	SevWarning  = "warning"
	SevCritical = "critical"
)

// Listener — olay dinleyici fonksiyonu
type Listener func(evt config.TacticalEvent)

// Bus — olay yolu
type Bus struct {
	mu        sync.RWMutex
	events    []config.TacticalEvent
	listeners []Listener
	maxEvents int
	counter   int64
	stats     Stats
}

// Stats — olay yolu istatistikleri
type Stats struct {
	TotalEmitted   int64 `json:"total_emitted"`
	InfoCount      int64 `json:"info_count"`
	WarningCount   int64 `json:"warning_count"`
	CriticalCount  int64 `json:"critical_count"`
	ListenerCount  int   `json:"listener_count"`
	BufferedEvents int   `json:"buffered_events"`
}

// NewBus — yeni olay yolu
func NewBus(maxEvents int) *Bus {
	if maxEvents <= 0 {
		maxEvents = 10000
	}
	return &Bus{
		events:    make([]config.TacticalEvent, 0, maxEvents),
		listeners: make([]Listener, 0),
		maxEvents: maxEvents,
	}
}

// Subscribe — olay dinleyici ekle, kaldırmak için dönen indeksi Unsubscribe'a ver
func (b *Bus) Subscribe(fn Listener) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners = append(b.listeners, fn)
	return len(b.listeners) - 1
}

// Unsubscribe — dinleyiciyi kaldır (indeks ile)
func (b *Bus) Unsubscribe(idx int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if idx >= 0 && idx < len(b.listeners) {
		b.listeners = append(b.listeners[:idx], b.listeners[idx+1:]...)
	}
}

// Emit — olay yayınla
func (b *Bus) Emit(evtType, source, severity, title, detail string, lat, lon float64) config.TacticalEvent {
	id := atomic.AddInt64(&b.counter, 1)
	evt := config.TacticalEvent{
		ID:        fmt.Sprintf("evt-%d", id),
		Type:      evtType,
		Source:    source,
		Severity:  severity,
		Title:     title,
		Detail:    detail,
		Lat:       lat,
		Lon:       lon,
		Timestamp: time.Now().UnixMilli(),
	}

	b.mu.Lock()
	b.events = append(b.events, evt)
	if len(b.events) > b.maxEvents {
		// Eski olayları sil, yenileri tut
		b.events = b.events[len(b.events)-b.maxEvents:]
	}
	listeners := make([]Listener, len(b.listeners))
	copy(listeners, b.listeners)
	b.mu.Unlock()

	// İstatistik güncelle
	atomic.AddInt64(&b.stats.TotalEmitted, 1)
	switch severity {
	case SevInfo:
		atomic.AddInt64(&b.stats.InfoCount, 1)
	case SevWarning:
		atomic.AddInt64(&b.stats.WarningCount, 1)
	case SevCritical:
		atomic.AddInt64(&b.stats.CriticalCount, 1)
	}

	// Listener'lara bildir
	for _, fn := range listeners {
		fn(evt)
	}

	return evt
}

// EmitSimple — basit olay yayınla
func (b *Bus) EmitSimple(evtType, source, severity, title string) config.TacticalEvent {
	return b.Emit(evtType, source, severity, title, "", 0, 0)
}

// GetEvents — son N olayı getir
func (b *Bus) GetEvents(n int) []config.TacticalEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if n <= 0 || n > len(b.events) {
		n = len(b.events)
	}
	result := make([]config.TacticalEvent, n)
	copy(result, b.events[len(b.events)-n:])
	return result
}

// GetEventsSince — belirli zamandan sonraki olaylar
func (b *Bus) GetEventsSince(sinceMs int64) []config.TacticalEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]config.TacticalEvent, 0)
	for _, evt := range b.events {
		if evt.Timestamp > sinceMs {
			result = append(result, evt)
		}
	}
	return result
}

// GetByType — tipe göre filtrele
func (b *Bus) GetByType(evtType string, n int) []config.TacticalEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]config.TacticalEvent, 0)
	for i := len(b.events) - 1; i >= 0 && len(result) < n; i-- {
		if b.events[i].Type == evtType {
			result = append(result, b.events[i])
		}
	}
	return result
}

// GetBySource — kaynağa göre filtrele
func (b *Bus) GetBySource(source string, n int) []config.TacticalEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]config.TacticalEvent, 0)
	for i := len(b.events) - 1; i >= 0 && len(result) < n; i-- {
		if b.events[i].Source == source {
			result = append(result, b.events[i])
		}
	}
	return result
}

// GetBySeverity — önem derecesine göre filtrele
func (b *Bus) GetBySeverity(severity string, n int) []config.TacticalEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]config.TacticalEvent, 0)
	for i := len(b.events) - 1; i >= 0 && len(result) < n; i-- {
		if b.events[i].Severity == severity {
			result = append(result, b.events[i])
		}
	}
	return result
}

// Count — toplam olay sayısı
func (b *Bus) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.events)
}

// Clear — geçmişi temizle
func (b *Bus) Clear() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(b.events)
	b.events = b.events[:0]
	return n
}

// GetStats — istatistikler
func (b *Bus) GetStats() Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return Stats{
		TotalEmitted:   atomic.LoadInt64(&b.stats.TotalEmitted),
		InfoCount:      atomic.LoadInt64(&b.stats.InfoCount),
		WarningCount:   atomic.LoadInt64(&b.stats.WarningCount),
		CriticalCount:  atomic.LoadInt64(&b.stats.CriticalCount),
		ListenerCount:  len(b.listeners),
		BufferedEvents: len(b.events),
	}
}
