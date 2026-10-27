package anomaly

import (
	"testing"
	"time"

	"github.com/otukenmatris/dirlik/internal/collector"
	"github.com/otukenmatris/dirlik/internal/config"
)

func testConfig() config.Config {
	cfg := config.DefaultConfig()
	cfg.WindowSize = 10
	cfg.Thresholds.CPUPercent = 80
	cfg.Thresholds.MemPercent = 85
	cfg.Thresholds.DiskPercent = 90
	cfg.Thresholds.ErrorRate = 0.05
	cfg.Thresholds.NetLatencyMs = 100
	return cfg
}

func TestNewDetector(t *testing.T) {
	d := NewDetector(testConfig())
	if d == nil {
		t.Fatal("detector nil")
	}
	if d.windowSize != 10 {
		t.Errorf("window size beklenen 10, aldık %d", d.windowSize)
	}
}

func TestAnalyzeNormal(t *testing.T) {
	d := NewDetector(testConfig())
	snap := collector.SystemSnapshot{
		Timestamp:  time.Now(),
		CPUPercent: 30,
		MemPercent: 40,
		DiskPercent: 50,
		Goroutines: 10,
	}
	alerts := d.Analyze(snap)
	if len(alerts) != 0 {
		t.Errorf("normal durumda alarm beklenmedi, %d geldi", len(alerts))
	}
}

func TestAnalyzeCPUHigh(t *testing.T) {
	d := NewDetector(testConfig())
	snap := collector.SystemSnapshot{
		Timestamp:  time.Now(),
		CPUPercent: 90,
		MemPercent: 40,
		DiskPercent: 50,
	}
	alerts := d.Analyze(snap)
	found := false
	for _, a := range alerts {
		if a.Metric == "cpu_percent" {
			found = true
			if a.Severity != SeverityWarning {
				t.Errorf("CPU 90%% severity WARNING beklendi, %s geldi", a.Severity)
			}
		}
	}
	if !found {
		t.Error("CPU yüksek alarmı beklendi")
	}
}

func TestAnalyzeCPUCritical(t *testing.T) {
	d := NewDetector(testConfig())
	snap := collector.SystemSnapshot{
		Timestamp:  time.Now(),
		CPUPercent: 97,
		MemPercent: 40,
		DiskPercent: 50,
	}
	alerts := d.Analyze(snap)
	found := false
	for _, a := range alerts {
		if a.Metric == "cpu_percent" && a.Severity == SeverityCritical {
			found = true
		}
	}
	if !found {
		t.Error("CPU 97%% CRITICAL alarmı beklendi")
	}
}

func TestAnalyzeMemHigh(t *testing.T) {
	d := NewDetector(testConfig())
	snap := collector.SystemSnapshot{
		Timestamp:  time.Now(),
		CPUPercent: 10,
		MemPercent: 92,
		DiskPercent: 50,
	}
	alerts := d.Analyze(snap)
	found := false
	for _, a := range alerts {
		if a.Metric == "mem_percent" {
			found = true
		}
	}
	if !found {
		t.Error("RAM yüksek alarmı beklendi")
	}
}

func TestAnalyzeDiskHigh(t *testing.T) {
	d := NewDetector(testConfig())
	snap := collector.SystemSnapshot{
		Timestamp:  time.Now(),
		CPUPercent: 10,
		MemPercent: 40,
		DiskPercent: 96,
	}
	alerts := d.Analyze(snap)
	found := false
	for _, a := range alerts {
		if a.Metric == "disk_percent" {
			found = true
		}
	}
	if !found {
		t.Error("disk yüksek alarmı beklendi")
	}
}

func TestAnalyzeGoroutineLeak(t *testing.T) {
	d := NewDetector(testConfig())
	snap := collector.SystemSnapshot{
		Timestamp:  time.Now(),
		CPUPercent: 10,
		MemPercent: 40,
		DiskPercent: 50,
		Goroutines: 15000,
	}
	alerts := d.Analyze(snap)
	found := false
	for _, a := range alerts {
		if a.Metric == "goroutine_leak" {
			found = true
		}
	}
	if !found {
		t.Error("goroutine sızıntı alarmı beklendi")
	}
}

func TestAnalyzeGCPause(t *testing.T) {
	d := NewDetector(testConfig())
	snap := collector.SystemSnapshot{
		Timestamp:  time.Now(),
		CPUPercent: 10,
		MemPercent: 40,
		DiskPercent: 50,
		GCPauseNs:  50_000_000, // 50ms
	}
	alerts := d.Analyze(snap)
	found := false
	for _, a := range alerts {
		if a.Metric == "gc_pause" {
			found = true
		}
	}
	if !found {
		t.Error("GC pause alarmı beklendi")
	}
}

func TestAnalyzeServiceErrorRate(t *testing.T) {
	d := NewDetector(testConfig())
	m := collector.ServiceMetric{
		ServiceID:    "bag",
		ErrorRate:    0.15,
		RequestCount: 100,
		ErrorCount:   15,
		Latency:      5,
	}
	alerts := d.AnalyzeService(m)
	found := false
	for _, a := range alerts {
		if a.Metric == "error_rate" && a.ServiceID == "bag" {
			found = true
		}
	}
	if !found {
		t.Error("servis error rate alarmı beklendi")
	}
}

func TestAnalyzeServiceLatency(t *testing.T) {
	d := NewDetector(testConfig())
	m := collector.ServiceMetric{
		ServiceID: "kutuk",
		Latency:   250,
		ErrorRate: 0,
	}
	alerts := d.AnalyzeService(m)
	found := false
	for _, a := range alerts {
		if a.Metric == "service_latency" && a.ServiceID == "kutuk" {
			found = true
		}
	}
	if !found {
		t.Error("servis latency alarmı beklendi")
	}
}

func TestAlertsBySeverity(t *testing.T) {
	d := NewDetector(testConfig())

	// Birkaç alarm üret
	d.Analyze(collector.SystemSnapshot{CPUPercent: 90, MemPercent: 40, DiskPercent: 50, Timestamp: time.Now()})
	d.Analyze(collector.SystemSnapshot{CPUPercent: 97, MemPercent: 40, DiskPercent: 50, Timestamp: time.Now()})

	counts := d.AlertsBySeverity()
	total := 0
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		t.Error("severity sayacı boş")
	}
}

func TestAlerts(t *testing.T) {
	d := NewDetector(testConfig())
	d.Analyze(collector.SystemSnapshot{CPUPercent: 95, MemPercent: 40, DiskPercent: 50, Timestamp: time.Now()})

	all := d.Alerts(0)
	if len(all) == 0 {
		t.Error("alarm geçmişi boş")
	}

	last1 := d.Alerts(1)
	if len(last1) != 1 {
		t.Errorf("1 alarm beklendi, %d geldi", len(last1))
	}
}

func TestOnAlertCallback(t *testing.T) {
	d := NewDetector(testConfig())
	called := false
	d.OnAlert(func(a Alert) {
		called = true
	})

	d.Analyze(collector.SystemSnapshot{CPUPercent: 95, MemPercent: 40, DiskPercent: 50, Timestamp: time.Now()})

	if !called {
		t.Error("OnAlert callback çağrılmadı")
	}
}

func TestSlidingWindow(t *testing.T) {
	sw := newSlidingWindow(5)
	sw.Add(10)
	sw.Add(20)
	sw.Add(30)

	mean := sw.Mean()
	if mean != 20 {
		t.Errorf("mean beklenen 20, aldık %f", mean)
	}

	std := sw.StdDev()
	if std < 8 || std > 9 {
		t.Errorf("stddev beklenen ~8.16, aldık %f", std)
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		s    Severity
		want string
	}{
		{SeverityInfo, "INFO"},
		{SeverityWarning, "WARNING"},
		{SeverityCritical, "CRITICAL"},
		{SeverityFatal, "FATAL"},
		{Severity(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Severity(%d).String() = %s, beklenen %s", tt.s, got, tt.want)
		}
	}
}
