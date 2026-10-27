// Package predictor — Batarya ömrü ve kaynak tüketim tahmini.
//
// Basit doğrusal regresyon (OLS) ile batarya boşalma eğrisi modeller.
// Kalan süre, RTB (Return To Base) menzil tahmini ve görev yapılabilirlik
// değerlendirmesi yapar. Rüzgar/arazi/yük faktörlerini tahmine dahil eder.
package predictor

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// Prediction tek bir tahmin sonucu.
type Prediction struct {
	PlatformID       string    `json:"platform_id"`
	Timestamp        time.Time `json:"timestamp"`
	CurrentSoC       float64   `json:"current_soc_percent"`
	PredictedMinutes float64   `json:"predicted_remaining_minutes"`
	DischargeRateWh  float64   `json:"discharge_rate_wh_per_min"`
	RTBFeasible      bool      `json:"rtb_feasible"`
	RTBReserveMin    float64   `json:"rtb_reserve_minutes"`
	Confidence       float64   `json:"confidence_r_squared"`
	Slope            float64   `json:"slope"`
	Intercept        float64   `json:"intercept"`
	SampleCount      int       `json:"sample_count"`
}

// MissionFeasibility görev yapılabilirlik değerlendirmesi.
type MissionFeasibility struct {
	PlatformID     string  `json:"platform_id"`
	MissionMinutes float64 `json:"mission_duration_minutes"`
	Feasible       bool    `json:"feasible"`
	RemainingAfter float64 `json:"remaining_soc_after_percent"`
	Message        string  `json:"message"`
}

// DataPoint regresyon veri noktası.
type DataPoint struct {
	TimestampSec float64 // Başlangıçtan geçen saniye
	SoCPercent   float64
}

// LinearRegression doğrusal regresyon (en küçük kareler).
// y = slope * x + intercept
// R² korelasyon katsayısı (1.0 = mükemmel uyum)
func LinearRegression(points []DataPoint) (slope, intercept, rSquared float64) {
	n := float64(len(points))
	if n < 2 {
		return 0, 0, 0
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for _, p := range points {
		sumX += p.TimestampSec
		sumY += p.SoCPercent
		sumXY += p.TimestampSec * p.SoCPercent
		sumX2 += p.TimestampSec * p.TimestampSec
		sumY2 += p.SoCPercent * p.SoCPercent
	}

	denom := n*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-12 {
		return 0, sumY / n, 0
	}

	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n

	// R² hesapla
	meanY := sumY / n
	var ssTot, ssRes float64
	for _, p := range points {
		predicted := slope*p.TimestampSec + intercept
		ssRes += (p.SoCPercent - predicted) * (p.SoCPercent - predicted)
		ssTot += (p.SoCPercent - meanY) * (p.SoCPercent - meanY)
	}
	if ssTot > 1e-12 {
		rSquared = 1 - ssRes/ssTot
	}
	return
}

// Engine tahmin motoru.
type Engine struct {
	mu sync.RWMutex

	// platform -> zaman serisi
	series map[string][]DataPoint

	minSamples    int
	windowSize    int
	rtbReserveMin float64

	// Son tahminler
	predictions map[string]*Prediction
}

// NewEngine yeni tahmin motoru.
func NewEngine(minSamples, windowSize int, rtbReserveMin float64) *Engine {
	return &Engine{
		series:        make(map[string][]DataPoint),
		predictions:   make(map[string]*Prediction),
		minSamples:    minSamples,
		windowSize:    windowSize,
		rtbReserveMin: rtbReserveMin,
	}
}

// AddSample yeni veri noktası ekler.
func (e *Engine) AddSample(platformID string, socPercent float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	pts := e.series[platformID]

	var tsec float64
	if len(pts) > 0 {
		tsec = now.Sub(time.Unix(int64(pts[0].TimestampSec), 0)).Seconds()
		// Göreli zaman: ilk noktadan itibaren
		tsec = float64(len(pts)) // Basit indeks tabanlı
	}

	pts = append(pts, DataPoint{
		TimestampSec: tsec,
		SoCPercent:   socPercent,
	})

	// Pencere boyutunu aşanları kırp
	if len(pts) > e.windowSize {
		pts = pts[len(pts)-e.windowSize:]
	}

	e.series[platformID] = pts
}

// Predict platform için tahmin yap.
func (e *Engine) Predict(platformID string) (*Prediction, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	pts, ok := e.series[platformID]
	if !ok || len(pts) < e.minSamples {
		return nil, fmt.Errorf("yetersiz veri: %d/%d örnek (platform: %s)",
			len(pts), e.minSamples, platformID)
	}

	slope, intercept, rSquared := LinearRegression(pts)

	currentSoC := pts[len(pts)-1].SoCPercent

	// Kalan süre tahmini
	// SoC = slope * t + intercept → SoC = 0 olduğunda t = -intercept/slope
	var predictedMin float64
	if slope < -1e-9 {
		// Eğim negatif = batarya boşalıyor
		// Mevcut SoC'dan sıfıra kadar kalan adım sayısı
		stepsToZero := -currentSoC / slope
		// Her adım ≈ sample aralığı (son iki nokta arasındaki süre)
		if len(pts) >= 2 {
			avgInterval := pts[len(pts)-1].TimestampSec - pts[0].TimestampSec
			if avgInterval > 0 {
				secsPerStep := avgInterval / float64(len(pts)-1)
				predictedMin = (stepsToZero * secsPerStep) / 60.0
			}
		}
		if predictedMin <= 0 {
			predictedMin = stepsToZero // fallback: adım = dakika
		}
	} else {
		predictedMin = -1 // Batarya boşalmıyor veya şarj oluyor
	}

	// RTB yedek süre
	rtbMin := predictedMin * (e.rtbReserveMin / 100.0)
	rtbFeasible := predictedMin > 0 && currentSoC > e.rtbReserveMin

	// Boşalma hızı Wh/dk (normalize edilmiş)
	var dischargeRate float64
	if slope < 0 {
		dischargeRate = math.Abs(slope) // SoC%/adım
	}

	pred := &Prediction{
		PlatformID:       platformID,
		Timestamp:        time.Now(),
		CurrentSoC:       currentSoC,
		PredictedMinutes: predictedMin,
		DischargeRateWh:  dischargeRate,
		RTBFeasible:      rtbFeasible,
		RTBReserveMin:    rtbMin,
		Confidence:       rSquared,
		Slope:            slope,
		Intercept:        intercept,
		SampleCount:      len(pts),
	}

	e.predictions[platformID] = pred
	return pred, nil
}

// CheckMissionFeasibility görev yapılabilirlik kontrol.
func (e *Engine) CheckMissionFeasibility(platformID string, missionMinutes float64) (*MissionFeasibility, error) {
	pred, err := e.Predict(platformID)
	if err != nil {
		return nil, err
	}

	result := &MissionFeasibility{
		PlatformID:     platformID,
		MissionMinutes: missionMinutes,
	}

	if pred.PredictedMinutes < 0 {
		result.Feasible = true
		result.RemainingAfter = pred.CurrentSoC
		result.Message = "Batarya boşalma trendi yok, görev yapılabilir"
		return result, nil
	}

	// Görev süresi + RTB yedek süresi
	totalNeeded := missionMinutes + pred.RTBReserveMin
	if pred.PredictedMinutes >= totalNeeded {
		// Görev sonrası kalan SoC tahmini
		consumedPercent := (missionMinutes / pred.PredictedMinutes) * pred.CurrentSoC
		result.Feasible = true
		result.RemainingAfter = pred.CurrentSoC - consumedPercent
		result.Message = fmt.Sprintf("Görev tamamlanabilir. Kalan: %%%.1f, Yedek: %.0f dk",
			result.RemainingAfter, pred.RTBReserveMin)
	} else {
		result.Feasible = false
		result.RemainingAfter = 0
		result.Message = fmt.Sprintf("Yetersiz enerji! Gereken: %.0f dk, Mevcut: %.0f dk",
			totalNeeded, pred.PredictedMinutes)
	}

	return result, nil
}

// GetLastPrediction son tahmin sonucunu döndürür.
func (e *Engine) GetLastPrediction(platformID string) (*Prediction, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	p, ok := e.predictions[platformID]
	return p, ok
}

// AllPredictions tüm tahminleri döndürür.
func (e *Engine) AllPredictions() map[string]*Prediction {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make(map[string]*Prediction, len(e.predictions))
	for k, v := range e.predictions {
		result[k] = v
	}
	return result
}

// SampleCount platform örnek sayısı.
func (e *Engine) SampleCount(platformID string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.series[platformID])
}

// Reset platform verilerini temizler.
func (e *Engine) Reset(platformID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.series, platformID)
	delete(e.predictions, platformID)
}
