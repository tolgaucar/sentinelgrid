// Package layer — YÜZEY harita katmanı yönetimi.
//
// Taktiksel harita üzerindeki katmanları (varlıklar, sensörler,
// rotalar, tehdit alanları, bölünme bölgeleri) yönetir.
package layer

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/yuzey/internal/config"
)

// LayerType — katman türü
type LayerType string

const (
	LayerEntities     LayerType = "entities"
	LayerSensors      LayerType = "sensors"
	LayerRoutes       LayerType = "routes"
	LayerThreats      LayerType = "threats"
	LayerPartitions   LayerType = "partitions"
	LayerCoverage     LayerType = "coverage"
	LayerTraffic      LayerType = "traffic"
	LayerAnnotations  LayerType = "annotations"
)

// Marker — harita işaretçisi
type Marker struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Icon     string  `json:"icon"`
	Color    string  `json:"color"`
	Size     float64 `json:"size"`
	Rotation float64 `json:"rotation"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Polyline — harita çizgisi (rota/iz)
type Polyline struct {
	ID     string      `json:"id"`
	Points [][]float64 `json:"points"` // [[lat,lon],...]
	Color  string      `json:"color"`
	Width  float64     `json:"width"`
	Style  string      `json:"style"` // solid, dashed, dotted
}

// Zone — harita bölgesi (tehdit/bölünme)
type Zone struct {
	ID     string      `json:"id"`
	Label  string      `json:"label"`
	Points [][]float64 `json:"points"`
	Color  string      `json:"color"`
	Fill   string      `json:"fill"`
	Type   string      `json:"type"` // circle, polygon
	Radius float64     `json:"radius,omitempty"` // circle için metre
}

// Layer — tek bir harita katmanı
type Layer struct {
	ID        LayerType `json:"id"`
	Name      string    `json:"name"`
	Visible   bool      `json:"visible"`
	Opacity   float64   `json:"opacity"`
	Markers   []Marker  `json:"markers"`
	Lines     []Polyline `json:"lines"`
	Zones     []Zone    `json:"zones"`
	UpdatedAt int64     `json:"updated_at"`
}

// Manager — katman yöneticisi
type Manager struct {
	mu     sync.RWMutex
	layers map[LayerType]*Layer
	stats  Stats
}

// Stats — katman istatistikleri
type Stats struct {
	TotalMarkers   int64 `json:"total_markers"`
	TotalLines     int64 `json:"total_lines"`
	TotalZones     int64 `json:"total_zones"`
	UpdateCount    int64 `json:"update_count"`
	LayerCount     int   `json:"layer_count"`
}

// NewManager — yeni katman yöneticisi
func NewManager() *Manager {
	m := &Manager{
		layers: make(map[LayerType]*Layer),
	}
	// Varsayılan katmanlar
	defaults := []struct {
		id   LayerType
		name string
	}{
		{LayerEntities, "Varlıklar"},
		{LayerSensors, "Sensörler"},
		{LayerRoutes, "Rotalar"},
		{LayerThreats, "Tehdit Alanları"},
		{LayerPartitions, "Bölünme Bölgeleri"},
		{LayerCoverage, "Sensör Kapsama"},
		{LayerTraffic, "Trafik Akışı"},
		{LayerAnnotations, "Notlar"},
	}
	for _, d := range defaults {
		m.layers[d.id] = &Layer{
			ID:      d.id,
			Name:    d.name,
			Visible: true,
			Opacity: 1.0,
			Markers: make([]Marker, 0),
			Lines:   make([]Polyline, 0),
			Zones:   make([]Zone, 0),
		}
	}
	return m
}

// GetLayer — katman getir
func (m *Manager) GetLayer(id LayerType) (*Layer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	l, ok := m.layers[id]
	if !ok {
		return nil, false
	}
	cp := *l
	cp.Markers = make([]Marker, len(l.Markers))
	copy(cp.Markers, l.Markers)
	cp.Lines = make([]Polyline, len(l.Lines))
	copy(cp.Lines, l.Lines)
	cp.Zones = make([]Zone, len(l.Zones))
	copy(cp.Zones, l.Zones)
	return &cp, true
}

// GetAllLayers — tüm katmanlar
func (m *Manager) GetAllLayers() []Layer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Layer, 0, len(m.layers))
	for _, l := range m.layers {
		result = append(result, *l)
	}
	return result
}

// SetVisibility — katman görünürlüğü
func (m *Manager) SetVisibility(id LayerType, visible bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.layers[id]
	if !ok {
		return fmt.Errorf("bilinmeyen katman: %s", id)
	}
	l.Visible = visible
	return nil
}

// SetOpacity — katman saydamlığı
func (m *Manager) SetOpacity(id LayerType, opacity float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.layers[id]
	if !ok {
		return fmt.Errorf("bilinmeyen katman: %s", id)
	}
	l.Opacity = opacity
	return nil
}

// ── Marker İşlemleri ──

// AddMarker — katmana işaretçi ekle
func (m *Manager) AddMarker(layerID LayerType, marker Marker) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.layers[layerID]
	if !ok {
		return fmt.Errorf("bilinmeyen katman: %s", layerID)
	}
	l.Markers = append(l.Markers, marker)
	l.UpdatedAt = time.Now().UnixMilli()
	atomic.AddInt64(&m.stats.TotalMarkers, 1)
	atomic.AddInt64(&m.stats.UpdateCount, 1)
	return nil
}

// SetMarkers — katman işaretçilerini toplu ayarla
func (m *Manager) SetMarkers(layerID LayerType, markers []Marker) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.layers[layerID]
	if !ok {
		return fmt.Errorf("bilinmeyen katman: %s", layerID)
	}
	l.Markers = markers
	l.UpdatedAt = time.Now().UnixMilli()
	atomic.StoreInt64(&m.stats.TotalMarkers, int64(m.countAllMarkers()))
	atomic.AddInt64(&m.stats.UpdateCount, 1)
	return nil
}

// RemoveMarker — işaretçi kaldır
func (m *Manager) RemoveMarker(layerID LayerType, markerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.layers[layerID]
	if !ok {
		return fmt.Errorf("bilinmeyen katman: %s", layerID)
	}
	for i, mk := range l.Markers {
		if mk.ID == markerID {
			l.Markers = append(l.Markers[:i], l.Markers[i+1:]...)
			l.UpdatedAt = time.Now().UnixMilli()
			return nil
		}
	}
	return fmt.Errorf("işaretçi bulunamadı: %s", markerID)
}

// ── Polyline İşlemleri ──

// AddLine — çizgi ekle
func (m *Manager) AddLine(layerID LayerType, line Polyline) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.layers[layerID]
	if !ok {
		return fmt.Errorf("bilinmeyen katman: %s", layerID)
	}
	l.Lines = append(l.Lines, line)
	l.UpdatedAt = time.Now().UnixMilli()
	atomic.AddInt64(&m.stats.TotalLines, 1)
	atomic.AddInt64(&m.stats.UpdateCount, 1)
	return nil
}

// SetLines — çizgileri toplu ayarla
func (m *Manager) SetLines(layerID LayerType, lines []Polyline) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.layers[layerID]
	if !ok {
		return fmt.Errorf("bilinmeyen katman: %s", layerID)
	}
	l.Lines = lines
	l.UpdatedAt = time.Now().UnixMilli()
	atomic.AddInt64(&m.stats.UpdateCount, 1)
	return nil
}

// ── Zone İşlemleri ──

// AddZone — bölge ekle
func (m *Manager) AddZone(layerID LayerType, zone Zone) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.layers[layerID]
	if !ok {
		return fmt.Errorf("bilinmeyen katman: %s", layerID)
	}
	l.Zones = append(l.Zones, zone)
	l.UpdatedAt = time.Now().UnixMilli()
	atomic.AddInt64(&m.stats.TotalZones, 1)
	atomic.AddInt64(&m.stats.UpdateCount, 1)
	return nil
}

// SetZones — bölgeleri toplu ayarla
func (m *Manager) SetZones(layerID LayerType, zones []Zone) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.layers[layerID]
	if !ok {
		return fmt.Errorf("bilinmeyen katman: %s", layerID)
	}
	l.Zones = zones
	l.UpdatedAt = time.Now().UnixMilli()
	atomic.AddInt64(&m.stats.UpdateCount, 1)
	return nil
}

// RemoveZone — bölge kaldır
func (m *Manager) RemoveZone(layerID LayerType, zoneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.layers[layerID]
	if !ok {
		return fmt.Errorf("bilinmeyen katman: %s", layerID)
	}
	for i, z := range l.Zones {
		if z.ID == zoneID {
			l.Zones = append(l.Zones[:i], l.Zones[i+1:]...)
			l.UpdatedAt = time.Now().UnixMilli()
			return nil
		}
	}
	return fmt.Errorf("bölge bulunamadı: %s", zoneID)
}

// ── Varlık → Katman Dönüşümü ──

// UpdateEntityLayer — varlıkları harita işaretçilerine dönüştür
func (m *Manager) UpdateEntityLayer(entities []config.Entity) {
	markers := make([]Marker, 0, len(entities))
	for _, e := range entities {
		icon := "⬡"
		color := "#888"
		switch e.Faction {
		case "friendly":
			color = "#3b82f6"
			icon = "◆"
		case "enemy":
			color = "#ef4444"
			icon = "◼"
		case "neutral":
			color = "#eab308"
			icon = "●"
		}
		switch e.Type {
		case "drone", "uav":
			icon = "✈"
		case "tank", "vehicle":
			icon = "▣"
		case "infantry":
			icon = "☺"
		case "radar", "sensor":
			icon = "◎"
		case "ship":
			icon = "⚓"
		case "aircraft":
			icon = "✈"
		}
		markers = append(markers, Marker{
			ID:       e.ID,
			Label:    fmt.Sprintf("%s [%s]", e.ID, e.Status),
			Lat:      e.Lat,
			Lon:      e.Lon,
			Icon:     icon,
			Color:    color,
			Size:     8,
			Rotation: e.Heading,
			Metadata: map[string]interface{}{
				"type":    e.Type,
				"faction": e.Faction,
				"status":  e.Status,
				"health":  e.Health,
				"fuel":    e.Fuel,
				"speed":   e.Speed,
			},
		})
	}
	m.SetMarkers(LayerEntities, markers)
}

func (m *Manager) countAllMarkers() int {
	count := 0
	for _, l := range m.layers {
		count += len(l.Markers)
	}
	return count
}

// GetStats — istatistikler
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	totalMarkers := 0
	totalLines := 0
	totalZones := 0
	for _, l := range m.layers {
		totalMarkers += len(l.Markers)
		totalLines += len(l.Lines)
		totalZones += len(l.Zones)
	}
	return Stats{
		TotalMarkers: int64(totalMarkers),
		TotalLines:   int64(totalLines),
		TotalZones:   int64(totalZones),
		UpdateCount:  atomic.LoadInt64(&m.stats.UpdateCount),
		LayerCount:   len(m.layers),
	}
}

// Summary — özet
func (m *Manager) Summary() map[string]interface{} {
	s := m.GetStats()
	return map[string]interface{}{
		"layers":   s.LayerCount,
		"markers":  s.TotalMarkers,
		"lines":    s.TotalLines,
		"zones":    s.TotalZones,
		"updates":  s.UpdateCount,
	}
}
