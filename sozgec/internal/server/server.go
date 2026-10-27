// Package server — SÖZGEÇ HTTP API sunucusu.
package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/otukenmatris/sozgec/internal/config"
	"github.com/otukenmatris/sozgec/internal/filter"
	"github.com/otukenmatris/sozgec/internal/spectrum"
)

// Server — SÖZGEÇ HTTP sunucusu.
type Server struct {
	spectrum  *spectrum.Engine
	filter    *filter.Engine
	cfg       config.Config
	startTime time.Time
	mux       *http.ServeMux
}

// New — yeni HTTP sunucu.
func New(spec *spectrum.Engine, filt *filter.Engine, cfg config.Config) *Server {
	s := &Server{
		spectrum:  spec,
		filter:    filt,
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

	// spektral analiz
	s.mux.HandleFunc("/analyze", s.handleAnalyze)
	s.mux.HandleFunc("/bands", s.handleBands)
	s.mux.HandleFunc("/bands/", s.handleBandByID)

	// filtreleme
	s.mux.HandleFunc("/signals", s.handleSignals)
	s.mux.HandleFunc("/threats", s.handleThreats)
	s.mux.HandleFunc("/friendly", s.handleFriendly)

	// dost frekans bantları yönetimi
	s.mux.HandleFunc("/friendly-bands", s.handleFriendlyBands)

	// simülasyon
	s.mux.HandleFunc("/process", s.handleProcess)

	// bakım
	s.mux.HandleFunc("/prune", s.handlePrune)
}

// ── Sağlık & Durum ──

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, 200, map[string]any{
		"status": "ok", "module": "sozgec",
		"uptime": time.Since(s.startTime).String(), "version": "1.0.0",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, 200, map[string]any{
		"module": "sozgec", "status": "operational",
		"uptime":   time.Since(s.startTime).String(),
		"spectrum": s.spectrum.Stats(),
		"filter":   s.filter.Stats(),
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, 200, map[string]any{
		"spectrum": s.spectrum.Stats(),
		"filter":   s.filter.Stats(),
	})
}

// ── Spektral Analiz ──

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, 405, map[string]string{"error": "POST gerekli"})
		return
	}
	var req struct {
		Signals []spectrum.RawSignal `json:"signals"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]string{"error": "geçersiz JSON: " + err.Error()})
		return
	}
	result := s.spectrum.Analyze(req.Signals)
	respondJSON(w, 200, result)
}

func (s *Server) handleBands(w http.ResponseWriter, r *http.Request) {
	bands := s.spectrum.GetAllBands()
	if bands == nil {
		bands = []spectrum.FrequencyBand{}
	}
	respondJSON(w, 200, map[string]any{"bands": bands, "count": len(bands)})
}

func (s *Server) handleBandByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/bands/")
	if id == "" {
		respondJSON(w, 400, map[string]string{"error": "id gerekli"})
		return
	}
	band, ok := s.spectrum.GetBand(id)
	if !ok {
		respondJSON(w, 404, map[string]string{"error": "bant bulunamadı"})
		return
	}
	respondJSON(w, 200, band)
}

// ── Filtreleme ──

func (s *Server) handleSignals(w http.ResponseWriter, r *http.Request) {
	signals := s.filter.GetAllSignals()
	if signals == nil {
		signals = []filter.FilteredSignal{}
	}
	respondJSON(w, 200, map[string]any{"signals": signals, "count": len(signals)})
}

func (s *Server) handleThreats(w http.ResponseWriter, r *http.Request) {
	threats := s.filter.GetHighThreat()
	if threats == nil {
		threats = []filter.FilteredSignal{}
	}
	respondJSON(w, 200, map[string]any{"threats": threats, "count": len(threats)})
}

func (s *Server) handleFriendly(w http.ResponseWriter, r *http.Request) {
	friendly := s.filter.GetFriendlySignals()
	if friendly == nil {
		friendly = []filter.FilteredSignal{}
	}
	respondJSON(w, 200, map[string]any{"friendly": friendly, "count": len(friendly)})
}

// ── Dost Frekans Bantları ──

func (s *Server) handleFriendlyBands(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		bands := s.filter.GetFriendlyBands()
		respondJSON(w, 200, map[string]any{"friendly_bands": bands, "count": len(bands)})
	case http.MethodPost:
		var band filter.FriendlyBandDef
		if err := json.NewDecoder(r.Body).Decode(&band); err != nil {
			respondJSON(w, 400, map[string]string{"error": "geçersiz JSON"})
			return
		}
		if err := s.filter.AddFriendlyBand(band); err != nil {
			respondJSON(w, 409, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, 201, map[string]string{"status": "created", "id": band.ID})
	case http.MethodDelete:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSON(w, 400, map[string]string{"error": "geçersiz JSON"})
			return
		}
		if err := s.filter.RemoveFriendlyBand(req.ID); err != nil {
			respondJSON(w, 404, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, 200, map[string]string{"status": "deleted", "id": req.ID})
	default:
		respondJSON(w, 405, map[string]string{"error": "GET, POST veya DELETE gerekli"})
	}
}

// ── Simülasyon Entegrasyonu ──

func (s *Server) handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, 405, map[string]string{"error": "POST gerekli"})
		return
	}
	var req struct {
		Entities []map[string]any `json:"entities"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]string{"error": "geçersiz JSON"})
		return
	}

	// 1. varlıklardan sentetik RF sinyalleri üret
	rawSignals := filter.GenerateSignalsFromEntities(req.Entities)

	// 2. spektral analiz (FFT + pik tespiti)
	spectralResult := s.spectrum.Analyze(rawSignals)

	// 3. DSP filtreleme (gürültü bastırma + dost/düşman ayrımı)
	filterResult := s.filter.Filter(spectralResult.Bands, spectralResult.Spectrum.NoiseFloor)

	specStats := s.spectrum.Stats()
	filtStats := s.filter.Stats()

	respondJSON(w, 200, map[string]any{
		"raw_signals":      len(rawSignals),
		"frequency_bands":  len(spectralResult.Bands),
		"extracted_signals": filterResult.Extracted,
		"noise_filtered":   filterResult.NoiseFiltered,
		"friendly_count":   filterResult.FriendlyCount,
		"hostile_count":    filterResult.HostileCount,
		"jamming_detected": filterResult.JammingDetected,
		"high_threats":     filtStats["high_threats"],
		"avg_snr_db":       filterResult.AvgSNR,
		"active_bands":     specStats["active_bands"],
		"elapsed_ms":       spectralResult.ElapsedMs + filterResult.ElapsedMs,
	})
}

// ── Bakım ──

func (s *Server) handlePrune(w http.ResponseWriter, r *http.Request) {
	bandsPruned := s.spectrum.Prune(s.cfg.Filter.ThreatDecayS)
	signalsPruned := s.filter.Prune()
	respondJSON(w, 200, map[string]any{
		"bands_pruned":   bandsPruned,
		"signals_pruned": signalsPruned,
	})
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
