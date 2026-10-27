package queue

import (
	"testing"
)

func TestPriorityQueue_Order(t *testing.T) {
	pq := NewPriorityQueue(10)

	// Düşük öncelik önce, yüksek öncelik sonra ekle
	pq.Push(Message{ID: "low", Priority: 10, TimestampNs: 1})
	pq.Push(Message{ID: "critical", Priority: 1, TimestampNs: 2})
	pq.Push(Message{ID: "medium", Priority: 5, TimestampNs: 3})

	// Yüksek öncelikli (düşük sayı) ilk çıkmalı
	msg, _ := pq.Pop()
	if msg.ID != "critical" {
		t.Fatalf("critical beklendi, alındı: %s", msg.ID)
	}

	msg, _ = pq.Pop()
	if msg.ID != "medium" {
		t.Fatalf("medium beklendi, alındı: %s", msg.ID)
	}

	msg, _ = pq.Pop()
	if msg.ID != "low" {
		t.Fatalf("low beklendi, alındı: %s", msg.ID)
	}
}

func TestPriorityQueue_SamePriorityFIFO(t *testing.T) {
	pq := NewPriorityQueue(10)

	pq.Push(Message{ID: "first", Priority: 5, TimestampNs: 100})
	pq.Push(Message{ID: "second", Priority: 5, TimestampNs: 200})
	pq.Push(Message{ID: "third", Priority: 5, TimestampNs: 300})

	msg, _ := pq.Pop()
	if msg.ID != "first" {
		t.Fatalf("first beklendi, alındı: %s", msg.ID)
	}
}

func TestPriorityQueue_Capacity(t *testing.T) {
	pq := NewPriorityQueue(2)

	pq.Push(Message{ID: "1"})
	pq.Push(Message{ID: "2"})

	err := pq.Push(Message{ID: "3"})
	if err != ErrQueueFull {
		t.Fatalf("ErrQueueFull beklendi, alındı: %v", err)
	}
}

func TestPriorityQueue_Empty(t *testing.T) {
	pq := NewPriorityQueue(10)

	_, err := pq.Pop()
	if err != ErrQueueEmpty {
		t.Fatalf("ErrQueueEmpty beklendi, alındı: %v", err)
	}
}

func TestPriorityQueue_Peek(t *testing.T) {
	pq := NewPriorityQueue(10)

	pq.Push(Message{ID: "peek", Priority: 1})

	msg, err := pq.Peek()
	if err != nil {
		t.Fatalf("peek hatası: %v", err)
	}
	if msg.ID != "peek" {
		t.Fatalf("peek beklendi: %s", msg.ID)
	}

	// Peek sonrası hala kuyrukta olmalı
	if pq.Len() != 1 {
		t.Fatalf("peek sonrası uzunluk 1 olmalı: %d", pq.Len())
	}
}
