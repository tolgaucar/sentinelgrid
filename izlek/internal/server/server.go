// Package server — İZLEK HTTP sunucusu.
package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/otukenmatris/izlek/internal/predictor"
	"github.com/otukenmatris/izlek/internal/tracker"
)

// Server — HTTP sunucusu
type Server struct {
	tracker   *tracker.Manager
	predictor *predictor.Engine
	mux       *http.ServeMux
	startTime time.Time
}

// New — yeni sunucu oluşturur.
func New(tm *tracker.Manager, pe *predictor.Engine) *Server {
	s := &Server{
		tracker:   tm,
		predictor: pe,
		mux:       http.NewServeMux(),
		startTime: time.Now(),
	}
	s.routes()
	return s
}

// Handler — HTTP handler döndürür.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/stats", s.handleStats)

	s.mux.HandleFunc("/tracks", s.handleTracks)
	s.mux.HandleFunc("/tracks/update", s.handleTracksUpdate)
	s.mux.HandleFunc("/tracks/", s.handleTrackByID)

	s.mux.HandleFunc("/predict", s.handlePredict)
	s.mux.HandleFunc("/predict/batch", s.handlePredictBatch)
	s.mux.HandleFunc("/intercept", s.handleIntercept)
}

// ── Sağlık & İstatistik ──

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"status":  "ok",
		"module":  "İZLEK",
		"uptime":  time.Since(s.startTime).Seconds(),
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"tracker":   s.tracker.Stats(),
		"predictor": s.predictor.Stats(),
		"uptime":    time.Since(s.startTime).Seconds(),
	})
}

// ── İz Yönetimi ──

func (s *Server) handleTracks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tracks := s.tracker.List()
		writeJSON(w, 200, map[string]interface{}{
			"tracks": tracks,
			"count":  len(tracks),
		})
	case http.MethodDelete:
		n := s.tracker.Clear()
		writeJSON(w, 200, map[string]interface{}{
			"cleared": n,
		})
	default:
		writeJSON(w, 405, errMsg("GET veya DELETE"))
	}
}

func (s *Server) handleTracksUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var entities []tracker.EntityUpdate
	if err := json.NewDecoder(r.Body).Decode(&entities); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	updated, newTracks := s.tracker.Update(entities)
	writeJSON(w, 200, map[string]interface{}{
		"updated":    updated,
		"new_tracks": newTracks,
	})
}

func (s *Server) handleTrackByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, errMsg("GET gerekli"))
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/tracks/")
	if id == "" {
		writeJSON(w, 400, errMsg("id gerekli"))
		return
	}
	track, ok := s.tracker.Get(id)
	if !ok {
		writeJSON(w, 404, errMsg("iz bulunamadı"))
		return
	}
	writeJSON(w, 200, track)
}

// ── Tahmin ──

func (s *Server) handlePredict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var req struct {
		EntityID   string  `json:"entity_id"`
		HorizonSec float64 `json:"horizon_sec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	track, ok := s.tracker.Get(req.EntityID)
	if !ok {
		writeJSON(w, 404, errMsg("iz bulunamadı: "+req.EntityID))
		return
	}
	if len(track.Points) < 2 {
		writeJSON(w, 400, errMsg("tahmin için en az 2 konum noktası gerekli"))
		return
	}
	pred := s.predictor.Predict(track, req.HorizonSec)
	writeJSON(w, 200, pred)
}

func (s *Server) handlePredictBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var req struct {
		EntityIDs  []string `json:"entity_ids"`
		HorizonSec float64  `json:"horizon_sec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}

	predictions := make([]predictor.Prediction, 0, len(req.EntityIDs))
	errors := make([]string, 0)

	for _, eid := range req.EntityIDs {
		track, ok := s.tracker.Get(eid)
		if !ok {
			errors = append(errors, "iz bulunamadı: "+eid)
			continue
		}
		if len(track.Points) < 2 {
			errors = append(errors, "yetersiz veri: "+eid)
			continue
		}
		pred := s.predictor.Predict(track, req.HorizonSec)
		predictions = append(predictions, pred)
	}

	writeJSON(w, 200, map[string]interface{}{
		"predictions": predictions,
		"count":       len(predictions),
		"errors":      errors,
	})
}

// ── Önleme Atışı ──

func (s *Server) handleIntercept(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var req struct {
		EntityID        string  `json:"entity_id"`
		ProjectileSpeed float64 `json:"projectile_speed"` // m/s
		LauncherLat     float64 `json:"launcher_lat"`
		LauncherLon     float64 `json:"launcher_lon"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	track, ok := s.tracker.Get(req.EntityID)
	if !ok {
		writeJSON(w, 404, errMsg("iz bulunamadı: "+req.EntityID))
		return
	}
	if len(track.Points) < 2 {
		writeJSON(w, 400, errMsg("tahmin için en az 2 konum noktası gerekli"))
		return
	}
	result := s.predictor.Intercept(track, req.ProjectileSpeed, req.LauncherLat, req.LauncherLon)
	writeJSON(w, 200, result)
}

// ── Yardımcı ──

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errMsg(msg string) map[string]string {
	return map[string]string{"error": msg}
}
