// Package scheduler — Bant genişliği farkında paket zamanlayıcı.
//
// Askeri haberleşmede sınırlı bant genişliğini öncelik bazlı paylaştırır.
// Weighted Fair Queuing (WFQ) ile yüksek öncelikli paketlere daha fazla
// bant genişliği ayrılır, düşük öncelikli paketler açlığa uğramaz.
package scheduler

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/durak/internal/queue"
)

// SchedulerConfig zamanlayıcı yapılandırması.
type SchedulerConfig struct {
	BandwidthBps  int64         // Bant genişliği (bit/saniye)
	TickInterval  time.Duration // Zamanlama periyodu
	BurstFactor   float64       // Patlama katsayısı (1.0 = burst yok)
}

// PriorityWeight öncelik ağırlıkları (toplam 100).
var PriorityWeight = map[queue.Priority]int{
	queue.PriorityFlash:    35,
	queue.PriorityCritical: 25,
	queue.PriorityHigh:     20,
	queue.PriorityNormal:   12,
	queue.PriorityLow:      5,
	queue.PriorityBulk:     3,
}

// SchedulerStats zamanlayıcı istatistikleri.
type SchedulerStats struct {
	TotalScheduled  int64 `json:"total_scheduled"`
	TotalBytes      int64 `json:"total_bytes"`
	BandwidthUsed   int64 `json:"bandwidth_used_bps"`
	Ticks           int64 `json:"ticks"`
	DroppedOverflow int64 `json:"dropped_overflow"`
}

// Scheduler bant genişliği farkında zamanlayıcı.
type Scheduler struct {
	mu       sync.RWMutex
	config   SchedulerConfig
	queues   map[string]*queue.PriorityQueue
	stats    SchedulerStats
	running  int32
	stopCh   chan struct{}
	outCh    chan *queue.Packet
}

// New yeni zamanlayıcı oluşturur.
func New(cfg SchedulerConfig) *Scheduler {
	if cfg.BurstFactor < 1.0 {
		cfg.BurstFactor = 1.0
	}
	if cfg.TickInterval == 0 {
		cfg.TickInterval = 10 * time.Millisecond
	}
	return &Scheduler{
		config: cfg,
		queues: make(map[string]*queue.PriorityQueue),
		stopCh: make(chan struct{}),
		outCh:  make(chan *queue.Packet, 1000),
	}
}

// AddQueue zamanlayıcıya kuyruk ekler.
func (s *Scheduler) AddQueue(pq *queue.PriorityQueue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queues[pq.Info().ID] = pq
}

// RemoveQueue kuyruk siler.
func (s *Scheduler) RemoveQueue(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.queues, id)
}

// GetQueue kuyruk döndürür.
func (s *Scheduler) GetQueue(id string) *queue.PriorityQueue {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queues[id]
}

// Queues tüm kuyrukları döndürür.
func (s *Scheduler) Queues() map[string]*queue.PriorityQueue {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]*queue.PriorityQueue, len(s.queues))
	for k, v := range s.queues {
		cp[k] = v
	}
	return cp
}

// Output zamanlanmış paketlerin çıkış kanalı.
func (s *Scheduler) Output() <-chan *queue.Packet {
	return s.outCh
}

// Start zamanlayıcıyı başlatır.
func (s *Scheduler) Start() error {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return fmt.Errorf("zamanlayıcı zaten çalışıyor")
	}
	go s.loop()
	return nil
}

// Stop zamanlayıcıyı durdurur.
func (s *Scheduler) Stop() {
	if atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		close(s.stopCh)
	}
}

// IsRunning çalışma durumu.
func (s *Scheduler) IsRunning() bool {
	return atomic.LoadInt32(&s.running) == 1
}

// Stats istatistikleri döndürür.
func (s *Scheduler) Stats() SchedulerStats {
	return SchedulerStats{
		TotalScheduled:  atomic.LoadInt64(&s.stats.TotalScheduled),
		TotalBytes:      atomic.LoadInt64(&s.stats.TotalBytes),
		BandwidthUsed:   atomic.LoadInt64(&s.stats.BandwidthUsed),
		Ticks:           atomic.LoadInt64(&s.stats.Ticks),
		DroppedOverflow: atomic.LoadInt64(&s.stats.DroppedOverflow),
	}
}

// SetBandwidth bant genişliğini günceller (canlı).
func (s *Scheduler) SetBandwidth(bps int64) {
	s.mu.Lock()
	s.config.BandwidthBps = bps
	s.mu.Unlock()
}

// ScheduleOnce tek bir tick döngüsü (test için export).
func (s *Scheduler) ScheduleOnce() int {
	return s.tick()
}

func (s *Scheduler) loop() {
	ticker := time.NewTicker(s.config.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *Scheduler) tick() int {
	atomic.AddInt64(&s.stats.Ticks, 1)

	s.mu.RLock()
	bwBps := s.config.BandwidthBps
	burstFactor := s.config.BurstFactor
	tickInterval := s.config.TickInterval
	s.mu.RUnlock()

	// Bu tick'te gönderilebilecek max byte
	byteBudget := int64(float64(bwBps) / 8.0 * tickInterval.Seconds() * burstFactor)
	if byteBudget <= 0 {
		return 0
	}

	scheduled := 0
	bytesUsed := int64(0)

	// Önce tüm kuyruklarda expire temizliği
	s.mu.RLock()
	for _, pq := range s.queues {
		pq.CleanExpired()
	}
	s.mu.RUnlock()

	// WFQ: ağırlık oranında her öncelikten paketle
	for pri := queue.PriorityFlash; pri <= queue.PriorityBulk; pri++ {
		weight := PriorityWeight[pri]
		priBudget := byteBudget * int64(weight) / 100
		if priBudget < 64 {
			priBudget = 64 // minimum 64 byte
		}

		priUsed := int64(0)
		s.mu.RLock()
		queues := make([]*queue.PriorityQueue, 0, len(s.queues))
		for _, pq := range s.queues {
			queues = append(queues, pq)
		}
		s.mu.RUnlock()

		for _, pq := range queues {
			for priUsed < priBudget && bytesUsed < byteBudget {
				pkt := pq.Peek()
				if pkt == nil || pkt.Priority != pri {
					break
				}
				pkt, err := pq.Dequeue()
				if err != nil {
					break
				}

				select {
				case s.outCh <- pkt:
					scheduled++
					bytesUsed += int64(pkt.Size)
					priUsed += int64(pkt.Size)
					atomic.AddInt64(&s.stats.TotalScheduled, 1)
					atomic.AddInt64(&s.stats.TotalBytes, int64(pkt.Size))
				default:
					atomic.AddInt64(&s.stats.DroppedOverflow, 1)
				}
			}
		}
	}

	if bytesUsed > 0 {
		bitsPerSec := bytesUsed * 8 * int64(time.Second/tickInterval)
		atomic.StoreInt64(&s.stats.BandwidthUsed, bitsPerSec)
	}

	return scheduled
}
