// Package server — YAYIN HTTP REST API sunucusu.
package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/yayin/internal/broadcast"
	"github.com/otukenmatris/yayin/internal/compress"
	"github.com/otukenmatris/yayin/internal/crypto"
	"github.com/otukenmatris/yayin/internal/fec"
)

// Server YAYIN HTTP sunucusu.
type Server struct {
	bcast      *broadcast.Manager
	cryptoEng  *crypto.Engine
	compressor *compress.Compressor
	fecEncoder *fec.Encoder
	startTime  time.Time
	reqCount   int64
}

// NewServer yeni sunucu oluşturur.
func NewServer(bcast *broadcast.Manager, cryptoEng *crypto.Engine, comp *compress.Compressor, fecEnc *fec.Encoder) *Server {
	return &Server{
		bcast:      bcast,
		cryptoEng:  cryptoEng,
		compressor: comp,
		fecEncoder: fecEnc,
		startTime:  time.Now(),
	}
}

// Handler HTTP mux döndürür.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Sistem
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)

	// Kanallar
	mux.HandleFunc("/channels", s.handleChannels)
	mux.HandleFunc("/channels/", s.handleChannelByID)

	// Yayın
	mux.HandleFunc("/publish", s.handlePublish)
	mux.HandleFunc("/decode", s.handleDecode)

	// Abonelik
	mux.HandleFunc("/subscribe", s.handleSubscribe)
	mux.HandleFunc("/unsubscribe", s.handleUnsubscribe)

	// Geçmiş
	mux.HandleFunc("/history/", s.handleHistory)

	// Şifreleme
	mux.HandleFunc("/crypto/encrypt", s.handleEncrypt)
	mux.HandleFunc("/crypto/decrypt", s.handleDecrypt)
	mux.HandleFunc("/crypto/rotate", s.handleCryptoRotate)
	mux.HandleFunc("/crypto/keys", s.handleCryptoKeys)

	// Sıkıştırma
	mux.HandleFunc("/compress", s.handleCompress)
	mux.HandleFunc("/decompress", s.handleDecompress)

	// FEC
	mux.HandleFunc("/fec/encode", s.handleFECEncode)
	mux.HandleFunc("/fec/decode", s.handleFECDecode)

	return mux
}

// ── Sistem ──

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	s.json(w, 200, map[string]any{
		"status":  "ok",
		"module":  "yayin",
		"version": "0.1.0",
		"uptime":  time.Since(s.startTime).String(),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	bcastStats := s.bcast.Stats()
	cryptoStats := s.cryptoEng.Stats()
	compStats := s.compressor.Stats()
	fecStats := s.fecEncoder.Stats()

	s.json(w, 200, map[string]any{
		"module":      "yayin",
		"uptime":      time.Since(s.startTime).String(),
		"requests":    atomic.LoadInt64(&s.reqCount),
		"broadcast":   bcastStats,
		"crypto":      cryptoStats,
		"compression": compStats,
		"fec":         fecStats,
	})
}

// ── Kanallar ──

func (s *Server) handleChannels(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)

	switch r.Method {
	case http.MethodGet:
		channels := s.bcast.ListChannels()
		type chInfo struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Priority    int    `json:"priority"`
			Encrypted   bool   `json:"encrypted"`
			Compressed  bool   `json:"compressed"`
			FEC         bool   `json:"fec"`
			Subscribers int    `json:"subscribers"`
		}
		result := make([]chInfo, 0, len(channels))
		for _, ch := range channels {
			subs, _ := s.bcast.ChannelSubscribers(ch.ID)
			result = append(result, chInfo{
				ID:          ch.ID,
				Name:        ch.Name,
				Description: ch.Description,
				Priority:    int(ch.Priority),
				Encrypted:   ch.Encrypted,
				Compressed:  ch.Compressed,
				FEC:         ch.FEC,
				Subscribers: len(subs),
			})
		}
		s.json(w, 200, result)

	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Priority    int    `json:"priority"`
			Encrypted   bool   `json:"encrypted"`
			Compressed  bool   `json:"compressed"`
			FEC         bool   `json:"fec"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.json(w, 400, map[string]string{"error": "geçersiz JSON: " + err.Error()})
			return
		}
		if req.Name == "" {
			s.json(w, 400, map[string]string{"error": "name zorunlu"})
			return
		}

		ch, err := s.bcast.CreateChannel(req.Name, req.Description, broadcast.Priority(req.Priority), req.Encrypted, req.Compressed, req.FEC)
		if err != nil {
			s.json(w, 409, map[string]string{"error": err.Error()})
			return
		}
		s.json(w, 201, map[string]any{
			"id":   ch.ID,
			"name": ch.Name,
		})

	default:
		s.json(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleChannelByID(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/channels/"), "/")
	channelID := parts[0]
	if channelID == "" {
		s.json(w, 400, map[string]string{"error": "channel_id zorunlu"})
		return
	}

	// /channels/{id}/subscribers
	if len(parts) > 1 && parts[1] == "subscribers" {
		subs, err := s.bcast.ChannelSubscribers(channelID)
		if err != nil {
			s.json(w, 404, map[string]string{"error": err.Error()})
			return
		}
		s.json(w, 200, subs)
		return
	}

	switch r.Method {
	case http.MethodGet:
		ch, err := s.bcast.GetChannel(channelID)
		if err != nil {
			s.json(w, 404, map[string]string{"error": err.Error()})
			return
		}
		subs, _ := s.bcast.ChannelSubscribers(channelID)
		s.json(w, 200, map[string]any{
			"id":          ch.ID,
			"name":        ch.Name,
			"description": ch.Description,
			"priority":    int(ch.Priority),
			"encrypted":   ch.Encrypted,
			"compressed":  ch.Compressed,
			"fec":         ch.FEC,
			"subscribers": len(subs),
			"created_at":  ch.CreatedAt,
		})

	case http.MethodDelete:
		if err := s.bcast.DeleteChannel(channelID); err != nil {
			s.json(w, 404, map[string]string{"error": err.Error()})
			return
		}
		s.json(w, 200, map[string]string{"status": "silindi"})

	default:
		s.json(w, 405, map[string]string{"error": "method not allowed"})
	}
}

// ── Yayın ──

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]string{"error": "POST bekleniyor"})
		return
	}

	var req struct {
		ChannelID   string `json:"channel_id"`
		SenderID    string `json:"sender_id"`
		ContentType string `json:"content_type"`
		Payload     string `json:"payload"`
		Priority    int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]string{"error": "geçersiz JSON"})
		return
	}

	msg, err := s.bcast.Publish(req.ChannelID, req.SenderID, req.ContentType, []byte(req.Payload), broadcast.Priority(req.Priority))
	if err != nil {
		s.json(w, 400, map[string]string{"error": err.Error()})
		return
	}

	s.json(w, 200, map[string]any{
		"message_id":  msg.ID,
		"channel_id":  msg.ChannelID,
		"raw_size":    msg.RawSize,
		"wire_size":   msg.WireSize,
		"compressed":  msg.Compressed,
		"encrypted":   msg.Encrypted,
		"fec_applied": msg.FECApplied,
		"expires_at":  msg.ExpiresAt,
	})
}

func (s *Server) handleDecode(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]string{"error": "POST bekleniyor"})
		return
	}

	var req struct {
		ChannelID string `json:"channel_id"`
		MessageID string `json:"message_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]string{"error": "geçersiz JSON"})
		return
	}

	hist, err := s.bcast.History(req.ChannelID, 0)
	if err != nil {
		s.json(w, 404, map[string]string{"error": err.Error()})
		return
	}

	for _, msg := range hist {
		if msg.ID == req.MessageID {
			decoded, err := s.bcast.Decode(msg)
			if err != nil {
				s.json(w, 500, map[string]string{"error": err.Error()})
				return
			}
			s.json(w, 200, map[string]any{
				"message_id": msg.ID,
				"payload":    string(decoded),
				"decoded":    true,
			})
			return
		}
	}
	s.json(w, 404, map[string]string{"error": "mesaj bulunamadı"})
}

// ── Abonelik ──

func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]string{"error": "POST bekleniyor"})
		return
	}
	var req struct {
		ChannelID string `json:"channel_id"`
		UnitID    string `json:"unit_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]string{"error": "geçersiz JSON"})
		return
	}
	sub, err := s.bcast.Subscribe(req.ChannelID, req.UnitID)
	if err != nil {
		s.json(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.json(w, 200, map[string]any{
		"subscriber_id": sub.ID,
		"unit_id":       sub.UnitID,
		"channel_id":    sub.ChannelID,
	})
}

func (s *Server) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]string{"error": "POST bekleniyor"})
		return
	}
	var req struct {
		ChannelID string `json:"channel_id"`
		UnitID    string `json:"unit_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]string{"error": "geçersiz JSON"})
		return
	}
	if err := s.bcast.Unsubscribe(req.ChannelID, req.UnitID); err != nil {
		s.json(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.json(w, 200, map[string]string{"status": "abonelik iptal edildi"})
}

// ── Geçmiş ──

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	channelID := strings.TrimPrefix(r.URL.Path, "/history/")
	if channelID == "" {
		s.json(w, 400, map[string]string{"error": "channel_id zorunlu"})
		return
	}
	n := 50
	if q := r.URL.Query().Get("n"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	hist, err := s.bcast.History(channelID, n)
	if err != nil {
		s.json(w, 404, map[string]string{"error": err.Error()})
		return
	}
	type msgInfo struct {
		ID          string    `json:"id"`
		SenderID    string    `json:"sender_id"`
		ContentType string    `json:"content_type"`
		Priority    int       `json:"priority"`
		RawSize     int       `json:"raw_size"`
		WireSize    int       `json:"wire_size"`
		Compressed  bool      `json:"compressed"`
		Encrypted   bool      `json:"encrypted"`
		FECApplied  bool      `json:"fec_applied"`
		CreatedAt   time.Time `json:"created_at"`
		ExpiresAt   time.Time `json:"expires_at"`
	}
	result := make([]msgInfo, 0, len(hist))
	for _, m := range hist {
		result = append(result, msgInfo{
			ID:          m.ID,
			SenderID:    m.SenderID,
			ContentType: m.ContentType,
			Priority:    int(m.Priority),
			RawSize:     m.RawSize,
			WireSize:    m.WireSize,
			Compressed:  m.Compressed,
			Encrypted:   m.Encrypted,
			FECApplied:  m.FECApplied,
			CreatedAt:   m.CreatedAt,
			ExpiresAt:   m.ExpiresAt,
		})
	}
	s.json(w, 200, result)
}

// ── Şifreleme API ──

func (s *Server) handleEncrypt(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]string{"error": "POST bekleniyor"})
		return
	}
	var req struct {
		Data string `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]string{"error": "geçersiz JSON"})
		return
	}
	encrypted, err := s.cryptoEng.Encrypt([]byte(req.Data))
	if err != nil {
		s.json(w, 500, map[string]string{"error": err.Error()})
		return
	}
	s.json(w, 200, map[string]any{
		"size_original":  len(req.Data),
		"size_encrypted": len(encrypted),
		"key_id":         s.cryptoEng.ActiveKeyID(),
	})
}

func (s *Server) handleDecrypt(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	s.json(w, 200, map[string]string{"info": "Decrypt: wire format üzerinden çalışır, /decode endpoint kullanın"})
}

func (s *Server) handleCryptoRotate(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]string{"error": "POST bekleniyor"})
		return
	}
	if err := s.cryptoEng.RotateKey(); err != nil {
		s.json(w, 500, map[string]string{"error": err.Error()})
		return
	}
	s.json(w, 200, map[string]any{
		"status":     "anahtar döndürüldü",
		"new_key_id": s.cryptoEng.ActiveKeyID(),
		"total_keys": s.cryptoEng.KeyCount(),
	})
}

func (s *Server) handleCryptoKeys(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	s.json(w, 200, map[string]any{
		"active_key_id": s.cryptoEng.ActiveKeyID(),
		"total_keys":    s.cryptoEng.KeyCount(),
		"keys":          s.cryptoEng.Keys(),
	})
}

// ── Sıkıştırma API ──

func (s *Server) handleCompress(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]string{"error": "POST bekleniyor"})
		return
	}
	var req struct {
		Data string `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]string{"error": "geçersiz JSON"})
		return
	}
	compressed, err := s.compressor.Compress([]byte(req.Data))
	if err != nil {
		s.json(w, 500, map[string]string{"error": err.Error()})
		return
	}
	ratio := 0.0
	if len(req.Data) > 0 {
		ratio = float64(len(compressed)) / float64(len(req.Data)) * 100
	}
	s.json(w, 200, map[string]any{
		"original_size":   len(req.Data),
		"compressed_size": len(compressed),
		"ratio_pct":       ratio,
		"level":           s.compressor.Level(),
	})
}

func (s *Server) handleDecompress(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	s.json(w, 200, map[string]string{"info": "Decompress: wire format üzerinden çalışır, /decode endpoint kullanın"})
}

// ── FEC API ──

func (s *Server) handleFECEncode(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]string{"error": "POST bekleniyor"})
		return
	}
	var req struct {
		Data string `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]string{"error": "geçersiz JSON"})
		return
	}
	blocks := s.fecEncoder.Encode([]byte(req.Data))
	dataBlocks := 0
	parityBlocks := 0
	for _, b := range blocks {
		if b.IsParity {
			parityBlocks++
		} else {
			dataBlocks++
		}
	}
	s.json(w, 200, map[string]any{
		"original_size": len(req.Data),
		"total_blocks":  len(blocks),
		"data_blocks":   dataBlocks,
		"parity_blocks": parityBlocks,
		"block_size":    s.fecEncoder.BlockSize(),
		"data_shards":   s.fecEncoder.DataShards(),
	})
}

func (s *Server) handleFECDecode(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	s.json(w, 200, map[string]any{
		"info":        "FEC decode: blok bazlı çalışır",
		"data_shards": s.fecEncoder.DataShards(),
		"block_size":  s.fecEncoder.BlockSize(),
	})
}

// ── Yardımcı ──

func (s *Server) json(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}
