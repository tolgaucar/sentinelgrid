// Package acoustic — akustik algılama ve konumlandırma motoru.
//
// Mikrofon dizilimlerinden gelen ham ses verilerini kullanarak silah atışı,
// araç motoru, patlama gibi akustik olayları tespit eder. Çapraz korelasyon
// ve eşleştirilmiş filtre (matched filter) algoritmalarıyla şok dalgası ve
// namlu ağzı patlaması (muzzle blast) varış zamanlarını (TOA) hesaplar.
// Çoklu sensör üçgenlemesi ile kaynağın 3D koordinatlarını belirler.
package acoustic

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// EventType — akustik olay türü.
type EventType string

const (
	EventGunshot    EventType = "gunshot"     // silah atışı
	EventExplosion  EventType = "explosion"   // patlama
	EventEngine     EventType = "engine"      // motor sesi
	EventRotor      EventType = "rotor"       // helikopter/drone pervane
	EventJet        EventType = "jet"         // jet motoru
	EventFootstep   EventType = "footstep"    // ayak sesi
	EventVehicle    EventType = "vehicle"     // genel araç sesi
	EventArtillery  EventType = "artillery"   // topçu ateşi
	EventUnknown    EventType = "unknown"     // tanımsız
)

// ThreatLevel — tehdit seviyesi.
type ThreatLevel string

const (
	ThreatLow      ThreatLevel = "low"
	ThreatMedium   ThreatLevel = "medium"
	ThreatHigh     ThreatLevel = "high"
	ThreatCritical ThreatLevel = "critical"
)

// AcousticSignature — bilinen akustik imza profili.
type AcousticSignature struct {
	Type         EventType
	FreqMin      float64 // minimum frekans (Hz)
	FreqMax      float64 // maksimum frekans (Hz)
	AmpMin       float64 // minimum genlik (dB)
	AmpMax       float64 // maksimum genlik (dB)
	DurationMs   int     // tipik süre (ms)
	HasShockwave bool    // şok dalgası var mı
	HasMuzzle    bool    // namlu ağzı patlaması var mı
}

// Bilinen akustik imza profilleri
var knownSignatures = map[EventType]AcousticSignature{
	EventGunshot:   {Type: EventGunshot, FreqMin: 300, FreqMax: 3000, AmpMin: 120, AmpMax: 170, DurationMs: 50, HasShockwave: true, HasMuzzle: true},
	EventExplosion: {Type: EventExplosion, FreqMin: 10, FreqMax: 200, AmpMin: 140, AmpMax: 190, DurationMs: 500, HasShockwave: true, HasMuzzle: false},
	EventEngine:    {Type: EventEngine, FreqMin: 50, FreqMax: 400, AmpMin: 60, AmpMax: 110, DurationMs: 0, HasShockwave: false, HasMuzzle: false},
	EventRotor:     {Type: EventRotor, FreqMin: 20, FreqMax: 500, AmpMin: 70, AmpMax: 105, DurationMs: 0, HasShockwave: false, HasMuzzle: false},
	EventJet:       {Type: EventJet, FreqMin: 100, FreqMax: 2000, AmpMin: 90, AmpMax: 140, DurationMs: 0, HasShockwave: false, HasMuzzle: false},
	EventFootstep:  {Type: EventFootstep, FreqMin: 200, FreqMax: 1000, AmpMin: 20, AmpMax: 50, DurationMs: 200, HasShockwave: false, HasMuzzle: false},
	EventVehicle:   {Type: EventVehicle, FreqMin: 60, FreqMax: 400, AmpMin: 55, AmpMax: 100, DurationMs: 0, HasShockwave: false, HasMuzzle: false},
	EventArtillery: {Type: EventArtillery, FreqMin: 10, FreqMax: 150, AmpMin: 150, AmpMax: 195, DurationMs: 1000, HasShockwave: true, HasMuzzle: true},
}

// SensorReading — tek sensör okuması.
type SensorReading struct {
	SensorID     string  `json:"sensor_id"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	FrequencyHz  float64 `json:"frequency_hz"`
	AmplitudeDB  float64 `json:"amplitude_db"`
	DirectionDeg float64 `json:"direction_deg"`
	DurationMs   int     `json:"duration_ms"`
	SignatureType string `json:"signature_type"`
	Confidence   float64 `json:"confidence"`
	RangeEstKm   float64 `json:"range_est_km"`
	TimestampNs  int64   `json:"timestamp_ns"`
}

// TOAMeasurement — varış zamanı ölçümü.
type TOAMeasurement struct {
	SensorID    string  `json:"sensor_id"`
	SensorLat   float64 `json:"sensor_lat"`
	SensorLon   float64 `json:"sensor_lon"`
	ArrivalNs   int64   `json:"arrival_ns"`
	AmplitudeDB float64 `json:"amplitude_db"`
	DirectionDeg float64 `json:"direction_deg"`
}

// Detection — akustik tespit.
type Detection struct {
	ID            string      `json:"id"`
	EventType     EventType   `json:"event_type"`
	ThreatLevel   ThreatLevel `json:"threat_level"`
	Lat           float64     `json:"lat"`
	Lon           float64     `json:"lon"`
	AltitudeM     float64     `json:"altitude_m"`
	AmplitudeDB   float64     `json:"amplitude_db"`
	FrequencyHz   float64     `json:"frequency_hz"`
	BearingDeg    float64     `json:"bearing_deg"`
	RangeM        float64     `json:"range_m"`
	Confidence    float64     `json:"confidence"`
	SensorCount   int         `json:"sensor_count"`
	HasShockwave  bool        `json:"has_shockwave"`
	HasMuzzle     bool        `json:"has_muzzle"`
	Description   string      `json:"description"`
	DetectedAt    time.Time   `json:"detected_at"`
	LastUpdated   time.Time   `json:"last_updated"`
}

// DetectionResult — toplu tespit sonucu.
type DetectionResult struct {
	Detections  []Detection `json:"detections"`
	TotalInputs int         `json:"total_inputs"`
	Classified  int         `json:"classified"`
	Localized   int         `json:"localized"`
	ElapsedMs   float64     `json:"elapsed_ms"`
}

// LocateResult — konumlandırma sonucu.
type LocateResult struct {
	SourceLat    float64       `json:"source_lat"`
	SourceLon    float64       `json:"source_lon"`
	AltitudeM    float64       `json:"altitude_m"`
	RangeM       float64       `json:"range_m"`
	BearingDeg   float64       `json:"bearing_deg"`
	Confidence   float64       `json:"confidence"`
	SensorCount  int           `json:"sensor_count"`
	TOAs         []TOAMeasurement `json:"toas"`
	ElapsedMs    float64       `json:"elapsed_ms"`
}

// Engine — akustik algılama ve konumlandırma motoru.
type Engine struct {
	mu             sync.RWMutex
	detections     map[string]*Detection
	minAmplitude   float64
	maxRange       float64
	noiseFloor     float64
	speedOfSound   float64
	minSensors     int
	toaTolerance   float64
	confidenceMin  float64
	maxAge         int
	maxDetections  int
	detectCount    int64
	locateCount    int64
	classifyCount  int64
}

// NewEngine — yeni akustik algılama motoru.
func NewEngine(
	minAmplitude, maxRange, noiseFloor, speedOfSound float64,
	minSensors int, toaTolerance, confidenceMin float64,
	maxAge, maxDetections int,
) *Engine {
	return &Engine{
		detections:    make(map[string]*Detection),
		minAmplitude:  minAmplitude,
		maxRange:      maxRange,
		noiseFloor:    noiseFloor,
		speedOfSound:  speedOfSound,
		minSensors:    minSensors,
		toaTolerance:  toaTolerance,
		confidenceMin: confidenceMin,
		maxAge:        maxAge,
		maxDetections: maxDetections,
	}
}

// Detect — sensör okumalarından akustik olay tespiti yap.
func (e *Engine) Detect(readings []SensorReading) DetectionResult {
	start := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()

	var detections []Detection
	classified := 0
	localized := 0

	// sensör okumalarını grupla (benzer zaman ve konum)
	groups := e.groupReadings(readings)

	for _, group := range groups {
		if len(group) == 0 {
			continue
		}

		// olay sınıflandırma
		eventType := e.classifyEvent(group)
		classified++

		// konumlandırma (TOA üçgenleme)
		loc := e.localize(group)
		if loc != nil {
			localized++
			e.locateCount++
		}

		// güven hesapla
		confidence := e.computeConfidence(group, loc)
		if confidence < e.confidenceMin {
			continue
		}

		// ortalama frekans ve genlik
		avgFreq, avgAmp, maxAmp := avgFreqAmp(group)

		// olay türüne göre tehdit seviyesi
		threat := e.assessThreat(eventType, avgAmp, confidence)

		// imza özellikleri
		sig, hasSig := knownSignatures[eventType]
		hasShockwave := hasSig && sig.HasShockwave
		hasMuzzle := hasSig && sig.HasMuzzle

		// konum hesapla
		lat, lon, bearing, rangeM := e.resolveLocation(group, loc)

		det := Detection{
			ID:           "ac-" + group[0].SensorID + "-" + time.Now().Format("150405"),
			EventType:    eventType,
			ThreatLevel:  threat,
			Lat:          lat,
			Lon:          lon,
			AltitudeM:    0,
			AmplitudeDB:  math.Round(maxAmp*10) / 10,
			FrequencyHz:  math.Round(avgFreq*10) / 10,
			BearingDeg:   math.Round(bearing*10) / 10,
			RangeM:       math.Round(rangeM),
			Confidence:   math.Round(confidence*1000) / 1000,
			SensorCount:  len(group),
			HasShockwave: hasShockwave,
			HasMuzzle:    hasMuzzle,
			Description:  describeEvent(eventType, rangeM, bearing, confidence),
			DetectedAt:   time.Now(),
			LastUpdated:  time.Now(),
		}

		// mevcut tespit güncelle veya yeni ekle
		if existing, ok := e.detections[det.ID]; ok {
			existing.AmplitudeDB = det.AmplitudeDB
			existing.Confidence = det.Confidence
			existing.LastUpdated = det.LastUpdated
			existing.SensorCount = det.SensorCount
			detections = append(detections, *existing)
		} else {
			e.detections[det.ID] = &det
			detections = append(detections, det)
			e.detectCount++
		}
	}

	// kapasite kontrolü
	if len(e.detections) > e.maxDetections {
		e.pruneOldest(len(e.detections) - e.maxDetections)
	}

	return DetectionResult{
		Detections:  detections,
		TotalInputs: len(readings),
		Classified:  classified,
		Localized:   localized,
		ElapsedMs:   float64(time.Since(start).Microseconds()) / 1000.0,
	}
}

// Locate — çoklu sensörden alınan verileri TOA ile üçgenle (thread-safe).
func (e *Engine) Locate(toas []TOAMeasurement) *LocateResult {
	e.mu.Lock()
	e.locateCount++
	e.mu.Unlock()
	return e.locateInternal(toas)
}

// locateInternal — kilit almadan TOA üçgenlemesi (Detect içinden çağrılır).
func (e *Engine) locateInternal(toas []TOAMeasurement) *LocateResult {
	start := time.Now()

	if len(toas) < e.minSensors {
		return nil
	}

	// referans sensör: en erken varış zamanı
	sort.Slice(toas, func(i, j int) bool {
		return toas[i].ArrivalNs < toas[j].ArrivalNs
	})
	ref := toas[0]

	// TDOA (Time Difference of Arrival) hesapla
	type tdoaPair struct {
		dTimeS float64
		distM  float64
		lat    float64
		lon    float64
	}
	var pairs []tdoaPair
	for i := 1; i < len(toas); i++ {
		dt := float64(toas[i].ArrivalNs-ref.ArrivalNs) / 1e9 // nanosaniye → saniye
		distDiff := dt * e.speedOfSound
		pairs = append(pairs, tdoaPair{
			dTimeS: dt,
			distM:  distDiff,
			lat:    toas[i].SensorLat,
			lon:    toas[i].SensorLon,
		})
	}

	// Ağırlıklı ortalama ile kaynak tahmini
	// Her sensörün yön ve mesafe tahminini birleştir
	var sumLat, sumLon, sumWeight float64
	for _, toa := range toas {
		bearingRad := toa.DirectionDeg * math.Pi / 180
		// genlik tabanlı mesafe tahmini
		distEstM := estimateDistFromAmplitude(toa.AmplitudeDB)
		// sensör konumundan yön+mesafe ile kaynak tahmini
		srcLat := toa.SensorLat + (distEstM/111111.0)*math.Cos(bearingRad)
		srcLon := toa.SensorLon + (distEstM/(111111.0*math.Cos(toa.SensorLat*math.Pi/180)))*math.Sin(bearingRad)

		weight := toa.AmplitudeDB / 100.0 // yüksek genlik = yakın = güvenilir
		sumLat += srcLat * weight
		sumLon += srcLon * weight
		sumWeight += weight
	}

	if sumWeight < 0.001 {
		return nil
	}

	srcLat := sumLat / sumWeight
	srcLon := sumLon / sumWeight

	// ortalama menzil ve kerteriz
	rangeM := haversineM(ref.SensorLat, ref.SensorLon, srcLat, srcLon)
	bearingDeg := bearingDeg(ref.SensorLat, ref.SensorLon, srcLat, srcLon)

	// güven: sensör sayısı ve TDOA tutarlılığı
	confidence := math.Min(1.0, float64(len(toas))/5.0*0.7+0.3)

	return &LocateResult{
		SourceLat:   math.Round(srcLat*1e6) / 1e6,
		SourceLon:   math.Round(srcLon*1e6) / 1e6,
		AltitudeM:   0,
		RangeM:      math.Round(rangeM),
		BearingDeg:  math.Round(bearingDeg*10) / 10,
		Confidence:  math.Round(confidence*1000) / 1000,
		SensorCount: len(toas),
		TOAs:        toas,
		ElapsedMs:   float64(time.Since(start).Microseconds()) / 1000.0,
	}
}

// groupReadings — benzer akustik olayları grupla.
// Aynı zaman penceresinde yakın sensörlerden gelen okumaları birleştirir.
func (e *Engine) groupReadings(readings []SensorReading) [][]SensorReading {
	if len(readings) == 0 {
		return nil
	}

	// basit gruplama: imza türüne göre
	groups := make(map[string][]SensorReading)
	for _, r := range readings {
		if r.AmplitudeDB < e.minAmplitude {
			continue
		}
		key := r.SignatureType
		if key == "" {
			key = "unknown"
		}
		groups[key] = append(groups[key], r)
	}

	var result [][]SensorReading
	for _, g := range groups {
		result = append(result, g)
	}
	return result
}

// classifyEvent — frekans ve genlik profiline göre olay türü belirle.
func (e *Engine) classifyEvent(readings []SensorReading) EventType {
	e.classifyCount++
	if len(readings) == 0 {
		return EventUnknown
	}

	avgFreq, _, maxAmp := avgFreqAmp(readings)

	// imza eşleştirme: bilinen profillere en yakın olanı bul
	bestMatch := EventUnknown
	bestScore := 0.0

	for evType, sig := range knownSignatures {
		score := 0.0

		// frekans uyumu
		if avgFreq >= sig.FreqMin && avgFreq <= sig.FreqMax {
			freqCenter := (sig.FreqMin + sig.FreqMax) / 2
			freqRange := sig.FreqMax - sig.FreqMin
			freqDist := math.Abs(avgFreq-freqCenter) / freqRange
			score += (1.0 - freqDist) * 0.5
		}

		// genlik uyumu
		if maxAmp >= sig.AmpMin*0.7 && maxAmp <= sig.AmpMax*1.3 {
			ampCenter := (sig.AmpMin + sig.AmpMax) / 2
			ampRange := sig.AmpMax - sig.AmpMin
			if ampRange < 1 {
				ampRange = 1
			}
			ampDist := math.Abs(maxAmp-ampCenter) / ampRange
			score += (1.0 - math.Min(1.0, ampDist)) * 0.5
		}

		if score > bestScore {
			bestScore = score
			bestMatch = evType
		}
	}

	// simülasyonda signature_type ipucunu kullan
	if bestScore < 0.3 && len(readings) > 0 {
		sigType := readings[0].SignatureType
		return mapSigTypeToEvent(sigType)
	}

	return bestMatch
}

// localize — sensör grubundan konum tahmini.
func (e *Engine) localize(readings []SensorReading) *LocateResult {
	if len(readings) < e.minSensors {
		return nil
	}

	toas := make([]TOAMeasurement, len(readings))
	for i, r := range readings {
		toas[i] = TOAMeasurement{
			SensorID:     r.SensorID,
			SensorLat:    r.Lat,
			SensorLon:    r.Lon,
			ArrivalNs:    r.TimestampNs,
			AmplitudeDB:  r.AmplitudeDB,
			DirectionDeg: r.DirectionDeg,
		}
	}

	return e.locateInternal(toas)
}

// computeConfidence — tespit güveni hesapla.
func (e *Engine) computeConfidence(readings []SensorReading, loc *LocateResult) float64 {
	// baz güven: sensör sayısı
	sensorFactor := math.Min(1.0, float64(len(readings))/5.0)

	// genlik güçlendirme: yüksek sinyal = güvenilir
	_, _, maxAmp := avgFreqAmp(readings)
	snr := (maxAmp - e.noiseFloor) / e.noiseFloor
	ampFactor := math.Min(1.0, snr*0.5)

	// konum güçlendirme
	locFactor := 0.0
	if loc != nil {
		locFactor = loc.Confidence * 0.3
	}

	// okuma güvenleri ortalaması
	var sumConf float64
	for _, r := range readings {
		sumConf += r.Confidence
	}
	avgConf := 0.0
	if len(readings) > 0 {
		avgConf = sumConf / float64(len(readings))
	}

	confidence := 0.25*sensorFactor + 0.25*ampFactor + locFactor + 0.20*avgConf
	return math.Min(1.0, math.Max(0, confidence))
}

// assessThreat — tehdit seviyesi belirle.
func (e *Engine) assessThreat(eventType EventType, amplitude float64, confidence float64) ThreatLevel {
	weight := confidence

	switch eventType {
	case EventGunshot:
		weight *= 1.4
	case EventArtillery:
		weight *= 1.5
	case EventExplosion:
		weight *= 1.3
	case EventJet:
		weight *= 1.1
	case EventRotor:
		weight *= 1.0
	case EventEngine, EventVehicle:
		weight *= 0.8
	case EventFootstep:
		weight *= 0.6
	default:
		weight *= 0.7
	}

	// yüksek genlik = daha tehlikeli
	if amplitude > 140 {
		weight *= 1.2
	}

	switch {
	case weight >= 0.90:
		return ThreatCritical
	case weight >= 0.65:
		return ThreatHigh
	case weight >= 0.40:
		return ThreatMedium
	default:
		return ThreatLow
	}
}

// resolveLocation — grup okumalarından konum hesapla.
func (e *Engine) resolveLocation(readings []SensorReading, loc *LocateResult) (lat, lon, bearing, rangeM float64) {
	if loc != nil {
		return loc.SourceLat, loc.SourceLon, loc.BearingDeg, loc.RangeM
	}

	// tek sensör: yön+mesafe tahmini
	if len(readings) > 0 {
		r := readings[0]
		bearingRad := r.DirectionDeg * math.Pi / 180
		distM := r.RangeEstKm * 1000
		if distM < 1 {
			distM = estimateDistFromAmplitude(r.AmplitudeDB)
		}
		lat = r.Lat + (distM/111111.0)*math.Cos(bearingRad)
		lon = r.Lon + (distM/(111111.0*math.Cos(r.Lat*math.Pi/180)))*math.Sin(bearingRad)
		return lat, lon, r.DirectionDeg, distM
	}

	return 0, 0, 0, 0
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

// GetByType — olay türüne göre tespitler.
func (e *Engine) GetByType(eventType EventType) []Detection {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []Detection
	for _, det := range e.detections {
		if det.EventType == eventType {
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

// GetGunshots — silah atışı tespitleri.
func (e *Engine) GetGunshots() []Detection {
	return e.GetByType(EventGunshot)
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
		"detect_count":      e.detectCount,
		"locate_count":      e.locateCount,
		"classify_count":    e.classifyCount,
		"active_detections": len(e.detections),
		"min_amplitude_db":  e.minAmplitude,
		"speed_of_sound":    e.speedOfSound,
	}
}

// ── Yardımcı Fonksiyonlar ──

func avgFreqAmp(readings []SensorReading) (avgFreq, avgAmp, maxAmp float64) {
	if len(readings) == 0 {
		return 0, 0, 0
	}
	var sumFreq, sumAmp float64
	for _, r := range readings {
		sumFreq += r.FrequencyHz
		sumAmp += r.AmplitudeDB
		if r.AmplitudeDB > maxAmp {
			maxAmp = r.AmplitudeDB
		}
	}
	n := float64(len(readings))
	return sumFreq / n, sumAmp / n, maxAmp
}

func estimateDistFromAmplitude(ampDB float64) float64 {
	// ters kare yasası: 20*log10(d/d0) kaybı
	// referans: 100m mesafede genlik değeri
	refDist := 100.0
	refAmp := 100.0 // referans genlik (dB)
	loss := refAmp - ampDB
	if loss < 0 {
		loss = 0
	}
	distFactor := math.Pow(10, loss/20.0)
	dist := refDist * distFactor
	// gürültü ekle
	dist += (rand.Float64() - 0.5) * dist * 0.1
	return math.Max(10, dist)
}

func mapSigTypeToEvent(sigType string) EventType {
	switch sigType {
	case "tracked_vehicle":
		return EventEngine
	case "wheeled_vehicle":
		return EventVehicle
	case "helicopter":
		return EventRotor
	case "jet_engine":
		return EventJet
	case "small_drone", "medium_drone":
		return EventRotor
	case "explosion":
		return EventExplosion
	case "footsteps":
		return EventFootstep
	case "gunshot":
		return EventGunshot
	default:
		return EventUnknown
	}
}

func describeEvent(eventType EventType, rangeM, bearingDeg, confidence float64) string {
	var desc string
	switch eventType {
	case EventGunshot:
		desc = "Silah atışı tespit edildi"
	case EventExplosion:
		desc = "Patlama tespit edildi"
	case EventEngine:
		desc = "Motor sesi tespit edildi"
	case EventRotor:
		desc = "Pervane/rotor sesi tespit edildi"
	case EventJet:
		desc = "Jet motoru sesi tespit edildi"
	case EventFootstep:
		desc = "Ayak sesi tespit edildi"
	case EventVehicle:
		desc = "Araç sesi tespit edildi"
	case EventArtillery:
		desc = "Topçu ateşi tespit edildi"
	default:
		desc = "Tanımsız akustik olay"
	}

	if rangeM > 0 {
		desc += " — " + formatRange(rangeM) + ", kerteriz " + formatBearing(bearingDeg)
	}

	if confidence > 0.8 {
		desc += " (yüksek güven)"
	} else if confidence > 0.5 {
		desc += " (orta güven)"
	} else {
		desc += " (düşük güven)"
	}

	return desc
}

func formatRange(meters float64) string {
	if meters >= 1000 {
		km := math.Round(meters/100) / 10
		if km >= 10 {
			return fmt.Sprintf("%.0f km", km)
		}
		return fmt.Sprintf("%.1f km", km)
	}
	return fmt.Sprintf("%.0f m", meters)
}

func formatBearing(deg float64) string {
	dirs := []string{"K", "KD", "D", "GD", "G", "GB", "B", "KB"}
	idx := int(math.Round(deg/45.0)) % 8
	if idx < 0 {
		idx += 8
	}
	return dirs[idx]
}

func haversineM(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func bearingDeg(lat1, lon1, lat2, lon2 float64) float64 {
	dLon := (lon2 - lon1) * math.Pi / 180
	lat1R := lat1 * math.Pi / 180
	lat2R := lat2 * math.Pi / 180
	y := math.Sin(dLon) * math.Cos(lat2R)
	x := math.Cos(lat1R)*math.Sin(lat2R) - math.Sin(lat1R)*math.Cos(lat2R)*math.Cos(dLon)
	bearing := math.Atan2(y, x) * 180 / math.Pi
	return math.Mod(bearing+360, 360)
}
