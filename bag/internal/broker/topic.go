// Package broker - Topic ve Partition yönetimi
package broker

import (
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"

	"github.com/otukenmatris/bag/internal/queue"
)

// Topic bir mesaj konusu (Kafka topic benzeri)
type Topic struct {
	Name           string
	partitions     []*Partition
	partitionCount int
	consumerGroups map[string]*ConsumerGroup
	mu             sync.RWMutex
	closed         atomic.Bool
	msgCount       atomic.Int64
}

// NewTopic yeni topic oluşturur
func NewTopic(name string, partitionCount, partitionSize int) *Topic {
	partitions := make([]*Partition, partitionCount)
	for i := 0; i < partitionCount; i++ {
		partitions[i] = NewPartition(i, partitionSize)
	}
	return &Topic{
		Name:           name,
		partitions:     partitions,
		partitionCount: partitionCount,
		consumerGroups: make(map[string]*ConsumerGroup),
	}
}

// Publish mesajı uygun partition'a yaz
func (t *Topic) Publish(msg queue.Message) error {
	if t.closed.Load() {
		return fmt.Errorf("topic kapalı: %s", t.Name)
	}

	partIdx := t.selectPartition(msg)
	if err := t.partitions[partIdx].Write(msg); err != nil {
		return fmt.Errorf("partition yazma hatası [%s/%d]: %w", t.Name, partIdx, err)
	}

	t.msgCount.Add(1)
	return nil
}

// Consume consumer group üzerinden mesaj tüket
func (t *Topic) Consume(groupID, consumerID string) (queue.Message, error) {
	t.mu.Lock()
	group, exists := t.consumerGroups[groupID]
	if !exists {
		group = NewConsumerGroup(groupID, t.partitionCount)
		t.consumerGroups[groupID] = group
	}
	t.mu.Unlock()

	return group.Consume(consumerID, t.partitions)
}

// Depth tüm partition'ların toplam derinliği
func (t *Topic) Depth() int64 {
	var total int64
	for _, p := range t.partitions {
		total += p.queue.Len()
	}
	return total
}

// PartitionCount partition sayısı
func (t *Topic) PartitionCount() int {
	return t.partitionCount
}

// Close topic'i kapat
func (t *Topic) Close() {
	if t.closed.Swap(true) {
		return
	}
	for _, p := range t.partitions {
		p.Close()
	}
}

// selectPartition mesaj ID'sine göre partition seç (consistent hashing)
func (t *Topic) selectPartition(msg queue.Message) int {
	key := msg.ID
	if key == "" {
		key = msg.Topic
	}
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32()) % t.partitionCount
}

// ────────────────────────────────────────
// Partition
// ────────────────────────────────────────

// Partition bir topic'in veri bölümü
type Partition struct {
	ID    int
	queue *queue.RingBuffer
}

// NewPartition yeni partition
func NewPartition(id, capacity int) *Partition {
	return &Partition{
		ID:    id,
		queue: queue.NewRingBuffer(capacity),
	}
}

// Write mesajı partition'a yaz
func (p *Partition) Write(msg queue.Message) error {
	return p.queue.Enqueue(msg)
}

// Read partition'dan mesaj oku
func (p *Partition) Read() (queue.Message, error) {
	return p.queue.Dequeue()
}

// Depth partition derinliği
func (p *Partition) Depth() int64 {
	return p.queue.Len()
}

// Close partition'ı kapat
func (p *Partition) Close() {
	p.queue.Close()
}

// ────────────────────────────────────────
// Consumer Group
// ────────────────────────────────────────

// ConsumerGroup Kafka benzeri consumer group
type ConsumerGroup struct {
	ID         string
	partitions int
	offsets    []atomic.Int64
	mu         sync.Mutex
	rrIndex    atomic.Int64 // round-robin partition seçimi
}

// NewConsumerGroup yeni consumer group
func NewConsumerGroup(id string, partitions int) *ConsumerGroup {
	cg := &ConsumerGroup{
		ID:         id,
		partitions: partitions,
		offsets:    make([]atomic.Int64, partitions),
	}
	return cg
}

// Consume round-robin ile partition'lardan mesaj tüket
func (cg *ConsumerGroup) Consume(consumerID string, partitions []*Partition) (queue.Message, error) {
	// Tüm partition'ları dene (round-robin başlangıçlı)
	start := int(cg.rrIndex.Add(1)) % cg.partitions
	for i := 0; i < cg.partitions; i++ {
		idx := (start + i) % cg.partitions
		msg, err := partitions[idx].Read()
		if err == nil {
			cg.offsets[idx].Add(1)
			return msg, nil
		}
	}
	return queue.Message{}, queue.ErrQueueEmpty
}
