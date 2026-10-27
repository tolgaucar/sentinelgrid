// Package server — YANKI HTTP API sunucusu.
package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/otukenmatris/yanki/internal/acoustic"
	"github.com/otukenmatris/yanki/internal/config"
)

// Server — YANKI HTTP sunucusu.
type Server struct {
	engine    *acoustic.Engine
	cfg       config.Config
	startTime time.Time
	mux       *http.ServeMux
}

// New — yeni HTTP sunucu oluştur.
func New(engine *acoustic.Engine, cfg config.Config) *Server {
	s := &Server{
		engine:    engine,
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
	s.mux.HandleFunc("/detect", s.handleDetect)
	s.mux.HandleFunc("/locate", s.handleLocate)
	s.mux.HandleFunc("/detections", s.handleDetections)
	s.mux.HandleFunc("/detections/", s.handleDetectionByID)
	s.mux.HandleFunc("/threats", s.handleThreats)
	s.mux.HandleFunc("/gunshots", s.handleGunshots)
	s.mux.HandleFunc("/process", s.handleProcess)
	s.mux.HandleFunc("/stats", s.handleStats)
	s.mux.HandleFunc("/prune", s.handlePrune)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"module":  "yanki",
		"uptime":  time.Since(s.startTime).String(),
		"version": "1.0.0",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	stats := s.engine.Stats()
	respondJSON(w, http.StatusOK, map[string]any{
		"module":  "yanki",
		"status":  "operational",
		"uptime":  time.Since(s.startTime).String(),
		"stats":   stats,
		"config": map[string]any{
			"sample_rate":      s.cfg.Acoustic.SampleRate,
			"fft_size":         s.cfg.Acoustic.FFTSize,
			"min_amplitude_db": s.cfg.Acoustic.MinAmplitudeDB,
			"max_range_m":      s.cfg.Acoustic.MaxRange,
			"speed_of_sound":   s.cfg.Locator.SpeedOfSound,
			"min_sensors":      s.cfg.Locator.MinSensors,
		},
	})
}

// handleDetect — sensör okumalarından akustik olay tespiti.
func (s *Server) handleDetect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req struct {
		Readings []acoustic.SensorReading `json:"readings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	result := s.engine.Detect(req.Readings)
	respondJSON(w, http.StatusOK, result)
}

// handleLocate — TOA üçgenlemesi ile konumlandırma.
func (s *Server) handleLocate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req struct {
		TOAs []acoustic.TOAMeasurement `json:"toas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	result := s.engine.Locate(req.TOAs)
	if result == nil {
		respondJSON(w, http.StatusOK, map[string]string{"status": "insufficient_sensors", "min_required": "3"})
		return
	}
	respondJSON(w, http.StatusOK, result)
}

// handleDetections — tüm aktif tespitler.
func (s *Server) handleDetections(w http.ResponseWriter, r *http.Request) {
	eventType := r.URL.Query().Get("type")
	var detections []acoustic.Detection
	if eventType != "" {
		detections = s.engine.GetByType(acoustic.EventType(eventType))
	} else {
		detections = s.engine.GetAllDetections()
	}
	if detections == nil {
		detections = []acoustic.Detection{}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"detections": detections,
		"count":      len(detections),
	})
}

// handleDetectionByID — ID ile tek tespit.
func (s *Server) handleDetectionByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/detections/"):]
	if id == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	det, ok := s.engine.GetDetection(id)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "detection not found"})
		return
	}
	respondJSON(w, http.StatusOK, det)
}

// handleThreats — yüksek ve kritik tehdit tespitleri.
func (s *Server) handleThreats(w http.ResponseWriter, r *http.Request) {
	threats := s.engine.GetHighThreat()
	if threats == nil {
		threats = []acoustic.Detection{}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"threats": threats,
		"count":   len(threats),
	})
}

// handleGunshots — silah atışı tespitleri.
func (s *Server) handleGunshots(w http.ResponseWriter, r *http.Request) {
	gunshots := s.engine.GetGunshots()
	if gunshots == nil {
		gunshots = []acoustic.Detection{}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"gunshots": gunshots,
		"count":    len(gunshots),
	})
}

// handleProcess — toplu veri işleme (simülasyon entegrasyonu).
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

	// entity listesinden akustik sensör okumaları oluştur
	var readings []acoustic.SensorReading
	for _, ent := range req.Entities {
		id, _ := ent["id"].(string)
		lat, _ := ent["lat"].(float64)
		lon, _ := ent["lon"].(float64)
		freq, _ := ent["frequency_hz"].(float64)
		amp, _ := ent["amplitude_db"].(float64)
		dir, _ := ent["direction_deg"].(float64)
		dur, _ := ent["duration_ms"].(float64)
		sig, _ := ent["signature_type"].(string)
		conf, _ := ent["confidence"].(float64)
		rangeKm, _ := ent["range_est_km"].(float64)

		if freq == 0 { freq = 500 }
		if amp == 0 { amp = 60 }
		if conf == 0 { conf = 0.5 }

		readings = append(readings, acoustic.SensorReading{
			SensorID:      id,
			Lat:           lat,
			Lon:           lon,
			FrequencyHz:   freq,
			AmplitudeDB:   amp,
			DirectionDeg:  dir,
			DurationMs:    int(dur),
			SignatureType:  sig,
			Confidence:    conf,
			RangeEstKm:    rangeKm,
			TimestampNs:   time.Now().UnixNano(),
		})
	}

	result := s.engine.Detect(readings)
	stats := s.engine.Stats()

	respondJSON(w, http.StatusOK, map[string]any{
		"detections":       result.Detections,
		"total_inputs":     result.TotalInputs,
		"classified":       result.Classified,
		"localized":        result.Localized,
		"elapsed_ms":       result.ElapsedMs,
		"active_detections": stats["active_detections"],
		"detect_count":     stats["detect_count"],
	})
}

// handleStats — motor istatistikleri.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, s.engine.Stats())
}

// handlePrune — süresi dolmuş tespitleri temizle.
func (s *Server) handlePrune(w http.ResponseWriter, r *http.Request) {
	removed := s.engine.Prune()
	respondJSON(w, http.StatusOK, map[string]any{
		"removed":   removed,
		"remaining": len(s.engine.GetAllDetections()),
	})
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
