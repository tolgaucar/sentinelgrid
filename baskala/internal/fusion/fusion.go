// Package fusion — SAR ve Optik uydu görüntüsü füzyon motoru.
//
// SAR radarının bulutları delme yeteneğini optik netlikle birleştirerek
// yanlış alarm oranını minimize eder. Çapraz doğrulama ile güven artışı sağlar.
package fusion

import (
	"math"

	"github.com/otukenmatris/baskala/internal/change"
)

// FusionResult — füzyon sonucu.
type FusionResult struct {
	Detections      []change.ChangeDetection `json:"detections"`
	TotalFused      int                      `json:"total_fused"`
	CrossValidated  int                      `json:"cross_validated"`
	FalseAlarmFiltered int                   `json:"false_alarm_filtered"`
	SAROnly         int                      `json:"sar_only"`
	OpticalOnly     int                      `json:"optical_only"`
	BothSources     int                      `json:"both_sources"`
}

// Engine — füzyon motoru.
type Engine struct {
	sarWeight     float64
	opticalWeight float64
	crossRefBoost float64
	cloudPenalty  float64
}

// NewEngine — yeni füzyon motoru.
func NewEngine(sarWeight, opticalWeight, crossRefBoost, cloudPenalty float64) *Engine {
	return &Engine{
		sarWeight:     sarWeight,
		opticalWeight: opticalWeight,
		crossRefBoost: crossRefBoost,
		cloudPenalty:  cloudPenalty,
	}
}

// Fuse — SAR ve optik değişim tespitlerini birleştir.
func (e *Engine) Fuse(sarDetections, opticalDetections []change.ChangeDetection, optCloudCover float64) FusionResult {
	var result FusionResult
	matched := make(map[int]bool) // optik tarafta eşleşen indeksler

	for _, sarDet := range sarDetections {
		bestMatch := -1
		bestDist := math.MaxFloat64

		// yakın optik eşleşme bul
		for j, optDet := range opticalDetections {
			if matched[j] {
				continue
			}
			dist := haversineKm(sarDet.CenterLat, sarDet.CenterLon, optDet.CenterLat, optDet.CenterLon)
			if dist < 0.5 && dist < bestDist { // 500m eşleşme yarıçapı
				bestDist = dist
				bestMatch = j
			}
		}

		if bestMatch >= 0 {
			// çapraz doğrulama — her iki kaynakta da var
			matched[bestMatch] = true
			fused := e.mergeDetections(sarDet, opticalDetections[bestMatch], optCloudCover)
			fused.Confidence = math.Min(fused.Confidence+e.crossRefBoost, 1.0)
			fused.Description += " [SAR+Optik çapraz doğrulanmış]"
			result.Detections = append(result.Detections, fused)
			result.CrossValidated++
			result.BothSources++
		} else {
			// sadece SAR — güvenilir (bulut engellemez)
			det := sarDet
			det.Confidence *= e.sarWeight * 2 // SAR tek başına %100 ağırlık
			det.Description += " [Yalnız SAR]"
			result.Detections = append(result.Detections, det)
			result.SAROnly++
		}
	}

	// eşleşmeyen optik tespitler
	for j, optDet := range opticalDetections {
		if matched[j] {
			continue
		}
		// bulutlu optik → ceza uygula
		if optCloudCover > 0.5 {
			optDet.Confidence *= (1.0 - e.cloudPenalty)
			optDet.Description += " [Bulutlu optik — düşürülmüş güven]"
		} else {
			optDet.Description += " [Yalnız Optik]"
		}

		// çok düşük güvenli olanları filtrele (false alarm)
		if optDet.Confidence < 0.15 {
			result.FalseAlarmFiltered++
			continue
		}

		result.Detections = append(result.Detections, optDet)
		result.OpticalOnly++
	}

	result.TotalFused = len(result.Detections)

	// tehdit seviyelerini yeniden hesapla
	for i := range result.Detections {
		result.Detections[i].ThreatLevel = recalcThreat(result.Detections[i].Confidence, result.Detections[i].Intensity, result.Detections[i].Type)
	}

	return result
}

// mergeDetections — iki tespiti ağırlıklı olarak birleştir.
func (e *Engine) mergeDetections(sar, opt change.ChangeDetection, cloudCover float64) change.ChangeDetection {
	// ağırlıklı konum
	sw := e.sarWeight
	ow := e.opticalWeight
	if cloudCover > 0.3 {
		ow *= (1.0 - cloudCover * e.cloudPenalty)
		sw += (1.0 - ow)
	}
	total := sw + ow

	merged := sar
	merged.CenterLat = (sar.CenterLat*sw + opt.CenterLat*ow) / total
	merged.CenterLon = (sar.CenterLon*sw + opt.CenterLon*ow) / total
	merged.Confidence = (sar.Confidence*sw + opt.Confidence*ow) / total
	merged.Intensity = (sar.Intensity*sw + opt.Intensity*ow) / total

	// büyük alanı al
	if opt.AreaSqKm > sar.AreaSqKm {
		merged.AreaSqKm = opt.AreaSqKm
	}

	// daha spesifik tipi tercih et (askeri > sivil)
	if isMilitaryType(opt.Type) && !isMilitaryType(sar.Type) {
		merged.Type = opt.Type
	}

	return merged
}

func isMilitaryType(ct change.ChangeType) bool {
	return ct == change.ChangeTrench || ct == change.ChangeFortify || ct == change.ChangeVehicle
}

func recalcThreat(confidence, intensity float64, cType change.ChangeType) change.ThreatLevel {
	score := confidence * 0.5 + intensity * 0.15
	switch cType {
	case change.ChangeTrench, change.ChangeFortify:
		score += 0.35
	case change.ChangeBuilding:
		score += 0.25
	case change.ChangeVehicle:
		score += 0.20
	default:
		score += 0.10
	}

	switch {
	case score >= 0.8:
		return change.ThreatCritical
	case score >= 0.6:
		return change.ThreatHigh
	case score >= 0.4:
		return change.ThreatMedium
	case score >= 0.2:
		return change.ThreatLow
	default:
		return change.ThreatNone
	}
}

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
