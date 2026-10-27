// Package queue - Öncelik kuyruğu (Priority Queue)
//
// Hayati veriler (hedef vurma onayı, saldırı alarmı) düşük öncelikli
// verilerden (video akışı) daha yüksek önceliğe sahiptir.
// DURAK modülü ile entegre çalışarak QoS sağlar.
package queue

import (
	"container/heap"
	"sync"
)

// PriorityQueue öncelik bazlı mesaj kuyruğu
// Düşük priority değeri = yüksek öncelik (1=kritik, 10=düşük)
type PriorityQueue struct {
	items *msgHeap
	mu    sync.Mutex
	cap   int
}

// NewPriorityQueue yeni öncelik kuyruğu
func NewPriorityQueue(capacity int) *PriorityQueue {
	h := &msgHeap{}
	heap.Init(h)
	return &PriorityQueue{
		items: h,
		cap:   capacity,
	}
}

// Push mesajı öncelik sırasına göre ekle
func (pq *PriorityQueue) Push(msg Message) error {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.cap > 0 && pq.items.Len() >= pq.cap {
		return ErrQueueFull
	}

	heap.Push(pq.items, msg)
	return nil
}

// Pop en yüksek öncelikli mesajı al
func (pq *PriorityQueue) Pop() (Message, error) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.items.Len() == 0 {
		return Message{}, ErrQueueEmpty
	}

	return heap.Pop(pq.items).(Message), nil
}

// Peek en yüksek öncelikli mesajı oku (çıkarmadan)
func (pq *PriorityQueue) Peek() (Message, error) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.items.Len() == 0 {
		return Message{}, ErrQueueEmpty
	}

	return (*pq.items)[0], nil
}

// Len kuyruk uzunluğu
func (pq *PriorityQueue) Len() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return pq.items.Len()
}

// ── min-heap implementasyonu ──

type msgHeap []Message

func (h msgHeap) Len() int { return len(h) }

func (h msgHeap) Less(i, j int) bool {
	// Düşük priority = yüksek öncelik
	if h[i].Priority != h[j].Priority {
		return h[i].Priority < h[j].Priority
	}
	// Aynı öncelikte eski mesaj önce
	return h[i].TimestampNs < h[j].TimestampNs
}

func (h msgHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *msgHeap) Push(x any) {
	*h = append(*h, x.(Message))
}

func (h *msgHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}
