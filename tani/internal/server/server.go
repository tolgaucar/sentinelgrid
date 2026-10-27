// Package server — TANI HTTP REST API.
//
// Nesne tespit, sınıflandırma ve izleme servislerini HTTP üzerinden sunar.
// Simülasyon ortamından gelen kare verilerini işler, tespit sonuçlarını
// JSON olarak döndürür.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/tani/internal/classifier"
	"github.com/otukenmatris/tani/internal/detector"
	"github.com/otukenmatris/tani/internal/tracker"
)

// Server TANI HTTP sunucusu.
type Server struct {
	detector   *detector.Engine
	classifier *classifier.Engine
	tracker    *tracker.Engine
	startTime  time.Time
	reqCount   int64
}

// NewServer yeni sunucu oluşturur.
func NewServer(det *detector.Engine, cls *classifier.Engine, trk *tracker.Engine) *Server {
	return &Server{
		detector:   det,
		classifier: cls,
		tracker:    trk,
		startTime:  time.Now(),
	}
}

// Handler HTTP route tanımları.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Sistem
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)

	// Tespit
	mux.HandleFunc("/detect", s.handleDetect)           // POST: kare gönder, tespit yap
	mux.HandleFunc("/detect/last", s.handleDetectLast)   // GET: son tespit sonucu

	// Sınıflandırma
	mux.HandleFunc("/classify", s.handleClassify)         // POST: tespit sınıflandır
	mux.HandleFunc("/classify/stats", s.handleClassifyStats) // GET: sınıflandırma istatistikleri

	// İzleme
	mux.HandleFunc("/tracks", s.handleTracks)             // GET: aktif izler
	mux.HandleFunc("/tracks/hostile", s.handleHostile)     // GET: düşman izler
	mux.HandleFunc("/tracks/", s.handleTrackByID)          // GET: belirli iz
	mux.HandleFunc("/tracks/stats", s.handleTrackStats)    // GET: izleme istatistikleri

	// İleri seviye
	mux.HandleFunc("/process", s.handleProcess)            // POST: kare → tespit + sınıflandır + izle (tam pipeline)
	mux.HandleFunc("/iff/register", s.handleIFFRegister)   // POST: dost kaydet
	mux.HandleFunc("/iff/unregister", s.handleIFFUnregister) // POST: dost kaydını sil
	mux.HandleFunc("/stats", s.handleFullStats)            // GET: tüm istatistikler

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, map[string]any{
		"status": "ok",
		"module": "TANI",
		"uptime": time.Since(s.startTime).String(),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	ds := s.detector.GetStats()
	cs := s.classifier.GetStats()
	ts := s.tracker.GetStats()

	s.json(w, 200, map[string]any{
		"module":           "TANI",
		"description":      "Nesne Tespit ve Sınıflandırma — GÖRGÜ Kümesi",
		"uptime":           time.Since(s.startTime).String(),
		"request_count":    atomic.LoadInt64(&s.reqCount),
		"detector_stats":   ds,
		"classifier_stats": cs,
		"tracker_stats":    ts,
	})
}

// ── Tespit ──

type detectReq struct {
	FrameID  string `json:"frame_id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	SourceID string `json:"source_id"`
}

func (s *Server) handleDetect(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]any{"error": "POST only"})
		return
	}

	var req detectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]any{"error": "geçersiz JSON: " + err.Error()})
		return
	}

	if req.FrameID == "" {
		req.FrameID = fmt.Sprintf("frame-%d", time.Now().UnixNano())
	}
	if req.Width <= 0 {
		req.Width = 640
	}
	if req.Height <= 0 {
		req.Height = 640
	}

	frame := detector.Frame{
		ID:        req.FrameID,
		Width:     req.Width,
		Height:    req.Height,
		Timestamp: time.Now(),
		SourceID:  req.SourceID,
	}

	result := s.detector.Detect(frame)
	s.json(w, 200, result)
}

func (s *Server) handleDetectLast(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	last := s.detector.LastResult()
	if last == nil {
		s.json(w, 404, map[string]any{"error": "henüz tespit yapılmadı"})
		return
	}
	s.json(w, 200, last)
}

// ── Sınıflandırma ──

type classifyReq struct {
	Detections []detector.Detection `json:"detections"`
}

func (s *Server) handleClassify(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]any{"error": "POST only"})
		return
	}

	var req classifyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]any{"error": "geçersiz JSON: " + err.Error()})
		return
	}

	results := s.classifier.ClassifyBatch(req.Detections)
	s.json(w, 200, map[string]any{
		"classified": results,
		"count":      len(results),
	})
}

func (s *Server) handleClassifyStats(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	s.json(w, 200, s.classifier.GetStats())
}

// ── İzleme ──

func (s *Server) handleTracks(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	tracks := s.tracker.ActiveTracks()
	s.json(w, 200, map[string]any{"tracks": tracks, "count": len(tracks)})
}

func (s *Server) handleHostile(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	tracks := s.tracker.HostileTracks()
	s.json(w, 200, map[string]any{"hostile_tracks": tracks, "count": len(tracks)})
}

func (s *Server) handleTrackByID(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	id := strings.TrimPrefix(r.URL.Path, "/tracks/")
	if id == "" || id == "stats" || id == "hostile" {
		s.json(w, 400, map[string]any{"error": "track id gerekli"})
		return
	}

	snap, ok := s.tracker.GetTrack(id)
	if !ok {
		s.json(w, 404, map[string]any{"error": "iz bulunamadı: " + id})
		return
	}
	s.json(w, 200, snap)
}

func (s *Server) handleTrackStats(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	s.json(w, 200, s.tracker.GetStats())
}

// ── Tam Pipeline ──

type processReq struct {
	FrameID  string `json:"frame_id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	SourceID string `json:"source_id"`
}

type processResp struct {
	FrameID      string                        `json:"frame_id"`
	Detections   []detector.Detection          `json:"detections"`
	Classified   []classifier.ClassifiedObject `json:"classified"`
	Tracks       []tracker.TrackSnapshot       `json:"tracks"`
	DetectCount  int                           `json:"detect_count"`
	TrackCount   int                           `json:"track_count"`
	InferenceMs  float64                       `json:"inference_ms"`
	TotalMs      float64                       `json:"total_ms"`
}

func (s *Server) handleProcess(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]any{"error": "POST only"})
		return
	}
	totalStart := time.Now()

	var req processReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]any{"error": "geçersiz JSON: " + err.Error()})
		return
	}
	if req.FrameID == "" {
		req.FrameID = fmt.Sprintf("frame-%d", time.Now().UnixNano())
	}
	if req.Width <= 0 {
		req.Width = 640
	}
	if req.Height <= 0 {
		req.Height = 640
	}

	// 1. Tespit
	frame := detector.Frame{
		ID:        req.FrameID,
		Width:     req.Width,
		Height:    req.Height,
		Timestamp: time.Now(),
		SourceID:  req.SourceID,
	}
	detResult := s.detector.Detect(frame)

	// 2. Sınıflandırma
	classified := s.classifier.ClassifyBatch(detResult.Detections)

	// 3. İzleme
	tracks := s.tracker.ProcessFrame(detResult.Detections, classified)

	totalMs := float64(time.Since(totalStart).Microseconds()) / 1000.0

	s.json(w, 200, processResp{
		FrameID:     req.FrameID,
		Detections:  detResult.Detections,
		Classified:  classified,
		Tracks:      tracks,
		DetectCount: detResult.Count,
		TrackCount:  len(tracks),
		InferenceMs: detResult.InferenceMs,
		TotalMs:     totalMs,
	})
}

// ── IFF ──

type iffReq struct {
	ID string `json:"id"`
}

func (s *Server) handleIFFRegister(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]any{"error": "POST only"})
		return
	}
	var req iffReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]any{"error": "geçersiz JSON"})
		return
	}
	if req.ID == "" {
		s.json(w, 400, map[string]any{"error": "id zorunlu"})
		return
	}
	s.classifier.RegisterFriendly(req.ID)
	s.json(w, 200, map[string]any{"message": "dost kaydedildi", "id": req.ID})
}

func (s *Server) handleIFFUnregister(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]any{"error": "POST only"})
		return
	}
	var req iffReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]any{"error": "geçersiz JSON"})
		return
	}
	s.classifier.UnregisterFriendly(req.ID)
	s.json(w, 200, map[string]any{"message": "dost kaydı silindi", "id": req.ID})
}

// ── Toplu İstatistik ──

func (s *Server) handleFullStats(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	s.json(w, 200, map[string]any{
		"detector":   s.detector.GetStats(),
		"classifier": s.classifier.GetStats(),
		"tracker":    s.tracker.GetStats(),
		"requests":   atomic.LoadInt64(&s.reqCount),
		"uptime":     time.Since(s.startTime).String(),
	})
}

// ── Helpers ──

func (s *Server) json(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		fmt.Fprintf(w, `{"error":"encoding: %s"}`, err)
	}
}
