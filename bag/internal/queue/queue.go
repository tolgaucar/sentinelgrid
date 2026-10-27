// Package queue - Yüksek performanslı mesaj kuyrukları
//
// BAĞ modülünün çekirdeğindeki lock-free ring buffer tabanlı kuyruk.
// Sensör verilerinin mikrosaniye hassasiyetinde iletilmesini sağlar.
package queue

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrQueueFull  = errors.New("kuyruk dolu")
	ErrQueueEmpty = errors.New("kuyruk boş")
	ErrClosed     = errors.New("kuyruk kapalı")
)

// Message kuyrukta taşınan mesaj birimi
type Message struct {
	ID            string
	Topic         string
	Payload       []byte
	Priority      int32
	TimestampNs   int64
	RetryCount    int32
	MaxRetries    int32
	CorrelationID string
	Headers       map[string]string
}

// RingBuffer lock-free ring buffer tabanlı FIFO kuyruk
type RingBuffer struct {
	buf      []Message
	capacity int64
	head     atomic.Int64
	tail     atomic.Int64
	count    atomic.Int64
	closed   atomic.Bool
	mu       sync.Mutex // sadece resize için
}

// NewRingBuffer yeni ring buffer oluşturur
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 65536
	}
	return &RingBuffer{
		buf:      make([]Message, capacity),
		capacity: int64(capacity),
	}
}

// Enqueue mesajı kuyruğun sonuna ekler
func (rb *RingBuffer) Enqueue(msg Message) error {
	if rb.closed.Load() {
		return ErrClosed
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count.Load() >= rb.capacity {
		return ErrQueueFull
	}

	tail := rb.tail.Load()
	rb.buf[tail%rb.capacity] = msg
	rb.tail.Store(tail + 1)
	rb.count.Add(1)
	return nil
}

// Dequeue kuyruğun başından mesaj alır
func (rb *RingBuffer) Dequeue() (Message, error) {
	if rb.closed.Load() && rb.count.Load() == 0 {
		return Message{}, ErrClosed
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count.Load() == 0 {
		return Message{}, ErrQueueEmpty
	}

	head := rb.head.Load()
	msg := rb.buf[head%rb.capacity]
	rb.head.Store(head + 1)
	rb.count.Add(-1)
	return msg, nil
}

// Len kuyruk uzunluğu
func (rb *RingBuffer) Len() int64 {
	return rb.count.Load()
}

// Cap kuyruk kapasitesi
func (rb *RingBuffer) Cap() int64 {
	return rb.capacity
}

// IsFull kuyruk dolu mu
func (rb *RingBuffer) IsFull() bool {
	return rb.count.Load() >= rb.capacity
}

// IsEmpty kuyruk boş mu
func (rb *RingBuffer) IsEmpty() bool {
	return rb.count.Load() == 0
}

// Close kuyruğu kapat
func (rb *RingBuffer) Close() {
	rb.closed.Store(true)
}

// IsClosed kuyruk kapalı mı
func (rb *RingBuffer) IsClosed() bool {
	return rb.closed.Load()
}

// Drain kuyruktan tüm mesajları çek
func (rb *RingBuffer) Drain() []Message {
	var msgs []Message
	for {
		msg, err := rb.Dequeue()
		if err != nil {
			break
		}
		msgs = append(msgs, msg)
	}
	return msgs
}

// NowNs nanosaniye hassasiyetinde zaman
func NowNs() int64 {
	return time.Now().UnixNano()
}
