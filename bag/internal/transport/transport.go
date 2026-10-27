// Package transport - Güvenilir mesaj taşıma katmanı
//
// Retry, ACK, mesaj doğrulama ve tekrar iletim mekanizmaları.
// Anlık kesintilere karşı veri kaybını önler.
package transport

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/bag/internal/queue"
)

var (
	ErrMaxRetries = errors.New("maksimum tekrar deneme sayısına ulaşıldı")
	ErrTimeout    = errors.New("zaman aşımı")
	ErrNoHandler  = errors.New("mesaj işleyici tanımlı değil")
)

// Handler mesaj işleme fonksiyonu
type Handler func(msg queue.Message) error

// Sender güvenilir mesaj gönderici (retry + exponential backoff)
type Sender struct {
	maxRetries    int
	baseDelay     time.Duration
	maxDelay      time.Duration
	ackTimeout    time.Duration
	sent          atomic.Int64
	failed        atomic.Int64
	retried       atomic.Int64
}

// NewSender yeni gönderici
func NewSender(maxRetries int, baseDelay, maxDelay, ackTimeout time.Duration) *Sender {
	return &Sender{
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
		maxDelay:   maxDelay,
		ackTimeout: ackTimeout,
	}
}

// Send mesajı handler'a güvenilir şekilde gönder (retry ile)
func (s *Sender) Send(ctx context.Context, msg queue.Message, handler Handler) error {
	var lastErr error

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if attempt > 0 {
			delay := s.calcBackoff(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			s.retried.Add(1)
			msg.RetryCount = int32(attempt)
		}

		// Zaman aşımlı gönderim
		done := make(chan error, 1)
		go func() {
			done <- handler(msg)
		}()

		select {
		case err := <-done:
			if err == nil {
				s.sent.Add(1)
				return nil
			}
			lastErr = err
			log.Printf("[BAĞ-TRANSPORT] Gönderim hatası (deneme %d/%d): %v", attempt+1, s.maxRetries+1, err)
		case <-time.After(s.ackTimeout):
			lastErr = ErrTimeout
			log.Printf("[BAĞ-TRANSPORT] Zaman aşımı (deneme %d/%d)", attempt+1, s.maxRetries+1)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	s.failed.Add(1)
	return fmt.Errorf("%w: son hata: %v", ErrMaxRetries, lastErr)
}

// Stats gönderim istatistikleri
func (s *Sender) Stats() (sent, failed, retried int64) {
	return s.sent.Load(), s.failed.Load(), s.retried.Load()
}

// calcBackoff exponential backoff with jitter
func (s *Sender) calcBackoff(attempt int) time.Duration {
	delay := float64(s.baseDelay) * math.Pow(2, float64(attempt-1))
	if delay > float64(s.maxDelay) {
		delay = float64(s.maxDelay)
	}
	return time.Duration(delay)
}

// ────────────────────────────────────────
// Receiver - Mesaj alıcı
// ────────────────────────────────────────

// Receiver mesaj alıcı ve doğrulayıcı
type Receiver struct {
	handlers   map[string]Handler // topic -> handler
	mu         sync.RWMutex
	received   atomic.Int64
	processed  atomic.Int64
	errors     atomic.Int64
}

// NewReceiver yeni alıcı
func NewReceiver() *Receiver {
	return &Receiver{
		handlers: make(map[string]Handler),
	}
}

// RegisterHandler topic için işleyici kaydet
func (r *Receiver) RegisterHandler(topic string, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[topic] = handler
}

// Process mesajı işle
func (r *Receiver) Process(msg queue.Message) error {
	r.received.Add(1)

	r.mu.RLock()
	handler, exists := r.handlers[msg.Topic]
	r.mu.RUnlock()

	if !exists {
		r.errors.Add(1)
		return fmt.Errorf("%w: topic=%s", ErrNoHandler, msg.Topic)
	}

	if err := handler(msg); err != nil {
		r.errors.Add(1)
		return err
	}

	r.processed.Add(1)
	return nil
}

// Stats alım istatistikleri
func (r *Receiver) Stats() (received, processed, errors int64) {
	return r.received.Load(), r.processed.Load(), r.errors.Load()
}

// ────────────────────────────────────────
// Dispatcher - Toplu mesaj dağıtıcı
// ────────────────────────────────────────

// Dispatcher mesajları toplu olarak hedefe dağıtan bileşen
type Dispatcher struct {
	sender    *Sender
	receiver  *Receiver
	batchSize int
	flushInterval time.Duration
	buffer    []queue.Message
	mu        sync.Mutex
	stopCh    chan struct{}
}

// NewDispatcher yeni dağıtıcı
func NewDispatcher(sender *Sender, receiver *Receiver, batchSize int, flushInterval time.Duration) *Dispatcher {
	return &Dispatcher{
		sender:        sender,
		receiver:      receiver,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		buffer:        make([]queue.Message, 0, batchSize),
		stopCh:        make(chan struct{}),
	}
}

// Add mesajı tampona ekle, tampon doluysa otomatik flush
func (d *Dispatcher) Add(msg queue.Message) error {
	d.mu.Lock()
	d.buffer = append(d.buffer, msg)
	shouldFlush := len(d.buffer) >= d.batchSize
	d.mu.Unlock()

	if shouldFlush {
		return d.Flush()
	}
	return nil
}

// Flush tampondaki mesajları gönder
func (d *Dispatcher) Flush() error {
	d.mu.Lock()
	if len(d.buffer) == 0 {
		d.mu.Unlock()
		return nil
	}
	batch := make([]queue.Message, len(d.buffer))
	copy(batch, d.buffer)
	d.buffer = d.buffer[:0]
	d.mu.Unlock()

	var firstErr error
	for _, msg := range batch {
		if err := d.receiver.Process(msg); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Start periyodik flush başlat
func (d *Dispatcher) Start() {
	go func() {
		ticker := time.NewTicker(d.flushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = d.Flush()
			case <-d.stopCh:
				return
			}
		}
	}()
}

// Stop dağıtıcıyı durdur
func (d *Dispatcher) Stop() {
	close(d.stopCh)
	_ = d.Flush() // Kalan mesajları gönder
}
