// Package detector — kamuflaj tespit motoru.
//
// Doku analizi sonuçlarını değerlendirerek kamuflajlı nesneleri tespit eder,
// sınıflandırır ve takip altına alır.
package detector

import (
	"math"
	"sync"
	"time"

	"github.com/otukenmatris/sezi/internal/texture"
)

// CamoType — kamuflaj türü.
type CamoType string

const (
	CamoNet       CamoType = "camo_net"       // kamuflaj ağı
	CamoTarp      CamoType = "camo_tarp"      // kamuflaj branda
	CamoPaint     CamoType = "camo_paint"      // boyama kamuflajı
	CamoFoliage   CamoType = "camo_foliage"    // dal/yaprak örtüsü
	CamoDugout    CamoType = "camo_dugout"      // kazılmış mevzi/siper
	CamoThermal   CamoType = "camo_thermal"     // termal kamuflaj örtüsü
	CamoDecoy     CamoType = "camo_decoy"       // sahte hedef/maket
	CamoNatural   CamoType = "camo_natural"     // doğal gizlenme (mağara, ağaç altı)
)

// ThreatLevel — tehdit seviyesi.
type ThreatLevel string

const (
	ThreatLow      ThreatLevel = "low"
	ThreatMedium   ThreatLevel = "medium"
	ThreatHigh     ThreatLevel = "high"
	ThreatCritical ThreatLevel = "critical"
)

// Detection — kamuflaj tespiti.
type Detection struct {
	ID             string              `json:"id"`
	RegionID       string              `json:"region_id"`
	CamoType       CamoType            `json:"camo_type"`
	Confidence     float64             `json:"confidence"`      // 0.0-1.0
	ThreatLevel    ThreatLevel         `json:"threat_level"`
	Lat            float64             `json:"lat"`
	Lon            float64             `json:"lon"`
	Terrain        texture.TerrainType `json:"terrain"`
	Divergence     float64             `json:"divergence"`
	ConvexScore    float64             `json:"convex_score"`
	ConcaveScore   float64             `json:"concave_score"`
	EstimatedSize  string              `json:"estimated_size"`  // "small", "medium", "large"
	Description    string              `json:"description"`
	DetectedAt     time.Time           `json:"detected_at"`
	LastUpdated    time.Time           `json:"last_updated"`
}

// DetectionResult — toplu tespit sonucu.
type DetectionResult struct {
	Detections   []Detection `json:"detections"`
	TotalRegions int         `json:"total_regions"`
	Anomalies    int         `json:"anomalies"`
	ElapsedMs    float64     `json:"elapsed_ms"`
}

// ScanInput — tarama girdisi (doku analizi öncesi).
type ScanInput struct {
	ID      string              `json:"id"`
	Lat     float64             `json:"lat"`
	Lon     float64             `json:"lon"`
	Terrain texture.TerrainType `json:"terrain"`
	Width   int                 `json:"width"`
	Height  int                 `json:"height"`
}

// Engine — kamuflaj tespit motoru.
type Engine struct {
	mu               sync.RWMutex
	textureEngine    *texture.Engine
	detections       map[string]*Detection
	maxAge           int
	confidenceMin    float64
	convexThreshold  float64
	concaveThreshold float64
	maxDetections    int
	scanCount        int64
	detectCount      int64
}

// NewEngine — yeni kamuflaj tespit motoru.
func NewEngine(
	texEngine *texture.Engine,
	maxAge int,
	confidenceMin float64,
	convexThreshold float64,
	concaveThreshold float64,
	maxDetections int,
) *Engine {
	return &Engine{
		textureEngine:    texEngine,
		detections:       make(map[string]*Detection),
		maxAge:           maxAge,
		confidenceMin:    confidenceMin,
		convexThreshold:  convexThreshold,
		concaveThreshold: concaveThreshold,
		maxDetections:    maxDetections,
	}
}

// Scan — bölge listesinden kamuflaj taraması yap.
func (e *Engine) Scan(inputs []ScanInput) DetectionResult {
	start := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()

	e.scanCount++

	// ScanInput → RegionInput dönüşümü
	regions := make([]texture.RegionInput, len(inputs))
	for i, inp := range inputs {
		regions[i] = texture.RegionInput{
			ID:      inp.ID,
			Lat:     inp.Lat,
			Lon:     inp.Lon,
			Terrain: inp.Terrain,
			Width:   inp.Width,
			Height:  inp.Height,
		}
	}

	// doku analizi
	analysisResults := e.textureEngine.Analyze(regions)

	var detected []Detection
	anomalyCount := 0

	for i, ar := range analysisResults {
		if !ar.IsAnomaly {
			continue
		}
		anomalyCount++

		// kamuflaj türünü belirle
		camoType := e.classifyCamoType(ar)

		// güven hesapla
		confidence := e.computeConfidence(ar)
		if confidence < e.confidenceMin {
			continue
		}

		// tehdit seviyesi belirle
		threatLevel := e.assessThreat(confidence, camoType)

		// tahmini boyut
		size := estimateSize(ar.ConvexScore, ar.ConcaveScore)

		det := Detection{
			ID:            "cm-" + inputs[i].ID,
			RegionID:      ar.RegionID,
			CamoType:      camoType,
			Confidence:    math.Round(confidence*1000) / 1000,
			ThreatLevel:   threatLevel,
			Lat:           inputs[i].Lat,
			Lon:           inputs[i].Lon,
			Terrain:       ar.Terrain,
			Divergence:    ar.Divergence,
			ConvexScore:   ar.ConvexScore,
			ConcaveScore:  ar.ConcaveScore,
			EstimatedSize: size,
			Description:   describeCamo(camoType, ar.Terrain, confidence),
			DetectedAt:    time.Now(),
			LastUpdated:   time.Now(),
		}

		// güncelle veya ekle
		if existing, ok := e.detections[det.ID]; ok {
			existing.Confidence = det.Confidence
			existing.ThreatLevel = det.ThreatLevel
			existing.Divergence = det.Divergence
			existing.ConvexScore = det.ConvexScore
			existing.ConcaveScore = det.ConcaveScore
			existing.LastUpdated = det.LastUpdated
			detected = append(detected, *existing)
		} else {
			e.detections[det.ID] = &det
			detected = append(detected, det)
			e.detectCount++
		}
	}

	// eski tespitleri temizle (maxDetections aşarsa)
	if len(e.detections) > e.maxDetections {
		e.pruneOldest(len(e.detections) - e.maxDetections)
	}

	return DetectionResult{
		Detections:   detected,
		TotalRegions: len(inputs),
		Anomalies:    anomalyCount,
		ElapsedMs:    float64(time.Since(start).Microseconds()) / 1000.0,
	}
}

// classifyCamoType — doku analiz sonucuna göre kamuflaj türü belirle.
func (e *Engine) classifyCamoType(ar texture.AnalysisResult) CamoType {
	// dışbükey dominant — kamuflaj ağı/branda/boyama
	if ar.ConvexScore > ar.ConcaveScore {
		if ar.Feature.Homogeneity > 0.7 {
			return CamoTarp // yüksek homojenlik = branda
		}
		if ar.Feature.Correlation > 0.6 {
			return CamoPaint // yüksek korelasyon = düzenli desen (boyama)
		}
		if ar.Feature.Entropy > 0.6 {
			return CamoFoliage // yüksek entropi = dal/yaprak
		}
		return CamoNet // varsayılan dışbükey = kamuflaj ağı
	}

	// içbükey dominant — kazılmış mevzi/siper
	if ar.ConcaveScore > e.concaveThreshold {
		if ar.Feature.Energy < 0.2 {
			return CamoNatural // çok düşük enerji = mağara/doğal gizlenme
		}
		return CamoDugout
	}

	// her ikisi de yüksek — termal kamuflaj veya sahte hedef
	if ar.ConvexScore > 0.4 && ar.ConcaveScore > 0.3 {
		if ar.Divergence > 0.7 {
			return CamoDecoy
		}
		return CamoThermal
	}

	return CamoNet
}

// computeConfidence — tespit güveni hesapla.
func (e *Engine) computeConfidence(ar texture.AnalysisResult) float64 {
	// temel güven: sapma skoru
	base := ar.Divergence

	// konveks/konkav güçlendirme
	shapeBoost := math.Max(ar.ConvexScore, ar.ConcaveScore) * 0.3

	// yüksek varyans sapması = daha güvenli tespit
	varianceBoost := 0.0
	if ar.Feature.Variance > 0 {
		varianceBoost = math.Min(0.2, math.Abs(ar.Feature.Variance-ar.Background.Variance)*2.0)
	}

	confidence := base*0.5 + shapeBoost + varianceBoost
	return math.Min(1.0, math.Max(0, confidence))
}

// assessThreat — tehdit seviyesi belirle.
func (e *Engine) assessThreat(confidence float64, camoType CamoType) ThreatLevel {
	// yüksek güven + tehlikeli kamuflaj türü
	threatWeight := confidence

	switch camoType {
	case CamoDugout:
		threatWeight *= 1.3 // mevzi = askeri varlık
	case CamoTarp, CamoNet:
		threatWeight *= 1.2 // araç gizleme
	case CamoThermal:
		threatWeight *= 1.4 // gelişmiş kamuflaj
	case CamoDecoy:
		threatWeight *= 0.8 // sahte hedef — yanıltma riski
	case CamoNatural:
		threatWeight *= 0.9 // doğal gizlenme
	case CamoFoliage:
		threatWeight *= 1.1
	case CamoPaint:
		threatWeight *= 1.0
	}

	switch {
	case threatWeight >= 0.85:
		return ThreatCritical
	case threatWeight >= 0.65:
		return ThreatHigh
	case threatWeight >= 0.40:
		return ThreatMedium
	default:
		return ThreatLow
	}
}

// GetDetection — tek tespit getir.
func (e *Engine) GetDetection(id string) (*Detection, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	det, ok := e.detections[id]
	if !ok {
		return nil, false
	}
	result := *det
	return &result, true
}

// GetAllDetections — tüm aktif tespitler.
func (e *Engine) GetAllDetections() []Detection {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]Detection, 0, len(e.detections))
	for _, det := range e.detections {
		result = append(result, *det)
	}
	return result
}

// GetByThreat — tehdit seviyesine göre tespitler.
func (e *Engine) GetByThreat(level ThreatLevel) []Detection {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []Detection
	for _, det := range e.detections {
		if det.ThreatLevel == level {
			result = append(result, *det)
		}
	}
	return result
}

// GetHighThreat — yüksek ve kritik tehdit tespitleri.
func (e *Engine) GetHighThreat() []Detection {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []Detection
	for _, det := range e.detections {
		if det.ThreatLevel == ThreatHigh || det.ThreatLevel == ThreatCritical {
			result = append(result, *det)
		}
	}
	return result
}

// Prune — süresi dolmuş tespitleri temizle.
func (e *Engine) Prune() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	removed := 0
	for id, det := range e.detections {
		if now.Sub(det.LastUpdated).Seconds() > float64(e.maxAge) {
			delete(e.detections, id)
			removed++
		}
	}
	return removed
}

// pruneOldest — en eski N tespiti temizle.
func (e *Engine) pruneOldest(n int) {
	if n <= 0 || len(e.detections) == 0 {
		return
	}
	type entry struct {
		id string
		t  time.Time
	}
	entries := make([]entry, 0, len(e.detections))
	for id, det := range e.detections {
		entries = append(entries, entry{id, det.LastUpdated})
	}
	// basit seçim: en eski n tanesini sil
	for i := 0; i < n && i < len(entries); i++ {
		oldest := i
		for j := i + 1; j < len(entries); j++ {
			if entries[j].t.Before(entries[oldest].t) {
				oldest = j
			}
		}
		entries[i], entries[oldest] = entries[oldest], entries[i]
		delete(e.detections, entries[i].id)
	}
}

// Stats — motor istatistikleri.
func (e *Engine) Stats() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return map[string]any{
		"scan_count":       e.scanCount,
		"detect_count":     e.detectCount,
		"active_detections": len(e.detections),
		"confidence_min":   e.confidenceMin,
	}
}

func estimateSize(convex, concave float64) string {
	total := convex + concave
	switch {
	case total > 1.2:
		return "large"
	case total > 0.7:
		return "medium"
	default:
		return "small"
	}
}

func describeCamo(camoType CamoType, terrain texture.TerrainType, confidence float64) string {
	var desc string
	switch camoType {
	case CamoNet:
		desc = "Kamuflaj ağı tespit edildi"
	case CamoTarp:
		desc = "Kamuflaj branda/örtü tespit edildi"
	case CamoPaint:
		desc = "Boyama kamuflajı tespit edildi"
	case CamoFoliage:
		desc = "Dal/yaprak örtüsü kamuflajı tespit edildi"
	case CamoDugout:
		desc = "Kazılmış mevzi/siper tespit edildi"
	case CamoThermal:
		desc = "Termal kamuflaj örtüsü tespit edildi"
	case CamoDecoy:
		desc = "Sahte hedef/maket tespit edildi"
	case CamoNatural:
		desc = "Doğal gizlenme noktası tespit edildi"
	default:
		desc = "Kamuflaj tespit edildi"
	}

	terrainDesc := ""
	switch terrain {
	case texture.TerrainForest:
		terrainDesc = " — ormanlık arazi"
	case texture.TerrainRocky:
		terrainDesc = " — kayalık arazi"
	case texture.TerrainDesert:
		terrainDesc = " — çöl arazisi"
	case texture.TerrainUrban:
		terrainDesc = " — kentsel alan"
	}

	confDesc := ""
	if confidence > 0.8 {
		confDesc = " (yüksek güven)"
	} else if confidence > 0.5 {
		confDesc = " (orta güven)"
	} else {
		confDesc = " (düşük güven)"
	}

	return desc + terrainDesc + confDesc
}
