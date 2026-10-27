// Package server — KÖZ HTTP REST API.
//
// Termal iz tespit, analiz ve sorgulama servislerini HTTP üzerinden sunar.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/koz/internal/analyzer"
	"github.com/otukenmatris/koz/internal/thermal"
)

// Server — KÖZ HTTP sunucusu.
type Server struct {
	engine    *thermal.Engine
	analyzer  *analyzer.Analyzer
	startTime time.Time
	reqCount  atomic.Int64
}

// New — yeni sunucu oluştur.
func New(engine *thermal.Engine, az *analyzer.Analyzer) *Server {
	return &Server{
		engine:    engine,
		analyzer:  az,
		startTime: time.Now(),
	}
}

// Handler — HTTP route'larını döndür.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)

	// termal tarama
	mux.HandleFunc("/scan", s.handleScan)

	// imza sorgu
	mux.HandleFunc("/signatures", s.handleSignatures)
	mux.HandleFunc("/signatures/hot", s.handleHotSignatures)
	mux.HandleFunc("/signatures/", s.handleSignatureByID)

	// analiz
	mux.HandleFunc("/analyze/clusters", s.handleClusters)
	mux.HandleFunc("/analyze/cooling", s.handleCooling)
	mux.HandleFunc("/analyze/anomalies", s.handleAnomalies)

	// tam boru hattı: tarama → tespit → analiz
	mux.HandleFunc("/process", s.handleProcess)

	// yapılandırma
	mux.HandleFunc("/ambient", s.handleAmbient)

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
		"module": "KÖZ",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, map[string]any{
		"status":    "ok",
		"module":    "KÖZ",
		"version":   "1.0.0",
		"uptime_s":  time.Since(s.startTime).Seconds(),
		"requests":  s.reqCount.Load(),
		"thermal":   s.engine.Stats(),
		"analyzer":  s.analyzer.Stats(),
	})
}

// ── Tarama ──

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.json(w, 405, errMsg("POST gerekli"))
		return
	}
	var entities []thermal.EntityInput
	if err := json.NewDecoder(r.Body).Decode(&entities); err != nil {
		s.json(w, 400, errMsg("JSON parse hatası: "+err.Error()))
		return
	}
	result := s.engine.Scan(entities)
	s.json(w, 200, result)
}

// ── İmza Sorgu ──

func (s *Server) handleSignatures(w http.ResponseWriter, r *http.Request) {
	sigs := s.engine.GetAllSignatures()
	s.json(w, 200, map[string]any{
		"count":      len(sigs),
		"signatures": sigs,
	})
}

func (s *Server) handleHotSignatures(w http.ResponseWriter, r *http.Request) {
	minDelta := 20.0
	if v := r.URL.Query().Get("min_delta"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			minDelta = f
		}
	}
	sigs := s.engine.GetHotSignatures(minDelta)
	s.json(w, 200, map[string]any{
		"min_delta":  minDelta,
		"count":      len(sigs),
		"signatures": sigs,
	})
}

func (s *Server) handleSignatureByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/signatures/")
	if id == "" {
		s.json(w, 400, errMsg("imza ID gerekli"))
		return
	}
	sig, ok := s.engine.GetSignature(id)
	if !ok {
		s.json(w, 404, errMsg("imza bulunamadı: "+id))
		return
	}
	s.json(w, 200, sig)
}

// ── Analiz ──

func (s *Server) handleClusters(w http.ResponseWriter, r *http.Request) {
	sigs := s.engine.GetAllSignatures()
	clusters := s.analyzer.ClusterSignatures(sigs)
	s.json(w, 200, map[string]any{
		"count":    len(clusters),
		"clusters": clusters,
	})
}

func (s *Server) handleCooling(w http.ResponseWriter, r *http.Request) {
	sigs := s.engine.GetAllSignatures()
	stats := s.engine.Stats()
	ambient := stats["ambient_c"].(float64)
	predictions := s.analyzer.PredictCooling(sigs, ambient)
	s.json(w, 200, map[string]any{
		"ambient_c":   ambient,
		"count":       len(predictions),
		"predictions": predictions,
	})
}

func (s *Server) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		n := 50
		if v := r.URL.Query().Get("n"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				n = i
			}
		}
		anoms := s.analyzer.GetAnomalies(n)
		s.json(w, 200, map[string]any{
			"count":     len(anoms),
			"anomalies": anoms,
		})
	case http.MethodPost:
		// tetikleme: mevcut imzalar üzerinden anomali analizi
		sigs := s.engine.GetAllSignatures()
		anoms := s.analyzer.DetectAnomalies(sigs)
		s.json(w, 200, map[string]any{
			"detected": len(anoms),
			"anomalies": anoms,
		})
	default:
		s.json(w, 405, errMsg("GET veya POST"))
	}
}

// ── Tam Boru Hattı ──

func (s *Server) handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.json(w, 405, errMsg("POST gerekli"))
		return
	}
	var entities []thermal.EntityInput
	if err := json.NewDecoder(r.Body).Decode(&entities); err != nil {
		s.json(w, 400, errMsg("JSON parse hatası: "+err.Error()))
		return
	}

	start := time.Now()

	// 1. Termal tarama
	scanResult := s.engine.Scan(entities)

	// 2. Kümeleme
	clusters := s.analyzer.ClusterSignatures(scanResult.Signatures)

	// 3. Soğuma tahmini
	stats := s.engine.Stats()
	ambient := stats["ambient_c"].(float64)
	cooling := s.analyzer.PredictCooling(scanResult.Signatures, ambient)

	// 4. Anomali tespiti
	anomalies := s.analyzer.DetectAnomalies(scanResult.Signatures)

	s.json(w, 200, map[string]any{
		"scan": map[string]any{
			"signatures": len(scanResult.Signatures),
			"hot_pixels": scanResult.HotPixels,
			"elapsed_ms": scanResult.ElapsedMs,
		},
		"clusters":    clusters,
		"cooling":     cooling,
		"anomalies":   anomalies,
		"total_ms":    float64(time.Since(start).Microseconds()) / 1000.0,
		"pipeline":    fmt.Sprintf("scan(%d) → cluster(%d) → cool(%d) → anomaly(%d)", len(scanResult.Signatures), len(clusters), len(cooling), len(anomalies)),
	})
}

// ── Yapılandırma ──

func (s *Server) handleAmbient(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		stats := s.engine.Stats()
		s.json(w, 200, map[string]any{"ambient_c": stats["ambient_c"]})
	case http.MethodPut:
		var body struct {
			TempC float64 `json:"temp_c"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			s.json(w, 400, errMsg(err.Error()))
			return
		}
		s.engine.SetAmbient(body.TempC)
		s.json(w, 200, map[string]any{"ambient_c": body.TempC, "updated": true})
	default:
		s.json(w, 405, errMsg("GET veya PUT"))
	}
}

// ── İstatistik ──

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, map[string]any{
		"thermal":  s.engine.Stats(),
		"analyzer": s.analyzer.Stats(),
		"requests": s.reqCount.Load(),
		"uptime_s": time.Since(s.startTime).Seconds(),
	})
}
