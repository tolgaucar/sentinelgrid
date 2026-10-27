// Package imagery — Uydu görüntüsü yönetimi ve piksel analizi.
//
// SAR ve optik uydu görüntülerini zaman serisi olarak depolar,
// piksel bazlı fark haritası üretir ve histogram analizi yapar.
package imagery

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"sync"
	"time"
)

// ImageType — görüntü türü.
type ImageType string

const (
	TypeOptical ImageType = "optical"
	TypeSAR     ImageType = "sar"
)

// SatelliteImage — tek bir uydu görüntüsü.
type SatelliteImage struct {
	ID           string    `json:"id"`
	Type         ImageType `json:"type"`
	CaptureTime  time.Time `json:"capture_time"`
	CenterLat    float64   `json:"center_lat"`
	CenterLon    float64   `json:"center_lon"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	Resolution   float64   `json:"resolution_m"` // metre/piksel
	CloudCover   float64   `json:"cloud_cover"`  // 0-1 arası bulut oranı
	Pixels       [][]float64 `json:"-"`           // NxN piksel ızgarası (0-1)
	CreatedAt    time.Time `json:"created_at"`
}

// DiffMap — iki görüntü arasındaki piksel fark haritası.
type DiffMap struct {
	ID        string      `json:"id"`
	ImageA_ID string      `json:"image_a_id"`
	ImageB_ID string      `json:"image_b_id"`
	Width     int         `json:"width"`
	Height    int         `json:"height"`
	Pixels    [][]float64 `json:"-"`    // fark değerleri 0-1
	MeanDiff  float64     `json:"mean_diff"`
	MaxDiff   float64     `json:"max_diff"`
	StdDev    float64     `json:"std_dev"`
	ChangePct float64     `json:"change_pct"` // eşik üstü piksel yüzdesi
}

// Histogram — görüntü veya fark haritası histogramı.
type Histogram struct {
	Bins   []int   `json:"bins"`    // 10 kutu
	Edges  []float64 `json:"edges"` // kutu sınırları
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
}

// Engine — görüntü yönetim motoru.
type Engine struct {
	mu         sync.RWMutex
	images     map[string]*SatelliteImage
	maxImages  int
	gridSize   int
	diffThresh float64

	// istatistikler
	totalIngested int
	totalDiffs    int
	totalPruned   int
}

// NewEngine — yeni görüntü motoru.
func NewEngine(maxImages, gridSize int, diffThreshold float64) *Engine {
	return &Engine{
		images:     make(map[string]*SatelliteImage),
		maxImages:  maxImages,
		gridSize:   gridSize,
		diffThresh: diffThreshold,
	}
}

// ── Görüntü CRUD ──

// Ingest — yeni görüntü ekle.
func (e *Engine) Ingest(img SatelliteImage) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.images) >= e.maxImages {
		return fmt.Errorf("azami görüntü kapasitesi (%d) aşıldı", e.maxImages)
	}
	if _, ok := e.images[img.ID]; ok {
		return fmt.Errorf("görüntü zaten var: %s", img.ID)
	}
	img.CreatedAt = time.Now()
	e.images[img.ID] = &img
	e.totalIngested++
	return nil
}

// Get — görüntü getir.
func (e *Engine) Get(id string) (*SatelliteImage, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	img, ok := e.images[id]
	return img, ok
}

// GetAll — tüm görüntüler (pikseller hariç).
func (e *Engine) GetAll() []SatelliteImage {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]SatelliteImage, 0, len(e.images))
	for _, img := range e.images {
		cp := *img
		cp.Pixels = nil
		result = append(result, cp)
	}
	return result
}

// Delete — görüntü sil.
func (e *Engine) Delete(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.images[id]; !ok {
		return fmt.Errorf("görüntü bulunamadı: %s", id)
	}
	delete(e.images, id)
	return nil
}

// GetByRegion — belirli bölgedeki görüntüler.
func (e *Engine) GetByRegion(lat, lon, radiusKm float64) []SatelliteImage {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []SatelliteImage
	for _, img := range e.images {
		d := haversine(lat, lon, img.CenterLat, img.CenterLon)
		if d <= radiusKm {
			cp := *img
			cp.Pixels = nil
			result = append(result, cp)
		}
	}
	return result
}

// ── Fark Analizi ──

// ComputeDiff — iki görüntü arasındaki fark haritasını hesapla.
func (e *Engine) ComputeDiff(idA, idB string) (*DiffMap, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	imgA, okA := e.images[idA]
	imgB, okB := e.images[idB]
	if !okA {
		return nil, fmt.Errorf("görüntü bulunamadı: %s", idA)
	}
	if !okB {
		return nil, fmt.Errorf("görüntü bulunamadı: %s", idB)
	}

	w := min(imgA.Width, imgB.Width)
	h := min(imgA.Height, imgB.Height)
	if w == 0 || h == 0 {
		w, h = e.gridSize, e.gridSize
	}

	diff := make([][]float64, h)
	var sum, maxD float64
	aboveThreshold := 0

	for y := 0; y < h; y++ {
		diff[y] = make([]float64, w)
		for x := 0; x < w; x++ {
			var vA, vB float64
			if y < len(imgA.Pixels) && x < len(imgA.Pixels[y]) {
				vA = imgA.Pixels[y][x]
			}
			if y < len(imgB.Pixels) && x < len(imgB.Pixels[y]) {
				vB = imgB.Pixels[y][x]
			}
			d := math.Abs(vA - vB)
			diff[y][x] = d
			sum += d
			if d > maxD {
				maxD = d
			}
			if d >= e.diffThresh {
				aboveThreshold++
			}
		}
	}

	total := float64(w * h)
	meanDiff := sum / total

	// standart sapma
	var variance float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			d := diff[y][x] - meanDiff
			variance += d * d
		}
	}
	stdDev := math.Sqrt(variance / total)

	e.totalDiffs++

	dm := &DiffMap{
		ID:        fmt.Sprintf("diff-%s-%s", idA, idB),
		ImageA_ID: idA,
		ImageB_ID: idB,
		Width:     w,
		Height:    h,
		Pixels:    diff,
		MeanDiff:  meanDiff,
		MaxDiff:   maxD,
		StdDev:    stdDev,
		ChangePct: float64(aboveThreshold) / total,
	}
	return dm, nil
}

// ComputeHistogram — fark haritasının histogramını hesapla.
func ComputeHistogram(dm *DiffMap) Histogram {
	numBins := 10
	bins := make([]int, numBins)
	edges := make([]float64, numBins+1)
	for i := range edges {
		edges[i] = float64(i) / float64(numBins)
	}

	var sum float64
	count := 0
	for y := 0; y < dm.Height; y++ {
		for x := 0; x < dm.Width; x++ {
			v := dm.Pixels[y][x]
			sum += v
			count++
			bin := int(v * float64(numBins))
			if bin >= numBins {
				bin = numBins - 1
			}
			bins[bin]++
		}
	}

	mean := sum / float64(count)
	var variance float64
	for y := 0; y < dm.Height; y++ {
		for x := 0; x < dm.Width; x++ {
			d := dm.Pixels[y][x] - mean
			variance += d * d
		}
	}
	stdDev := math.Sqrt(variance / float64(count))

	return Histogram{Bins: bins, Edges: edges, Mean: mean, StdDev: stdDev}
}

// ── Sentetik Görüntü Üretimi (Simülasyon) ──

// GenerateFromEntities — savaş alanı varlıklarından sentetik uydu görüntüsü üret.
func (e *Engine) GenerateFromEntities(entities []map[string]any, imgType ImageType) *SatelliteImage {
	grid := e.gridSize
	pixels := make([][]float64, grid)
	for y := range pixels {
		pixels[y] = make([]float64, grid)
		for x := range pixels[y] {
			pixels[y][x] = 0.05 + randFloat()*0.10 // zemin gürültüsü
		}
	}

	var cLat, cLon float64
	for _, ent := range entities {
		lat, _ := toFloat(ent["lat"])
		lon, _ := toFloat(ent["lon"])
		cLat += lat
		cLon += lon

		// varlığı grid'e yerleştir
		gx := int((lon - 32.0) * 50) % grid
		gy := int((lat - 39.0) * 50) % grid
		if gx < 0 {
			gx = -gx
		}
		if gy < 0 {
			gy = -gy
		}
		gx = gx % grid
		gy = gy % grid

		faction, _ := ent["faction"].(string)
		entType, _ := ent["type"].(string)

		// düşman varlıklar daha yoğun iz bırakır
		intensity := 0.3 + randFloat()*0.3
		if faction == "enemy" {
			intensity += 0.2
		}
		// araç tipleri farklı boyutta iz bırakır
		radius := 1
		switch entType {
		case "tank", "apc", "artillery":
			radius = 2
			intensity += 0.15
		case "helicopter", "jet", "drone":
			radius = 1
			intensity += 0.10
		case "building", "base":
			radius = 3
			intensity += 0.25
		}

		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				py := (gy + dy + grid) % grid
				px := (gx + dx + grid) % grid
				pixels[py][px] = clamp(pixels[py][px]+intensity, 0, 1)
			}
		}
	}

	n := len(entities)
	if n > 0 {
		cLat /= float64(n)
		cLon /= float64(n)
	} else {
		cLat, cLon = 39.925, 32.837
	}

	cloud := 0.0
	if imgType == TypeOptical {
		cloud = randFloat() * 0.3
	}

	id := fmt.Sprintf("img-%s-%d", imgType, time.Now().UnixNano())

	return &SatelliteImage{
		ID:          id,
		Type:        imgType,
		CaptureTime: time.Now(),
		CenterLat:   cLat,
		CenterLon:   cLon,
		Width:       grid,
		Height:      grid,
		Resolution:  0.5,
		CloudCover:  cloud,
		Pixels:      pixels,
	}
}

// ── Bakım ──

// Prune — eski görüntüleri temizle.
func (e *Engine) Prune(maxAgeHours int) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	cutoff := time.Now().Add(-time.Duration(maxAgeHours) * time.Hour)
	removed := 0
	for id, img := range e.images {
		if img.CreatedAt.Before(cutoff) {
			delete(e.images, id)
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

	sarCount, optCount := 0, 0
	for _, img := range e.images {
		switch img.Type {
		case TypeSAR:
			sarCount++
		case TypeOptical:
			optCount++
		}
	}

	return map[string]any{
		"total_images":  len(e.images),
		"sar_images":    sarCount,
		"optical_images": optCount,
		"total_ingested": e.totalIngested,
		"total_diffs":    e.totalDiffs,
		"total_pruned":   e.totalPruned,
	}
}

// ── Yardımcılar ──

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func randFloat() float64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(10000))
	return float64(n.Int64()) / 10000.0
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
