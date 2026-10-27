// Package server — SEZİ HTTP REST API.
//
// Doku analizi ile kamuflaj tespit, sorgulama ve tarama
// servislerini HTTP üzerinden sunar.
package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/sezi/internal/detector"
	"github.com/otukenmatris/sezi/internal/texture"
)

// Server — SEZİ HTTP sunucusu.
type Server struct {
	texEngine *texture.Engine
	detector  *detector.Engine
	startTime time.Time
	reqCount  atomic.Int64
}

// New — yeni sunucu oluştur.
func New(texEngine *texture.Engine, det *detector.Engine) *Server {
	return &Server{
		texEngine: texEngine,
		detector:  det,
		startTime: time.Now(),
	}
}

// Handler — HTTP route'larını döndür.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)

	// doku analizi
	mux.HandleFunc("/analyze", s.handleAnalyze)

	// kamuflaj tarama
	mux.HandleFunc("/scan", s.handleScan)

	// tespit sorgu
	mux.HandleFunc("/detections", s.handleDetections)
	mux.HandleFunc("/detections/threats", s.handleThreats)
	mux.HandleFunc("/detections/", s.handleDetectionByID)

	// tam boru hattı: analiz → tespit → değerlendirme
	mux.HandleFunc("/process", s.handleProcess)

	// arazi profilleri
	mux.HandleFunc("/terrains", s.handleTerrains)

	mux.HandleFunc("/stats", s.handleStats)

	return mux
}

func (s *Server) json(w http.ResponseWriter, code int, data any) {
	s.reqCount.Add(1)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func errMsg(msg string) map[string]string { return map[string]string{"error": msg} }

// ── Health & Status ──

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, map[string]any{
		"status": "ok",
		"module": "SEZİ",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, map[string]any{
		"status":   "ok",
		"module":   "SEZİ",
		"version":  "1.0.0",
		"uptime_s": time.Since(s.startTime).Seconds(),
		"requests": s.reqCount.Load(),
		"texture":  s.texEngine.Stats(),
		"detector": s.detector.Stats(),
	})
}

// ── Doku Analizi ──

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.json(w, 405, errMsg("POST gerekli"))
		return
	}
	var regions []texture.RegionInput
	if err := json.NewDecoder(r.Body).Decode(&regions); err != nil {
		s.json(w, 400, errMsg("JSON parse hatası: "+err.Error()))
		return
	}
	results := s.texEngine.Analyze(regions)
	s.json(w, 200, map[string]any{
		"count":   len(results),
		"results": results,
	})
}

// ── Kamuflaj Tarama ──

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.json(w, 405, errMsg("POST gerekli"))
		return
	}
	var inputs []detector.ScanInput
	if err := json.NewDecoder(r.Body).Decode(&inputs); err != nil {
		s.json(w, 400, errMsg("JSON parse hatası: "+err.Error()))
		return
	}
	result := s.detector.Scan(inputs)
	s.json(w, 200, result)
}

// ── Tespit Sorgu ──

func (s *Server) handleDetections(w http.ResponseWriter, r *http.Request) {
	dets := s.detector.GetAllDetections()
	s.json(w, 200, map[string]any{
		"count":      len(dets),
		"detections": dets,
	})
}

func (s *Server) handleThreats(w http.ResponseWriter, r *http.Request) {
	level := r.URL.Query().Get("level")
	var dets []detector.Detection
	switch level {
	case "critical":
		dets = s.detector.GetByThreat(detector.ThreatCritical)
	case "high":
		dets = s.detector.GetHighThreat()
	case "medium":
		dets = s.detector.GetByThreat(detector.ThreatMedium)
	case "low":
		dets = s.detector.GetByThreat(detector.ThreatLow)
	default:
		dets = s.detector.GetHighThreat()
	}
	s.json(w, 200, map[string]any{
		"level":      level,
		"count":      len(dets),
		"detections": dets,
	})
}

func (s *Server) handleDetectionByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/detections/")
	if id == "" || id == "threats" {
		s.json(w, 400, errMsg("tespit ID gerekli"))
		return
	}
	det, ok := s.detector.GetDetection(id)
	if !ok {
		s.json(w, 404, errMsg("tespit bulunamadı: "+id))
		return
	}
	s.json(w, 200, det)
}

// ── Tam Boru Hattı ──

func (s *Server) handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.json(w, 405, errMsg("POST gerekli"))
		return
	}
	var inputs []detector.ScanInput
	if err := json.NewDecoder(r.Body).Decode(&inputs); err != nil {
		s.json(w, 400, errMsg("JSON parse hatası: "+err.Error()))
		return
	}

	start := time.Now()

	// 1. Doku analizi
	regions := make([]texture.RegionInput, len(inputs))
	for i, inp := range inputs {
		regions[i] = texture.RegionInput{
			ID:      inp.ID,
			Lat:     inp.Lat,
			Lon:     inp.Lon,
			Terrain: inp.Terrain,
			Width:   inp.Width,
			Height:  inp.Height,
		}
	}
	analysisResults := s.texEngine.Analyze(regions)

	// 2. Kamuflaj tarama
	scanResult := s.detector.Scan(inputs)

	// 3. Yüksek tehdit tespitleri
	threats := s.detector.GetHighThreat()

	s.json(w, 200, map[string]any{
		"analysis": map[string]any{
			"regions":  len(analysisResults),
			"results":  analysisResults,
		},
		"scan": map[string]any{
			"detections": len(scanResult.Detections),
			"anomalies":  scanResult.Anomalies,
			"results":    scanResult.Detections,
		},
		"threats": map[string]any{
			"count": len(threats),
			"items": threats,
		},
		"total_ms": float64(time.Since(start).Microseconds()) / 1000.0,
		"pipeline": pipelineDesc(len(regions), len(scanResult.Detections), len(threats)),
	})
}

// ── Arazi Profilleri ──

func (s *Server) handleTerrains(w http.ResponseWriter, r *http.Request) {
	profiles := make([]texture.TextureProfile, 0, len(texture.TerrainProfiles))
	for _, p := range texture.TerrainProfiles {
		profiles = append(profiles, p)
	}
	s.json(w, 200, map[string]any{
		"count":    len(profiles),
		"terrains": profiles,
	})
}

// ── İstatistik ──

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, map[string]any{
		"texture":  s.texEngine.Stats(),
		"detector": s.detector.Stats(),
		"requests": s.reqCount.Load(),
		"uptime_s": time.Since(s.startTime).Seconds(),
	})
}

func pipelineDesc(regions, detections, threats int) string {
	return strconv.Itoa(regions) + " bölge → " +
		strconv.Itoa(detections) + " tespit → " +
		strconv.Itoa(threats) + " tehdit"
}
