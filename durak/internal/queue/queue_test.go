package queue

import (
	"fmt"
	"testing"
	"time"
)

func newTestQueue(maxSize int) *PriorityQueue {
	return NewPriorityQueue(QueueInfo{
		ID: "test-q", Name: "Test Queue", MaxSize: maxSize,
	}, maxSize)
}

func TestEnqueueDequeue(t *testing.T) {
	q := newTestQueue(100)

	err := q.Enqueue(&Packet{ID: "p1", Priority: PriorityNormal, Size: 100, CreatedAt: time.Now()})
	if err != nil {
		t.Fatalf("Enqueue hatası: %v", err)
	}
	if q.Size() != 1 {
		t.Errorf("Size beklenen: 1, alınan: %d", q.Size())
	}

	pkt, err := q.Dequeue()
	if err != nil {
		t.Fatalf("Dequeue hatası: %v", err)
	}
	if pkt.ID != "p1" {
		t.Errorf("ID beklenen: p1, alınan: %s", pkt.ID)
	}
	if q.Size() != 0 {
		t.Errorf("Dequeue sonrası size beklenen: 0, alınan: %d", q.Size())
	}
}

func TestPriorityOrder(t *testing.T) {
	q := newTestQueue(100)

	// Düşük öncelik, orta, yüksek sırayla ekle
	q.Enqueue(&Packet{ID: "low", Priority: PriorityLow, Size: 10, CreatedAt: time.Now()})
	q.Enqueue(&Packet{ID: "flash", Priority: PriorityFlash, Size: 10, CreatedAt: time.Now()})
	q.Enqueue(&Packet{ID: "normal", Priority: PriorityNormal, Size: 10, CreatedAt: time.Now()})
	q.Enqueue(&Packet{ID: "critical", Priority: PriorityCritical, Size: 10, CreatedAt: time.Now()})

	// Çıkış sırası: flash → critical → normal → low
	expected := []string{"flash", "critical", "normal", "low"}
	for i, exp := range expected {
		pkt, err := q.Dequeue()
		if err != nil {
			t.Fatalf("Dequeue %d hatası: %v", i, err)
		}
		if pkt.ID != exp {
			t.Errorf("Sıra %d: beklenen %s, alınan %s", i, exp, pkt.ID)
		}
	}
}

func TestFIFOSamePriority(t *testing.T) {
	q := newTestQueue(100)

	for i := 0; i < 5; i++ {
		q.Enqueue(&Packet{ID: fmt.Sprintf("p%d", i), Priority: PriorityNormal, Size: 10, CreatedAt: time.Now()})
	}

	// Aynı öncelikte FIFO sırası korunmalı
	for i := 0; i < 5; i++ {
		pkt, _ := q.Dequeue()
		expected := fmt.Sprintf("p%d", i)
		if pkt.ID != expected {
			t.Errorf("FIFO sıra %d: beklenen %s, alınan %s", i, expected, pkt.ID)
		}
	}
}

func TestQueueFull(t *testing.T) {
	q := newTestQueue(3)

	q.Enqueue(&Packet{ID: "p1", Priority: PriorityNormal, Size: 10, CreatedAt: time.Now()})
	q.Enqueue(&Packet{ID: "p2", Priority: PriorityNormal, Size: 10, CreatedAt: time.Now()})
	q.Enqueue(&Packet{ID: "p3", Priority: PriorityNormal, Size: 10, CreatedAt: time.Now()})

	// Aynı veya düşük öncelik → düşürülür
	err := q.Enqueue(&Packet{ID: "p4", Priority: PriorityLow, Size: 10, CreatedAt: time.Now()})
	if err == nil {
		t.Fatal("Dolu kuyruk düşük öncelikli paket için hata bekleniyor")
	}

	s := q.Stats()
	if s.TotalDropped < 1 {
		t.Errorf("TotalDropped beklenen: >=1, alınan: %d", s.TotalDropped)
	}
}

func TestPriorityPreemption(t *testing.T) {
	q := newTestQueue(2)

	q.Enqueue(&Packet{ID: "low1", Priority: PriorityLow, Size: 10, CreatedAt: time.Now()})
	q.Enqueue(&Packet{ID: "low2", Priority: PriorityBulk, Size: 10, CreatedAt: time.Now()})

	// Flash paket: düşük önceliklilerin yerine geçmeli
	err := q.Enqueue(&Packet{ID: "flash", Priority: PriorityFlash, Size: 10, CreatedAt: time.Now()})
	if err != nil {
		t.Fatalf("Flash paket eklenemedi: %v", err)
	}

	// Flash ilk çıkmalı
	pkt, _ := q.Dequeue()
	if pkt.ID != "flash" {
		t.Errorf("İlk çıkan beklenen: flash, alınan: %s", pkt.ID)
	}
}

func TestDequeueEmpty(t *testing.T) {
	q := newTestQueue(100)
	_, err := q.Dequeue()
	if err == nil {
		t.Fatal("Boş kuyruktan dequeue için hata bekleniyor")
	}
}

func TestPeek(t *testing.T) {
	q := newTestQueue(100)
	if q.Peek() != nil {
		t.Error("Boş kuyrukta Peek nil olmalı")
	}

	q.Enqueue(&Packet{ID: "p1", Priority: PriorityHigh, Size: 10, CreatedAt: time.Now()})
	pkt := q.Peek()
	if pkt == nil || pkt.ID != "p1" {
		t.Error("Peek doğru paketi döndürmedi")
	}
	if q.Size() != 1 {
		t.Error("Peek boyutu değiştirmemeli")
	}
}

func TestExpiredPackets(t *testing.T) {
	q := newTestQueue(100)

	q.Enqueue(&Packet{ID: "exp", Priority: PriorityFlash, Size: 10, CreatedAt: time.Now(), TTL: 1 * time.Millisecond})
	q.Enqueue(&Packet{ID: "valid", Priority: PriorityNormal, Size: 10, CreatedAt: time.Now(), TTL: 1 * time.Hour})

	time.Sleep(5 * time.Millisecond)

	pkt, err := q.Dequeue()
	if err != nil {
		t.Fatalf("Dequeue hatası: %v", err)
	}
	// Süresi dolmuş flash temizlendikten sonra valid kalmali
	if pkt.ID != "valid" {
		t.Errorf("Beklenen: valid, alınan: %s", pkt.ID)
	}
}

func TestStats(t *testing.T) {
	q := newTestQueue(100)
	q.Enqueue(&Packet{ID: "p1", Priority: PriorityNormal, Size: 256, CreatedAt: time.Now()})
	q.Enqueue(&Packet{ID: "p2", Priority: PriorityHigh, Size: 512, CreatedAt: time.Now()})
	q.Dequeue()

	s := q.Stats()
	if s.TotalEnqueued != 2 {
		t.Errorf("TotalEnqueued beklenen: 2, alınan: %d", s.TotalEnqueued)
	}
	if s.TotalDequeued != 1 {
		t.Errorf("TotalDequeued beklenen: 1, alınan: %d", s.TotalDequeued)
	}
	if s.BytesEnqueued != 768 {
		t.Errorf("BytesEnqueued beklenen: 768, alınan: %d", s.BytesEnqueued)
	}
	if s.CurrentSize != 1 {
		t.Errorf("CurrentSize beklenen: 1, alınan: %d", s.CurrentSize)
	}
	if s.PeakSize != 2 {
		t.Errorf("PeakSize beklenen: 2, alınan: %d", s.PeakSize)
	}
}

func TestPriorityName(t *testing.T) {
	if PriorityName(PriorityFlash) != "FLASH" {
		t.Error("FLASH adı yanlış")
	}
	if PriorityName(PriorityCritical) != "CRITICAL" {
		t.Error("CRITICAL adı yanlış")
	}
	if PriorityName(Priority(99)) != "UNKNOWN" {
		t.Error("UNKNOWN bekleniyor")
	}
}
