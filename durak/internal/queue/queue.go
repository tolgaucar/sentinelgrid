// Package queue — Öncelikli paket kuyruğu.
//
// Askeri haberleşmede farklı aciliyet düzeyindeki veri paketlerini
// öncelik sırasına göre bekletir. Min-heap tabanlı öncelik kuyruğu
// kullanır: düşük priority değeri = yüksek öncelik (0 = en acil).
// Aynı öncelikte FIFO sırası korunur.
package queue

import (
	"container/heap"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Priority paket öncelik seviyesi (düşük = daha acil).
type Priority int

const (
	PriorityFlash    Priority = 0 // Anlık: NBC alarm, geri çekilme
	PriorityCritical Priority = 1 // Kritik: hedef vurma onayı, düşman temas
	PriorityHigh     Priority = 2 // Yüksek: komut, koordinat
	PriorityNormal   Priority = 3 // Normal: telemetri, sensör
	PriorityLow      Priority = 4 // Düşük: güncelleme, log
	PriorityBulk     Priority = 5 // Toplu: görüntü, video akışı
)

// PriorityName öncelik adını döndürür.
func PriorityName(p Priority) string {
	switch p {
	case PriorityFlash:
		return "FLASH"
	case PriorityCritical:
		return "CRITICAL"
	case PriorityHigh:
		return "HIGH"
	case PriorityNormal:
		return "NORMAL"
	case PriorityLow:
		return "LOW"
	case PriorityBulk:
		return "BULK"
	default:
		return "UNKNOWN"
	}
}

// Packet veri paketi.
type Packet struct {
	ID          string    `json:"id"`
	QueueID     string    `json:"queue_id"`
	Priority    Priority  `json:"priority"`
	Source      string    `json:"source"`
	Destination string    `json:"destination"`
	ContentType string    `json:"content_type"`
	Payload     []byte    `json:"payload"`
	Size        int       `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	EnqueuedAt  time.Time `json:"enqueued_at"`
	DequeuedAt  time.Time `json:"dequeued_at,omitempty"`
	TTL         time.Duration `json:"ttl"`

	// Dahili heap sıralama
	seqNo int64 // FIFO sırası koruma
	index int   // heap index
}

// QueueInfo kuyruk bilgisi.
type QueueInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	MaxSize     int       `json:"max_size"`
	CreatedAt   time.Time `json:"created_at"`
}

// PriorityQueue öncelikli kuyruk.
type PriorityQueue struct {
	info    QueueInfo
	mu      sync.Mutex
	items   packetHeap
	seq     int64
	maxSize int
	stats   QueueStats
}

// QueueStats kuyruk istatistikleri.
type QueueStats struct {
	TotalEnqueued  int64 `json:"total_enqueued"`
	TotalDequeued  int64 `json:"total_dequeued"`
	TotalDropped   int64 `json:"total_dropped"`
	TotalExpired   int64 `json:"total_expired"`
	CurrentSize    int64 `json:"current_size"`
	PeakSize       int64 `json:"peak_size"`
	BytesEnqueued  int64 `json:"bytes_enqueued"`
	BytesDequeued  int64 `json:"bytes_dequeued"`
	AvgWaitMs      int64 `json:"avg_wait_ms"`
	totalWaitNs    int64
}

// NewPriorityQueue yeni öncelikli kuyruk oluşturur.
func NewPriorityQueue(info QueueInfo, maxSize int) *PriorityQueue {
	pq := &PriorityQueue{
		info:    info,
		maxSize: maxSize,
	}
	heap.Init(&pq.items)
	return pq
}

// Enqueue paketi kuyruğa ekler. Kuyruk doluysa drop policy uygulanır.
func (pq *PriorityQueue) Enqueue(pkt *Packet) error {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.items.Len() >= pq.maxSize {
		// Tail drop: yeni paketi düşür (düşük öncelikli ise)
		if pq.items.Len() > 0 {
			worst := pq.items[pq.items.Len()-1]
			if pkt.Priority < worst.Priority {
				// Yeni paket daha öncelikli, en düşüğü düşür
				heap.Remove(&pq.items, worst.index)
				atomic.AddInt64(&pq.stats.TotalDropped, 1)
				atomic.AddInt64(&pq.stats.CurrentSize, -1)
			} else {
				atomic.AddInt64(&pq.stats.TotalDropped, 1)
				return fmt.Errorf("kuyruk dolu (%d/%d), paket düşürüldü (priority=%s)",
					pq.items.Len(), pq.maxSize, PriorityName(pkt.Priority))
			}
		}
	}

	pkt.seqNo = atomic.AddInt64(&pq.seq, 1)
	pkt.EnqueuedAt = time.Now()
	pkt.QueueID = pq.info.ID
	heap.Push(&pq.items, pkt)

	size := atomic.AddInt64(&pq.stats.CurrentSize, 1)
	atomic.AddInt64(&pq.stats.TotalEnqueued, 1)
	atomic.AddInt64(&pq.stats.BytesEnqueued, int64(pkt.Size))

	peak := atomic.LoadInt64(&pq.stats.PeakSize)
	if size > peak {
		atomic.StoreInt64(&pq.stats.PeakSize, size)
	}

	return nil
}

// Dequeue en öncelikli paketi çıkarır.
func (pq *PriorityQueue) Dequeue() (*Packet, error) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// Süresi dolmuş paketleri temizle
	pq.cleanExpiredLocked()

	if pq.items.Len() == 0 {
		return nil, fmt.Errorf("kuyruk boş")
	}

	pkt := heap.Pop(&pq.items).(*Packet)
	pkt.DequeuedAt = time.Now()

	atomic.AddInt64(&pq.stats.CurrentSize, -1)
	atomic.AddInt64(&pq.stats.TotalDequeued, 1)
	atomic.AddInt64(&pq.stats.BytesDequeued, int64(pkt.Size))

	waitNs := pkt.DequeuedAt.Sub(pkt.EnqueuedAt).Nanoseconds()
	total := atomic.AddInt64(&pq.stats.totalWaitNs, waitNs)
	count := atomic.LoadInt64(&pq.stats.TotalDequeued)
	if count > 0 {
		atomic.StoreInt64(&pq.stats.AvgWaitMs, total/count/1_000_000)
	}

	return pkt, nil
}

// Peek en öncelikli pakete bakar (çıkarmaz).
func (pq *PriorityQueue) Peek() *Packet {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	if pq.items.Len() == 0 {
		return nil
	}
	return pq.items[0]
}

// Size kuyruk boyutu.
func (pq *PriorityQueue) Size() int {
	return int(atomic.LoadInt64(&pq.stats.CurrentSize))
}

// Info kuyruk bilgisini döndürür.
func (pq *PriorityQueue) Info() QueueInfo {
	return pq.info
}

// Stats istatistikleri döndürür.
func (pq *PriorityQueue) Stats() QueueStats {
	return QueueStats{
		TotalEnqueued: atomic.LoadInt64(&pq.stats.TotalEnqueued),
		TotalDequeued: atomic.LoadInt64(&pq.stats.TotalDequeued),
		TotalDropped:  atomic.LoadInt64(&pq.stats.TotalDropped),
		TotalExpired:  atomic.LoadInt64(&pq.stats.TotalExpired),
		CurrentSize:   atomic.LoadInt64(&pq.stats.CurrentSize),
		PeakSize:      atomic.LoadInt64(&pq.stats.PeakSize),
		BytesEnqueued: atomic.LoadInt64(&pq.stats.BytesEnqueued),
		BytesDequeued: atomic.LoadInt64(&pq.stats.BytesDequeued),
		AvgWaitMs:     atomic.LoadInt64(&pq.stats.AvgWaitMs),
	}
}

// CleanExpired süresi dolmuş paketleri temizler.
func (pq *PriorityQueue) CleanExpired() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return pq.cleanExpiredLocked()
}

func (pq *PriorityQueue) cleanExpiredLocked() int {
	now := time.Now()
	expired := 0
	for pq.items.Len() > 0 {
		pkt := pq.items[0]
		if pkt.TTL > 0 && now.Sub(pkt.CreatedAt) > pkt.TTL {
			heap.Pop(&pq.items)
			atomic.AddInt64(&pq.stats.CurrentSize, -1)
			atomic.AddInt64(&pq.stats.TotalExpired, 1)
			expired++
		} else {
			break
		}
	}
	return expired
}

// ── Min-heap implementasyonu ──
// Düşük priority değeri = yüksek öncelik. Eşit öncelikte FIFO.

type packetHeap []*Packet

func (h packetHeap) Len() int { return len(h) }

func (h packetHeap) Less(i, j int) bool {
	if h[i].Priority != h[j].Priority {
		return h[i].Priority < h[j].Priority
	}
	return h[i].seqNo < h[j].seqNo // FIFO
}

func (h packetHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *packetHeap) Push(x any) {
	n := len(*h)
	pkt := x.(*Packet)
	pkt.index = n
	*h = append(*h, pkt)
}

func (h *packetHeap) Pop() any {
	old := *h
	n := len(old)
	pkt := old[n-1]
	old[n-1] = nil
	pkt.index = -1
	*h = old[:n-1]
	return pkt
}
