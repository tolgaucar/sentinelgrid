// Package detector — YOLO tabanlı nesne tespit motoru.
//
// TANI modülünün çekirdeği. Sahadaki video/kare akışlarını işleyerek
// hedefleri tespit eder. Simülasyon ortamında gerçekçi tespit mantığı:
// - Bounding box hesaplama
// - Güven skoru üretimi
// - Non-Maximum Suppression (NMS)
// - Çoklu nesne eşzamanlı tespiti
package detector

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ObjectClass tespit edilen nesne sınıfı.
type ObjectClass string

const (
	ClassTank       ObjectClass = "tank"
	ClassAPC        ObjectClass = "apc"           // Zırhlı personel taşıyıcı
	ClassTruck      ObjectClass = "truck"
	ClassHelicopter ObjectClass = "helicopter"
	ClassFighterJet ObjectClass = "fighter_jet"
	ClassDrone      ObjectClass = "drone"
	ClassSoldier    ObjectClass = "soldier"
	ClassSniper     ObjectClass = "sniper"
	ClassCivilian   ObjectClass = "civilian"
	ClassRadar      ObjectClass = "radar"
	ClassSAM        ObjectClass = "sam"           // Hava savunma
	ClassArtillery  ObjectClass = "artillery"
	ClassCommandPost ObjectClass = "command_post"
	ClassSupplyDepot ObjectClass = "supply_depot"
	ClassUnknown    ObjectClass = "unknown"
)

// AllClasses tüm tanımlı nesne sınıfları.
var AllClasses = []ObjectClass{
	ClassTank, ClassAPC, ClassTruck, ClassHelicopter, ClassFighterJet,
	ClassDrone, ClassSoldier, ClassSniper, ClassCivilian,
	ClassRadar, ClassSAM, ClassArtillery, ClassCommandPost, ClassSupplyDepot,
}

// classCharacteristics her sınıfın tespit edilme olasılığı ve boyut aralığı.
var classCharacteristics = map[ObjectClass]struct {
	minSize, maxSize float64 // piksel genişlik
	frequency        float64 // sahne başına çıkma olasılığı
}{
	ClassTank:        {40, 120, 0.15},
	ClassAPC:         {35, 100, 0.12},
	ClassTruck:       {30, 90, 0.20},
	ClassHelicopter:  {25, 80, 0.08},
	ClassFighterJet:  {15, 60, 0.05},
	ClassDrone:       {8, 30, 0.10},
	ClassSoldier:     {5, 25, 0.25},
	ClassSniper:      {4, 20, 0.03},
	ClassCivilian:    {5, 25, 0.15},
	ClassRadar:       {20, 60, 0.06},
	ClassSAM:         {25, 70, 0.04},
	ClassArtillery:   {30, 80, 0.07},
	ClassCommandPost: {40, 100, 0.03},
	ClassSupplyDepot: {50, 120, 0.05},
}

// BoundingBox tespit kutusu (piksel koordinatları).
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Area kutu alanı.
func (b BoundingBox) Area() float64 {
	return b.Width * b.Height
}

// IoU iki kutu arasındaki Intersection-over-Union.
func (b BoundingBox) IoU(other BoundingBox) float64 {
	x1 := math.Max(b.X, other.X)
	y1 := math.Max(b.Y, other.Y)
	x2 := math.Min(b.X+b.Width, other.X+other.Width)
	y2 := math.Min(b.Y+b.Height, other.Y+other.Height)

	if x2 <= x1 || y2 <= y1 {
		return 0
	}
	intersection := (x2 - x1) * (y2 - y1)
	union := b.Area() + other.Area() - intersection
	if union <= 0 {
		return 0
	}
	return intersection / union
}

// Center kutu merkezi.
func (b BoundingBox) Center() (float64, float64) {
	return b.X + b.Width/2, b.Y + b.Height/2
}

// Detection tek bir tespit sonucu.
type Detection struct {
	ID         string      `json:"id"`
	Class      ObjectClass `json:"class"`
	Confidence float64     `json:"confidence"`
	BBox       BoundingBox `json:"bbox"`
	Timestamp  time.Time   `json:"timestamp"`
	FrameID    string      `json:"frame_id"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// Frame video/görüntü karesi.
type Frame struct {
	ID        string    `json:"id"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Timestamp time.Time `json:"timestamp"`
	SourceID  string    `json:"source_id"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// DetectionResult bir kare üzerindeki tüm tespit sonuçları.
type DetectionResult struct {
	FrameID       string      `json:"frame_id"`
	Detections    []Detection `json:"detections"`
	Count         int         `json:"count"`
	InferenceMs   float64     `json:"inference_ms"`
	Timestamp     time.Time   `json:"timestamp"`
	ModelName     string      `json:"model_name"`
	Suppressed    int         `json:"suppressed"`
}

// Engine YOLO tabanlı nesne tespit motoru.
type Engine struct {
	mu sync.RWMutex

	confThreshold float64
	nmsThreshold  float64
	maxDetections int
	inputW, inputH int
	modelName     string

	// İstatistikler
	totalFrames   int64
	totalDetects  int64
	totalSuppress int64
	lastResult    *DetectionResult

	rng *rand.Rand
}

// NewEngine yeni tespit motoru oluşturur.
func NewEngine(confThreshold, nmsThreshold float64, maxDet, w, h int, model string) *Engine {
	return &Engine{
		confThreshold: confThreshold,
		nmsThreshold:  nmsThreshold,
		maxDetections: maxDet,
		inputW:        w,
		inputH:        h,
		modelName:     model,
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Detect bir karede nesne tespiti yapar (simülasyon).
//
// Gerçek dünyada: YOLO modeli GPU'da çalıştırılır, kare tensör'e dönüştürülür,
// model çıktısı yorumlanır. Simülasyon ortamında gerçekçi tespit üretilir.
func (e *Engine) Detect(frame Frame) *DetectionResult {
	start := time.Now()
	atomic.AddInt64(&e.totalFrames, 1)

	e.mu.Lock()
	rng := e.rng
	e.mu.Unlock()

	// Simüle edilmiş tespitler üret
	var rawDetections []Detection
	for _, cls := range AllClasses {
		chars := classCharacteristics[cls]

		// Bu sınıftan nesne var mı?
		if rng.Float64() > chars.frequency {
			continue
		}

		// Kaç tane?
		count := 1
		if cls == ClassSoldier || cls == ClassCivilian {
			count = 1 + rng.Intn(4) // Personel genelde grup halinde
		}

		for i := 0; i < count; i++ {
			w := chars.minSize + rng.Float64()*(chars.maxSize-chars.minSize)
			h := w * (0.7 + rng.Float64()*0.6) // En-boy oranı

			// Güven skoru: sınıfa göre değişen baz + rastgele
			baseConf := 0.5 + rng.Float64()*0.45
			// Küçük nesnelerin güveni düşük
			sizeFactor := math.Min(w/50.0, 1.0)
			conf := baseConf * (0.7 + 0.3*sizeFactor)

			if conf < e.confThreshold {
				continue
			}

			maxX := float64(frame.Width) - w
			maxY := float64(frame.Height) - h
			if maxX <= 0 {
				maxX = 1
			}
			if maxY <= 0 {
				maxY = 1
			}

			det := Detection{
				ID:         fmt.Sprintf("det-%s-%d-%d", frame.ID, len(rawDetections), i),
				Class:      cls,
				Confidence: math.Round(conf*1000) / 1000,
				BBox: BoundingBox{
					X:      rng.Float64() * maxX,
					Y:      rng.Float64() * maxY,
					Width:  w,
					Height: h,
				},
				Timestamp: frame.Timestamp,
				FrameID:   frame.ID,
			}
			rawDetections = append(rawDetections, det)
		}
	}

	// NMS uygula
	filtered := e.nms(rawDetections)
	suppressed := len(rawDetections) - len(filtered)

	// Maksimum tespit limiti
	if len(filtered) > e.maxDetections {
		filtered = filtered[:e.maxDetections]
	}

	elapsed := time.Since(start)
	result := &DetectionResult{
		FrameID:     frame.ID,
		Detections:  filtered,
		Count:       len(filtered),
		InferenceMs: float64(elapsed.Microseconds()) / 1000.0,
		Timestamp:   time.Now(),
		ModelName:   e.modelName,
		Suppressed:  suppressed,
	}

	atomic.AddInt64(&e.totalDetects, int64(len(filtered)))
	atomic.AddInt64(&e.totalSuppress, int64(suppressed))

	e.mu.Lock()
	e.lastResult = result
	e.mu.Unlock()

	return result
}

// nms Non-Maximum Suppression — örtüşen kutuları eler.
func (e *Engine) nms(detections []Detection) []Detection {
	if len(detections) == 0 {
		return nil
	}

	// Güvene göre azalan sırala
	sort.Slice(detections, func(i, j int) bool {
		return detections[i].Confidence > detections[j].Confidence
	})

	kept := make([]bool, len(detections))
	for i := range kept {
		kept[i] = true
	}

	for i := 0; i < len(detections); i++ {
		if !kept[i] {
			continue
		}
		for j := i + 1; j < len(detections); j++ {
			if !kept[j] {
				continue
			}
			// Aynı sınıf ve yüksek IoU → düşük güvenliyi ele
			if detections[i].Class == detections[j].Class {
				iou := detections[i].BBox.IoU(detections[j].BBox)
				if iou > e.nmsThreshold {
					kept[j] = false
				}
			}
		}
	}

	var result []Detection
	for i, k := range kept {
		if k {
			result = append(result, detections[i])
		}
	}
	return result
}

// Stats tespit istatistikleri.
type Stats struct {
	TotalFrames    int64   `json:"total_frames"`
	TotalDetections int64  `json:"total_detections"`
	TotalSuppressed int64  `json:"total_suppressed"`
	AvgDetPerFrame float64 `json:"avg_detections_per_frame"`
	ModelName      string  `json:"model_name"`
	ConfThreshold  float64 `json:"confidence_threshold"`
	NMSThreshold   float64 `json:"nms_threshold"`
}

// GetStats istatistikleri döndürür.
func (e *Engine) GetStats() Stats {
	frames := atomic.LoadInt64(&e.totalFrames)
	detects := atomic.LoadInt64(&e.totalDetects)
	suppressed := atomic.LoadInt64(&e.totalSuppress)

	avg := 0.0
	if frames > 0 {
		avg = float64(detects) / float64(frames)
	}

	return Stats{
		TotalFrames:     frames,
		TotalDetections: detects,
		TotalSuppressed: suppressed,
		AvgDetPerFrame:  math.Round(avg*100) / 100,
		ModelName:       e.modelName,
		ConfThreshold:   e.confThreshold,
		NMSThreshold:    e.nmsThreshold,
	}
}

// LastResult son tespit sonucu.
func (e *Engine) LastResult() *DetectionResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastResult
}
