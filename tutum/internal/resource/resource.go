// Package resource — Platform kaynak (CPU, bellek, disk, ağ, sıcaklık) izleme.
//
// Her platform için kaynak tüketimini takip eder, eşiklere göre
// durumunu belirler ve kaynak darboğazlarını raporlar.
package resource

import (
	"fmt"
	"sync"
	"time"
)

// ResourceType kaynak tipi.
type ResourceType string

const (
	ResCPU     ResourceType = "cpu"
	ResMemory  ResourceType = "memory"
	ResDisk    ResourceType = "disk"
	ResNetwork ResourceType = "network"
	ResTemp    ResourceType = "temperature"
)

// Level kaynak seviyesi.
type Level string

const (
	LevelNormal   Level = "normal"
	LevelWarning  Level = "warning"
	LevelCritical Level = "critical"
)

// Metric tek bir kaynak ölçümü.
type Metric struct {
	Timestamp time.Time    `json:"timestamp"`
	Type      ResourceType `json:"type"`
	Value     float64      `json:"value"`     // Yüzde veya °C
	Unit      string       `json:"unit"`
	Level     Level        `json:"level"`
}

// Thresholds eşik değerleri.
type Thresholds struct {
	Warning  float64 `json:"warning"`
	Critical float64 `json:"critical"`
}

// PlatformResources bir platformun kaynak durumu.
type PlatformResources struct {
	mu sync.RWMutex

	PlatformID string                      `json:"platform_id"`
	CPU        float64                     `json:"cpu_percent"`
	Memory     float64                     `json:"memory_percent"`
	Disk       float64                     `json:"disk_percent"`
	NetworkBps float64                     `json:"network_bps"`
	TempC      float64                     `json:"temp_c"`
	History    map[ResourceType][]Metric   `json:"history,omitempty"`
	MaxHistory int                         `json:"-"`
	Thresholds map[ResourceType]Thresholds `json:"-"`
	LastUpdate time.Time                   `json:"last_update"`
}

// NewPlatformResources yeni platform kaynak izleyicisi.
func NewPlatformResources(platformID string, maxHistory int, th map[ResourceType]Thresholds) *PlatformResources {
	if th == nil {
		th = DefaultThresholds()
	}
	return &PlatformResources{
		PlatformID: platformID,
		History:    make(map[ResourceType][]Metric),
		MaxHistory: maxHistory,
		Thresholds: th,
		LastUpdate: time.Now(),
	}
}

// DefaultThresholds varsayılan eşik değerleri.
func DefaultThresholds() map[ResourceType]Thresholds {
	return map[ResourceType]Thresholds{
		ResCPU:    {Warning: 70, Critical: 90},
		ResMemory: {Warning: 75, Critical: 90},
		ResDisk:   {Warning: 80, Critical: 95},
		ResTemp:   {Warning: 65, Critical: 80},
	}
}

// RecordMetric yeni kaynak ölçümü kaydeder.
func (pr *PlatformResources) RecordMetric(rtype ResourceType, value float64, unit string) Metric {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	level := pr.classify(rtype, value)
	m := Metric{
		Timestamp: time.Now(),
		Type:      rtype,
		Value:     value,
		Unit:      unit,
		Level:     level,
	}

	// Değeri güncelle
	switch rtype {
	case ResCPU:
		pr.CPU = value
	case ResMemory:
		pr.Memory = value
	case ResDisk:
		pr.Disk = value
	case ResNetwork:
		pr.NetworkBps = value
	case ResTemp:
		pr.TempC = value
	}

	// Geçmişe ekle
	pr.History[rtype] = append(pr.History[rtype], m)
	if len(pr.History[rtype]) > pr.MaxHistory {
		pr.History[rtype] = pr.History[rtype][len(pr.History[rtype])-pr.MaxHistory:]
	}

	pr.LastUpdate = time.Now()
	return m
}

func (pr *PlatformResources) classify(rtype ResourceType, value float64) Level {
	th, ok := pr.Thresholds[rtype]
	if !ok {
		return LevelNormal
	}
	if value >= th.Critical {
		return LevelCritical
	}
	if value >= th.Warning {
		return LevelWarning
	}
	return LevelNormal
}

// Snapshot anlık kaynak durumu.
type Snapshot struct {
	PlatformID string  `json:"platform_id"`
	CPU        float64 `json:"cpu_percent"`
	CPULevel   Level   `json:"cpu_level"`
	Memory     float64 `json:"memory_percent"`
	MemLevel   Level   `json:"memory_level"`
	Disk       float64 `json:"disk_percent"`
	DiskLevel  Level   `json:"disk_level"`
	NetworkBps float64 `json:"network_bps"`
	TempC      float64 `json:"temp_c"`
	TempLevel  Level   `json:"temp_level"`
	LastUpdate time.Time `json:"last_update"`
}

// GetSnapshot anlık durum döndürür.
func (pr *PlatformResources) GetSnapshot() Snapshot {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return Snapshot{
		PlatformID: pr.PlatformID,
		CPU:        pr.CPU,
		CPULevel:   pr.classify(ResCPU, pr.CPU),
		Memory:     pr.Memory,
		MemLevel:   pr.classify(ResMemory, pr.Memory),
		Disk:       pr.Disk,
		DiskLevel:  pr.classify(ResDisk, pr.Disk),
		NetworkBps: pr.NetworkBps,
		TempC:      pr.TempC,
		TempLevel:  pr.classify(ResTemp, pr.TempC),
		LastUpdate: pr.LastUpdate,
	}
}

// Tracker tüm platformların kaynaklarını izler.
type Tracker struct {
	mu        sync.RWMutex
	platforms map[string]*PlatformResources
	maxHist   int
	thresh    map[ResourceType]Thresholds
}

// NewTracker yeni kaynak izleyicisi.
func NewTracker(maxHistory int, th map[ResourceType]Thresholds) *Tracker {
	if th == nil {
		th = DefaultThresholds()
	}
	return &Tracker{
		platforms: make(map[string]*PlatformResources),
		maxHist:   maxHistory,
		thresh:    th,
	}
}

// GetOrCreate platform kaynağını döndürür, yoksa oluşturur.
func (t *Tracker) GetOrCreate(platformID string) *PlatformResources {
	t.mu.Lock()
	defer t.mu.Unlock()
	if pr, ok := t.platforms[platformID]; ok {
		return pr
	}
	pr := NewPlatformResources(platformID, t.maxHist, t.thresh)
	t.platforms[platformID] = pr
	return pr
}

// Get platform kaynağını döndürür.
func (t *Tracker) Get(platformID string) (*PlatformResources, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	pr, ok := t.platforms[platformID]
	return pr, ok
}

// Remove platform kaynağını siler.
func (t *Tracker) Remove(platformID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.platforms[platformID]; ok {
		delete(t.platforms, platformID)
		return true
	}
	return false
}

// AllSnapshots tüm platformların durumunu döndürür.
func (t *Tracker) AllSnapshots() []Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]Snapshot, 0, len(t.platforms))
	for _, pr := range t.platforms {
		result = append(result, pr.GetSnapshot())
	}
	return result
}

// CriticalResources kritik seviyedeki kaynakları döndürür.
func (t *Tracker) CriticalResources() []Metric {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var result []Metric
	for _, pr := range t.platforms {
		snap := pr.GetSnapshot()
		if snap.CPULevel == LevelCritical {
			result = append(result, Metric{Type: ResCPU, Value: snap.CPU, Level: LevelCritical, Unit: "%"})
		}
		if snap.MemLevel == LevelCritical {
			result = append(result, Metric{Type: ResMemory, Value: snap.Memory, Level: LevelCritical, Unit: "%"})
		}
		if snap.TempLevel == LevelCritical {
			result = append(result, Metric{Type: ResTemp, Value: snap.TempC, Level: LevelCritical, Unit: "°C"})
		}
	}
	return result
}

// Count kayıtlı platform sayısı.
func (t *Tracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.platforms)
}

// TrackerStats genel istatistikler.
type TrackerStats struct {
	TotalPlatforms int     `json:"total_platforms"`
	AvgCPU         float64 `json:"avg_cpu_percent"`
	AvgMemory      float64 `json:"avg_memory_percent"`
	MaxCPU         float64 `json:"max_cpu_percent"`
	MaxMemory      float64 `json:"max_memory_percent"`
	MaxTemp        float64 `json:"max_temp_c"`
	CriticalCount  int     `json:"critical_resource_count"`
}

// GetStats genel istatistik.
func (t *Tracker) GetStats() TrackerStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s := TrackerStats{TotalPlatforms: len(t.platforms)}
	if len(t.platforms) == 0 {
		return s
	}

	for _, pr := range t.platforms {
		snap := pr.GetSnapshot()
		s.AvgCPU += snap.CPU
		s.AvgMemory += snap.Memory
		if snap.CPU > s.MaxCPU {
			s.MaxCPU = snap.CPU
		}
		if snap.Memory > s.MaxMemory {
			s.MaxMemory = snap.Memory
		}
		if snap.TempC > s.MaxTemp {
			s.MaxTemp = snap.TempC
		}
		if snap.CPULevel == LevelCritical || snap.MemLevel == LevelCritical || snap.TempLevel == LevelCritical {
			s.CriticalCount++
		}
	}

	n := float64(len(t.platforms))
	s.AvgCPU /= n
	s.AvgMemory /= n
	return s
}

// HistoryForPlatform belirli bir platformun belirli kaynağının geçmişini döndürür.
func (t *Tracker) HistoryForPlatform(platformID string, rtype ResourceType, limit int) ([]Metric, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	pr, ok := t.platforms[platformID]
	if !ok {
		return nil, fmt.Errorf("platform bulunamadı: %s", platformID)
	}
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	hist := pr.History[rtype]
	if limit > 0 && len(hist) > limit {
		hist = hist[len(hist)-limit:]
	}
	result := make([]Metric, len(hist))
	copy(result, hist)
	return result, nil
}
