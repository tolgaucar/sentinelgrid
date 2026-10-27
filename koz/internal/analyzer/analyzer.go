// Package analyzer — termal imza analiz ve kümeleme motoru.
//
// Tespit edilen termal imzaları analiz eder: soğuma eğrisi tahmin,
// araç türü sınıflandırma, kümeleme (birlik tespiti), anomali algılama.
package analyzer

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/otukenmatris/koz/internal/thermal"
)

// ClusterResult — termal imza kümesi (birlik grubu).
type ClusterResult struct {
	ID         string              `json:"id"`
	CenterLat  float64             `json:"center_lat"`
	CenterLon  float64             `json:"center_lon"`
	Members    []string            `json:"members"`
	AvgTempC   float64             `json:"avg_temp_c"`
	MaxTempC   float64             `json:"max_temp_c"`
	RadiusM    float64             `json:"radius_m"`
	Assessment string              `json:"assessment"` // "convoy", "camp", "engagement", "scattered"
	Count      int                 `json:"count"`
}

// CoolingPrediction — soğuma eğrisi tahmini.
type CoolingPrediction struct {
	EntityID       string  `json:"entity_id"`
	CurrentTempC   float64 `json:"current_temp_c"`
	PredictedTempC float64 `json:"predicted_temp_c"` // 5 dakika sonra
	CoolingRateC   float64 `json:"cooling_rate_c"`
	TimeToAmbientS float64 `json:"time_to_ambient_s"` // ortam sıcaklığına dönüş süresi
	EngineState    string  `json:"engine_state"`       // "running", "recently_off", "cold"
}

// Anomaly — termal anomali.
type Anomaly struct {
	ID          string    `json:"id"`
	EntityID    string    `json:"entity_id"`
	Type        string    `json:"type"` // "sudden_heat", "abnormal_cool", "masked_sig"
	Description string    `json:"description"`
	Severity    string    `json:"severity"` // "low", "medium", "high"
	TempC       float64   `json:"temp_c"`
	DetectedAt  time.Time `json:"detected_at"`
}

// Analyzer — termal analiz motoru.
type Analyzer struct {
	mu              sync.RWMutex
	clusterRadiusM  float64
	coolingThreshold float64
	anomalies       []Anomaly
	analysisCount   int64
	anomalyCount    int64
}

// NewAnalyzer — yeni analiz motoru.
func NewAnalyzer(clusterRadiusM, coolingThreshold float64) *Analyzer {
	return &Analyzer{
		clusterRadiusM:  clusterRadiusM,
		coolingThreshold: coolingThreshold,
	}
}

// ClusterSignatures — termal imzaları mesafeye göre kümele.
func (a *Analyzer) ClusterSignatures(sigs []thermal.Signature) []ClusterResult {
	a.mu.Lock()
	a.analysisCount++
	a.mu.Unlock()

	if len(sigs) == 0 {
		return nil
	}

	assigned := make(map[int]int) // sig index → cluster index
	var clusters []ClusterResult

	for i, sig := range sigs {
		if _, ok := assigned[i]; ok {
			continue
		}

		cluster := ClusterResult{
			CenterLat: sig.Lat,
			CenterLon: sig.Lon,
			Members:   []string{sig.EntityID},
			AvgTempC:  sig.TempC,
			MaxTempC:  sig.TempC,
			Count:     1,
		}
		assigned[i] = len(clusters)

		for j := i + 1; j < len(sigs); j++ {
			if _, ok := assigned[j]; ok {
				continue
			}
			dist := haversineM(sig.Lat, sig.Lon, sigs[j].Lat, sigs[j].Lon)
			if dist <= a.clusterRadiusM {
				assigned[j] = len(clusters)
				cluster.Members = append(cluster.Members, sigs[j].EntityID)
				cluster.AvgTempC = (cluster.AvgTempC*float64(cluster.Count) + sigs[j].TempC) / float64(cluster.Count+1)
				if sigs[j].TempC > cluster.MaxTempC {
					cluster.MaxTempC = sigs[j].TempC
				}
				cluster.Count++
				// merkez güncelle (ortalama)
				cluster.CenterLat = (cluster.CenterLat*float64(cluster.Count-1) + sigs[j].Lat) / float64(cluster.Count)
				cluster.CenterLon = (cluster.CenterLon*float64(cluster.Count-1) + sigs[j].Lon) / float64(cluster.Count)
			}
		}

		// yarıçap hesapla
		maxDist := 0.0
		for _, idx := range cluster.Members {
			for _, s := range sigs {
				if s.EntityID == idx {
					d := haversineM(cluster.CenterLat, cluster.CenterLon, s.Lat, s.Lon)
					if d > maxDist {
						maxDist = d
					}
				}
			}
		}
		cluster.RadiusM = maxDist

		// değerlendirme
		cluster.Assessment = assessCluster(cluster)
		cluster.ID = "cl-" + sig.ID
		clusters = append(clusters, cluster)
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Count > clusters[j].Count
	})

	return clusters
}

// PredictCooling — soğuma eğrisi tahmini yap.
func (a *Analyzer) PredictCooling(sigs []thermal.Signature, ambientC float64) []CoolingPrediction {
	var results []CoolingPrediction
	for _, sig := range sigs {
		rate := sig.CoolingRateC
		if rate == 0 {
			// varsayılan soğuma hızı tahmin et
			rate = (sig.TempC - ambientC) / 600.0 // 10 dakikada ortama dönüş varsayımı
		}

		predicted := sig.TempC - rate*300 // 5 dakika sonra
		if predicted < ambientC {
			predicted = ambientC
		}

		timeToAmbient := 0.0
		if rate > 0 {
			timeToAmbient = (sig.TempC - ambientC) / rate
		}

		var engineState string
		switch {
		case sig.State == thermal.StateHeating || sig.State == thermal.StateSteady:
			engineState = "running"
		case rate > a.coolingThreshold:
			engineState = "recently_off"
		default:
			engineState = "cold"
		}

		results = append(results, CoolingPrediction{
			EntityID:       sig.EntityID,
			CurrentTempC:   sig.TempC,
			PredictedTempC: math.Round(predicted*10) / 10,
			CoolingRateC:   math.Round(rate*100) / 100,
			TimeToAmbientS: math.Round(timeToAmbient),
			EngineState:    engineState,
		})
	}
	return results
}

// DetectAnomalies — termal anomalileri tespit et.
func (a *Analyzer) DetectAnomalies(sigs []thermal.Signature) []Anomaly {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	var anomalies []Anomaly

	for _, sig := range sigs {
		// ani ısınma: geçmişe göre çok hızlı sıcaklık artışı
		if len(sig.History) >= 2 {
			last := sig.History[len(sig.History)-1]
			prev := sig.History[len(sig.History)-2]
			dt := last.Timestamp.Sub(prev.Timestamp).Seconds()
			if dt > 0 {
				rise := (last.TempC - prev.TempC) / dt
				if rise > 5.0 { // 5°C/s üstü ani ısınma
					anom := Anomaly{
						ID:          "an-" + sig.EntityID + "-heat",
						EntityID:    sig.EntityID,
						Type:        "sudden_heat",
						Description: "Ani sıcaklık artışı tespit edildi — olası ateşleme/patlama",
						Severity:    "high",
						TempC:       sig.TempC,
						DetectedAt:  now,
					}
					anomalies = append(anomalies, anom)
				}
			}
		}

		// anormal düşük sıcaklık (kamuflaj şüphesi)
		if sig.Type == thermal.SigEngine && sig.DeltaC < 5 && sig.DeltaC > 0 {
			anom := Anomaly{
				ID:          "an-" + sig.EntityID + "-mask",
				EntityID:    sig.EntityID,
				Type:        "masked_sig",
				Description: "Düşük termal iz — olası termal kamuflaj",
				Severity:    "medium",
				TempC:       sig.TempC,
				DetectedAt:  now,
			}
			anomalies = append(anomalies, anom)
		}
	}

	a.anomalies = append(a.anomalies, anomalies...)
	if len(a.anomalies) > 500 {
		a.anomalies = a.anomalies[len(a.anomalies)-500:]
	}
	a.anomalyCount += int64(len(anomalies))
	return anomalies
}

// GetAnomalies — kayıtlı anomaliler.
func (a *Analyzer) GetAnomalies(n int) []Anomaly {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if n <= 0 || n > len(a.anomalies) {
		n = len(a.anomalies)
	}
	start := len(a.anomalies) - n
	result := make([]Anomaly, n)
	copy(result, a.anomalies[start:])
	return result
}

// Stats — analiz istatistikleri.
func (a *Analyzer) Stats() map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return map[string]any{
		"analysis_count": a.analysisCount,
		"anomaly_count":  a.anomalyCount,
		"stored_anomalies": len(a.anomalies),
	}
}

func assessCluster(c ClusterResult) string {
	switch {
	case c.Count >= 5 && c.RadiusM < 200:
		return "convoy"
	case c.Count >= 3 && c.MaxTempC > 100:
		return "engagement"
	case c.Count >= 2 && c.RadiusM < 100:
		return "camp"
	default:
		return "scattered"
	}
}

func haversineM(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // metre
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
