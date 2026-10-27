// Package server — DUYUM HTTP API sunucusu.
package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/otukenmatris/duyum/internal/analyzer"
	"github.com/otukenmatris/duyum/internal/collector"
	"github.com/otukenmatris/duyum/internal/config"
)

// Server — DUYUM HTTP sunucusu.
type Server struct {
	collector *collector.Engine
	analyzer  *analyzer.Engine
	cfg       config.Config
	startTime time.Time
	mux       *http.ServeMux
}

// New — yeni HTTP sunucu oluştur.
func New(coll *collector.Engine, anal *analyzer.Engine, cfg config.Config) *Server {
	s := &Server{
		collector: coll,
		analyzer:  anal,
		cfg:       cfg,
		startTime: time.Now(),
		mux:       http.NewServeMux(),
	}
	s.routes()
	return s
}

// Handler — HTTP handler.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/status", s.handleStatus)
	s.mux.HandleFunc("/stats", s.handleStats)

	// kaynak yönetimi (CRUD)
	s.mux.HandleFunc("/sources", s.handleSources)
	s.mux.HandleFunc("/sources/", s.handleSourceByID)

	// veri toplama
	s.mux.HandleFunc("/collect", s.handleCollect)
	s.mux.HandleFunc("/collect/all", s.handleCollectAll)

	// analiz
	s.mux.HandleFunc("/analyze", s.handleAnalyze)

	// raporlar
	s.mux.HandleFunc("/reports", s.handleReports)
	s.mux.HandleFunc("/reports/", s.handleReportByID)
	s.mux.HandleFunc("/threats", s.handleThreats)
	s.mux.HandleFunc("/trends", s.handleTrends)

	// simülasyon entegrasyonu
	s.mux.HandleFunc("/process", s.handleProcess)

	// bakım
	s.mux.HandleFunc("/prune", s.handlePrune)
}

// ── Sağlık & Durum ──

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"module":  "duyum",
		"uptime":  time.Since(s.startTime).String(),
		"version": "1.0.0",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	collStats := s.collector.Stats()
	analStats := s.analyzer.Stats()
	respondJSON(w, http.StatusOK, map[string]any{
		"module":         "duyum",
		"status":         "operational",
		"uptime":         time.Since(s.startTime).String(),
		"collector":      collStats,
		"analyzer":       analStats,
		"source_count":   collStats["total_sources"],
		"active_reports": analStats["active_reports"],
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	collStats := s.collector.Stats()
	analStats := s.analyzer.Stats()
	respondJSON(w, http.StatusOK, map[string]any{
		"collector": collStats,
		"analyzer":  analStats,
	})
}

// ── Kaynak Yönetimi (CRUD) ──

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sources := s.collector.GetAllSources()
		respondJSON(w, http.StatusOK, map[string]any{
			"sources": sources,
			"count":   len(sources),
		})
	case http.MethodPost:
		var src collector.Source
		if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
			return
		}
		if err := s.collector.AddSource(src); err != nil {
			respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusCreated, map[string]string{"status": "created", "id": src.ID})
	default:
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET or POST required"})
	}
}

func (s *Server) handleSourceByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/sources/")
	if id == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		src, ok := s.collector.GetSource(id)
		if !ok {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "source not found"})
			return
		}
		respondJSON(w, http.StatusOK, src)
	case http.MethodPut:
		var update collector.Source
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
			return
		}
		if err := s.collector.UpdateSource(id, update); err != nil {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"status": "updated", "id": id})
	case http.MethodDelete:
		if err := s.collector.DeleteSource(id); err != nil {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
	default:
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET, PUT, or DELETE required"})
	}
}

// ── Veri Toplama ──

func (s *Server) handleCollect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req struct {
		SourceID string `json:"source_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	result, err := s.collector.Collect(req.SourceID)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, result)
}

func (s *Server) handleCollectAll(w http.ResponseWriter, r *http.Request) {
	results := s.collector.CollectAll()
	total := 0
	for _, r := range results {
		total += r.ItemCount
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"results":      results,
		"source_count": len(results),
		"total_items":  total,
	})
}

// ── Analiz ──

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req struct {
		Items []collector.SourceItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	result := s.analyzer.Analyze(req.Items)
	respondJSON(w, http.StatusOK, result)
}

// ── Raporlar ──

func (s *Server) handleReports(w http.ResponseWriter, r *http.Request) {
	reports := s.analyzer.GetAllReports()
	if reports == nil {
		reports = []analyzer.IntelReport{}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"reports": reports,
		"count":   len(reports),
	})
}

func (s *Server) handleReportByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/reports/")
	if id == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	report, ok := s.analyzer.GetReport(id)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "report not found"})
		return
	}
	respondJSON(w, http.StatusOK, report)
}

func (s *Server) handleThreats(w http.ResponseWriter, r *http.Request) {
	threats := s.analyzer.GetHighThreat()
	if threats == nil {
		threats = []analyzer.IntelReport{}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"threats": threats,
		"count":   len(threats),
	})
}

func (s *Server) handleTrends(w http.ResponseWriter, r *http.Request) {
	trends := s.analyzer.GetTrends()
	if trends == nil {
		trends = []analyzer.TrendInfo{}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"trends": trends,
		"count":  len(trends),
	})
}

// ── Simülasyon Entegrasyonu ──

func (s *Server) handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req struct {
		Entities []map[string]any `json:"entities"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	// 1. varlıklardan sentetik OSINT verisi topla
	items := s.collector.CollectFromEntities(req.Entities)

	// 2. toplanan veriyi analiz et
	result := s.analyzer.Analyze(items)

	// istatistikler
	collStats := s.collector.Stats()
	analStats := s.analyzer.Stats()

	respondJSON(w, http.StatusOK, map[string]any{
		"items_collected": len(items),
		"reports":         result.Reports,
		"reports_count":   result.ReportsGen,
		"entities_found":  result.EntitiesExt,
		"cross_referenced": result.CrossRefed,
		"high_threats":    analStats["high_threats"],
		"total_sources":   collStats["total_sources"],
		"elapsed_ms":      result.ElapsedMs,
	})
}

// ── Bakım ──

func (s *Server) handlePrune(w http.ResponseWriter, r *http.Request) {
	itemsRemoved := s.collector.Prune()
	reportsRemoved := s.analyzer.Prune()
	respondJSON(w, http.StatusOK, map[string]any{
		"items_removed":   itemsRemoved,
		"reports_removed": reportsRemoved,
	})
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
