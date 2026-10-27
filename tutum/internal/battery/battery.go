// Package battery — Platform batarya durumu izleme ve yönetimi.
//
// Her askeri platform (İHA, otonom araç, taktik radyo) için batarya
// durumunu takip eder. Coulomb sayımı, SoC (State of Charge) ve
// SoH (State of Health) hesaplar. Çevresel faktörleri (rüzgar, arazi,
// yük, sıcaklık) boşalma hızına yansıtır.
package battery

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// PlatformType platform tipi.
type PlatformType string

const (
	PlatformDrone  PlatformType = "iha"       // İnsansız Hava Aracı
	PlatformUGV    PlatformType = "otonom"     // Otonom Kara Aracı
	PlatformRadio  PlatformType = "radyo"      // Taktik Radyo
	PlatformSensor PlatformType = "sensor"     // Alan Sensörü
	PlatformRelay  PlatformType = "röle"       // İletişim Rölesi
)

// BatteryStatus batarya durum seviyesi.
type BatteryStatus string

const (
	StatusFull     BatteryStatus = "full"      // > LowPercent
	StatusLow      BatteryStatus = "low"       // <= LowPercent
	StatusWarning  BatteryStatus = "warning"   // <= WarningPercent
	StatusCritical BatteryStatus = "critical"  // <= CriticalPercent
	StatusDead     BatteryStatus = "dead"      // <= 0
)

// TerrainType arazi tipi — enerji tüketimini etkiler.
type TerrainType string

const (
	TerrainFlat      TerrainType = "düz"
	TerrainHilly     TerrainType = "engebeli"
	TerrainMountain  TerrainType = "dağlık"
	TerrainUrban     TerrainType = "kentsel"
)

// terrainFactors arazi tipi → enerji çarpanı.
var terrainFactors = map[TerrainType]float64{
	TerrainFlat:     1.0,
	TerrainHilly:    1.35,
	TerrainMountain: 1.70,
	TerrainUrban:    1.15,
}

// WindFactor rüzgar hızından (m/s) enerji çarpanı hesaplar.
// Hafif rüzgar (0-5 m/s) etkisiz, güçlü rüzgar katlanarak artar.
func WindFactor(windSpeedMS float64) float64 {
	if windSpeedMS <= 0 {
		return 1.0
	}
	if windSpeedMS <= 5 {
		return 1.0 + (windSpeedMS * 0.02) // %0-%10
	}
	// 5+ m/s: kuadratik artış
	return 1.10 + math.Pow((windSpeedMS-5)/10, 2)*0.5
}

// Sample tek bir batarya ölçümü.
type Sample struct {
	Timestamp   time.Time `json:"timestamp"`
	SoCPercent  float64   `json:"soc_percent"`  // 0-100 arası şarj yüzdesi
	VoltageV    float64   `json:"voltage_v"`     // Anlık voltaj
	CurrentA    float64   `json:"current_a"`     // Anlık akım (negatif=boşalma)
	TempC       float64   `json:"temp_c"`        // Hücre sıcaklığı °C
	PowerW      float64   `json:"power_w"`       // Anlık güç tüketimi Watt
}

// Platform izlenen bir platformun batarya durumu.
type Platform struct {
	mu sync.RWMutex

	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Type          PlatformType `json:"type"`
	CapacityWh    float64      `json:"capacity_wh"`     // Toplam kapasite Watt-saat
	RemainingWh   float64      `json:"remaining_wh"`    // Kalan enerji Watt-saat
	SoCPercent    float64      `json:"soc_percent"`     // State of Charge %
	SoHPercent    float64      `json:"soh_percent"`     // State of Health %
	Status        BatteryStatus `json:"status"`
	Terrain       TerrainType  `json:"terrain"`
	WindSpeedMS   float64      `json:"wind_speed_ms"`
	LoadFactor    float64      `json:"load_factor"`     // 1.0=normal, >1=ağır yük
	DischargeRate float64      `json:"discharge_rate"`  // Wh/dakika (EMA)
	CycleCount    int          `json:"cycle_count"`     // Şarj döngüsü sayısı
	LastUpdate    time.Time    `json:"last_update"`
	Samples       []Sample     `json:"samples,omitempty"`
	MaxSamples    int          `json:"-"`
	EMAAlpha      float64      `json:"-"`

	// thresholds
	criticalPct float64
	warningPct  float64
	lowPct      float64
}

// NewPlatform yeni bir platform oluşturur.
func NewPlatform(id, name string, ptype PlatformType, capacityWh float64) *Platform {
	return &Platform{
		ID:          id,
		Name:        name,
		Type:        ptype,
		CapacityWh:  capacityWh,
		RemainingWh: capacityWh,
		SoCPercent:  100.0,
		SoHPercent:  100.0,
		Status:      StatusFull,
		Terrain:     TerrainFlat,
		LoadFactor:  1.0,
		MaxSamples:  1000,
		EMAAlpha:    0.3,
		criticalPct: 10.0,
		warningPct:  25.0,
		lowPct:      40.0,
		LastUpdate:  time.Now(),
	}
}

// SetThresholds eşik değerlerini ayarlar.
func (p *Platform) SetThresholds(critical, warning, low float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.criticalPct = critical
	p.warningPct = warning
	p.lowPct = low
}

// UpdateConditions çevresel koşulları günceller.
func (p *Platform) UpdateConditions(terrain TerrainType, windMS, loadFactor float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Terrain = terrain
	p.WindSpeedMS = windMS
	if loadFactor > 0 {
		p.LoadFactor = loadFactor
	}
}

// EffectiveDischargeMultiplier anlık çevresel çarpanı.
func (p *Platform) EffectiveDischargeMultiplier() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.effectiveMultiplier()
}

func (p *Platform) effectiveMultiplier() float64 {
	tf, ok := terrainFactors[p.Terrain]
	if !ok {
		tf = 1.0
	}
	wf := WindFactor(p.WindSpeedMS)
	return tf * wf * p.LoadFactor
}

// RecordSample yeni bir ölçüm kaydeder ve batarya durumunu günceller.
func (p *Platform) RecordSample(voltageV, currentA, tempC float64) Sample {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	powerW := math.Abs(voltageV * currentA)

	// Çevresel çarpan ile efektif tüketim
	mult := p.effectiveMultiplier()
	effectivePower := powerW * mult

	// Geçen süre (dakika)
	dt := now.Sub(p.LastUpdate).Minutes()
	if dt < 0 {
		dt = 0
	}

	// Coulomb sayımı: tüketilen enerji (Wh)
	consumed := effectivePower * (dt / 60.0) // W × saat
	p.RemainingWh -= consumed
	if p.RemainingWh < 0 {
		p.RemainingWh = 0
	}

	// SoC güncelle
	if p.CapacityWh > 0 {
		p.SoCPercent = (p.RemainingWh / p.CapacityWh) * 100.0
	}

	// Boşalma hızı EMA (Wh/dakika)
	if dt > 0 {
		instantRate := consumed / dt
		if p.DischargeRate == 0 {
			p.DischargeRate = instantRate
		} else {
			p.DischargeRate = p.EMAAlpha*instantRate + (1-p.EMAAlpha)*p.DischargeRate
		}
	}

	// SoH: döngü sayısına göre basit bozunma modeli
	// Tipik Li-ion: ~500 döngüde %80 SoH
	if p.CycleCount > 0 {
		p.SoHPercent = math.Max(0, 100.0*(1.0-float64(p.CycleCount)/2500.0))
	}

	// Durum güncelle
	p.Status = p.calculateStatus()
	p.LastUpdate = now

	// Sıcaklık etkisi: aşırı sıcak/soğuk batarya kapasitesini düşürür
	tempFactor := TempDegradation(tempC)
	p.SoCPercent *= tempFactor

	s := Sample{
		Timestamp:  now,
		SoCPercent: p.SoCPercent,
		VoltageV:   voltageV,
		CurrentA:   currentA,
		TempC:      tempC,
		PowerW:     effectivePower,
	}

	p.Samples = append(p.Samples, s)
	if len(p.Samples) > p.MaxSamples {
		p.Samples = p.Samples[len(p.Samples)-p.MaxSamples:]
	}

	return s
}

// TempDegradation sıcaklık bozunma katsayısı.
// Optimum 20-35°C arası. Bu aralık dışında kapasite düşer.
func TempDegradation(tempC float64) float64 {
	if tempC >= 20 && tempC <= 35 {
		return 1.0
	}
	if tempC < 20 {
		// Soğukta kapasite düşüşü: -10°C'de ~%70
		return math.Max(0.5, 1.0-(20-tempC)*0.01)
	}
	// Sıcakta kapasite düşüşü: 50°C'de ~%85
	return math.Max(0.7, 1.0-(tempC-35)*0.01)
}

func (p *Platform) calculateStatus() BatteryStatus {
	if p.SoCPercent <= 0 {
		return StatusDead
	}
	if p.SoCPercent <= p.criticalPct {
		return StatusCritical
	}
	if p.SoCPercent <= p.warningPct {
		return StatusWarning
	}
	if p.SoCPercent <= p.lowPct {
		return StatusLow
	}
	return StatusFull
}

// GetSoC thread-safe SoC okuma.
func (p *Platform) GetSoC() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.SoCPercent
}

// GetStatus thread-safe durum okuma.
func (p *Platform) GetStatus() BatteryStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Status
}

// GetDischargeRate thread-safe boşalma hızı (Wh/dk).
func (p *Platform) GetDischargeRate() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.DischargeRate
}

// EstimateRemainingMinutes kalan süre tahmini (dakika).
func (p *Platform) EstimateRemainingMinutes() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.DischargeRate <= 0 {
		return math.Inf(1)
	}
	return p.RemainingWh / p.DischargeRate
}

// Snapshot thread-safe anlık kopyasını döndürür.
func (p *Platform) Snapshot() PlatformSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return PlatformSnapshot{
		ID:               p.ID,
		Name:             p.Name,
		Type:             p.Type,
		CapacityWh:       p.CapacityWh,
		RemainingWh:      p.RemainingWh,
		SoCPercent:       p.SoCPercent,
		SoHPercent:       p.SoHPercent,
		Status:           p.Status,
		Terrain:          p.Terrain,
		WindSpeedMS:      p.WindSpeedMS,
		LoadFactor:       p.LoadFactor,
		DischargeRate:    p.DischargeRate,
		CycleCount:       p.CycleCount,
		EffMultiplier:    p.effectiveMultiplier(),
		RemainingMinutes: p.remainingMinutes(),
		SampleCount:      len(p.Samples),
		LastUpdate:       p.LastUpdate,
	}
}

func (p *Platform) remainingMinutes() float64 {
	if p.DischargeRate <= 0 {
		return -1
	}
	return p.RemainingWh / p.DischargeRate
}

// PlatformSnapshot platform anlık durum görüntüsü.
type PlatformSnapshot struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Type             PlatformType  `json:"type"`
	CapacityWh       float64       `json:"capacity_wh"`
	RemainingWh      float64       `json:"remaining_wh"`
	SoCPercent       float64       `json:"soc_percent"`
	SoHPercent       float64       `json:"soh_percent"`
	Status           BatteryStatus `json:"status"`
	Terrain          TerrainType   `json:"terrain"`
	WindSpeedMS      float64       `json:"wind_speed_ms"`
	LoadFactor       float64       `json:"load_factor"`
	DischargeRate    float64       `json:"discharge_rate_wh_min"`
	CycleCount       int           `json:"cycle_count"`
	EffMultiplier    float64       `json:"effective_multiplier"`
	RemainingMinutes float64       `json:"remaining_minutes"`
	SampleCount      int           `json:"sample_count"`
	LastUpdate       time.Time     `json:"last_update"`
}

// Monitor tüm platformların batarya durumunu izler.
type Monitor struct {
	mu        sync.RWMutex
	platforms map[string]*Platform
	maxPlatforms int
}

// NewMonitor yeni batarya monitörü oluşturur.
func NewMonitor(maxPlatforms int) *Monitor {
	return &Monitor{
		platforms:    make(map[string]*Platform),
		maxPlatforms: maxPlatforms,
	}
}

// Register yeni platform kaydeder.
func (m *Monitor) Register(p *Platform) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.platforms) >= m.maxPlatforms {
		return fmt.Errorf("maksimum platform sayısına ulaşıldı: %d", m.maxPlatforms)
	}
	m.platforms[p.ID] = p
	return nil
}

// Remove platformu siler.
func (m *Monitor) Remove(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.platforms[id]; ok {
		delete(m.platforms, id)
		return true
	}
	return false
}

// Get platform döndürür.
func (m *Monitor) Get(id string) (*Platform, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.platforms[id]
	return p, ok
}

// Count kayıtlı platform sayısı.
func (m *Monitor) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.platforms)
}

// All tüm platformların anlık durumunu döndürür.
func (m *Monitor) All() []PlatformSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]PlatformSnapshot, 0, len(m.platforms))
	for _, p := range m.platforms {
		result = append(result, p.Snapshot())
	}
	return result
}

// CriticalPlatforms kritik durumdaki platformları döndürür.
func (m *Monitor) CriticalPlatforms() []PlatformSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []PlatformSnapshot
	for _, p := range m.platforms {
		st := p.GetStatus()
		if st == StatusCritical || st == StatusDead {
			result = append(result, p.Snapshot())
		}
	}
	return result
}

// Stats genel istatistikler.
type Stats struct {
	TotalPlatforms    int     `json:"total_platforms"`
	FullCount         int     `json:"full_count"`
	LowCount          int     `json:"low_count"`
	WarningCount      int     `json:"warning_count"`
	CriticalCount     int     `json:"critical_count"`
	DeadCount         int     `json:"dead_count"`
	AvgSoCPercent     float64 `json:"avg_soc_percent"`
	MinSoCPercent     float64 `json:"min_soc_percent"`
	TotalEnergyWh     float64 `json:"total_energy_wh"`
	RemainingEnergyWh float64 `json:"remaining_energy_wh"`
}

// GetStats toplu istatistik.
func (m *Monitor) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s := Stats{
		TotalPlatforms: len(m.platforms),
		MinSoCPercent:  100.0,
	}

	if len(m.platforms) == 0 {
		s.MinSoCPercent = 0
		return s
	}

	var socSum float64
	for _, p := range m.platforms {
		snap := p.Snapshot()
		socSum += snap.SoCPercent
		s.TotalEnergyWh += snap.CapacityWh
		s.RemainingEnergyWh += snap.RemainingWh
		if snap.SoCPercent < s.MinSoCPercent {
			s.MinSoCPercent = snap.SoCPercent
		}
		switch snap.Status {
		case StatusFull:
			s.FullCount++
		case StatusLow:
			s.LowCount++
		case StatusWarning:
			s.WarningCount++
		case StatusCritical:
			s.CriticalCount++
		case StatusDead:
			s.DeadCount++
		}
	}
	s.AvgSoCPercent = socSum / float64(len(m.platforms))
	return s
}
