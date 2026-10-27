// Package predictor — Kalman filtresi ile rota tahmini ve önleme atışı hesaplayıcı.
//
// Hareket halindeki hedefin kinematik modelini kullanarak gelecek konumlarını
// tahmin eder. Topçu birlikleri için önleme (interception) koordinatları hesaplar.
package predictor

import (
	"math"
	"time"

	"github.com/otukenmatris/izlek/internal/tracker"
)

const (
	earthRadiusM = 6_371_000.0
	deg2rad      = math.Pi / 180.0
	rad2deg      = 180.0 / math.Pi
)

// KalmanState — 2D Kalman filtre durum vektörü [x, y, vx, vy, ax, ay]
type KalmanState struct {
	X  float64 // kuzey konum (m, referansa göre)
	Y  float64 // doğu konum (m, referansa göre)
	Vx float64 // kuzey hız (m/s)
	Vy float64 // doğu hız (m/s)
	Ax float64 // kuzey ivme (m/s²)
	Ay float64 // doğu ivme (m/s²)
}

// PredictedPoint — tahmin edilen konum
type PredictedPoint struct {
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	TimeSec    float64 `json:"time_sec"`    // şimdiden itibaren saniye
	Speed      float64 `json:"speed_mps"`   // tahmini hız m/s
	Heading    float64 `json:"heading_deg"` // tahmini yön
	Confidence float64 `json:"confidence"`  // güven (0-1, zamanla azalır)
}

// Prediction — tam tahmin sonucu
type Prediction struct {
	EntityID     string           `json:"entity_id"`
	StartLat     float64          `json:"start_lat"`
	StartLon     float64          `json:"start_lon"`
	HorizonSec   float64          `json:"horizon_sec"`
	Points       []PredictedPoint `json:"points"`
	Maneuver     bool             `json:"maneuver"`     // manevra tespit edildi mi
	ManeuverProb float64          `json:"maneuver_prob"` // manevra olasılığı
	Timestamp    time.Time        `json:"timestamp"`
}

// InterceptResult — önleme atışı hesap sonucu
type InterceptResult struct {
	EntityID     string  `json:"entity_id"`
	TargetLat    float64 `json:"target_lat"`
	TargetLon    float64 `json:"target_lon"`
	InterceptLat float64 `json:"intercept_lat"`
	InterceptLon float64 `json:"intercept_lon"`
	TimeSec      float64 `json:"time_sec"`
	Distance     float64 `json:"distance_m"`
	Confidence   float64 `json:"confidence"`
	LeadAngle    float64 `json:"lead_angle_deg"`
	Feasible     bool    `json:"feasible"`
}

// Engine — tahmin motoru
type Engine struct {
	processNoise     float64
	measurementNoise float64
	defaultHorizon   float64
	maxHorizon       float64
	stepSec          float64
	maxAccel         float64
	maneuverThresh   float64
	totalPredictions int
	totalIntercepts  int
}

// NewEngine — yeni tahmin motoru.
func NewEngine(processNoise, measurementNoise, defaultHorizon, maxHorizon, stepSec, maxAccel, maneuverThresh float64) *Engine {
	return &Engine{
		processNoise:     processNoise,
		measurementNoise: measurementNoise,
		defaultHorizon:   defaultHorizon,
		maxHorizon:       maxHorizon,
		stepSec:          stepSec,
		maxAccel:         maxAccel,
		maneuverThresh:   maneuverThresh,
	}
}

// Predict — Kalman filtreli kinematik rota tahmini.
func (e *Engine) Predict(track *tracker.Track, horizonSec float64) Prediction {
	e.totalPredictions++

	if horizonSec <= 0 {
		horizonSec = e.defaultHorizon
	}
	if horizonSec > e.maxHorizon {
		horizonSec = e.maxHorizon
	}

	last := track.LastPoint()
	pred := Prediction{
		EntityID:   track.EntityID,
		StartLat:   last.Lat,
		StartLon:   last.Lon,
		HorizonSec: horizonSec,
		Maneuver:   track.Maneuver,
		Timestamp:  time.Now(),
	}

	// Kalman durum vektörünü başlat
	state := KalmanState{
		X:  0, // referans noktası = mevcut konum
		Y:  0,
		Vx: track.VelNorth,
		Vy: track.VelEast,
		Ax: track.AccNorth,
		Ay: track.AccEast,
	}

	// Manevra olasılığı: ivme büyüklüğüne göre
	accelMag := math.Sqrt(state.Ax*state.Ax + state.Ay*state.Ay)
	pred.ManeuverProb = math.Min(accelMag/e.maxAccel, 1.0)

	// Tahmin noktalarını hesapla
	cosLat := math.Cos(last.Lat * deg2rad)
	if cosLat < 0.001 {
		cosLat = 0.001
	}

	steps := int(horizonSec / e.stepSec)
	if steps < 1 {
		steps = 1
	}

	pred.Points = make([]PredictedPoint, 0, steps)
	dt := e.stepSec

	// Kalman süreç gürültüsü katsayısı
	q := e.processNoise

	for i := 1; i <= steps; i++ {
		t := float64(i) * dt

		// Kalman tahmin adımı (constant acceleration model)
		// x = x0 + vx*t + 0.5*ax*t²
		px := state.Vx*t + 0.5*state.Ax*t*t
		py := state.Vy*t + 0.5*state.Ay*t*t

		// Manevra halindeyse ivme azalmasını modelle (Markov geçiş)
		if track.Maneuver {
			decay := math.Exp(-0.05 * t) // ivme zaman içinde sönümlenir
			px = state.Vx*t + 0.5*state.Ax*t*t*decay
			py = state.Vy*t + 0.5*state.Ay*t*t*decay
		}

		// Konum → lat/lon dönüşümü
		predLat := last.Lat + (px/(earthRadiusM))*rad2deg
		predLon := last.Lon + (py/(earthRadiusM*cosLat))*rad2deg

		// Hız ve yön
		vx := state.Vx + state.Ax*t
		vy := state.Vy + state.Ay*t
		if track.Maneuver {
			decay := math.Exp(-0.05 * t)
			vx = state.Vx + state.Ax*t*decay
			vy = state.Vy + state.Ay*t*decay
		}
		speed := math.Sqrt(vx*vx + vy*vy)
		heading := math.Atan2(vy, vx) * rad2deg
		if heading < 0 {
			heading += 360
		}

		// Güven: zaman + süreç gürültüsü ile azalır
		noiseSpread := q * t * t
		confidence := math.Exp(-noiseSpread / (e.measurementNoise * 100))
		if track.Maneuver {
			confidence *= 0.7 // manevradayken güven düşer
		}
		if confidence < 0.01 {
			confidence = 0.01
		}

		pred.Points = append(pred.Points, PredictedPoint{
			Lat:        predLat,
			Lon:        predLon,
			TimeSec:    t,
			Speed:      speed,
			Heading:    heading,
			Confidence: confidence,
		})
	}

	return pred
}

// Intercept — önleme atışı koordinatı hesaplar.
//
// projectileSpeed: mermi/roket hızı (m/s)
// launcherLat/Lon: atıcı konumu
func (e *Engine) Intercept(track *tracker.Track, projectileSpeed float64, launcherLat, launcherLon float64) InterceptResult {
	e.totalIntercepts++

	last := track.LastPoint()
	result := InterceptResult{
		EntityID:  track.EntityID,
		TargetLat: last.Lat,
		TargetLon: last.Lon,
	}

	// Hedef sabit veya çok yavaşsa
	targetSpeed := math.Sqrt(track.VelNorth*track.VelNorth + track.VelEast*track.VelEast)
	if targetSpeed < 0.1 || projectileSpeed < 1.0 {
		result.InterceptLat = last.Lat
		result.InterceptLon = last.Lon
		result.TimeSec = e.distanceM(launcherLat, launcherLon, last.Lat, last.Lon) / math.Max(projectileSpeed, 1.0)
		result.Distance = e.distanceM(launcherLat, launcherLon, last.Lat, last.Lon)
		result.Confidence = 0.95
		result.Feasible = result.TimeSec < e.maxHorizon
		return result
	}

	// İteratif çözüm: mermi uçuş süresi = hedefin o sürede varacağı yer
	cosLat := math.Cos(last.Lat * deg2rad)
	if cosLat < 0.001 {
		cosLat = 0.001
	}

	bestT := 0.0
	bestLat := last.Lat
	bestLon := last.Lon
	converged := false

	for iter := 0; iter < 50; iter++ {
		// Hedefin t anındaki konumu
		t := bestT
		if t == 0 {
			t = e.distanceM(launcherLat, launcherLon, last.Lat, last.Lon) / projectileSpeed
		}

		px := track.VelNorth*t + 0.5*track.AccNorth*t*t
		py := track.VelEast*t + 0.5*track.AccEast*t*t

		predLat := last.Lat + (px/earthRadiusM)*rad2deg
		predLon := last.Lon + (py/(earthRadiusM*cosLat))*rad2deg

		// Mermi uçuş süresi
		dist := e.distanceM(launcherLat, launcherLon, predLat, predLon)
		newT := dist / projectileSpeed

		if math.Abs(newT-t) < 0.01 {
			converged = true
			bestT = newT
			bestLat = predLat
			bestLon = predLon
			break
		}

		bestT = newT
		bestLat = predLat
		bestLon = predLon
	}

	result.InterceptLat = bestLat
	result.InterceptLon = bestLon
	result.TimeSec = bestT
	result.Distance = e.distanceM(launcherLat, launcherLon, bestLat, bestLon)
	result.Feasible = converged && bestT < e.maxHorizon && bestT > 0

	// Güven: yakınsama + zaman + manevra durumu
	result.Confidence = 0.9
	if !converged {
		result.Confidence = 0.3
	}
	if track.Maneuver {
		result.Confidence *= 0.6
	}
	timeFactor := math.Exp(-0.01 * bestT)
	result.Confidence *= timeFactor

	// Önleme açısı
	dx := (bestLat - launcherLat) * deg2rad * earthRadiusM
	dy := (bestLon - launcherLon) * deg2rad * earthRadiusM * cosLat
	result.LeadAngle = math.Atan2(dy, dx) * rad2deg

	return result
}

// distanceM — iki koordinat arası mesafe (metre, Haversine).
func (e *Engine) distanceM(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := (lat2 - lat1) * deg2rad
	dLon := (lon2 - lon1) * deg2rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*deg2rad)*math.Cos(lat2*deg2rad)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}

// Stats — motor istatistikleri.
func (e *Engine) Stats() map[string]interface{} {
	return map[string]interface{}{
		"total_predictions": e.totalPredictions,
		"total_intercepts":  e.totalIntercepts,
		"process_noise":     e.processNoise,
		"measurement_noise": e.measurementNoise,
		"default_horizon":   e.defaultHorizon,
		"max_horizon":       e.maxHorizon,
		"step_sec":          e.stepSec,
	}
}
