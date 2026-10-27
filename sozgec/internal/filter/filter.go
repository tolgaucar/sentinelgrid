// Package filter — DSP filtre motoru.
//
// Gürültü bastırma, dost/düşman sinyal ayrımı, SNR hesaplama ve
// uyarlanabilir filtreleme ile karmaşık sinyal ortamını temizler.
package filter

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/otukenmatris/sozgec/internal/spectrum"
)

// Allegiance — sinyal aidiyeti.
type Allegiance string

const (
	AllegianceFriendly Allegiance = "friendly"
	AllegianceHostile  Allegiance = "hostile"
	AllegianceNeutral  Allegiance = "neutral"
	AllegianceUnknown  Allegiance = "unknown"
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

// FriendlyBandDef — dost frekans bandı tanımı.
type FriendlyBandDef struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	LowMHz   float64 `json:"low_mhz"`
	HighMHz  float64 `json:"high_mhz"`
	Priority int     `json:"priority"` // 1=en yüksek
}

// FilteredSignal — filtrelenmiş ve sınıflandırılmış sinyal.
type FilteredSignal struct {
	ID          string                `json:"id"`
	Band        spectrum.FrequencyBand `json:"band"`
	Allegiance  Allegiance            `json:"allegiance"`
	ThreatLevel ThreatLevel           `json:"threat_level"`
	SNR_DB      float64               `json:"snr_db"`
	Filtered    bool                  `json:"filtered"` // gürültü olarak filtrelendi mi
	MatchedBand string                `json:"matched_band,omitempty"` // eşleşen dost bant
	Notes       string                `json:"notes"`
	ProcessedAt time.Time             `json:"processed_at"`
}

// FilterResult — filtreleme sonucu.
type FilterResult struct {
	Signals         []FilteredSignal `json:"signals"`
	TotalInput      int              `json:"total_input"`
	Extracted       int              `json:"extracted"`
	NoiseFiltered   int              `json:"noise_filtered"`
	FriendlyCount   int              `json:"friendly_count"`
	HostileCount    int              `json:"hostile_count"`
	JammingDetected int              `json:"jamming_detected"`
	AvgSNR          float64          `json:"avg_snr_db"`
	ElapsedMs       int64            `json:"elapsed_ms"`
}

// Engine — DSP filtre motoru.
type Engine struct {
	mu            sync.RWMutex
	noiseFloorDB  float64
	snrMinDB      float64
	maxSignals    int
	adaptiveAlpha float64
	threatDecayS  int

	friendlyBands []FriendlyBandDef
	signals       map[string]*FilteredSignal

	totalProcessed  int
	totalFiltered   int
	totalFriendly   int
	totalHostile    int
	totalJamming    int
}

// NewEngine — yeni filtre motoru.
func NewEngine(noiseFloorDB, snrMinDB float64, maxSignals int, adaptiveAlpha float64, threatDecayS int) *Engine {
	e := &Engine{
		noiseFloorDB:  noiseFloorDB,
		snrMinDB:      snrMinDB,
		maxSignals:    maxSignals,
		adaptiveAlpha: adaptiveAlpha,
		threatDecayS:  threatDecayS,
		signals:       make(map[string]*FilteredSignal),
	}
	e.initDefaultFriendlyBands()
	return e
}

// varsayılan dost frekans bantları (NATO UHF/VHF + Link-16)
func (e *Engine) initDefaultFriendlyBands() {
	e.friendlyBands = []FriendlyBandDef{
		{ID: "vhf-lo", Name: "VHF Düşük (Taktik)", LowMHz: 30, HighMHz: 88, Priority: 2},
		{ID: "vhf-hi", Name: "VHF Yüksek (Havacılık)", LowMHz: 108, HighMHz: 174, Priority: 3},
		{ID: "uhf-mil", Name: "UHF Askeri", LowMHz: 225, HighMHz: 400, Priority: 1},
		{ID: "link16", Name: "Link-16 Datalink", LowMHz: 960, HighMHz: 1215, Priority: 1},
		{ID: "gps-l1", Name: "GPS L1", LowMHz: 1575.2, HighMHz: 1575.6, Priority: 1},
		{ID: "gps-l2", Name: "GPS L2", LowMHz: 1227.4, HighMHz: 1227.8, Priority: 2},
		{ID: "satcom", Name: "SATCOM UHF", LowMHz: 290, HighMHz: 320, Priority: 2},
		{ID: "iff", Name: "IFF Transponder", LowMHz: 1030, HighMHz: 1090, Priority: 1},
	}
}

// ── Dost Bant Yönetimi ──

// AddFriendlyBand — dost frekans bandı ekle.
func (e *Engine) AddFriendlyBand(band FriendlyBandDef) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, b := range e.friendlyBands {
		if b.ID == band.ID {
			return fmt.Errorf("bant zaten var: %s", band.ID)
		}
	}
	e.friendlyBands = append(e.friendlyBands, band)
	return nil
}

// RemoveFriendlyBand — dost frekans bandını kaldır.
func (e *Engine) RemoveFriendlyBand(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, b := range e.friendlyBands {
		if b.ID == id {
			e.friendlyBands = append(e.friendlyBands[:i], e.friendlyBands[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("bant bulunamadı: %s", id)
}

// GetFriendlyBands — tüm dost frekans bantları.
func (e *Engine) GetFriendlyBands() []FriendlyBandDef {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]FriendlyBandDef, len(e.friendlyBands))
	copy(result, e.friendlyBands)
	return result
}

// ── Ana Filtreleme ──

// Filter — frekans bantlarını filtrele ve sınıflandır.
func (e *Engine) Filter(bands []spectrum.FrequencyBand, noiseFloor float64) FilterResult {
	start := time.Now()

	// uyarlanabilir gürültü tabanı güncelle
	e.mu.Lock()
	e.noiseFloorDB = e.noiseFloorDB*(1-e.adaptiveAlpha) + noiseFloor*e.adaptiveAlpha
	currentNF := e.noiseFloorDB
	friendlyBands := make([]FriendlyBandDef, len(e.friendlyBands))
	copy(friendlyBands, e.friendlyBands)
	e.mu.Unlock()

	var signals []FilteredSignal
	noiseFiltered := 0
	friendlyCount := 0
	hostileCount := 0
	jammingCount := 0
	var totalSNR float64

	for _, band := range bands {
		snr := band.PeakPowerDB - currentNF

		// gürültü filtresi
		if snr < e.snrMinDB {
			noiseFiltered++
			continue
		}

		allegiance, matchedBand := e.classifyAllegiance(band, friendlyBands)
		threat := assessThreat(band, allegiance, snr)
		notes := buildNotes(band, allegiance, snr)

		sig := FilteredSignal{
			ID:          fmt.Sprintf("sig-%s", band.ID),
			Band:        band,
			Allegiance:  allegiance,
			ThreatLevel: threat,
			SNR_DB:      snr,
			Filtered:    false,
			MatchedBand: matchedBand,
			Notes:       notes,
			ProcessedAt: time.Now(),
		}

		switch allegiance {
		case AllegianceFriendly:
			friendlyCount++
		case AllegianceHostile:
			hostileCount++
		}

		if band.SignalType == spectrum.SignalJamming {
			jammingCount++
		}

		totalSNR += snr
		signals = append(signals, sig)
	}

	// kaydet
	e.mu.Lock()
	for i := range signals {
		if len(e.signals) < e.maxSignals {
			e.signals[signals[i].ID] = &signals[i]
		}
	}
	e.totalProcessed += len(bands)
	e.totalFiltered += noiseFiltered
	e.totalFriendly += friendlyCount
	e.totalHostile += hostileCount
	e.totalJamming += jammingCount
	e.mu.Unlock()

	avgSNR := 0.0
	if len(signals) > 0 {
		avgSNR = totalSNR / float64(len(signals))
	}

	return FilterResult{
		Signals:         signals,
		TotalInput:      len(bands),
		Extracted:       len(signals),
		NoiseFiltered:   noiseFiltered,
		FriendlyCount:   friendlyCount,
		HostileCount:    hostileCount,
		JammingDetected: jammingCount,
		AvgSNR:          avgSNR,
		ElapsedMs:       time.Since(start).Milliseconds(),
	}
}

// classifyAllegiance — sinyal aidiyetini belirle.
func (e *Engine) classifyAllegiance(band spectrum.FrequencyBand, friendly []FriendlyBandDef) (Allegiance, string) {
	for _, fb := range friendly {
		if band.CenterMHz >= fb.LowMHz && band.CenterMHz <= fb.HighMHz {
			return AllegianceFriendly, fb.ID
		}
	}

	// karıştırma sinyali → düşman
	if band.SignalType == spectrum.SignalJamming {
		return AllegianceHostile, ""
	}

	// bilinmeyen radar → muhtemel düşman
	if band.SignalType == spectrum.SignalRadar {
		return AllegianceHostile, ""
	}

	return AllegianceUnknown, ""
}

func assessThreat(band spectrum.FrequencyBand, allegiance Allegiance, snr float64) ThreatLevel {
	if allegiance == AllegianceFriendly {
		return ThreatNone
	}

	score := 0.0

	// karıştırma → yüksek tehdit
	if band.SignalType == spectrum.SignalJamming {
		score += 0.5
	}
	// radar → orta tehdit
	if band.SignalType == spectrum.SignalRadar {
		score += 0.3
	}
	// düşman aidiyeti
	if allegiance == AllegianceHostile {
		score += 0.2
	}
	// güçlü sinyal
	if snr > 20 {
		score += 0.15
	}
	// güven skoru
	score += band.Confidence * 0.15

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

func buildNotes(band spectrum.FrequencyBand, allegiance Allegiance, snr float64) string {
	note := fmt.Sprintf("%.1f MHz, SNR %.1f dB", band.CenterMHz, snr)
	switch allegiance {
	case AllegianceFriendly:
		note += " — Dost sinyal"
	case AllegianceHostile:
		note += " — Düşman sinyal"
	case AllegianceUnknown:
		note += " — Tanımlanmamış"
	}
	if band.SignalType == spectrum.SignalJamming {
		note += " [KARIŞTIRMA TESPİT]"
	}
	return note
}

// ── Sorgu ──

// GetAllSignals — tüm aktif sinyaller.
func (e *Engine) GetAllSignals() []FilteredSignal {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]FilteredSignal, 0, len(e.signals))
	for _, s := range e.signals {
		result = append(result, *s)
	}
	return result
}

// GetHighThreat — yüksek tehdit sinyaller.
func (e *Engine) GetHighThreat() []FilteredSignal {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []FilteredSignal
	for _, s := range e.signals {
		if s.ThreatLevel == ThreatHigh || s.ThreatLevel == ThreatCritical {
			result = append(result, *s)
		}
	}
	return result
}

// GetFriendlySignals — dost sinyaller.
func (e *Engine) GetFriendlySignals() []FilteredSignal {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []FilteredSignal
	for _, s := range e.signals {
		if s.Allegiance == AllegianceFriendly {
			result = append(result, *s)
		}
	}
	return result
}

// ── Bakım ──

// Prune — eski sinyalleri temizle.
func (e *Engine) Prune() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	cutoff := time.Now().Add(-time.Duration(e.threatDecayS) * time.Second)
	removed := 0
	for id, s := range e.signals {
		if s.ProcessedAt.Before(cutoff) {
			delete(e.signals, id)
			removed++
		}
	}
	return removed
}

// Stats — istatistikler.
func (e *Engine) Stats() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()

	highCount := 0
	for _, s := range e.signals {
		if s.ThreatLevel == ThreatHigh || s.ThreatLevel == ThreatCritical {
			highCount++
		}
	}

	return map[string]any{
		"active_signals":   len(e.signals),
		"high_threats":     highCount,
		"total_processed":  e.totalProcessed,
		"total_filtered":   e.totalFiltered,
		"total_friendly":   e.totalFriendly,
		"total_hostile":    e.totalHostile,
		"total_jamming":    e.totalJamming,
		"noise_floor_db":   math.Round(e.noiseFloorDB*10) / 10,
		"friendly_bands":   len(e.friendlyBands),
	}
}

// GenerateSignalsFromEntities — savaş alanı varlıklarından sentetik sinyal üret.
func GenerateSignalsFromEntities(entities []map[string]any) []spectrum.RawSignal {
	var signals []spectrum.RawSignal

	freqMap := map[string]float64{
		"tank":       250.0,  // UHF taktik
		"apc":        260.0,
		"infantry":   50.0,   // VHF
		"artillery":  280.0,  // UHF
		"helicopter": 300.0,  // UHF havacılık
		"jet":        340.0,  // UHF havacılık
		"drone":      1100.0, // Link-16 yakını
		"radar":      3200.0, // S-bant radar
		"jammer":     270.0,  // geniş bant karıştırma
		"ship":       320.0,  // SATCOM
	}

	bwMap := map[string]float64{
		"tank": 2.0, "apc": 2.0, "infantry": 1.0, "artillery": 3.0,
		"helicopter": 5.0, "jet": 5.0, "drone": 10.0, "radar": 8.0,
		"jammer": 50.0, "ship": 4.0,
	}

	for _, ent := range entities {
		id, _ := ent["id"].(string)
		entType, _ := ent["type"].(string)
		faction, _ := ent["faction"].(string)
		lat, _ := toFloat(ent["lat"])
		lon, _ := toFloat(ent["lon"])

		freq := freqMap[entType]
		if freq == 0 {
			freq = 200 + float64(hashInt(id)%200)
		}
		bw := bwMap[entType]
		if bw == 0 {
			bw = 2.0
		}

		// düşman birimler farklı frekans bandında
		if faction == "enemy" {
			freq += 100 // dost banttan kayık
			if entType == "jammer" {
				bw = 80.0 // karıştırma genişliği
			}
		}

		amp := -30.0 + float64(hashInt(id+"-amp")%30) // -30 ile 0 dB arası
		dir := float64(hashInt(id+"-dir") % 360)
		phase := float64(hashInt(id+"-phase")%628) / 100.0

		signals = append(signals, spectrum.RawSignal{
			ID:           fmt.Sprintf("rf-%s", id),
			FrequencyMHz: freq,
			BandwidthMHz: bw,
			AmplitudeDB:  amp,
			PhaseRad:     phase,
			DirectionDeg: dir,
			Lat:          lat,
			Lon:          lon,
			Timestamp:    time.Now().UnixMilli(),
		})
	}

	return signals
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

func hashInt(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}
