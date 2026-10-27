// Package collector — Sistem metriklerini ms-bazlı toplayan çekirdek paket.
// CPU, RAM, Disk I/O ve ağ gecikme değerlerini ölçer.
package collector

import (
	"math"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Metric — Tek bir metrik ölçümü
type Metric struct {
	Name      string    `json:"name"`
	Value     float64   `json:"value"`
	Unit      string    `json:"unit"`
	Timestamp time.Time `json:"timestamp"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// SystemSnapshot — Anlık sistem durumu
type SystemSnapshot struct {
	Timestamp   time.Time `json:"timestamp"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemTotal    uint64    `json:"mem_total_bytes"`
	MemUsed     uint64    `json:"mem_used_bytes"`
	MemPercent  float64   `json:"mem_percent"`
	DiskTotal   uint64    `json:"disk_total_bytes"`
	DiskUsed    uint64    `json:"disk_used_bytes"`
	DiskPercent float64   `json:"disk_percent"`
	Goroutines  int       `json:"goroutines"`
	HeapAlloc   uint64    `json:"heap_alloc_bytes"`
	HeapSys     uint64    `json:"heap_sys_bytes"`
	GCPauseNs   uint64    `json:"gc_pause_ns"`
	NumGC       uint32    `json:"num_gc"`
	Uptime      float64   `json:"uptime_sec"`
}

// ServiceMetric — Servis bazlı metrik
type ServiceMetric struct {
	ServiceID    string    `json:"service_id"`
	Latency      float64   `json:"latency_ms"`
	RequestCount int64     `json:"request_count"`
	ErrorCount   int64     `json:"error_count"`
	ErrorRate    float64   `json:"error_rate"`
	Timestamp    time.Time `json:"timestamp"`
}

// Collector — Metrik toplayıcı
type Collector struct {
	mu          sync.RWMutex
	interval    time.Duration
	snapshots   []SystemSnapshot
	maxHistory  int
	startTime   time.Time
	running     int32
	stopCh      chan struct{}
	subscribers []chan SystemSnapshot
	subMu       sync.RWMutex

	// Servis metrikleri
	services   map[string]*serviceTracker
	serviceMu  sync.RWMutex

	// CPU hesaplama için önceki değerler
	prevCPUIdle  uint64
	prevCPUTotal uint64
}

type serviceTracker struct {
	id           string
	requests     int64
	errors       int64
	latencySum   float64
	latencyCount int64
	mu           sync.Mutex
}

// New — Yeni collector oluştur
func New(interval time.Duration, maxHistory int) *Collector {
	if maxHistory <= 0 {
		maxHistory = 3600 // 1 saatlik 1s aralıkla
	}
	return &Collector{
		interval:   interval,
		snapshots:  make([]SystemSnapshot, 0, maxHistory),
		maxHistory: maxHistory,
		startTime:  time.Now(),
		stopCh:     make(chan struct{}),
		services:   make(map[string]*serviceTracker),
	}
}

// Start — Periyodik metrik toplamayı başlat
func (c *Collector) Start() {
	if !atomic.CompareAndSwapInt32(&c.running, 0, 1) {
		return
	}
	go c.loop()
}

// Stop — Metrik toplamayı durdur
func (c *Collector) Stop() {
	if atomic.CompareAndSwapInt32(&c.running, 1, 0) {
		close(c.stopCh)
	}
}

// IsRunning — Çalışıyor mu
func (c *Collector) IsRunning() bool {
	return atomic.LoadInt32(&c.running) == 1
}

// Subscribe — Snapshot bildirimi al
func (c *Collector) Subscribe() chan SystemSnapshot {
	ch := make(chan SystemSnapshot, 16)
	c.subMu.Lock()
	c.subscribers = append(c.subscribers, ch)
	c.subMu.Unlock()
	return ch
}

// Unsubscribe — Abonelikten çık
func (c *Collector) Unsubscribe(ch chan SystemSnapshot) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	for i, sub := range c.subscribers {
		if sub == ch {
			c.subscribers = append(c.subscribers[:i], c.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// Collect — Tek seferlik metrik topla
func (c *Collector) Collect() SystemSnapshot {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	snap := SystemSnapshot{
		Timestamp:   time.Now(),
		CPUPercent:  c.collectCPU(),
		MemTotal:    ms.Sys,
		MemUsed:     ms.Alloc,
		MemPercent:  float64(ms.Alloc) / float64(ms.Sys) * 100,
		Goroutines:  runtime.NumGoroutine(),
		HeapAlloc:   ms.HeapAlloc,
		HeapSys:     ms.HeapSys,
		GCPauseNs:   ms.PauseNs[(ms.NumGC+255)%256],
		NumGC:       ms.NumGC,
		Uptime:      time.Since(c.startTime).Seconds(),
	}

	// Disk bilgisi
	snap.DiskTotal, snap.DiskUsed, snap.DiskPercent = c.collectDisk()

	// Sistem bellek (Go runtime değil, işletim sistemi)
	sysTotal, sysUsed := c.collectSystemMemory()
	if sysTotal > 0 {
		snap.MemTotal = sysTotal
		snap.MemUsed = sysUsed
		snap.MemPercent = float64(sysUsed) / float64(sysTotal) * 100
	}

	return snap
}

// RecordRequest — Servis isteği kaydet
func (c *Collector) RecordRequest(serviceID string, latencyMs float64, isError bool) {
	c.serviceMu.RLock()
	tracker, ok := c.services[serviceID]
	c.serviceMu.RUnlock()

	if !ok {
		c.serviceMu.Lock()
		tracker, ok = c.services[serviceID]
		if !ok {
			tracker = &serviceTracker{id: serviceID}
			c.services[serviceID] = tracker
		}
		c.serviceMu.Unlock()
	}

	tracker.mu.Lock()
	atomic.AddInt64(&tracker.requests, 1)
	tracker.latencySum += latencyMs
	tracker.latencyCount++
	if isError {
		atomic.AddInt64(&tracker.errors, 1)
	}
	tracker.mu.Unlock()
}

// GetServiceMetrics — Servis metriklerini getir
func (c *Collector) GetServiceMetrics() []ServiceMetric {
	c.serviceMu.RLock()
	defer c.serviceMu.RUnlock()

	result := make([]ServiceMetric, 0, len(c.services))
	now := time.Now()
	for _, t := range c.services {
		reqs := atomic.LoadInt64(&t.requests)
		errs := atomic.LoadInt64(&t.errors)
		t.mu.Lock()
		avgLat := 0.0
		if t.latencyCount > 0 {
			avgLat = t.latencySum / float64(t.latencyCount)
		}
		t.mu.Unlock()

		errRate := 0.0
		if reqs > 0 {
			errRate = float64(errs) / float64(reqs)
		}

		result = append(result, ServiceMetric{
			ServiceID:    t.id,
			Latency:      math.Round(avgLat*100) / 100,
			RequestCount: reqs,
			ErrorCount:   errs,
			ErrorRate:    math.Round(errRate*10000) / 10000,
			Timestamp:    now,
		})
	}
	return result
}

// Latest — Son snapshot
func (c *Collector) Latest() (SystemSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.snapshots) == 0 {
		return SystemSnapshot{}, false
	}
	return c.snapshots[len(c.snapshots)-1], true
}

// History — Snapshot geçmişi (son N)
func (c *Collector) History(n int) []SystemSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if n <= 0 || n > len(c.snapshots) {
		n = len(c.snapshots)
	}
	start := len(c.snapshots) - n
	result := make([]SystemSnapshot, n)
	copy(result, c.snapshots[start:])
	return result
}

// SnapshotCount — Toplam snapshot sayısı
func (c *Collector) SnapshotCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.snapshots)
}

func (c *Collector) loop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			snap := c.Collect()
			c.mu.Lock()
			c.snapshots = append(c.snapshots, snap)
			if len(c.snapshots) > c.maxHistory {
				c.snapshots = c.snapshots[1:]
			}
			c.mu.Unlock()

			// Abonelere bildir
			c.subMu.RLock()
			for _, ch := range c.subscribers {
				select {
				case ch <- snap:
				default: // kanal doluysa atla
				}
			}
			c.subMu.RUnlock()
		}
	}
}

// collectCPU — CPU kullanım yüzdesini hesapla (runtime tabanlı)
func (c *Collector) collectCPU() float64 {
	// GC istatistikleri üzerinden CPU kullanımı hesapla
	numCPU := runtime.NumCPU()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Goroutine/CPU + GC yükü birlikte yaklaşık CPU tahmini
	numGoroutine := runtime.NumGoroutine()
	goroutineLoad := float64(numGoroutine) / float64(numCPU) * 5

	// GC CPU yükü: son GC pause süresi / toplam çalışma süresi
	uptime := time.Since(c.startTime).Seconds()
	gcLoad := 0.0
	if uptime > 0 {
		gcLoad = float64(m.PauseTotalNs) / (uptime * 1e9) * 100
	}

	cpuPercent := goroutineLoad + gcLoad
	if cpuPercent > 100 {
		cpuPercent = 100
	}
	return math.Round(cpuPercent*100) / 100
}

// collectDisk — Disk kullanım bilgisi
func (c *Collector) collectDisk() (total, used uint64, percent float64) {
	// Statfs çağrısı platforma özel, basit fallback
	info, err := os.Stat("/")
	if err != nil {
		return 0, 0, 0
	}
	_ = info
	return getDiskUsage("/")
}

// collectSystemMemory — İşletim sistemi bellek bilgisi
func (c *Collector) collectSystemMemory() (total, used uint64) {
	return getSystemMemory()
}
