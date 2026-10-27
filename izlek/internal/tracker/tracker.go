// Package tracker — hedef iz yönetimi.
//
// Hareket halindeki varlıkların konum geçmişini tutar, hız ve ivme hesaplar.
package tracker

import (
	"math"
	"sync"
	"time"
)

// Point — zaman damgalı konum noktası
type Point struct {
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	Alt       float64   `json:"alt,omitempty"`
	Speed     float64   `json:"speed"`      // m/s
	Heading   float64   `json:"heading"`    // derece (0-360)
	Timestamp time.Time `json:"timestamp"`
}

// Track — tek bir varlığın iz kaydı
type Track struct {
	ID        string    `json:"id"`
	EntityID  string    `json:"entity_id"`
	Type      string    `json:"type"`
	Faction   string    `json:"faction"`
	Points    []Point   `json:"points"`
	VelNorth  float64   `json:"vel_north"`  // m/s kuzey bileşeni
	VelEast   float64   `json:"vel_east"`   // m/s doğu bileşeni
	AccNorth  float64   `json:"acc_north"`  // m/s² kuzey ivme
	AccEast   float64   `json:"acc_east"`   // m/s² doğu ivme
	Maneuver  bool      `json:"maneuver"`   // manevra halinde mi
	LastSeen  time.Time `json:"last_seen"`
	Created   time.Time `json:"created"`
	Updates   int       `json:"updates"`
}

// LastPoint — en son konum
func (t *Track) LastPoint() Point {
	if len(t.Points) == 0 {
		return Point{}
	}
	return t.Points[len(t.Points)-1]
}

// Manager — iz yöneticisi
type Manager struct {
	mu            sync.RWMutex
	tracks        map[string]*Track
	maxTracks     int
	historyDepth  int
	staleAfterSec float64
	maneuverThr   float64
}

// NewManager — yeni iz yöneticisi oluşturur.
func NewManager(maxTracks, historyDepth int, staleSec, maneuverThr float64) *Manager {
	return &Manager{
		tracks:        make(map[string]*Track),
		maxTracks:     maxTracks,
		historyDepth:  historyDepth,
		staleAfterSec: staleSec,
		maneuverThr:   maneuverThr,
	}
}

// EntityUpdate — varlık güncellemesi
type EntityUpdate struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"`
	Faction string  `json:"faction"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Alt     float64 `json:"alt,omitempty"`
	Speed   float64 `json:"speed"`
	Heading float64 `json:"heading"`
}

const earthRadiusM = 6_371_000.0
const deg2rad = math.Pi / 180.0

// Update — varlıkları toplu günceller. Hız/ivme otomatik hesaplanır.
func (m *Manager) Update(entities []EntityUpdate) (updated int, newTracks int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, e := range entities {
		t, exists := m.tracks[e.ID]
		if !exists {
			if len(m.tracks) >= m.maxTracks {
				continue
			}
			t = &Track{
				ID:       e.ID,
				EntityID: e.ID,
				Type:     e.Type,
				Faction:  e.Faction,
				Points:   make([]Point, 0, m.historyDepth),
				Created:  now,
			}
			m.tracks[e.ID] = t
			newTracks++
		}

		pt := Point{
			Lat:       e.Lat,
			Lon:       e.Lon,
			Alt:       e.Alt,
			Speed:     e.Speed,
			Heading:   e.Heading,
			Timestamp: now,
		}

		// Hız ve ivme hesapla (en az 2 nokta gerekir)
		if len(t.Points) > 0 {
			prev := t.Points[len(t.Points)-1]
			dt := now.Sub(prev.Timestamp).Seconds()
			if dt > 0.01 {
				// Konum farkından hız hesapla (m/s)
				dlat := (pt.Lat - prev.Lat) * deg2rad * earthRadiusM
				dlon := (pt.Lon - prev.Lon) * deg2rad * earthRadiusM * math.Cos(pt.Lat*deg2rad)

				newVelN := dlat / dt
				newVelE := dlon / dt

				// İvme hesapla
				if t.Updates > 1 {
					oldAccN := t.AccNorth
					oldAccE := t.AccEast
					t.AccNorth = (newVelN - t.VelNorth) / dt
					t.AccEast = (newVelE - t.VelEast) / dt

					// Manevra tespiti: ivme değişim hızı (jerk)
					jerkN := math.Abs(t.AccNorth - oldAccN)
					jerkE := math.Abs(t.AccEast - oldAccE)
					jerk := math.Sqrt(jerkN*jerkN + jerkE*jerkE)
					t.Maneuver = jerk > m.maneuverThr
				} else {
					t.AccNorth = (newVelN - t.VelNorth) / dt
					t.AccEast = (newVelE - t.VelEast) / dt
				}

				t.VelNorth = newVelN
				t.VelEast = newVelE
			}
		}

		t.Points = append(t.Points, pt)
		if len(t.Points) > m.historyDepth {
			t.Points = t.Points[len(t.Points)-m.historyDepth:]
		}

		t.LastSeen = now
		t.Updates++
		updated++
	}

	return updated, newTracks
}

// Get — belirli izi döndürür.
func (m *Manager) Get(id string) (*Track, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tracks[id]
	if !ok {
		return nil, false
	}
	cp := *t
	cp.Points = make([]Point, len(t.Points))
	copy(cp.Points, t.Points)
	return &cp, true
}

// List — tüm izleri döndürür (points olmadan).
func (m *Manager) List() []Track {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Track, 0, len(m.tracks))
	for _, t := range m.tracks {
		cp := *t
		cp.Points = nil // hafif sonuç
		result = append(result, cp)
	}
	return result
}

// Prune — bayat izleri siler.
func (m *Manager) Prune() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	pruned := 0
	for id, t := range m.tracks {
		if now.Sub(t.LastSeen).Seconds() > m.staleAfterSec {
			delete(m.tracks, id)
			pruned++
		}
	}
	return pruned
}

// Clear — tüm izleri temizler.
func (m *Manager) Clear() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.tracks)
	m.tracks = make(map[string]*Track)
	return n
}

// Stats — istatistikler.
func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active := 0
	maneuvering := 0
	totalPoints := 0
	factions := make(map[string]int)

	for _, t := range m.tracks {
		active++
		if t.Maneuver {
			maneuvering++
		}
		totalPoints += len(t.Points)
		factions[t.Faction]++
	}

	return map[string]interface{}{
		"active_tracks":     active,
		"maneuvering":       maneuvering,
		"total_points":      totalPoints,
		"factions":          factions,
		"max_tracks":        m.maxTracks,
		"history_depth":     m.historyDepth,
	}
}
