package scheduler

import (
	"testing"
	"time"

	"github.com/otukenmatris/durak/internal/queue"
)

func newTestScheduler(bwBps int64) (*Scheduler, *queue.PriorityQueue) {
	cfg := SchedulerConfig{
		BandwidthBps: bwBps,
		TickInterval: 100 * time.Millisecond,
		BurstFactor:  1.0,
	}
	s := New(cfg)
	pq := queue.NewPriorityQueue(queue.QueueInfo{ID: "test", Name: "test"}, 1000)
	s.AddQueue(pq)
	return s, pq
}

func TestScheduleOnce(t *testing.T) {
	s, pq := newTestScheduler(1_000_000) // 1 Mbps

	pq.Enqueue(&queue.Packet{ID: "p1", Priority: queue.PriorityNormal, Size: 100, CreatedAt: time.Now()})
	pq.Enqueue(&queue.Packet{ID: "p2", Priority: queue.PriorityFlash, Size: 50, CreatedAt: time.Now()})

	count := s.ScheduleOnce()
	if count != 2 {
		t.Errorf("Beklenen 2 paket zamanlandı, alınan: %d", count)
	}

	stats := s.Stats()
	if stats.TotalScheduled != 2 {
		t.Errorf("TotalScheduled beklenen: 2, alınan: %d", stats.TotalScheduled)
	}
}

func TestPriorityScheduling(t *testing.T) {
	s, pq := newTestScheduler(1_000_000)

	pq.Enqueue(&queue.Packet{ID: "bulk", Priority: queue.PriorityBulk, Size: 50, CreatedAt: time.Now()})
	pq.Enqueue(&queue.Packet{ID: "flash", Priority: queue.PriorityFlash, Size: 50, CreatedAt: time.Now()})

	s.ScheduleOnce()

	// İlk çıkan flash olmalı
	pkt := <-s.Output()
	if pkt.ID != "flash" {
		t.Errorf("İlk zamanlanması gereken flash, alınan: %s", pkt.ID)
	}
}

func TestBandwidthLimit(t *testing.T) {
	// 800 bps = 100 byte/s, 100ms tick = 10 byte/tick
	s, pq := newTestScheduler(800)

	for i := 0; i < 10; i++ {
		pq.Enqueue(&queue.Packet{ID: "big", Priority: queue.PriorityFlash, Size: 20, CreatedAt: time.Now()})
	}

	count := s.ScheduleOnce()
	// 10 byte bütçe, her paket 20 byte — en az 1 flash geçmeli (min 64B bütçe)
	if count == 0 {
		t.Error("En az 1 paket zamanlanmalı")
	}
}

func TestStartStop(t *testing.T) {
	s, _ := newTestScheduler(1_000_000)

	err := s.Start()
	if err != nil {
		t.Fatalf("Start hatası: %v", err)
	}
	if !s.IsRunning() {
		t.Error("Çalışmıyor olarak raporlandı")
	}

	// Tekrar start → hata
	err = s.Start()
	if err == nil {
		t.Error("Çift start'ta hata bekleniyor")
	}

	s.Stop()
	time.Sleep(50 * time.Millisecond)
	if s.IsRunning() {
		t.Error("Stop'tan sonra hala çalışıyor")
	}
}

func TestMultipleQueues(t *testing.T) {
	s, pq1 := newTestScheduler(10_000_000)
	pq2 := queue.NewPriorityQueue(queue.QueueInfo{ID: "q2", Name: "q2"}, 1000)
	s.AddQueue(pq2)

	pq1.Enqueue(&queue.Packet{ID: "q1-p1", Priority: queue.PriorityNormal, Size: 50, CreatedAt: time.Now()})
	pq2.Enqueue(&queue.Packet{ID: "q2-p1", Priority: queue.PriorityHigh, Size: 50, CreatedAt: time.Now()})

	count := s.ScheduleOnce()
	if count != 2 {
		t.Errorf("Beklenen 2, alınan: %d", count)
	}
}

func TestRemoveQueue(t *testing.T) {
	s, _ := newTestScheduler(1_000_000)
	s.RemoveQueue("test")
	queues := s.Queues()
	if len(queues) != 0 {
		t.Errorf("Kuyruk silindikten sonra hala %d kuyruk var", len(queues))
	}
}

func TestSetBandwidth(t *testing.T) {
	s, _ := newTestScheduler(1_000_000)
	s.SetBandwidth(2_000_000)

	// Verify through a tick (internal field)
	s.mu.RLock()
	bw := s.config.BandwidthBps
	s.mu.RUnlock()
	if bw != 2_000_000 {
		t.Errorf("BandwidthBps beklenen: 2000000, alınan: %d", bw)
	}
}

func TestOutputChannel(t *testing.T) {
	s, pq := newTestScheduler(10_000_000)
	pq.Enqueue(&queue.Packet{ID: "out1", Priority: queue.PriorityFlash, Size: 10, CreatedAt: time.Now()})

	s.ScheduleOnce()

	select {
	case pkt := <-s.Output():
		if pkt.ID != "out1" {
			t.Errorf("Output'ta beklenen: out1, alınan: %s", pkt.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Output'ta paket bekleniyor")
	}
}

func TestExpiredNotScheduled(t *testing.T) {
	s, pq := newTestScheduler(10_000_000)

	pq.Enqueue(&queue.Packet{ID: "exp", Priority: queue.PriorityFlash, Size: 10, CreatedAt: time.Now(), TTL: 1 * time.Millisecond})
	time.Sleep(5 * time.Millisecond)

	count := s.ScheduleOnce()
	if count != 0 {
		t.Errorf("Süresi dolmuş paket zamanlanmamalı, alınan: %d", count)
	}
}

func TestGetQueue(t *testing.T) {
	s, _ := newTestScheduler(1_000_000)
	pq := s.GetQueue("test")
	if pq == nil {
		t.Fatal("test kuyruğu bulunamadı")
	}
	if pq.Info().ID != "test" {
		t.Errorf("Kuyruk ID beklenen: test, alınan: %s", pq.Info().ID)
	}

	missing := s.GetQueue("nonexistent")
	if missing != nil {
		t.Error("Olmayan kuyruk nil olmalı")
	}
}

func TestPriorityWeights(t *testing.T) {
	total := 0
	for _, w := range PriorityWeight {
		total += w
	}
	if total != 100 {
		t.Errorf("Ağırlık toplamı 100 olmalı, alınan: %d", total)
	}
}
