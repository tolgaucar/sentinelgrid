// Package grpcserver - BAĞ gRPC sunucu implementasyonu
//
// Protobuf tanımlı servisleri (BagTransfer, BagTopicManager, BagStatus)
// implement eden gRPC sunucu. Sensör akışı, mesaj iletimi ve topic
// yönetimi sağlar.
package grpcserver

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/bag/internal/broker"
	"github.com/otukenmatris/bag/internal/queue"
	"github.com/otukenmatris/bag/internal/transport"
)

// Server BAĞ gRPC sunucu - protobuf olmadan saf Go ile
type Server struct {
	broker     *broker.Broker
	sender     *transport.Sender
	receiver   *transport.Receiver
	dispatcher *transport.Dispatcher
	startTime  time.Time
	msgIn      atomic.Int64
	msgOut     atomic.Int64
}

// NewServer yeni gRPC sunucu
func NewServer(brk *broker.Broker, sender *transport.Sender, receiver *transport.Receiver) *Server {
	dispatcher := transport.NewDispatcher(sender, receiver, 100, 50*time.Millisecond)
	return &Server{
		broker:     brk,
		sender:     sender,
		receiver:   receiver,
		dispatcher: dispatcher,
		startTime:  time.Now(),
	}
}

// Envelope modüller arası mesaj zarfı (proto yerine Go struct)
type Envelope struct {
	ID            string
	Topic         string
	Source        string
	Destination   string
	Payload       []byte
	TimestampNs   int64
	Priority      int32
	RetryCount    int32
	MaxRetries    int32
	CorrelationID string
	Headers       map[string]string
}

// Ack alındı onayı
type Ack struct {
	MessageID    string
	Success      bool
	Error        string
	ProcessedAt  int64
}

// SensorData sensör verisi
type SensorData struct {
	SensorID    string
	SensorType  string
	SourceModule string
	Payload     []byte
	TimestampNs int64
	Priority    int32
	Metadata    map[string]string
}

// TopicInfo topic bilgisi
type TopicInfo struct {
	Name            string
	PartitionCount  int
}

// StatusInfo durum bilgisi
type StatusInfo struct {
	Module          string
	Version         string
	UptimeMs        int64
	MessagesIn      int64
	MessagesOut     int64
	MessagesFailed  int64
	ActiveTopics    int
	TopicDepths     map[string]int64
}

// ── Transfer Servisi ──

// Send tekil mesaj gönder
func (s *Server) Send(ctx context.Context, env Envelope) Ack {
	s.msgIn.Add(1)

	msg := envelopeToMessage(env)

	err := s.broker.Publish(env.Topic, msg)
	if err != nil {
		return Ack{
			MessageID: env.ID,
			Success:   false,
			Error:     err.Error(),
			ProcessedAt: time.Now().UnixNano(),
		}
	}

	s.msgOut.Add(1)
	return Ack{
		MessageID:   env.ID,
		Success:     true,
		ProcessedAt: time.Now().UnixNano(),
	}
}

// SendBatch toplu mesaj gönder
func (s *Server) SendBatch(ctx context.Context, envelopes []Envelope) (acks []Ack, successCount, failCount int32) {
	acks = make([]Ack, len(envelopes))

	for i, env := range envelopes {
		ack := s.Send(ctx, env)
		acks[i] = ack
		if ack.Success {
			successCount++
		} else {
			failCount++
		}
	}
	return
}

// ProcessSensorData sensör verisini işle ve topic'e yayınla
func (s *Server) ProcessSensorData(data SensorData) error {
	s.msgIn.Add(1)

	topicName := fmt.Sprintf("sensor.%s", data.SensorType)

	// Topic yoksa oluştur — TOCTOU race'i önlemek için direkt dene
	if _, err := s.broker.CreateTopic(topicName, 4); err != nil {
		// "zaten var" hatası normaldir, diğer hataları logla
		if _, exists := s.broker.GetTopic(topicName); !exists {
			return fmt.Errorf("[BAĞ-GRPC] topic oluşturulamadı: %w", err)
		}
	}

	msg := queue.Message{
		ID:          data.SensorID + fmt.Sprintf("-%d", data.TimestampNs),
		Topic:       topicName,
		Payload:     data.Payload,
		Priority:    data.Priority,
		TimestampNs: data.TimestampNs,
		Headers:     data.Metadata,
	}

	return s.broker.Publish(topicName, msg)
}

// Consume topic'ten mesaj tüket
func (s *Server) Consume(topicName, groupID, consumerID string) (queue.Message, error) {
	msg, err := s.broker.Consume(topicName, groupID, consumerID)
	if err != nil {
		return msg, err
	}
	s.msgOut.Add(1)
	return msg, nil
}

// ── Topic Yönetimi ──

// CreateTopic yeni topic oluştur
func (s *Server) CreateTopic(name string, partitions int) error {
	_, err := s.broker.CreateTopic(name, partitions)
	return err
}

// DeleteTopic topic sil
func (s *Server) DeleteTopic(name string) error {
	return s.broker.DeleteTopic(name)
}

// ListTopics topic'leri listele
func (s *Server) ListTopics() []TopicInfo {
	topics := s.broker.ListTopics()
	result := make([]TopicInfo, len(topics))
	for i, t := range topics {
		result[i] = TopicInfo{
			Name:           t.Name,
			PartitionCount: t.PartitionCount(),
		}
	}
	return result
}

// ── Durum Servisi ──

// GetStatus sistem durumu
func (s *Server) GetStatus() StatusInfo {
	in, out, failed, topicCount := s.broker.GetStats()
	return StatusInfo{
		Module:         "BAĞ",
		Version:        "0.1.0",
		UptimeMs:       time.Since(s.startTime).Milliseconds(),
		MessagesIn:     in + s.msgIn.Load(),
		MessagesOut:    out + s.msgOut.Load(),
		MessagesFailed: failed,
		ActiveTopics:   topicCount,
		TopicDepths:    s.broker.TopicDepths(),
	}
}

// HealthCheck sağlık kontrolü
func (s *Server) HealthCheck() (healthy bool, status string, latencyNs int64) {
	start := time.Now()

	// Basit self-test: topic oluştur, yaz, oku, sil
	testTopic := "__bag_health_check"
	_, err := s.broker.CreateTopic(testTopic, 1)
	if err != nil {
		// Zaten varsa problem değil
	}

	testMsg := queue.Message{
		ID:          "health-check",
		Topic:       testTopic,
		Payload:     []byte("ping"),
		TimestampNs: time.Now().UnixNano(),
		Priority:    1,
	}

	if err := s.broker.Publish(testTopic, testMsg); err != nil {
		return false, fmt.Sprintf("publish hatası: %v", err), time.Since(start).Nanoseconds()
	}

	if _, err := s.broker.Consume(testTopic, "health", "hc-0"); err != nil {
		return false, fmt.Sprintf("consume hatası: %v", err), time.Since(start).Nanoseconds()
	}

	_ = s.broker.DeleteTopic(testTopic)
	return true, "sağlıklı", time.Since(start).Nanoseconds()
}

// Start dispatcher'ı başlat
func (s *Server) Start() {
	s.dispatcher.Start()
}

// Stop sunucuyu durdur
func (s *Server) Stop() {
	s.dispatcher.Stop()
	s.broker.Close()
}

// ── Yardımcı ──

func envelopeToMessage(env Envelope) queue.Message {
	return queue.Message{
		ID:            env.ID,
		Topic:         env.Topic,
		Payload:       env.Payload,
		Priority:      env.Priority,
		TimestampNs:   env.TimestampNs,
		RetryCount:    env.RetryCount,
		MaxRetries:    env.MaxRetries,
		CorrelationID: env.CorrelationID,
		Headers:       env.Headers,
	}
}
