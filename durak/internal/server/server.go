// Package server — DURAK HTTP API sunucusu.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/otukenmatris/durak/internal/queue"
	"github.com/otukenmatris/durak/internal/scheduler"
)

// Server HTTP sunucu.
type Server struct {
	scheduler *scheduler.Scheduler
	mux       *http.ServeMux
	startTime time.Time
}

// New yeni sunucu oluşturur.
func New(sched *scheduler.Scheduler) *Server {
	s := &Server{
		scheduler: sched,
		mux:       http.NewServeMux(),
		startTime: time.Now(),
	}
	s.routes()
	return s
}

// Handler HTTP handler'ını döndürür.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/status", s.handleStatus)
	s.mux.HandleFunc("/queues", s.handleQueues)
	s.mux.HandleFunc("/enqueue", s.handleEnqueue)
	s.mux.HandleFunc("/dequeue", s.handleDequeue)
	s.mux.HandleFunc("/scheduler/stats", s.handleSchedulerStats)
	s.mux.HandleFunc("/scheduler/bandwidth", s.handleBandwidth)
	s.mux.HandleFunc("/flush", s.handleFlush)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, 200, map[string]any{
		"status":  "ok",
		"module":  "durak",
		"uptime":  time.Since(s.startTime).String(),
		"running": s.scheduler.IsRunning(),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	queues := s.scheduler.Queues()
	queueList := make([]map[string]any, 0, len(queues))
	totalPackets := 0

	for _, pq := range queues {
		stats := pq.Stats()
		totalPackets += int(stats.CurrentSize)
		queueList = append(queueList, map[string]any{
			"id":       pq.Info().ID,
			"name":     pq.Info().Name,
			"size":     stats.CurrentSize,
			"enqueued": stats.TotalEnqueued,
			"dequeued": stats.TotalDequeued,
			"dropped":  stats.TotalDropped,
		})
	}

	schStats := s.scheduler.Stats()
	jsonResp(w, 200, map[string]any{
		"module":        "durak",
		"uptime":        time.Since(s.startTime).String(),
		"queues":        queueList,
		"total_packets": totalPackets,
		"scheduler": map[string]any{
			"running":        s.scheduler.IsRunning(),
			"scheduled":      schStats.TotalScheduled,
			"bytes":          schStats.TotalBytes,
			"bandwidth_used": schStats.BandwidthUsed,
			"ticks":          schStats.Ticks,
		},
	})
}

func (s *Server) handleQueues(w http.ResponseWriter, r *http.Request) {
	queues := s.scheduler.Queues()
	list := make([]map[string]any, 0, len(queues))
	for _, pq := range queues {
		info := pq.Info()
		stats := pq.Stats()
		list = append(list, map[string]any{
			"id":          info.ID,
			"name":        info.Name,
			"description": info.Description,
			"max_size":    info.MaxSize,
			"current":     stats.CurrentSize,
			"peak":        stats.PeakSize,
			"enqueued":    stats.TotalEnqueued,
			"dequeued":    stats.TotalDequeued,
			"dropped":     stats.TotalDropped,
			"expired":     stats.TotalExpired,
			"avg_wait_ms": stats.AvgWaitMs,
		})
	}
	jsonResp(w, 200, map[string]any{"queues": list})
}

type enqueueReq struct {
	QueueID     string `json:"queue_id"`
	Priority    int    `json:"priority"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	ContentType string `json:"content_type"`
	Payload     string `json:"payload"`
	TTLSeconds  int    `json:"ttl_seconds"`
}

func (s *Server) handleEnqueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResp(w, 405, map[string]any{"error": "POST gerekli"})
		return
	}

	var req enqueueReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResp(w, 400, map[string]any{"error": "Geçersiz JSON: " + err.Error()})
		return
	}

	pq := s.scheduler.GetQueue(req.QueueID)
	if pq == nil {
		jsonResp(w, 404, map[string]any{"error": "Kuyruk bulunamadı: " + req.QueueID})
		return
	}

	payload := []byte(req.Payload)
	ttl := time.Duration(req.TTLSeconds) * time.Second
	if ttl == 0 {
		ttl = 5 * time.Minute // varsayılan
	}

	pkt := &queue.Packet{
		ID:          fmt.Sprintf("pkt-%d", time.Now().UnixNano()),
		Priority:    queue.Priority(req.Priority),
		Source:      req.Source,
		Destination: req.Destination,
		ContentType: req.ContentType,
		Payload:     payload,
		Size:        len(payload),
		CreatedAt:   time.Now(),
		TTL:         ttl,
	}

	if err := pq.Enqueue(pkt); err != nil {
		jsonResp(w, 429, map[string]any{"error": err.Error()})
		return
	}

	jsonResp(w, 200, map[string]any{"id": pkt.ID, "queue": req.QueueID, "priority": queue.PriorityName(pkt.Priority)})
}

type dequeueReq struct {
	QueueID string `json:"queue_id"`
}

func (s *Server) handleDequeue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResp(w, 405, map[string]any{"error": "POST gerekli"})
		return
	}

	var req dequeueReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResp(w, 400, map[string]any{"error": "Geçersiz JSON: " + err.Error()})
		return
	}

	pq := s.scheduler.GetQueue(req.QueueID)
	if pq == nil {
		jsonResp(w, 404, map[string]any{"error": "Kuyruk bulunamadı: " + req.QueueID})
		return
	}

	pkt, err := pq.Dequeue()
	if err != nil {
		jsonResp(w, 204, nil)
		return
	}

	jsonResp(w, 200, map[string]any{
		"id":           pkt.ID,
		"priority":     queue.PriorityName(pkt.Priority),
		"source":       pkt.Source,
		"destination":  pkt.Destination,
		"content_type": pkt.ContentType,
		"size":         pkt.Size,
		"wait_ms":      pkt.DequeuedAt.Sub(pkt.EnqueuedAt).Milliseconds(),
	})
}

func (s *Server) handleSchedulerStats(w http.ResponseWriter, r *http.Request) {
	stats := s.scheduler.Stats()
	jsonResp(w, 200, map[string]any{
		"total_scheduled":  stats.TotalScheduled,
		"total_bytes":      stats.TotalBytes,
		"bandwidth_used":   stats.BandwidthUsed,
		"ticks":            stats.Ticks,
		"dropped_overflow": stats.DroppedOverflow,
	})
}

type bandwidthReq struct {
	Bps int64 `json:"bps"`
}

func (s *Server) handleBandwidth(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.scheduler.Stats()
		jsonResp(w, 200, map[string]any{"bandwidth_bps": s.scheduler.Stats().BandwidthUsed})
		return
	}
	if r.Method != http.MethodPost {
		jsonResp(w, 405, map[string]any{"error": "GET/POST gerekli"})
		return
	}

	var req bandwidthReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResp(w, 400, map[string]any{"error": "Geçersiz JSON"})
		return
	}
	if req.Bps <= 0 {
		jsonResp(w, 400, map[string]any{"error": "bps > 0 olmalı"})
		return
	}

	s.scheduler.SetBandwidth(req.Bps)
	jsonResp(w, 200, map[string]any{"bandwidth_bps": req.Bps, "status": "güncellendi"})
}

func (s *Server) handleFlush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResp(w, 405, map[string]any{"error": "POST gerekli"})
		return
	}

	// queue_id varsa sadece o kuyruğu, yoksa hepsini boşalt
	var req struct {
		QueueID string `json:"queue_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	flushed := 0
	queues := s.scheduler.Queues()
	for id, pq := range queues {
		if req.QueueID != "" && !strings.EqualFold(id, req.QueueID) {
			continue
		}
		for pq.Size() > 0 {
			_, err := pq.Dequeue()
			if err != nil {
				break
			}
			flushed++
		}
	}

	jsonResp(w, 200, map[string]any{"flushed": flushed})
}

func jsonResp(w http.ResponseWriter, code int, data any) {
	if data == nil {
		w.WriteHeader(code)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}
