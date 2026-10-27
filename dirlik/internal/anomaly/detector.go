// Package anomaly — Sliding window tabanlı anomali tespit motoru.
// CPU, RAM, Disk, ağ gecikme spike'larını eşik ve standart sapma ile algılar.
package anomaly

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/otukenmatris/dirlik/internal/collector"
	"github.com/otukenmatris/dirlik/internal/config"
)

// Severity — Alarm şiddeti
type Severity int

const (
	SeverityInfo     Severity = 0
	SeverityWarning  Severity = 1
	SeverityCritical Severity = 2
	SeverityFatal    Severity = 3
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityCritical:
		return "CRITICAL"
	case SeverityFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Alert — Tespit edilen anomali
type Alert struct {
	ID        string    `json:"id"`
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Severity  Severity  `json:"severity"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	ServiceID string    `json:"service_id,omitempty"`
}

// Detector — Anomali tespit motoru
type Detector struct {
	mu         sync.RWMutex
	thresholds config.Thresholds
	windowSize int
	alerts     []Alert
	maxAlerts  int
	alertCount int64

	// Sliding window verileri
	cpuWindow  *slidingWindow
	memWindow  *slidingWindow
	diskWindow *slidingWindow

	// Callback
	onAlert func(Alert)
}

type slidingWindow struct {
	values []float64
	size   int
	pos    int
	full   bool
}

func newSlidingWindow(size int) *slidingWindow {
	return &slidingWindow{
		values: make([]float64, size),
		size:   size,
	}
}

func (sw *slidingWindow) Add(v float64) {
	sw.values[sw.pos] = v
	sw.pos = (sw.pos + 1) % sw.size
	if sw.pos == 0 {
		sw.full = true
	}
}

func (sw *slidingWindow) Mean() float64 {
	n := sw.count()
	if n == 0 {
		return 0
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += sw.values[i]
	}
	return sum / float64(n)
}

func (sw *slidingWindow) StdDev() float64 {
	n := sw.count()
	if n < 2 {
		return 0
	}
	mean := sw.Mean()
	sumSq := 0.0
	for i := 0; i < n; i++ {
		d := sw.values[i] - mean
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(n))
}

func (sw *slidingWindow) count() int {
	if sw.full {
		return sw.size
	}
	return sw.pos
}

// NewDetector — Yeni anomali dedektörü
func NewDetector(cfg config.Config) *Detector {
	return &Detector{
		thresholds: cfg.Thresholds,
		windowSize: cfg.WindowSize,
		alerts:     make([]Alert, 0, 1000),
		maxAlerts:  10000,
		cpuWindow:  newSlidingWindow(cfg.WindowSize),
		memWindow:  newSlidingWindow(cfg.WindowSize),
		diskWindow: newSlidingWindow(cfg.WindowSize),
	}
}

// OnAlert — Alarm callback'i ayarla
func (d *Detector) OnAlert(fn func(Alert)) {
	d.mu.Lock()
	d.onAlert = fn
	d.mu.Unlock()
}

// Analyze — Snapshot'u analiz et, anomalileri tespit et
func (d *Detector) Analyze(snap collector.SystemSnapshot) []Alert {
	d.mu.Lock()
	defer d.mu.Unlock()

	var alerts []Alert

	// CPU kontrolü
	d.cpuWindow.Add(snap.CPUPercent)
	if snap.CPUPercent > d.thresholds.CPUPercent {
		severity := SeverityWarning
		if snap.CPUPercent > 95 {
			severity = SeverityCritical
		}
		alerts = append(alerts, d.createAlert("cpu_percent", snap.CPUPercent,
			d.thresholds.CPUPercent, severity,
			fmt.Sprintf("CPU kullanımı yüksek: %.1f%% (eşik: %.1f%%)", snap.CPUPercent, d.thresholds.CPUPercent)))
	}

	// CPU spike tespiti (ortalamadan 3 sigma)
	if cpuStd := d.cpuWindow.StdDev(); cpuStd > 0 {
		cpuMean := d.cpuWindow.Mean()
		if snap.CPUPercent > cpuMean+3*cpuStd && snap.CPUPercent > 30 {
			alerts = append(alerts, d.createAlert("cpu_spike", snap.CPUPercent,
				cpuMean+3*cpuStd, SeverityWarning,
				fmt.Sprintf("CPU spike tespit edildi: %.1f%% (ort: %.1f%%, σ: %.1f)", snap.CPUPercent, cpuMean, cpuStd)))
		}
	}

	// RAM kontrolü
	d.memWindow.Add(snap.MemPercent)
	if snap.MemPercent > d.thresholds.MemPercent {
		severity := SeverityWarning
		if snap.MemPercent > 98 {
			severity = SeverityCritical
		}
		alerts = append(alerts, d.createAlert("mem_percent", snap.MemPercent,
			d.thresholds.MemPercent, severity,
			fmt.Sprintf("Bellek kullanımı yüksek: %.1f%% (eşik: %.1f%%)", snap.MemPercent, d.thresholds.MemPercent)))
	}

	// Disk kontrolü
	d.diskWindow.Add(snap.DiskPercent)
	if snap.DiskPercent > d.thresholds.DiskPercent {
		severity := SeverityWarning
		if snap.DiskPercent > 99 {
			severity = SeverityFatal
		}
		alerts = append(alerts, d.createAlert("disk_percent", snap.DiskPercent,
			d.thresholds.DiskPercent, severity,
			fmt.Sprintf("Disk doluluk yüksek: %.1f%% (eşik: %.1f%%)", snap.DiskPercent, d.thresholds.DiskPercent)))
	}

	// GC pause kontrolü (>10ms uyarı)
	gcPauseMs := float64(snap.GCPauseNs) / 1e6
	if gcPauseMs > 10 {
		alerts = append(alerts, d.createAlert("gc_pause", gcPauseMs, 10,
			SeverityInfo,
			fmt.Sprintf("GC duraksama yüksek: %.2fms", gcPauseMs)))
	}

	// Goroutine sızıntı kontrolü (>10000)
	if snap.Goroutines > 10000 {
		alerts = append(alerts, d.createAlert("goroutine_leak", float64(snap.Goroutines), 10000,
			SeverityWarning,
			fmt.Sprintf("Goroutine sayısı yüksek: %d", snap.Goroutines)))
	}

	// Alarm kaydet ve bildir
	for _, a := range alerts {
		d.alerts = append(d.alerts, a)
		if len(d.alerts) > d.maxAlerts {
			d.alerts = d.alerts[1:]
		}
		d.alertCount++
		if d.onAlert != nil {
			d.onAlert(a)
		}
	}

	return alerts
}

// AnalyzeService — Servis metriklerini analiz et
func (d *Detector) AnalyzeService(m collector.ServiceMetric) []Alert {
	d.mu.Lock()
	defer d.mu.Unlock()

	var alerts []Alert

	// Hata oranı kontrolü
	if m.ErrorRate > d.thresholds.ErrorRate {
		severity := SeverityWarning
		if m.ErrorRate > 0.2 {
			severity = SeverityCritical
		}
		a := d.createAlert("error_rate", m.ErrorRate, d.thresholds.ErrorRate, severity,
			fmt.Sprintf("Servis %s hata oranı yüksek: %.2f%% (eşik: %.2f%%)", m.ServiceID, m.ErrorRate*100, d.thresholds.ErrorRate*100))
		a.ServiceID = m.ServiceID
		alerts = append(alerts, a)
	}

	// Gecikme kontrolü
	if m.Latency > d.thresholds.NetLatencyMs {
		severity := SeverityWarning
		if m.Latency > d.thresholds.NetLatencyMs*2 {
			severity = SeverityCritical
		}
		a := d.createAlert("service_latency", m.Latency, d.thresholds.NetLatencyMs, severity,
			fmt.Sprintf("Servis %s gecikme yüksek: %.1fms (eşik: %.1fms)", m.ServiceID, m.Latency, d.thresholds.NetLatencyMs))
		a.ServiceID = m.ServiceID
		alerts = append(alerts, a)
	}

	for _, a := range alerts {
		d.alerts = append(d.alerts, a)
		if len(d.alerts) > d.maxAlerts {
			d.alerts = d.alerts[1:]
		}
		d.alertCount++
		if d.onAlert != nil {
			d.onAlert(a)
		}
	}

	return alerts
}

// Alerts — Tüm alarmlar (son N)
func (d *Detector) Alerts(n int) []Alert {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if n <= 0 || n > len(d.alerts) {
		n = len(d.alerts)
	}
	start := len(d.alerts) - n
	result := make([]Alert, n)
	copy(result, d.alerts[start:])
	return result
}

// AlertCount — Toplam alarm sayısı
func (d *Detector) AlertCount() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.alertCount
}

// AlertsBySeverity — Şiddete göre alarm sayıları
func (d *Detector) AlertsBySeverity() map[string]int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	counts := map[string]int{
		"INFO": 0, "WARNING": 0, "CRITICAL": 0, "FATAL": 0,
	}
	for _, a := range d.alerts {
		counts[a.Severity.String()]++
	}
	return counts
}

func (d *Detector) createAlert(metric string, value, threshold float64, sev Severity, msg string) Alert {
	d.alertCount++
	return Alert{
		ID:        fmt.Sprintf("ALR-%06d", d.alertCount),
		Metric:    metric,
		Value:     math.Round(value*100) / 100,
		Threshold: threshold,
		Severity:  sev,
		Message:   msg,
		Timestamp: time.Now(),
	}
}
