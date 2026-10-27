// Package change — Değişim tespiti motoru.
//
// Uydu görüntü çiftleri arasındaki stratejik değişimleri tespit eder:
// siper kazılması, bina yıkımı, araç hareketi, mevzi inşası,
// orman tahribi, yol açılması, su kütlesi değişimi, tesis genişlemesi.
package change

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/otukenmatris/baskala/internal/imagery"
)

// ChangeType — değişim türü.
type ChangeType string

const (
	ChangeTrench      ChangeType = "trench_excavation"    // siper/hendek kazılması
	ChangeBuilding    ChangeType = "building_destruction"  // bina yıkımı
	ChangeVehicle     ChangeType = "vehicle_movement"      // araç hareketi
	ChangeFortify     ChangeType = "fortification"         // mevzi inşası
	ChangeForest      ChangeType = "deforestation"         // orman tahribi
	ChangeRoad        ChangeType = "road_construction"     // yol açılması
	ChangeWater       ChangeType = "water_body_change"     // su kütlesi değişimi
	ChangeFacility    ChangeType = "facility_expansion"    // tesis genişlemesi
)

// ThreatLevel — tehdit seviyesi.
type ThreatLevel string

const (
	ThreatNone     ThreatLevel = "none"
	ThreatLow      ThreatLevel = "low"
	ThreatMedium   ThreatLevel = "medium"
	ThreatHigh     ThreatLevel = "high"
	ThreatCritical ThreatLevel = "critical"
)

// ChangeDetection — tespit edilen tek bir değişim.
type ChangeDetection struct {
	ID          string      `json:"id"`
	Type        ChangeType  `json:"type"`
	ThreatLevel ThreatLevel `json:"threat_level"`
	Confidence  float64     `json:"confidence"`
	CenterLat   float64     `json:"center_lat"`
	CenterLon   float64     `json:"center_lon"`
	AreaSqKm    float64     `json:"area_sq_km"`
	Intensity   float64     `json:"intensity"`   // ortalama değişim yoğunluğu
	GridX       int         `json:"grid_x"`
	GridY       int         `json:"grid_y"`
	ImageA_ID   string      `json:"image_a_id"`
	ImageB_ID   string      `json:"image_b_id"`
	DetectedAt  time.Time   `json:"detected_at"`
	Description string      `json:"description"`
}

// CompareResult — karşılaştırma sonucu.
type CompareResult struct {
	Detections    []ChangeDetection `json:"detections"`
	TotalChanges  int               `json:"total_changes"`
	HighThreats   int               `json:"high_threats"`
	MeanDiff      float64           `json:"mean_diff"`
	ChangePct     float64           `json:"change_pct"`
	ElapsedMs     int64             `json:"elapsed_ms"`
}

// TimelineEntry — bölge bazlı zaman çizelgesi girişi.
type TimelineEntry struct {
	Region      string     `json:"region"`
	Lat         float64    `json:"lat"`
	Lon         float64    `json:"lon"`
	TotalEvents int        `json:"total_events"`
	Trend       string     `json:"trend"`  // rising, stable, declining
	LastType    ChangeType `json:"last_type"`
	LastSeen    time.Time  `json:"last_seen"`
	ThreatScore float64    `json:"threat_score"`
}

// Engine — değişim tespiti motoru.
type Engine struct {
	mu           sync.RWMutex
	detections   map[string]*ChangeDetection
	maxChanges   int
	confMin      float64
	diffThresh   float64
	clusterRadius float64
	threatDecayS int

	totalDetected int
	totalCompared int
	totalPruned   int
}

// NewEngine — yeni değişim motoru.
func NewEngine(maxChanges int, confMin, diffThresh, clusterRadius float64, threatDecayS int) *Engine {
	return &Engine{
		detections:    make(map[string]*ChangeDetection),
		maxChanges:    maxChanges,
		confMin:       confMin,
		diffThresh:    diffThresh,
		clusterRadius: clusterRadius,
		threatDecayS:  threatDecayS,
	}
}

// Compare — fark haritasından değişim tespit et.
func (e *Engine) Compare(dm *imagery.DiffMap, imgA, imgB *imagery.SatelliteImage) CompareResult {
	start := time.Now()

	// bölgeleri tespit et (kümeleme)
	regions := e.segmentRegions(dm)

	var detections []ChangeDetection
	highThreats := 0

	for _, reg := range regions {
		cType := classifyChange(reg, imgA, imgB)
		conf := computeConfidence(reg, dm)
		if conf < e.confMin {
			continue
		}

		lat, lon := gridToGeo(reg.centerY, reg.centerX, dm.Width, dm.Height, imgA.CenterLat, imgA.CenterLon)
		threat := assessThreat(cType, conf, reg.intensity)

		det := ChangeDetection{
			ID:          fmt.Sprintf("chg-%s-%d-%d", dm.ID, reg.centerX, reg.centerY),
			Type:        cType,
			ThreatLevel: threat,
			Confidence:  conf,
			CenterLat:   lat,
			CenterLon:   lon,
			AreaSqKm:    float64(reg.pixelCount) * imgA.Resolution * imgA.Resolution / 1e6,
			Intensity:   reg.intensity,
			GridX:       reg.centerX,
			GridY:       reg.centerY,
			ImageA_ID:   dm.ImageA_ID,
			ImageB_ID:   dm.ImageB_ID,
			DetectedAt:  time.Now(),
			Description: describeChange(cType, threat),
		}

		if threat == ThreatHigh || threat == ThreatCritical {
			highThreats++
		}

		detections = append(detections, det)
	}

	// kaydet
	e.mu.Lock()
	for i := range detections {
		if len(e.detections) < e.maxChanges {
			e.detections[detections[i].ID] = &detections[i]
		}
	}
	e.totalDetected += len(detections)
	e.totalCompared++
	e.mu.Unlock()

	return CompareResult{
		Detections:   detections,
		TotalChanges: len(detections),
		HighThreats:  highThreats,
		MeanDiff:     dm.MeanDiff,
		ChangePct:    dm.ChangePct,
		ElapsedMs:    time.Since(start).Milliseconds(),
	}
}

// ── Bölge Segmentasyonu ──

type region struct {
	centerX, centerY int
	pixelCount       int
	intensity        float64
	maxDiff          float64
	pattern          string // linear, irregular, cluster, rectangular
}

func (e *Engine) segmentRegions(dm *imagery.DiffMap) []region {
	visited := make([][]bool, dm.Height)
	for y := range visited {
		visited[y] = make([]bool, dm.Width)
	}

	var regions []region
	for y := 0; y < dm.Height; y++ {
		for x := 0; x < dm.Width; x++ {
			if visited[y][x] || dm.Pixels[y][x] < e.diffThresh {
				continue
			}
			// flood-fill ile bölge bul
			reg := e.floodFill(dm, visited, x, y)
			if reg.pixelCount >= 2 { // en az 2 piksel
				reg.pattern = detectPattern(dm, reg)
				regions = append(regions, reg)
			}
		}
	}
	return regions
}

func (e *Engine) floodFill(dm *imagery.DiffMap, visited [][]bool, startX, startY int) region {
	stack := [][2]int{{startX, startY}}
	var sumX, sumY float64
	var sumIntensity, maxD float64
	count := 0

	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		x, y := p[0], p[1]

		if x < 0 || x >= dm.Width || y < 0 || y >= dm.Height {
			continue
		}
		if visited[y][x] || dm.Pixels[y][x] < e.diffThresh {
			continue
		}

		visited[y][x] = true
		count++
		sumX += float64(x)
		sumY += float64(y)
		sumIntensity += dm.Pixels[y][x]
		if dm.Pixels[y][x] > maxD {
			maxD = dm.Pixels[y][x]
		}

		// 4 yön komşuluk
		stack = append(stack, [2]int{x + 1, y}, [2]int{x - 1, y}, [2]int{x, y + 1}, [2]int{x, y - 1})
	}

	return region{
		centerX:    int(sumX / float64(count)),
		centerY:    int(sumY / float64(count)),
		pixelCount: count,
		intensity:  sumIntensity / float64(count),
		maxDiff:    maxD,
	}
}

// ── Desen Tespiti ──

func detectPattern(dm *imagery.DiffMap, reg region) string {
	if reg.pixelCount <= 3 {
		return "cluster"
	}

	// aspect ratio ile şekil tahmini
	minX, maxX, minY, maxY := dm.Width, 0, dm.Height, 0
	for y := 0; y < dm.Height; y++ {
		for x := 0; x < dm.Width; x++ {
			if dm.Pixels[y][x] >= 0.15 {
				dx := x - reg.centerX
				dy := y - reg.centerY
				if dx*dx+dy*dy <= reg.pixelCount*2 {
					if x < minX { minX = x }
					if x > maxX { maxX = x }
					if y < minY { minY = y }
					if y > maxY { maxY = y }
				}
			}
		}
	}

	w := maxX - minX + 1
	h := maxY - minY + 1
	if w == 0 || h == 0 {
		return "cluster"
	}

	ratio := float64(w) / float64(h)
	if ratio > 3.0 || ratio < 0.33 {
		return "linear"
	}
	if ratio > 0.8 && ratio < 1.2 {
		return "rectangular"
	}
	return "irregular"
}

// ── Sınıflandırma ──

func classifyChange(reg region, imgA, imgB *imagery.SatelliteImage) ChangeType {
	switch reg.pattern {
	case "linear":
		if reg.intensity > 0.5 {
			return ChangeTrench
		}
		return ChangeRoad
	case "rectangular":
		if reg.intensity > 0.6 {
			return ChangeFortify
		}
		return ChangeFacility
	case "irregular":
		if reg.intensity > 0.7 {
			return ChangeBuilding
		}
		if reg.pixelCount > 10 {
			return ChangeForest
		}
		return ChangeWater
	default: // cluster
		if reg.pixelCount <= 4 {
			return ChangeVehicle
		}
		return ChangeFortify
	}
}

// ── Güven Skoru ──

func computeConfidence(reg region, dm *imagery.DiffMap) float64 {
	base := reg.intensity * 0.6
	sizeBonus := math.Min(float64(reg.pixelCount)*0.03, 0.2)
	contrastBonus := reg.maxDiff * 0.2
	return math.Min(base+sizeBonus+contrastBonus, 1.0)
}

// ── Tehdit Değerlendirmesi ──

func assessThreat(cType ChangeType, confidence, intensity float64) ThreatLevel {
	score := confidence * 0.5

	switch cType {
	case ChangeTrench, ChangeFortify:
		score += 0.35
	case ChangeBuilding:
		score += 0.25
	case ChangeVehicle:
		score += 0.20
	case ChangeRoad, ChangeFacility:
		score += 0.15
	case ChangeForest, ChangeWater:
		score += 0.05
	}

	score += intensity * 0.15

	switch {
	case score >= 0.8:
		return ThreatCritical
	case score >= 0.6:
		return ThreatHigh
	case score >= 0.4:
		return ThreatMedium
	case score >= 0.2:
		return ThreatLow
	default:
		return ThreatNone
	}
}

func describeChange(cType ChangeType, threat ThreatLevel) string {
	descriptions := map[ChangeType]string{
		ChangeTrench:   "Siper/hendek kazılması tespit edildi",
		ChangeBuilding: "Bina yıkımı veya hasar tespit edildi",
		ChangeVehicle:  "Araç hareketi/konvoy tespit edildi",
		ChangeFortify:  "Mevzi inşası tespit edildi",
		ChangeForest:   "Orman tahribi/arazi değişikliği tespit edildi",
		ChangeRoad:     "Yol yapımı/genişletme tespit edildi",
		ChangeWater:    "Su kütlesi değişimi tespit edildi",
		ChangeFacility: "Tesis genişlemesi tespit edildi",
	}
	desc := descriptions[cType]
	if threat == ThreatHigh || threat == ThreatCritical {
		desc += " [YÜKSEK TEHDİT]"
	}
	return desc
}

// ── Coğrafi Dönüşüm ──

func gridToGeo(gridY, gridX, width, height int, centerLat, centerLon float64) (float64, float64) {
	// grid merkezinden piksel uzaklığını coğrafi koordinata dönüştür
	dx := float64(gridX-width/2) * 0.0001
	dy := float64(gridY-height/2) * 0.0001
	return centerLat + dy, centerLon + dx
}

// ── Sorgu ──

// GetAll — tüm değişimler.
func (e *Engine) GetAll() []ChangeDetection {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]ChangeDetection, 0, len(e.detections))
	for _, d := range e.detections {
		result = append(result, *d)
	}
	return result
}

// Get — belirli değişim.
func (e *Engine) Get(id string) (*ChangeDetection, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	d, ok := e.detections[id]
	return d, ok
}

// GetHighThreat — yüksek tehdit değişimler.
func (e *Engine) GetHighThreat() []ChangeDetection {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []ChangeDetection
	for _, d := range e.detections {
		if d.ThreatLevel == ThreatHigh || d.ThreatLevel == ThreatCritical {
			result = append(result, *d)
		}
	}
	return result
}

// GetTimeline — bölge bazlı zaman çizelgesi.
func (e *Engine) GetTimeline() []TimelineEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// coğrafi kümeleme (basit grid bucketing)
	type bucket struct {
		lat, lon    float64
		events      int
		lastType    ChangeType
		lastSeen    time.Time
		threatScore float64
	}

	buckets := make(map[string]*bucket)
	for _, d := range e.detections {
		key := fmt.Sprintf("%.2f_%.2f", d.CenterLat, d.CenterLon)
		b, ok := buckets[key]
		if !ok {
			b = &bucket{lat: d.CenterLat, lon: d.CenterLon}
			buckets[key] = b
		}
		b.events++
		if d.DetectedAt.After(b.lastSeen) {
			b.lastSeen = d.DetectedAt
			b.lastType = d.Type
		}
		b.threatScore += d.Confidence
	}

	var timeline []TimelineEntry
	for key, b := range buckets {
		trend := "stable"
		if b.events > 5 {
			trend = "rising"
		} else if b.events <= 1 {
			trend = "declining"
		}

		timeline = append(timeline, TimelineEntry{
			Region:      key,
			Lat:         b.lat,
			Lon:         b.lon,
			TotalEvents: b.events,
			Trend:       trend,
			LastType:    b.lastType,
			LastSeen:    b.lastSeen,
			ThreatScore: b.threatScore / float64(b.events),
		})
	}
	return timeline
}

// ── Bakım ──

// Prune — eski değişimleri temizle.
func (e *Engine) Prune() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	cutoff := time.Now().Add(-time.Duration(e.threatDecayS) * time.Second)
	removed := 0
	for id, d := range e.detections {
		if d.DetectedAt.Before(cutoff) {
			delete(e.detections, id)
			removed++
		}
	}
	e.totalPruned += removed
	return removed
}

// Stats — istatistikler.
func (e *Engine) Stats() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()

	highCount := 0
	byType := make(map[string]int)
	for _, d := range e.detections {
		byType[string(d.Type)]++
		if d.ThreatLevel == ThreatHigh || d.ThreatLevel == ThreatCritical {
			highCount++
		}
	}

	return map[string]any{
		"active_changes":  len(e.detections),
		"high_threats":    highCount,
		"total_detected":  e.totalDetected,
		"total_compared":  e.totalCompared,
		"total_pruned":    e.totalPruned,
		"by_type":         byType,
	}
}
