// Package broker - Kafka benzeri mesaj broker sistemi
//
// BAĞ modülünün olay güdümlü (event-driven) mesajlaşma çekirdeği.
// Topic, partition ve consumer group kavramlarını destekler.
package broker

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/bag/internal/queue"
)

// Broker merkezi mesaj aracısı
type Broker struct {
	topics    map[string]*Topic
	mu        sync.RWMutex
	config    BrokerOpts
	stats     Stats
	closed    atomic.Bool
}

// BrokerOpts broker yapılandırması
type BrokerOpts struct {
	DefaultPartitions int
	MaxQueueSize      int
	MaxTopics         int
	RetentionDuration time.Duration
}

// Stats broker istatistikleri
type Stats struct {
	MessagesIn    atomic.Int64
	MessagesOut   atomic.Int64
	MessagesFailed atomic.Int64
}

// NewBroker yeni broker oluşturur
func NewBroker(opts BrokerOpts) *Broker {
	if opts.DefaultPartitions <= 0 {
		opts.DefaultPartitions = 4
	}
	if opts.MaxQueueSize <= 0 {
		opts.MaxQueueSize = 1_000_000
	}
	if opts.MaxTopics <= 0 {
		opts.MaxTopics = 1000
	}
	return &Broker{
		topics: make(map[string]*Topic),
		config: opts,
	}
}

// CreateTopic yeni topic oluşturur
func (b *Broker) CreateTopic(name string, partitionCount int) (*Topic, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.topics[name]; exists {
		return nil, fmt.Errorf("topic zaten var: %s", name)
	}

	if len(b.topics) >= b.config.MaxTopics {
		return nil, fmt.Errorf("maksimum topic sayısına ulaşıldı: %d", b.config.MaxTopics)
	}

	if partitionCount <= 0 {
		partitionCount = b.config.DefaultPartitions
	}

	topic := NewTopic(name, partitionCount, b.config.MaxQueueSize/partitionCount)
	b.topics[name] = topic

	log.Printf("[BAĞ-BROKER] Topic oluşturuldu: %s (partitions=%d)", name, partitionCount)
	return topic, nil
}

// DeleteTopic topic siler
func (b *Broker) DeleteTopic(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	topic, exists := b.topics[name]
	if !exists {
		return fmt.Errorf("topic bulunamadı: %s", name)
	}

	topic.Close()
	delete(b.topics, name)
	log.Printf("[BAĞ-BROKER] Topic silindi: %s", name)
	return nil
}

// GetTopic topic getir
func (b *Broker) GetTopic(name string) (*Topic, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	t, ok := b.topics[name]
	return t, ok
}

// ListTopics tüm topic'leri listele
func (b *Broker) ListTopics() []*Topic {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]*Topic, 0, len(b.topics))
	for _, t := range b.topics {
		result = append(result, t)
	}
	return result
}

// Publish mesajı ilgili topic'e yayınla
func (b *Broker) Publish(topicName string, msg queue.Message) error {
	b.mu.RLock()
	topic, exists := b.topics[topicName]
	b.mu.RUnlock()

	if !exists {
		b.stats.MessagesFailed.Add(1)
		return fmt.Errorf("topic bulunamadı: %s", topicName)
	}

	if err := topic.Publish(msg); err != nil {
		b.stats.MessagesFailed.Add(1)
		return err
	}

	b.stats.MessagesIn.Add(1)
	return nil
}

// Consume belirtilen topic ve consumer group'tan mesaj tüket
func (b *Broker) Consume(topicName, groupID, consumerID string) (queue.Message, error) {
	b.mu.RLock()
	topic, exists := b.topics[topicName]
	b.mu.RUnlock()

	if !exists {
		return queue.Message{}, fmt.Errorf("topic bulunamadı: %s", topicName)
	}

	msg, err := topic.Consume(groupID, consumerID)
	if err != nil {
		return msg, err
	}

	b.stats.MessagesOut.Add(1)
	return msg, nil
}

// GetStats istatistikleri döndür
func (b *Broker) GetStats() (in, out, failed int64, topicCount int) {
	b.mu.RLock()
	tc := len(b.topics)
	b.mu.RUnlock()
	return b.stats.MessagesIn.Load(), b.stats.MessagesOut.Load(), b.stats.MessagesFailed.Load(), tc
}

// TopicDepths tüm topic derinliklerini döndür
func (b *Broker) TopicDepths() map[string]int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	depths := make(map[string]int64, len(b.topics))
	for name, topic := range b.topics {
		depths[name] = topic.Depth()
	}
	return depths
}

// Close broker'ı kapat
func (b *Broker) Close() {
	if b.closed.Swap(true) {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, topic := range b.topics {
		topic.Close()
	}
	log.Printf("[BAĞ-BROKER] Broker kapatıldı")
}
