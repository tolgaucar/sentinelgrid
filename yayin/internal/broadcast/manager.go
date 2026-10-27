// Package broadcast — Kanal tabanlı yayın yöneticisi.
//
// Komuta merkezinden çıkan emirlerin, koordinatların ve
// güncellemelerin sahadaki birimlere güvenli ve sıkıştırılmış
// olarak yayınlanmasını yönetir. Her yayın kanalı: sıkıştırma →
// şifreleme → FEC → abonelere iletme zincirini uygular.
package broadcast

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/yayin/internal/compress"
	"github.com/otukenmatris/yayin/internal/crypto"
	"github.com/otukenmatris/yayin/internal/fec"
)

// Priority mesaj önceliği.
type Priority int

const (
	PriorityLow      Priority = 0
	PriorityNormal   Priority = 1
	PriorityHigh     Priority = 2
	PriorityCritical Priority = 3 // Hayati: düşman alarmı, vurma onayı
	PriorityFlash    Priority = 4 // Anlık: NBC alarm, geri çekilme
)

// Message yayın mesajı.
type Message struct {
	ID          string    `json:"id"`
	ChannelID   string    `json:"channel_id"`
	SenderID    string    `json:"sender_id"`
	Priority    Priority  `json:"priority"`
	ContentType string    `json:"content_type"` // command, coordinate, update, alert
	Payload     []byte    `json:"payload"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Compressed  bool      `json:"compressed"`
	Encrypted   bool      `json:"encrypted"`
	FECApplied  bool      `json:"fec_applied"`

	// İşlenmiş veri (wire format)
	Wire     []byte `json:"-"`
	WireSize int    `json:"wire_size"`
	RawSize  int    `json:"raw_size"`
}

// Subscriber bir yayın abonesi.
type Subscriber struct {
	ID           string    `json:"id"`
	UnitID       string    `json:"unit_id"`
	ChannelID    string    `json:"channel_id"`
	SubscribedAt time.Time `json:"subscribed_at"`
	LastDelivery time.Time `json:"last_delivery"`
	Delivered    int64     `json:"delivered"`
	Errors       int64     `json:"errors"`
}

// Channel yayın kanalı.
type Channel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	Priority    Priority  `json:"default_priority"`
	Encrypted   bool      `json:"encrypted"`
	Compressed  bool      `json:"compressed"`
	FEC         bool      `json:"fec_enabled"`

	mu          sync.RWMutex
	subscribers map[string]*Subscriber
	history     []*Message
	historyMax  int
}

// Manager yayın yöneticisi.
type Manager struct {
	mu       sync.RWMutex
	channels map[string]*Channel

	cryptoEngine *crypto.Engine
	compressor   *compress.Compressor
	fecEncoder   *fec.Encoder

	maxChannels    int
	maxSubscribers int
	maxMsgSize     int
	historySize    int
	messageTTL     time.Duration

	stats ManagerStats
}

// ManagerStats yayın istatistikleri.
type ManagerStats struct {
	TotalPublished    int64 `json:"total_published"`
	TotalDelivered    int64 `json:"total_delivered"`
	TotalChannels     int64 `json:"total_channels"`
	TotalSubscribers  int64 `json:"total_subscribers"`
	BytesRaw          int64 `json:"bytes_raw"`
	BytesWire         int64 `json:"bytes_wire"`
	CompressionSaved  int64 `json:"compression_saved"`
	FECBlocksCreated  int64 `json:"fec_blocks_created"`
}

// NewManager yeni yayın yöneticisi oluşturur.
func NewManager(
	cryptoEngine *crypto.Engine,
	compressor *compress.Compressor,
	fecEncoder *fec.Encoder,
	maxChannels, maxSubscribers, maxMsgSize, historySize int,
	messageTTL time.Duration,
) *Manager {
	return &Manager{
		channels:       make(map[string]*Channel),
		cryptoEngine:   cryptoEngine,
		compressor:     compressor,
		fecEncoder:     fecEncoder,
		maxChannels:    maxChannels,
		maxSubscribers: maxSubscribers,
		maxMsgSize:     maxMsgSize,
		historySize:    historySize,
		messageTTL:     messageTTL,
	}
}

// CreateChannel yeni yayın kanalı oluşturur.
func (m *Manager) CreateChannel(name, description string, priority Priority, encrypted, compressed, fecEnabled bool) (*Channel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.channels) >= m.maxChannels {
		return nil, fmt.Errorf("kanal limiti aşıldı: %d/%d", len(m.channels), m.maxChannels)
	}

	// Aynı isimde kanal var mı?
	for _, ch := range m.channels {
		if ch.Name == name {
			return nil, fmt.Errorf("kanal zaten var: %s", name)
		}
	}

	id := generateID()
	ch := &Channel{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		Priority:    priority,
		Encrypted:   encrypted,
		Compressed:  compressed,
		FEC:         fecEnabled,
		subscribers: make(map[string]*Subscriber),
		historyMax:  m.historySize,
	}

	m.channels[id] = ch
	atomic.AddInt64(&m.stats.TotalChannels, 1)
	return ch, nil
}

// DeleteChannel kanalı siler.
func (m *Manager) DeleteChannel(channelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch, ok := m.channels[channelID]
	if !ok {
		return fmt.Errorf("kanal bulunamadı: %s", channelID)
	}

	ch.mu.RLock()
	subCount := len(ch.subscribers)
	ch.mu.RUnlock()

	delete(m.channels, channelID)
	atomic.AddInt64(&m.stats.TotalChannels, -1)
	atomic.AddInt64(&m.stats.TotalSubscribers, -int64(subCount))
	return nil
}

// GetChannel kanal bilgisi döndürür.
func (m *Manager) GetChannel(channelID string) (*Channel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ch, ok := m.channels[channelID]
	if !ok {
		return nil, fmt.Errorf("kanal bulunamadı: %s", channelID)
	}
	return ch, nil
}

// ListChannels tüm kanalları listeler.
func (m *Manager) ListChannels() []*Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		result = append(result, ch)
	}
	return result
}

// Subscribe birim kanalı abone olur.
func (m *Manager) Subscribe(channelID, unitID string) (*Subscriber, error) {
	m.mu.RLock()
	ch, ok := m.channels[channelID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("kanal bulunamadı: %s", channelID)
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()

	if len(ch.subscribers) >= m.maxSubscribers {
		return nil, fmt.Errorf("abone limiti aşıldı: %d/%d", len(ch.subscribers), m.maxSubscribers)
	}

	// Zaten abone mi?
	for _, s := range ch.subscribers {
		if s.UnitID == unitID {
			return s, nil
		}
	}

	sub := &Subscriber{
		ID:           generateID(),
		UnitID:       unitID,
		ChannelID:    channelID,
		SubscribedAt: time.Now(),
	}

	ch.subscribers[sub.ID] = sub
	atomic.AddInt64(&m.stats.TotalSubscribers, 1)
	return sub, nil
}

// Unsubscribe aboneliği iptal eder.
func (m *Manager) Unsubscribe(channelID, unitID string) error {
	m.mu.RLock()
	ch, ok := m.channels[channelID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("kanal bulunamadı: %s", channelID)
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()

	for id, s := range ch.subscribers {
		if s.UnitID == unitID {
			delete(ch.subscribers, id)
			atomic.AddInt64(&m.stats.TotalSubscribers, -1)
			return nil
		}
	}

	return fmt.Errorf("abone bulunamadı: %s kanalında %s", channelID, unitID)
}

// Publish mesaj yayınlar. Sıkıştırma → şifreleme → FEC zinciri uygulanır.
func (m *Manager) Publish(channelID, senderID, contentType string, payload []byte, priority Priority) (*Message, error) {
	m.mu.RLock()
	ch, ok := m.channels[channelID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("kanal bulunamadı: %s", channelID)
	}

	if len(payload) > m.maxMsgSize {
		return nil, fmt.Errorf("mesaj çok büyük: %d > %d byte", len(payload), m.maxMsgSize)
	}

	msg := &Message{
		ID:          generateID(),
		ChannelID:   channelID,
		SenderID:    senderID,
		Priority:    priority,
		ContentType: contentType,
		Payload:     payload,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(m.messageTTL),
		RawSize:     len(payload),
	}

	// İşleme zinciri: veri → sıkıştır → şifrele
	wire := payload

	// 1. Sıkıştırma
	if ch.Compressed && m.compressor != nil {
		compressed, err := m.compressor.Compress(wire)
		if err != nil {
			return nil, fmt.Errorf("sıkıştırma hatası: %w", err)
		}
		saved := len(wire) - len(compressed)
		if saved > 0 {
			atomic.AddInt64(&m.stats.CompressionSaved, int64(saved))
		}
		wire = compressed
		msg.Compressed = true
	}

	// 2. Şifreleme
	if ch.Encrypted && m.cryptoEngine != nil {
		encrypted, err := m.cryptoEngine.Encrypt(wire)
		if err != nil {
			return nil, fmt.Errorf("şifreleme hatası: %w", err)
		}
		wire = encrypted
		msg.Encrypted = true
	}

	// 3. FEC (blok bilgisi meta olarak saklanır, wire'a eklenmez)
	if ch.FEC && m.fecEncoder != nil {
		blocks := m.fecEncoder.Encode(wire)
		atomic.AddInt64(&m.stats.FECBlocksCreated, int64(len(blocks)))
		msg.FECApplied = true
	}

	msg.Wire = wire
	msg.WireSize = len(wire)

	atomic.AddInt64(&m.stats.BytesRaw, int64(msg.RawSize))
	atomic.AddInt64(&m.stats.BytesWire, int64(msg.WireSize))

	// Geçmişe ekle
	ch.mu.Lock()
	ch.history = append(ch.history, msg)
	if len(ch.history) > ch.historyMax {
		ch.history = ch.history[len(ch.history)-ch.historyMax:]
	}
	subCount := len(ch.subscribers)
	ch.mu.Unlock()

	atomic.AddInt64(&m.stats.TotalPublished, 1)
	atomic.AddInt64(&m.stats.TotalDelivered, int64(subCount))

	return msg, nil
}

// History kanal geçmişini döndürür.
func (m *Manager) History(channelID string, n int) ([]*Message, error) {
	m.mu.RLock()
	ch, ok := m.channels[channelID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("kanal bulunamadı: %s", channelID)
	}

	ch.mu.RLock()
	defer ch.mu.RUnlock()

	hist := ch.history
	if n > 0 && n < len(hist) {
		hist = hist[len(hist)-n:]
	}

	result := make([]*Message, len(hist))
	copy(result, hist)
	return result, nil
}

// ChannelSubscribers kanal abonelerini listeler.
func (m *Manager) ChannelSubscribers(channelID string) ([]*Subscriber, error) {
	m.mu.RLock()
	ch, ok := m.channels[channelID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("kanal bulunamadı: %s", channelID)
	}

	ch.mu.RLock()
	defer ch.mu.RUnlock()

	result := make([]*Subscriber, 0, len(ch.subscribers))
	for _, s := range ch.subscribers {
		result = append(result, s)
	}
	return result, nil
}

// Decode mesajı çözer: şifre çöz → sıkıştırma aç.
func (m *Manager) Decode(msg *Message) ([]byte, error) {
	wire := msg.Wire

	// 1. Şifre çöz
	if msg.Encrypted && m.cryptoEngine != nil {
		decrypted, err := m.cryptoEngine.Decrypt(wire)
		if err != nil {
			return nil, fmt.Errorf("şifre çözme hatası: %w", err)
		}
		wire = decrypted
	}

	// 2. Sıkıştırma aç
	if msg.Compressed && m.compressor != nil {
		decompressed, err := m.compressor.Decompress(wire)
		if err != nil {
			return nil, fmt.Errorf("sıkıştırma açma hatası: %w", err)
		}
		wire = decompressed
	}

	return wire, nil
}

// CleanExpired süresi dolmuş mesajları temizler.
func (m *Manager) CleanExpired() int {
	m.mu.RLock()
	channels := make([]*Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		channels = append(channels, ch)
	}
	m.mu.RUnlock()

	total := 0
	now := time.Now()
	for _, ch := range channels {
		ch.mu.Lock()
		var kept []*Message
		for _, msg := range ch.history {
			if msg.ExpiresAt.After(now) {
				kept = append(kept, msg)
			} else {
				total++
			}
		}
		ch.history = kept
		ch.mu.Unlock()
	}
	return total
}

// Stats istatistikleri döndürür.
func (m *Manager) Stats() ManagerStats {
	return ManagerStats{
		TotalPublished:   atomic.LoadInt64(&m.stats.TotalPublished),
		TotalDelivered:   atomic.LoadInt64(&m.stats.TotalDelivered),
		TotalChannels:    atomic.LoadInt64(&m.stats.TotalChannels),
		TotalSubscribers: atomic.LoadInt64(&m.stats.TotalSubscribers),
		BytesRaw:         atomic.LoadInt64(&m.stats.BytesRaw),
		BytesWire:        atomic.LoadInt64(&m.stats.BytesWire),
		CompressionSaved: atomic.LoadInt64(&m.stats.CompressionSaved),
		FECBlocksCreated: atomic.LoadInt64(&m.stats.FECBlocksCreated),
	}
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
