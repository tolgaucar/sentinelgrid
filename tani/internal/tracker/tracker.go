// Package tracker — Çoklu nesne izleme motoru.
//
// Karelere yayılmış tespitleri ilişkilendirerek sürekli izler (track) oluşturur:
// - Takip ID ataması (yeni nesne vs mevcut)
// - IoU bazlı eşleştirme (Macar algoritması basitleştirilmiş)
// - Konum geçmişi ve hız vektörü hesaplama
// - Track durumu: tentative → confirmed → lost → deleted
// - Kayıp karelerde tahmin ile konum güncelleme
package tracker

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/tani/internal/classifier"
	"github.com/otukenmatris/tani/internal/detector"
)

// TrackStatus izleme durumu.
type TrackStatus string

const (
	StatusTentative TrackStatus = "tentative"  // Yeni, henüz doğrulanmamış
	StatusConfirmed TrackStatus = "confirmed"  // Doğrulanmış aktif iz
	StatusLost      TrackStatus = "lost"       // Kayıp — son konum tahmini
	StatusDeleted   TrackStatus = "deleted"    // Silinmiş (max kayıp aşıldı)
)

// Position 2D konum.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Velocity hız vektörü (piksel/kare).
type Velocity struct {
	VX float64 `json:"vx"`
	VY float64 `json:"vy"`
}

// Speed hız büyüklüğü.
func (v Velocity) Speed() float64 {
	return math.Sqrt(v.VX*v.VX + v.VY*v.VY)
}

// Heading hareket yönü (derece).
func (v Velocity) Heading() float64 {
	return math.Atan2(v.VY, v.VX) * 180 / math.Pi
}

// Track tek bir izleme kaydı.
type Track struct {
	mu sync.RWMutex

	ID          string                       `json:"id"`
	Class       detector.ObjectClass         `json:"class"`
	Allegiance  classifier.Allegiance        `json:"allegiance"`
	ThreatLevel classifier.ThreatLevel       `json:"threat_level"`
	ThreatScore float64                      `json:"threat_score"`
	Status      TrackStatus                  `json:"status"`
	Position    Position                     `json:"position"`
	Velocity    Velocity                     `json:"velocity"`
	BBox        detector.BoundingBox         `json:"bbox"`
	Confidence  float64                      `json:"confidence"`
	FirstSeen   time.Time                    `json:"first_seen"`
	LastSeen    time.Time                    `json:"last_seen"`
	UpdateCount int                          `json:"update_count"`
	LostFrames  int                          `json:"lost_frames"`
	History     []Position                   `json:"history,omitempty"`

	maxHistory int
}

// NewTrack yeni iz oluşturur.
func NewTrack(id string, det detector.Detection, classified classifier.ClassifiedObject, maxHist int) *Track {
	cx, cy := det.BBox.Center()
	return &Track{
		ID:          id,
		Class:       det.Class,
		Allegiance:  classified.Allegiance,
		ThreatLevel: classified.ThreatLevel,
		ThreatScore: classified.ThreatScore,
		Status:      StatusTentative,
		Position:    Position{X: cx, Y: cy},
		BBox:        det.BBox,
		Confidence:  det.Confidence,
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		UpdateCount: 1,
		maxHistory:  maxHist,
		History:     []Position{{X: cx, Y: cy}},
	}
}

// Update tespitle güncelle.
func (t *Track) Update(det detector.Detection, classified classifier.ClassifiedObject) {
	t.mu.Lock()
	defer t.mu.Unlock()

	cx, cy := det.BBox.Center()

	// Hız hesapla (önceki konumdan)
	t.Velocity = Velocity{
		VX: cx - t.Position.X,
		VY: cy - t.Position.Y,
	}

	t.Position = Position{X: cx, Y: cy}
	t.BBox = det.BBox
	t.Confidence = det.Confidence
	t.Allegiance = classified.Allegiance
	t.ThreatLevel = classified.ThreatLevel
	t.ThreatScore = classified.ThreatScore
	t.LastSeen = time.Now()
	t.UpdateCount++
	t.LostFrames = 0

	// Durum geçişi
	if t.Status == StatusTentative && t.UpdateCount >= 3 {
		t.Status = StatusConfirmed
	} else if t.Status == StatusLost {
		t.Status = StatusConfirmed
	}

	// Geçmiş
	t.History = append(t.History, Position{X: cx, Y: cy})
	if len(t.History) > t.maxHistory {
		t.History = t.History[len(t.History)-t.maxHistory:]
	}
}

// MarkLost kaybedildi olarak işaretle.
func (t *Track) MarkLost() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.LostFrames++
	if t.Status != StatusDeleted {
		t.Status = StatusLost
	}

	// Tahminle konum güncelle
	t.Position.X += t.Velocity.VX
	t.Position.Y += t.Velocity.VY
}

// Snapshot thread-safe snapshot.
func (t *Track) Snapshot() TrackSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	hist := make([]Position, len(t.History))
	copy(hist, t.History)

	return TrackSnapshot{
		ID:          t.ID,
		Class:       t.Class,
		Allegiance:  t.Allegiance,
		ThreatLevel: t.ThreatLevel,
		ThreatScore: t.ThreatScore,
		Status:      t.Status,
		Position:    t.Position,
		Velocity:    t.Velocity,
		Speed:       math.Round(t.Velocity.Speed()*100) / 100,
		Heading:     math.Round(t.Velocity.Heading()*10) / 10,
		BBox:        t.BBox,
		Confidence:  t.Confidence,
		FirstSeen:   t.FirstSeen,
		LastSeen:    t.LastSeen,
		UpdateCount: t.UpdateCount,
		LostFrames:  t.LostFrames,
		History:     hist,
	}
}

// TrackSnapshot değiştirilemez iz görüntüsü.
type TrackSnapshot struct {
	ID          string                       `json:"id"`
	Class       detector.ObjectClass         `json:"class"`
	Allegiance  classifier.Allegiance        `json:"allegiance"`
	ThreatLevel classifier.ThreatLevel       `json:"threat_level"`
	ThreatScore float64                      `json:"threat_score"`
	Status      TrackStatus                  `json:"status"`
	Position    Position                     `json:"position"`
	Velocity    Velocity                     `json:"velocity"`
	Speed       float64                      `json:"speed"`
	Heading     float64                      `json:"heading_deg"`
	BBox        detector.BoundingBox         `json:"bbox"`
	Confidence  float64                      `json:"confidence"`
	FirstSeen   time.Time                    `json:"first_seen"`
	LastSeen    time.Time                    `json:"last_seen"`
	UpdateCount int                          `json:"update_count"`
	LostFrames  int                          `json:"lost_frames"`
	History     []Position                   `json:"history,omitempty"`
}

// Engine çoklu nesne izleme motoru.
type Engine struct {
	mu sync.RWMutex

	tracks       map[string]*Track
	maxTracks    int
	maxLostFrames int
	iouThreshold float64
	historySize  int

	nextID int64

	// İstatistikler
	totalCreated  int64
	totalDeleted  int64
	totalUpdated  int64
}

// NewEngine yeni izleme motoru.
func NewEngine(maxTracks, maxLost int, iouThresh float64, histSize int) *Engine {
	return &Engine{
		tracks:        make(map[string]*Track),
		maxTracks:     maxTracks,
		maxLostFrames: maxLost,
		iouThreshold:  iouThresh,
		historySize:   histSize,
	}
}

// ProcessFrame bir karenin tespit ve sınıflandırma sonuçlarını izleyiciye besler.
func (e *Engine) ProcessFrame(detections []detector.Detection, classified []classifier.ClassifiedObject) []TrackSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Sınıflandırma haritası
	classMap := make(map[string]classifier.ClassifiedObject, len(classified))
	for _, c := range classified {
		classMap[c.DetectionID] = c
	}

	// Mevcut izler — eşleşmemiş olanlar
	matched := make(map[string]bool)    // track ID → eşleşti mi
	detMatched := make(map[int]bool)    // detection index → eşleşti mi

	// IoU bazlı eşleştirme (greedy)
	type match struct {
		trackID  string
		detIdx   int
		iou      float64
	}
	var candidates []match

	for tid, trk := range e.tracks {
		if trk.Status == StatusDeleted {
			continue
		}
		for di, det := range detections {
			iou := trk.BBox.IoU(det.BBox)
			if iou >= e.iouThreshold && trk.Class == det.Class {
				candidates = append(candidates, match{tid, di, iou})
			}
		}
	}

	// En yüksek IoU'dan başlayarak greedy eşleştirme
	for len(candidates) > 0 {
		bestIdx := 0
		for i := 1; i < len(candidates); i++ {
			if candidates[i].iou > candidates[bestIdx].iou {
				bestIdx = i
			}
		}
		best := candidates[bestIdx]

		if !matched[best.trackID] && !detMatched[best.detIdx] {
			det := detections[best.detIdx]
			cls := classMap[det.ID]
			e.tracks[best.trackID].Update(det, cls)
			atomic.AddInt64(&e.totalUpdated, 1)
			matched[best.trackID] = true
			detMatched[best.detIdx] = true
		}

		// Bu adayı sil
		candidates[bestIdx] = candidates[len(candidates)-1]
		candidates = candidates[:len(candidates)-1]
	}

	// Eşleşmeyen izler → kayıp
	for tid, trk := range e.tracks {
		if trk.Status == StatusDeleted {
			continue
		}
		if !matched[tid] {
			trk.MarkLost()
			if trk.LostFrames > e.maxLostFrames {
				trk.mu.Lock()
				trk.Status = StatusDeleted
				trk.mu.Unlock()
				atomic.AddInt64(&e.totalDeleted, 1)
			}
		}
	}

	// Eşleşmeyen tespitler → yeni iz
	for di, det := range detections {
		if detMatched[di] {
			continue
		}
		if len(e.tracks) >= e.maxTracks {
			break
		}
		id := fmt.Sprintf("TRK-%06d", atomic.AddInt64(&e.nextID, 1))
		cls := classMap[det.ID]
		e.tracks[id] = NewTrack(id, det, cls, e.historySize)
		atomic.AddInt64(&e.totalCreated, 1)
	}

	// Aktif iz snapshot'ları
	var result []TrackSnapshot
	for _, trk := range e.tracks {
		if trk.Status != StatusDeleted {
			result = append(result, trk.Snapshot())
		}
	}
	return result
}

// GetTrack belirli bir izi döndürür.
func (e *Engine) GetTrack(id string) (TrackSnapshot, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	trk, ok := e.tracks[id]
	if !ok || trk.Status == StatusDeleted {
		return TrackSnapshot{}, false
	}
	return trk.Snapshot(), true
}

// ActiveTracks aktif izleri döndürür.
func (e *Engine) ActiveTracks() []TrackSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []TrackSnapshot
	for _, trk := range e.tracks {
		if trk.Status != StatusDeleted {
			result = append(result, trk.Snapshot())
		}
	}
	return result
}

// HostileTracks düşman izleri.
func (e *Engine) HostileTracks() []TrackSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []TrackSnapshot
	for _, trk := range e.tracks {
		if trk.Status != StatusDeleted && trk.Allegiance == classifier.AllegianceHostile {
			result = append(result, trk.Snapshot())
		}
	}
	return result
}

// Prune silinmiş izleri temizle.
func (e *Engine) Prune() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	count := 0
	for id, trk := range e.tracks {
		if trk.Status == StatusDeleted {
			delete(e.tracks, id)
			count++
		}
	}
	return count
}

// Stats izleme istatistikleri.
type Stats struct {
	ActiveTracks   int   `json:"active_tracks"`
	TotalCreated   int64 `json:"total_created"`
	TotalDeleted   int64 `json:"total_deleted"`
	TotalUpdated   int64 `json:"total_updated"`
	ConfirmedCount int   `json:"confirmed_count"`
	TentativeCount int   `json:"tentative_count"`
	LostCount      int   `json:"lost_count"`
	HostileCount   int   `json:"hostile_count"`
	FriendlyCount  int   `json:"friendly_count"`
}

// GetStats istatistikleri döndürür.
func (e *Engine) GetStats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	s := Stats{
		TotalCreated: atomic.LoadInt64(&e.totalCreated),
		TotalDeleted: atomic.LoadInt64(&e.totalDeleted),
		TotalUpdated: atomic.LoadInt64(&e.totalUpdated),
	}

	for _, trk := range e.tracks {
		if trk.Status == StatusDeleted {
			continue
		}
		s.ActiveTracks++
		switch trk.Status {
		case StatusConfirmed:
			s.ConfirmedCount++
		case StatusTentative:
			s.TentativeCount++
		case StatusLost:
			s.LostCount++
		}
		if trk.Allegiance == classifier.AllegianceHostile {
			s.HostileCount++
		}
		if trk.Allegiance == classifier.AllegianceFriendly {
			s.FriendlyCount++
		}
	}
	return s
}
