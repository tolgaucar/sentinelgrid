// Package server — BAŞKALA HTTP API sunucusu.
package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/otukenmatris/baskala/internal/change"
	"github.com/otukenmatris/baskala/internal/config"
	"github.com/otukenmatris/baskala/internal/fusion"
	"github.com/otukenmatris/baskala/internal/imagery"
)

// Server — BAŞKALA HTTP sunucusu.
type Server struct {
	imagery   *imagery.Engine
	change    *change.Engine
	fusion    *fusion.Engine
	cfg       config.Config
	startTime time.Time
	mux       *http.ServeMux
}

// New — yeni HTTP sunucu.
func New(img *imagery.Engine, chg *change.Engine, fus *fusion.Engine, cfg config.Config) *Server {
	s := &Server{
		imagery:   img,
		change:    chg,
		fusion:    fus,
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

	// görüntü yönetimi
	s.mux.HandleFunc("/images", s.handleImages)
	s.mux.HandleFunc("/images/", s.handleImageByID)

	// karşılaştırma ve analiz
	s.mux.HandleFunc("/compare", s.handleCompare)
	s.mux.HandleFunc("/analyze", s.handleAnalyze)

	// değişimler
	s.mux.HandleFunc("/changes", s.handleChanges)
	s.mux.HandleFunc("/changes/", s.handleChangeByID)
	s.mux.HandleFunc("/threats", s.handleThreats)
	s.mux.HandleFunc("/timeline", s.handleTimeline)

	// simülasyon
	s.mux.HandleFunc("/process", s.handleProcess)

	// bakım
	s.mux.HandleFunc("/prune", s.handlePrune)
}

// ── Sağlık & Durum ──

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, 200, map[string]any{
		"status": "ok", "module": "baskala",
		"uptime": time.Since(s.startTime).String(), "version": "1.0.0",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	imgStats := s.imagery.Stats()
	chgStats := s.change.Stats()
	respondJSON(w, 200, map[string]any{
		"module": "baskala", "status": "operational",
		"uptime": time.Since(s.startTime).String(),
		"imagery": imgStats, "changes": chgStats,
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, 200, map[string]any{
		"imagery": s.imagery.Stats(),
		"changes": s.change.Stats(),
	})
}

// ── Görüntü Yönetimi ──

func (s *Server) handleImages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		images := s.imagery.GetAll()
		respondJSON(w, 200, map[string]any{"images": images, "count": len(images)})
	case http.MethodPost:
		var img imagery.SatelliteImage
		if err := json.NewDecoder(r.Body).Decode(&img); err != nil {
			respondJSON(w, 400, map[string]string{"error": "geçersiz JSON: " + err.Error()})
			return
		}
		if err := s.imagery.Ingest(img); err != nil {
			respondJSON(w, 409, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, 201, map[string]string{"status": "ingested", "id": img.ID})
	default:
		respondJSON(w, 405, map[string]string{"error": "GET veya POST gerekli"})
	}
}

func (s *Server) handleImageByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/images/")
	if id == "" {
		respondJSON(w, 400, map[string]string{"error": "id gerekli"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		img, ok := s.imagery.Get(id)
		if !ok {
			respondJSON(w, 404, map[string]string{"error": "görüntü bulunamadı"})
			return
		}
		// pikselleri döndürme, sadece metadata
		cp := *img
		cp.Pixels = nil
		respondJSON(w, 200, cp)
	case http.MethodDelete:
		if err := s.imagery.Delete(id); err != nil {
			respondJSON(w, 404, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, 200, map[string]string{"status": "deleted", "id": id})
	default:
		respondJSON(w, 405, map[string]string{"error": "GET veya DELETE gerekli"})
	}
}

// ── Karşılaştırma ve Analiz ──

func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, 405, map[string]string{"error": "POST gerekli"})
		return
	}
	var req struct {
		ImageA string `json:"image_a_id"`
		ImageB string `json:"image_b_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]string{"error": "geçersiz JSON: " + err.Error()})
		return
	}

	dm, err := s.imagery.ComputeDiff(req.ImageA, req.ImageB)
	if err != nil {
		respondJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	imgA, _ := s.imagery.Get(req.ImageA)
	imgB, _ := s.imagery.Get(req.ImageB)
	result := s.change.Compare(dm, imgA, imgB)
	hist := imagery.ComputeHistogram(dm)

	respondJSON(w, 200, map[string]any{
		"result":    result,
		"histogram": hist,
		"diff_summary": map[string]any{
			"mean_diff":  dm.MeanDiff,
			"max_diff":   dm.MaxDiff,
			"std_dev":    dm.StdDev,
			"change_pct": dm.ChangePct,
		},
	})
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, 405, map[string]string{"error": "POST gerekli"})
		return
	}
	var req struct {
		Lat      float64 `json:"lat"`
		Lon      float64 `json:"lon"`
		RadiusKm float64 `json:"radius_km"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]string{"error": "geçersiz JSON"})
		return
	}

	images := s.imagery.GetByRegion(req.Lat, req.Lon, req.RadiusKm)
	respondJSON(w, 200, map[string]any{
		"region_images": len(images),
		"images":        images,
		"lat":           req.Lat,
		"lon":           req.Lon,
		"radius_km":     req.RadiusKm,
	})
}

// ── Değişimler ──

func (s *Server) handleChanges(w http.ResponseWriter, r *http.Request) {
	changes := s.change.GetAll()
	if changes == nil {
		changes = []change.ChangeDetection{}
	}
	respondJSON(w, 200, map[string]any{"changes": changes, "count": len(changes)})
}

func (s *Server) handleChangeByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/changes/")
	if id == "" {
		respondJSON(w, 400, map[string]string{"error": "id gerekli"})
		return
	}
	det, ok := s.change.Get(id)
	if !ok {
		respondJSON(w, 404, map[string]string{"error": "değişim bulunamadı"})
		return
	}
	respondJSON(w, 200, det)
}

func (s *Server) handleThreats(w http.ResponseWriter, r *http.Request) {
	threats := s.change.GetHighThreat()
	if threats == nil {
		threats = []change.ChangeDetection{}
	}
	respondJSON(w, 200, map[string]any{"threats": threats, "count": len(threats)})
}

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	timeline := s.change.GetTimeline()
	if timeline == nil {
		timeline = []change.TimelineEntry{}
	}
	respondJSON(w, 200, map[string]any{"timeline": timeline, "count": len(timeline)})
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

	// 1. sentetik SAR görüntüsü üret
	sarImg := s.imagery.GenerateFromEntities(req.Entities, imagery.TypeSAR)
	s.imagery.Ingest(*sarImg)

	// 2. sentetik optik görüntüsü üret
	optImg := s.imagery.GenerateFromEntities(req.Entities, imagery.TypeOptical)
	s.imagery.Ingest(*optImg)

	// 3. önceki görüntülerle karşılaştır
	var sarDetections, optDetections []change.ChangeDetection
	allImages := s.imagery.GetAll()

	// son SAR çifti bul
	var prevSAR *imagery.SatelliteImage
	for i := len(allImages) - 1; i >= 0; i-- {
		if allImages[i].Type == imagery.TypeSAR && allImages[i].ID != sarImg.ID {
			full, ok := s.imagery.Get(allImages[i].ID)
			if ok {
				prevSAR = full
				break
			}
		}
	}

	// son optik çifti bul
	var prevOpt *imagery.SatelliteImage
	for i := len(allImages) - 1; i >= 0; i-- {
		if allImages[i].Type == imagery.TypeOptical && allImages[i].ID != optImg.ID {
			full, ok := s.imagery.Get(allImages[i].ID)
			if ok {
				prevOpt = full
				break
			}
		}
	}

	// SAR karşılaştırma
	if prevSAR != nil {
		dm, err := s.imagery.ComputeDiff(prevSAR.ID, sarImg.ID)
		if err == nil {
			result := s.change.Compare(dm, prevSAR, sarImg)
			sarDetections = result.Detections
		}
	}

	// optik karşılaştırma
	if prevOpt != nil {
		dm, err := s.imagery.ComputeDiff(prevOpt.ID, optImg.ID)
		if err == nil {
			result := s.change.Compare(dm, prevOpt, optImg)
			optDetections = result.Detections
		}
	}

	// 4. SAR+Optik füzyon
	fusionResult := s.fusion.Fuse(sarDetections, optDetections, optImg.CloudCover)

	imgStats := s.imagery.Stats()
	chgStats := s.change.Stats()

	respondJSON(w, 200, map[string]any{
		"images_generated":  2,
		"sar_image_id":      sarImg.ID,
		"optical_image_id":  optImg.ID,
		"sar_detections":    len(sarDetections),
		"optical_detections": len(optDetections),
		"fused_detections":  fusionResult.TotalFused,
		"cross_validated":   fusionResult.CrossValidated,
		"false_alarms_filtered": fusionResult.FalseAlarmFiltered,
		"high_threats":      chgStats["high_threats"],
		"total_images":      imgStats["total_images"],
		"detections":        fusionResult.Detections,
	})
}

// ── Bakım ──

func (s *Server) handlePrune(w http.ResponseWriter, r *http.Request) {
	imgRemoved := s.imagery.Prune(s.cfg.Imagery.RetentionHours)
	chgRemoved := s.change.Prune()
	respondJSON(w, 200, map[string]any{
		"images_removed":  imgRemoved,
		"changes_removed": chgRemoved,
	})
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
