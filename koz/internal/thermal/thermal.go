// Package thermal — termal iz tespit ve takip motoru.
//
// IR sensör verilerinden termal imzaları tespit eder, sınıflandırır
// ve motor sıcaklığı profilleri üzerinden hedef türünü belirler.
package thermal

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// SignatureType — termal imza türü.
type SignatureType string

const (
	SigEngine     SignatureType = "engine"      // motor ısısı
	SigExhaust    SignatureType = "exhaust"      // egzoz/jet nozul
	SigFriction   SignatureType = "friction"     // aerodinamik sürtünme
	SigGenerator  SignatureType = "generator"    // jeneratör/güç kaynağı
	SigPersonnel  SignatureType = "personnel"    // insan vücut ısısı
	SigElectrical SignatureType = "electrical"   // elektronik ekipman
	SigExplosion  SignatureType = "post_explosion" // patlama sonrası
)

// ThermalState — termal durum.
type ThermalState string

const (
	StateHeating  ThermalState = "heating"  // ısınıyor
	StateSteady   ThermalState = "steady"   // sabit
	StateCooling  ThermalState = "cooling"  // soğuyor
	StateOff      ThermalState = "off"      // kapalı/soğuk
)

// Signature — tespit edilen termal imza.
type Signature struct {
	ID            string        `json:"id"`
	EntityID      string        `json:"entity_id,omitempty"`
	Type          SignatureType `json:"type"`
	State         ThermalState  `json:"state"`
	TempC         float64       `json:"temp_c"`          // ölçülen sıcaklık
	AmbientC      float64       `json:"ambient_c"`       // ortam sıcaklığı
	DeltaC        float64       `json:"delta_c"`         // fark
	PeakC         float64       `json:"peak_c"`          // zirve sıcaklık
	CoolingRateC  float64       `json:"cooling_rate_c"`  // soğuma hızı (°C/s)
	Lat           float64       `json:"lat"`
	Lon           float64       `json:"lon"`
	Confidence    float64       `json:"confidence"`      // 0.0-1.0
	Intensity     float64       `json:"intensity"`       // 0.0-1.0 normalize yoğunluk
	RadiusM       float64       `json:"radius_m"`        // termal iz yarıçapı
	VehicleClass  string        `json:"vehicle_class"`   // tahmin edilen araç sınıfı
	DetectedAt    time.Time     `json:"detected_at"`
	LastUpdated   time.Time     `json:"last_updated"`
	History       []TempReading `json:"history,omitempty"`
}

// TempReading — sıcaklık ölçümü zaman serisi noktası.
type TempReading struct {
	TempC     float64   `json:"temp_c"`
	Timestamp time.Time `json:"timestamp"`
}

// ScanResult — tarama sonucu.
type ScanResult struct {
	Signatures []Signature `json:"signatures"`
	ScanTime   time.Time   `json:"scan_time"`
	SensorID   string      `json:"sensor_id"`
	AmbientC   float64     `json:"ambient_c"`
	TotalPixels int        `json:"total_pixels"`
	HotPixels   int        `json:"hot_pixels"`
	ElapsedMs   float64    `json:"elapsed_ms"`
}

// VehicleProfile — araç termal profili (bilinen motor sıcaklık aralıkları).
type VehicleProfile struct {
	Class       string  // araç sınıfı
	IdleTempC   float64 // rölanti sıcaklığı
	ActiveTempC float64 // aktif sıcaklık
	PeakTempC   float64 // maksimum sıcaklık
	CoolingRate float64 // ortalama soğuma hızı (°C/s)
	RadiusM     float64 // termal iz yarıçapı
}

// bilinen araç termal profilleri
var vehicleProfiles = map[string]VehicleProfile{
	"tank":          {Class: "tank", IdleTempC: 45, ActiveTempC: 120, PeakTempC: 180, CoolingRate: 0.3, RadiusM: 8},
	"apc":           {Class: "apc", IdleTempC: 40, ActiveTempC: 95, PeakTempC: 140, CoolingRate: 0.4, RadiusM: 6},
	"truck":         {Class: "truck", IdleTempC: 35, ActiveTempC: 85, PeakTempC: 120, CoolingRate: 0.5, RadiusM: 5},
	"humvee":        {Class: "humvee", IdleTempC: 32, ActiveTempC: 75, PeakTempC: 110, CoolingRate: 0.6, RadiusM: 4},
	"helicopter":    {Class: "helicopter", IdleTempC: 50, ActiveTempC: 200, PeakTempC: 350, CoolingRate: 0.8, RadiusM: 10},
	"fighter_jet":   {Class: "fighter_jet", IdleTempC: 60, ActiveTempC: 400, PeakTempC: 800, CoolingRate: 1.5, RadiusM: 15},
	"drone":         {Class: "drone", IdleTempC: 28, ActiveTempC: 45, PeakTempC: 65, CoolingRate: 1.0, RadiusM: 2},
	"uav":           {Class: "uav", IdleTempC: 28, ActiveTempC: 45, PeakTempC: 65, CoolingRate: 1.0, RadiusM: 2},
	"ucav":          {Class: "ucav", IdleTempC: 35, ActiveTempC: 90, PeakTempC: 150, CoolingRate: 0.9, RadiusM: 4},
	"sam_launcher":  {Class: "sam_launcher", IdleTempC: 30, ActiveTempC: 55, PeakTempC: 500, CoolingRate: 0.4, RadiusM: 6},
	"artillery":     {Class: "artillery", IdleTempC: 25, ActiveTempC: 60, PeakTempC: 300, CoolingRate: 0.6, RadiusM: 5},
	"generator":     {Class: "generator", IdleTempC: 35, ActiveTempC: 70, PeakTempC: 100, CoolingRate: 0.3, RadiusM: 3},
	"radar":         {Class: "radar", IdleTempC: 30, ActiveTempC: 50, PeakTempC: 75, CoolingRate: 0.7, RadiusM: 4},
	"command_post":  {Class: "command_post", IdleTempC: 28, ActiveTempC: 42, PeakTempC: 55, CoolingRate: 0.5, RadiusM: 6},
	"infantry":      {Class: "infantry", IdleTempC: 36, ActiveTempC: 37, PeakTempC: 38, CoolingRate: 0.1, RadiusM: 0.5},
	"soldier":       {Class: "soldier", IdleTempC: 36, ActiveTempC: 37, PeakTempC: 38, CoolingRate: 0.1, RadiusM: 0.5},
}

// Engine — termal iz tespit motoru.
type Engine struct {
	mu         sync.RWMutex
	signatures map[string]*Signature
	ambientC   float64
	deltaMin   float64
	maxAge     int
	resolution int
	fovDeg     float64
	history    int
	scanCount  int64
	detectCount int64
}

// NewEngine — yeni termal iz motoru oluştur.
func NewEngine(ambientC, deltaMin float64, maxAge, resolution int, fovDeg float64, history int) *Engine {
	return &Engine{
		signatures: make(map[string]*Signature),
		ambientC:   ambientC,
		deltaMin:   deltaMin,
		maxAge:     maxAge,
		resolution: resolution,
		fovDeg:     fovDeg,
		history:    history,
	}
}

// Scan — termal tarama yap. Varlık listesinden termal imzalar üret.
func (e *Engine) Scan(entities []EntityInput) ScanResult {
	start := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()

	e.scanCount++
	now := time.Now()
	var detected []Signature
	hotPixels := 0

	for _, ent := range entities {
		sig := e.processEntity(ent, now)
		if sig != nil {
			detected = append(detected, *sig)
			hotPixels += int(sig.Intensity * 100)
			e.detectCount++
		}
	}

	// eski imzaları temizle
	for id, sig := range e.signatures {
		if now.Sub(sig.LastUpdated).Seconds() > float64(e.maxAge) {
			delete(e.signatures, id)
		}
	}

	return ScanResult{
		Signatures:  detected,
		ScanTime:    now,
		SensorID:    "IR-PRIMARY",
		AmbientC:    e.ambientC,
		TotalPixels: e.resolution * e.resolution,
		HotPixels:   hotPixels,
		ElapsedMs:   float64(time.Since(start).Microseconds()) / 1000.0,
	}
}

// EntityInput — tarama için varlık girişi.
type EntityInput struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"`
	Faction string  `json:"faction"`
	Status  string  `json:"status"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Speed   float64 `json:"speed"`
	Health  float64 `json:"health"`
}

func (e *Engine) processEntity(ent EntityInput, now time.Time) *Signature {
	profile, ok := vehicleProfiles[ent.Type]
	if !ok {
		profile = VehicleProfile{
			Class: ent.Type, IdleTempC: 30, ActiveTempC: 50,
			PeakTempC: 70, CoolingRate: 0.5, RadiusM: 3,
		}
	}

	// durum bazlı sıcaklık hesapla
	var tempC float64
	var state ThermalState
	var sigType SignatureType

	switch {
	case ent.Status == "destroyed":
		// imha sonrası soğuyan enkaz
		tempC = profile.PeakTempC * 0.6 * (0.3 + rand.Float64()*0.4)
		state = StateCooling
		sigType = SigExplosion
	case ent.Speed > 50:
		// yüksek hızda aktif motor + sürtünme
		speedFactor := math.Min(1.0, ent.Speed/200.0)
		tempC = profile.ActiveTempC + (profile.PeakTempC-profile.ActiveTempC)*speedFactor
		state = StateHeating
		sigType = SigEngine
		if isAircraft(ent.Type) {
			sigType = SigFriction
		}
	case ent.Speed > 5:
		// orta hız — normal motor çalışması
		speedFactor := ent.Speed / 50.0
		tempC = profile.IdleTempC + (profile.ActiveTempC-profile.IdleTempC)*speedFactor
		state = StateSteady
		sigType = SigEngine
	case ent.Speed <= 5 && ent.Speed > 0:
		// rölanti
		tempC = profile.IdleTempC + rand.Float64()*5
		state = StateSteady
		sigType = SigEngine
	default:
		// durağan — jeneratör/elektronik ısısı
		if isInstallation(ent.Type) {
			tempC = profile.IdleTempC + rand.Float64()*10
			state = StateSteady
			sigType = SigElectrical
		} else if isPersonnel(ent.Type) {
			tempC = 36.5 + rand.Float64()*1.0
			state = StateSteady
			sigType = SigPersonnel
		} else {
			tempC = profile.IdleTempC * (0.5 + rand.Float64()*0.3)
			state = StateCooling
			sigType = SigEngine
		}
	}

	// sağlık hasarı — hasarlı araçlar daha sıcak
	if ent.Health < 50 && ent.Health > 0 {
		damageFactor := 1.0 + (50-ent.Health)/100.0
		tempC *= damageFactor
	}

	// rastgele gürültü ekle (sensör hatası)
	tempC += (rand.Float64() - 0.5) * 2.0

	deltaC := tempC - e.ambientC
	if deltaC < e.deltaMin {
		return nil // eşik altı — tespit yok
	}

	// güven hesapla: yüksek delta = yüksek güven
	confidence := math.Min(1.0, deltaC/(profile.PeakTempC-e.ambientC+1)*1.5)
	confidence = math.Max(0.1, confidence)

	// yoğunluk (normalize edilmiş)
	intensity := math.Min(1.0, deltaC/200.0)

	// mevcut imza güncelle veya yeni oluştur
	existing, exists := e.signatures[ent.ID]
	if exists {
		existing.TempC = tempC
		existing.DeltaC = deltaC
		existing.State = state
		existing.Confidence = confidence
		existing.Intensity = intensity
		existing.Lat = ent.Lat
		existing.Lon = ent.Lon
		existing.LastUpdated = now
		if tempC > existing.PeakC {
			existing.PeakC = tempC
		}
		// soğuma hızı hesapla
		if len(existing.History) > 0 {
			last := existing.History[len(existing.History)-1]
			dt := now.Sub(last.Timestamp).Seconds()
			if dt > 0 {
				existing.CoolingRateC = (last.TempC - tempC) / dt
			}
		}
		existing.History = append(existing.History, TempReading{TempC: tempC, Timestamp: now})
		if len(existing.History) > e.history {
			existing.History = existing.History[len(existing.History)-e.history:]
		}
		result := *existing
		return &result
	}

	// araç sınıfı tahmini
	vehicleClass := classifyByTemp(tempC, sigType)

	sig := &Signature{
		ID:           "th-" + ent.ID,
		EntityID:     ent.ID,
		Type:         sigType,
		State:        state,
		TempC:        tempC,
		AmbientC:     e.ambientC,
		DeltaC:       deltaC,
		PeakC:        tempC,
		CoolingRateC: 0,
		Lat:          ent.Lat,
		Lon:          ent.Lon,
		Confidence:   confidence,
		Intensity:    intensity,
		RadiusM:      profile.RadiusM,
		VehicleClass: vehicleClass,
		DetectedAt:   now,
		LastUpdated:  now,
		History:      []TempReading{{TempC: tempC, Timestamp: now}},
	}
	e.signatures[ent.ID] = sig
	result := *sig
	return &result
}

// GetSignature — tek imza getir.
func (e *Engine) GetSignature(id string) (*Signature, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	sig, ok := e.signatures[id]
	if !ok {
		return nil, false
	}
	result := *sig
	return &result, true
}

// GetAllSignatures — tüm aktif imzalar.
func (e *Engine) GetAllSignatures() []Signature {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]Signature, 0, len(e.signatures))
	for _, sig := range e.signatures {
		result = append(result, *sig)
	}
	return result
}

// GetHotSignatures — belirli sıcaklık üstü imzalar.
func (e *Engine) GetHotSignatures(minDeltaC float64) []Signature {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []Signature
	for _, sig := range e.signatures {
		if sig.DeltaC >= minDeltaC {
			result = append(result, *sig)
		}
	}
	return result
}

// SetAmbient — ortam sıcaklığını güncelle.
func (e *Engine) SetAmbient(tempC float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ambientC = tempC
}

// Stats — motor istatistikleri.
func (e *Engine) Stats() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return map[string]any{
		"scan_count":      e.scanCount,
		"detect_count":    e.detectCount,
		"active_sigs":     len(e.signatures),
		"ambient_c":       e.ambientC,
		"detection_delta": e.deltaMin,
	}
}

// Prune — süresi dolmuş imzaları temizle.
func (e *Engine) Prune() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	removed := 0
	for id, sig := range e.signatures {
		if now.Sub(sig.LastUpdated).Seconds() > float64(e.maxAge) {
			delete(e.signatures, id)
			removed++
		}
	}
	return removed
}

func classifyByTemp(tempC float64, sigType SignatureType) string {
	if sigType == SigPersonnel {
		return "personnel"
	}
	switch {
	case tempC > 300:
		return "jet_engine"
	case tempC > 150:
		return "heavy_vehicle"
	case tempC > 80:
		return "medium_vehicle"
	case tempC > 50:
		return "light_vehicle"
	case tempC > 40:
		return "electrical_equipment"
	default:
		return "unknown"
	}
}

func isAircraft(t string) bool {
	return t == "drone" || t == "uav" || t == "ucav" || t == "helicopter" || t == "fighter_jet" || t == "aircraft"
}

func isInstallation(t string) bool {
	return t == "radar" || t == "sam_launcher" || t == "command_post" || t == "base" || t == "sensor" || t == "generator"
}

func isPersonnel(t string) bool {
	return t == "infantry" || t == "soldier" || t == "sniper" || t == "civilian"
}
