// Package texture — Gabor dalgacıkları ile doku analiz motoru.
//
// Görüntü bloklarından doku özellikleri çıkarır, istatistiksel varyans
// modelleri üzerinden doğal/yapay doku ayrımı yapar.
// Kamuflajlı nesnelerin arkaplan dokusuyla uyumsuzluğunu tespit eder.
package texture

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// TerrainType — arazi türü.
type TerrainType string

const (
	TerrainForest TerrainType = "forest"  // ormanlık
	TerrainRocky  TerrainType = "rocky"   // kayalık
	TerrainDesert TerrainType = "desert"  // çöl
	TerrainUrban  TerrainType = "urban"   // kentsel
	TerrainGrass  TerrainType = "grass"   // çayırlık
	TerrainSnow   TerrainType = "snow"    // karlı
	TerrainWater  TerrainType = "water"   // su
)

// GaborParams — tek bir Gabor filtre parametresi.
type GaborParams struct {
	Frequency   float64 `json:"frequency"`
	Orientation float64 `json:"orientation"` // radyan
	Scale       int     `json:"scale"`
}

// TextureFeature — bir bölgenin doku özellik vektörü.
type TextureFeature struct {
	Energy      float64 `json:"energy"`       // Gabor enerji
	Contrast    float64 `json:"contrast"`     // doku kontrast
	Homogeneity float64 `json:"homogeneity"`  // homojenlik
	Entropy     float64 `json:"entropy"`      // doku entropi
	Variance    float64 `json:"variance"`     // varyans (anahtar metrik)
	Correlation float64 `json:"correlation"`  // otokorelasyon
	Orientation float64 `json:"orientation"`  // baskın yönelim (radyan)
	Frequency   float64 `json:"frequency"`    // baskın frekans
}

// TextureProfile — bilinen arazi türü doku profili.
type TextureProfile struct {
	Terrain     TerrainType `json:"terrain"`
	AvgEnergy   float64     `json:"avg_energy"`
	AvgContrast float64     `json:"avg_contrast"`
	AvgVariance float64     `json:"avg_variance"`
	AvgEntropy  float64     `json:"avg_entropy"`
	StdVariance float64     `json:"std_variance"` // varyans standart sapması
}

// TerrainProfiles — doğal arazi doku profilleri (referans değerleri).
var TerrainProfiles = map[TerrainType]TextureProfile{
	TerrainForest: {Terrain: TerrainForest, AvgEnergy: 0.45, AvgContrast: 0.60, AvgVariance: 0.55, AvgEntropy: 0.72, StdVariance: 0.15},
	TerrainRocky:  {Terrain: TerrainRocky, AvgEnergy: 0.52, AvgContrast: 0.70, AvgVariance: 0.48, AvgEntropy: 0.65, StdVariance: 0.12},
	TerrainDesert: {Terrain: TerrainDesert, AvgEnergy: 0.30, AvgContrast: 0.25, AvgVariance: 0.18, AvgEntropy: 0.35, StdVariance: 0.08},
	TerrainUrban:  {Terrain: TerrainUrban, AvgEnergy: 0.65, AvgContrast: 0.80, AvgVariance: 0.38, AvgEntropy: 0.58, StdVariance: 0.10},
	TerrainGrass:  {Terrain: TerrainGrass, AvgEnergy: 0.35, AvgContrast: 0.40, AvgVariance: 0.30, AvgEntropy: 0.48, StdVariance: 0.10},
	TerrainSnow:   {Terrain: TerrainSnow, AvgEnergy: 0.20, AvgContrast: 0.15, AvgVariance: 0.10, AvgEntropy: 0.22, StdVariance: 0.05},
	TerrainWater:  {Terrain: TerrainWater, AvgEnergy: 0.15, AvgContrast: 0.10, AvgVariance: 0.08, AvgEntropy: 0.18, StdVariance: 0.04},
}

// AnalysisResult — doku analiz sonucu.
type AnalysisResult struct {
	RegionID      string         `json:"region_id"`
	Feature       TextureFeature `json:"feature"`
	Background    TextureFeature `json:"background"`
	Terrain       TerrainType    `json:"terrain"`
	Divergence    float64        `json:"divergence"`     // arkaplan ile fark skoru (0-1)
	IsAnomaly     bool           `json:"is_anomaly"`     // doku anomalisi tespit edildi mi
	ConvexScore   float64        `json:"convex_score"`   // dışbükey nesne skoru (0-1)
	ConcaveScore  float64        `json:"concave_score"`  // içbükey nesne skoru (0-1)
	GaborResponse []float64      `json:"gabor_response"` // Gabor filtre yanıtları
	ElapsedMs     float64        `json:"elapsed_ms"`
}

// Engine — doku analiz motoru.
type Engine struct {
	mu             sync.RWMutex
	gaborBank      []GaborParams // önceden hesaplanan Gabor filtre bankası
	scales         int
	orientations   int
	minFreq        float64
	maxFreq        float64
	blockSize      int
	varThreshold   float64
	analysisCount  int64
	anomalyCount   int64
}

// NewEngine — yeni doku analiz motoru oluştur.
func NewEngine(scales, orientations int, minFreq, maxFreq float64, blockSize int, varThreshold float64) *Engine {
	e := &Engine{
		scales:       scales,
		orientations: orientations,
		minFreq:      minFreq,
		maxFreq:      maxFreq,
		blockSize:    blockSize,
		varThreshold: varThreshold,
	}
	e.gaborBank = e.buildGaborBank()
	return e
}

// buildGaborBank — Gabor filtre bankasını önceden hesapla.
func (e *Engine) buildGaborBank() []GaborParams {
	var bank []GaborParams
	for s := 0; s < e.scales; s++ {
		freq := e.minFreq + (e.maxFreq-e.minFreq)*float64(s)/math.Max(1, float64(e.scales-1))
		for o := 0; o < e.orientations; o++ {
			theta := math.Pi * float64(o) / float64(e.orientations)
			bank = append(bank, GaborParams{
				Frequency:   freq,
				Orientation: theta,
				Scale:       s,
			})
		}
	}
	return bank
}

// RegionInput — analiz için bölge girdisi.
type RegionInput struct {
	ID      string      `json:"id"`
	Lat     float64     `json:"lat"`
	Lon     float64     `json:"lon"`
	Terrain TerrainType `json:"terrain"` // arazi türü
	Width   int         `json:"width"`   // bölge genişliği (piksel)
	Height  int         `json:"height"`  // bölge yüksekliği (piksel)
}

// Analyze — bölge listesinden doku analizi yap.
func (e *Engine) Analyze(regions []RegionInput) []AnalysisResult {
	start := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()

	e.analysisCount++
	var results []AnalysisResult

	for _, region := range regions {
		result := e.analyzeRegion(region)
		result.ElapsedMs = float64(time.Since(start).Microseconds()) / 1000.0
		if result.IsAnomaly {
			e.anomalyCount++
		}
		results = append(results, result)
	}

	return results
}

// analyzeRegion — tek bölge doku analizi.
func (e *Engine) analyzeRegion(region RegionInput) AnalysisResult {
	// Gabor filtre bankası üzerinden doku özellik çıkarımı
	gaborResp := e.computeGaborResponse(region)

	// doku özellik vektörü hesapla
	feature := e.extractFeatures(gaborResp, region.Terrain)

	// arkaplan doku profili
	bgProfile := e.getBackgroundProfile(region.Terrain)
	bgFeature := TextureFeature{
		Energy:      bgProfile.AvgEnergy,
		Contrast:    bgProfile.AvgContrast,
		Variance:    bgProfile.AvgVariance,
		Entropy:     bgProfile.AvgEntropy,
		Homogeneity: 1.0 - bgProfile.AvgContrast,
		Correlation: 0.5,
	}

	// arkaplan ile sapma hesapla
	divergence := e.computeDivergence(feature, bgFeature, bgProfile)

	// 3D konveks/konkav nesne skoru
	convexScore := e.computeConvexScore(gaborResp, feature)
	concaveScore := e.computeConcaveScore(gaborResp, feature)

	isAnomaly := divergence > e.varThreshold ||
		convexScore > 0.6 ||
		concaveScore > 0.55

	return AnalysisResult{
		RegionID:      region.ID,
		Feature:       feature,
		Background:    bgFeature,
		Terrain:       region.Terrain,
		Divergence:    math.Round(divergence*1000) / 1000,
		IsAnomaly:     isAnomaly,
		ConvexScore:   math.Round(convexScore*1000) / 1000,
		ConcaveScore:  math.Round(concaveScore*1000) / 1000,
		GaborResponse: gaborResp,
	}
}

// computeGaborResponse — Gabor filtre bankası yanıtlarını hesapla.
func (e *Engine) computeGaborResponse(region RegionInput) []float64 {
	responses := make([]float64, len(e.gaborBank))

	terrain, ok := TerrainProfiles[region.Terrain]
	if !ok {
		terrain = TerrainProfiles[TerrainGrass]
	}

	for i, gp := range e.gaborBank {
		// Gabor enerji yanıtı: frekans ve yönelim tabanlı
		// Gerçek uygulamada konvolüsyon yapılır; burada simüle ediyoruz
		baseEnergy := terrain.AvgEnergy
		freqFactor := math.Exp(-math.Abs(gp.Frequency-0.15) * 5.0)
		orientFactor := 0.5 + 0.5*math.Cos(2*gp.Orientation)

		response := baseEnergy * freqFactor * orientFactor
		// sensör gürültüsü
		response += (rand.Float64() - 0.5) * terrain.StdVariance * 0.5

		responses[i] = math.Max(0, math.Min(1.0, response))
	}

	return responses
}

// extractFeatures — Gabor yanıtlarından doku özellik vektörü çıkar.
func (e *Engine) extractFeatures(gaborResp []float64, terrain TerrainType) TextureFeature {
	if len(gaborResp) == 0 {
		return TextureFeature{}
	}

	// enerji: ortalama Gabor yanıtı
	var sumEnergy, sumSq float64
	maxResp := 0.0
	maxOrientIdx := 0
	maxFreqIdx := 0

	for i, r := range gaborResp {
		sumEnergy += r
		sumSq += r * r
		if r > maxResp {
			maxResp = r
			maxOrientIdx = i % e.orientations
			maxFreqIdx = i / e.orientations
		}
	}

	n := float64(len(gaborResp))
	mean := sumEnergy / n
	variance := (sumSq / n) - (mean * mean)
	if variance < 0 {
		variance = 0
	}

	// kontrast: standart sapma
	contrast := math.Sqrt(variance) * 2.0
	if contrast > 1.0 {
		contrast = 1.0
	}

	// homojenlik: düşük varyans = yüksek homojenlik
	homogeneity := 1.0 - math.Min(1.0, variance*4.0)

	// entropi: bilgi içeriği
	entropy := 0.0
	for _, r := range gaborResp {
		if r > 0.001 {
			p := r / sumEnergy
			entropy -= p * math.Log2(p+1e-10)
		}
	}
	entropy = math.Min(1.0, entropy/math.Log2(n+1))

	// otokorelasyon tahmini
	correlation := 0.0
	if len(gaborResp) > 1 {
		for i := 0; i < len(gaborResp)-1; i++ {
			correlation += gaborResp[i] * gaborResp[i+1]
		}
		correlation /= (float64(len(gaborResp)-1) * (mean*mean + variance + 1e-10))
		correlation = math.Min(1.0, math.Max(0, correlation))
	}

	// baskın yönelim ve frekans
	orientation := math.Pi * float64(maxOrientIdx) / float64(e.orientations)
	frequency := e.minFreq + (e.maxFreq-e.minFreq)*float64(maxFreqIdx)/math.Max(1, float64(e.scales-1))

	return TextureFeature{
		Energy:      math.Round(mean*1000) / 1000,
		Contrast:    math.Round(contrast*1000) / 1000,
		Homogeneity: math.Round(homogeneity*1000) / 1000,
		Entropy:     math.Round(entropy*1000) / 1000,
		Variance:    math.Round(variance*10000) / 10000,
		Correlation: math.Round(correlation*1000) / 1000,
		Orientation: math.Round(orientation*1000) / 1000,
		Frequency:   math.Round(frequency*1000) / 1000,
	}
}

// getBackgroundProfile — arazi türüne göre arkaplan profili getir.
func (e *Engine) getBackgroundProfile(terrain TerrainType) TextureProfile {
	if profile, ok := TerrainProfiles[terrain]; ok {
		return profile
	}
	return TerrainProfiles[TerrainGrass]
}

// computeDivergence — doku özellik vektörü ile arkaplan arasındaki sapma.
// Kullback-Leibler benzeri asimetrik uzaklık ölçümü.
func (e *Engine) computeDivergence(feature, bg TextureFeature, profile TextureProfile) float64 {
	// çok boyutlu mesafe: enerji, kontrast, varyans, entropi
	dEnergy := math.Abs(feature.Energy-bg.Energy) / (bg.Energy + 0.01)
	dContrast := math.Abs(feature.Contrast-bg.Contrast) / (bg.Contrast + 0.01)
	dVariance := math.Abs(feature.Variance-bg.Variance) / (profile.StdVariance + 0.01)
	dEntropy := math.Abs(feature.Entropy-bg.Entropy) / (bg.Entropy + 0.01)

	// ağırlıklı birleşim — varyans en önemli gösterge
	divergence := 0.15*dEnergy + 0.20*dContrast + 0.40*dVariance + 0.25*dEntropy

	return math.Min(1.0, divergence)
}

// computeConvexScore — dışbükey (kamuflaj örtüsü, araç branda) nesne skoru.
// Dışbükey nesneler etrafında keskin kenar geçişleri ve düşük iç varyans gösterir.
func (e *Engine) computeConvexScore(gaborResp []float64, feature TextureFeature) float64 {
	if len(gaborResp) < 4 {
		return 0
	}

	// yüksek enerji + düşük iç varyans = yapay dışbükey yüzey
	energyFactor := feature.Energy
	variancePenalty := 1.0 - math.Min(1.0, feature.Variance*8.0)

	// yönelim düzenliliği — yapay nesneler genelde düzenli kenar yapısına sahip
	orientRegularity := 0.0
	if e.orientations > 0 {
		orientBins := make([]float64, e.orientations)
		for i, r := range gaborResp {
			orientBins[i%e.orientations] += r
		}
		maxBin, minBin := orientBins[0], orientBins[0]
		for _, b := range orientBins {
			if b > maxBin {
				maxBin = b
			}
			if b < minBin {
				minBin = b
			}
		}
		if maxBin > 0.001 {
			orientRegularity = (maxBin - minBin) / maxBin
		}
	}

	score := 0.35*energyFactor + 0.35*variancePenalty + 0.30*orientRegularity
	return math.Min(1.0, math.Max(0, score))
}

// computeConcaveScore — içbükey (kazılmış mevzi, siper, tünel girişi) nesne skoru.
// İçbükey yapılar gölge/karanlık bölge + kenar geçişi deseni gösterir.
func (e *Engine) computeConcaveScore(gaborResp []float64, feature TextureFeature) float64 {
	if len(gaborResp) < 4 {
		return 0
	}

	// düşük enerji (gölge) + yüksek kontrast (kenar) = içbükey yapı
	shadowFactor := 1.0 - feature.Energy
	contrastFactor := feature.Contrast

	// frekans yanıtı düzensizliği — doğal olmayan frekans deseni
	freqIrregularity := 0.0
	if e.scales > 1 {
		scaleBins := make([]float64, e.scales)
		for i, r := range gaborResp {
			scaleBins[i/e.orientations] += r
		}
		var sumDiff float64
		for i := 1; i < len(scaleBins); i++ {
			sumDiff += math.Abs(scaleBins[i] - scaleBins[i-1])
		}
		freqIrregularity = math.Min(1.0, sumDiff/float64(e.scales))
	}

	score := 0.30*shadowFactor + 0.35*contrastFactor + 0.35*freqIrregularity
	return math.Min(1.0, math.Max(0, score))
}

// Stats — motor istatistikleri.
func (e *Engine) Stats() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return map[string]any{
		"analysis_count":  e.analysisCount,
		"anomaly_count":   e.anomalyCount,
		"gabor_bank_size": len(e.gaborBank),
		"scales":          e.scales,
		"orientations":    e.orientations,
		"var_threshold":   e.varThreshold,
	}
}
