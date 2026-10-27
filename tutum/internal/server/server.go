// Package server — TUTUM HTTP REST API.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/tutum/internal/alert"
	"github.com/otukenmatris/tutum/internal/battery"
	"github.com/otukenmatris/tutum/internal/predictor"
	"github.com/otukenmatris/tutum/internal/resource"
)

// Server TUTUM HTTP sunucusu.
type Server struct {
	monitor   *battery.Monitor
	tracker   *resource.Tracker
	predictor *predictor.Engine
	alerts    *alert.Manager
	startTime time.Time
	reqCount  int64

	// Config
	criticalPct float64
	warningPct  float64
	lowPct      float64
	tempWarnC   float64
	tempCritC   float64
}

// NewServer yeni sunucu oluşturur.
func NewServer(mon *battery.Monitor, trk *resource.Tracker, pred *predictor.Engine, alm *alert.Manager,
	criticalPct, warningPct, lowPct, tempWarn, tempCrit float64) *Server {
	return &Server{
		monitor:     mon,
		tracker:     trk,
		predictor:   pred,
		alerts:      alm,
		startTime:   time.Now(),
		criticalPct: criticalPct,
		warningPct:  warningPct,
		lowPct:      lowPct,
		tempWarnC:   tempWarn,
		tempCritC:   tempCrit,
	}
}

// Handler HTTP route tanımları.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Sistem
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)

	// Platform yönetimi
	mux.HandleFunc("/platforms", s.handlePlatforms)           // GET=list, POST=register
	mux.HandleFunc("/platforms/", s.handlePlatformByID)       // GET=detail, DELETE=remove

	// Batarya
	mux.HandleFunc("/battery/sample", s.handleBatterySample)     // POST: yeni ölçüm
	mux.HandleFunc("/battery/stats", s.handleBatteryStats)       // GET: toplu istatistik
	mux.HandleFunc("/battery/critical", s.handleBatteryCritical) // GET: kritik platformlar

	// Kaynak
	mux.HandleFunc("/resource/metric", s.handleResourceMetric) // POST: yeni ölçüm
	mux.HandleFunc("/resource/stats", s.handleResourceStats)   // GET: toplu istatistik

	// Tahmin
	mux.HandleFunc("/predict", s.handlePredict)                      // POST: tahmin yap
	mux.HandleFunc("/predict/mission", s.handleMissionFeasibility)   // POST: görev uygunluğu

	// Uyarılar
	mux.HandleFunc("/alerts", s.handleAlerts)         // GET: aktif uyarılar
	mux.HandleFunc("/alerts/ack", s.handleAlertAck)   // POST: onayla
	mux.HandleFunc("/alerts/resolve", s.handleAlertResolve) // POST: çöz
	mux.HandleFunc("/alerts/stats", s.handleAlertStats)     // GET: istatistik

	// RTB & Handoff
	mux.HandleFunc("/rtb", s.handleRTB)           // GET: RTB önerileri
	mux.HandleFunc("/handoff", s.handleHandoff)   // GET: görev devri önerileri

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, map[string]any{
		"status":  "ok",
		"module":  "TUTUM",
		"uptime":  time.Since(s.startTime).String(),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	bs := s.monitor.GetStats()
	rs := s.tracker.GetStats()
	as := s.alerts.GetStats()

	s.json(w, 200, map[string]any{
		"module":         "TUTUM",
		"description":    "Batarya ve Kaynak Tüketimi İzleyicisi",
		"uptime":         time.Since(s.startTime).String(),
		"request_count":  atomic.LoadInt64(&s.reqCount),
		"battery_stats":  bs,
		"resource_stats": rs,
		"alert_stats":    as,
	})
}

// --- Platform ---

type registerReq struct {
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Type       battery.PlatformType `json:"type"`
	CapacityWh float64              `json:"capacity_wh"`
}

func (s *Server) handlePlatforms(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	switch r.Method {
	case http.MethodGet:
		platforms := s.monitor.All()
		s.json(w, 200, map[string]any{"platforms": platforms, "count": len(platforms)})
	case http.MethodPost:
		var req registerReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.json(w, 400, map[string]any{"error": "geçersiz JSON: " + err.Error()})
			return
		}
		if req.ID == "" || req.Name == "" || req.CapacityWh <= 0 {
			s.json(w, 400, map[string]any{"error": "id, name ve capacity_wh zorunlu"})
			return
		}
		p := battery.NewPlatform(req.ID, req.Name, req.Type, req.CapacityWh)
		p.SetThresholds(s.criticalPct, s.warningPct, s.lowPct)
		if err := s.monitor.Register(p); err != nil {
			s.json(w, 409, map[string]any{"error": err.Error()})
			return
		}
		s.json(w, 201, map[string]any{"message": "platform kaydedildi", "id": req.ID})
	default:
		s.json(w, 405, map[string]any{"error": "Method Not Allowed"})
	}
}

func (s *Server) handlePlatformByID(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	id := strings.TrimPrefix(r.URL.Path, "/platforms/")
	if id == "" {
		s.json(w, 400, map[string]any{"error": "platform id gerekli"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		p, ok := s.monitor.Get(id)
		if !ok {
			s.json(w, 404, map[string]any{"error": "platform bulunamadı"})
			return
		}
		snap := p.Snapshot()
		resSnap := resource.Snapshot{}
		if pr, ok := s.tracker.Get(id); ok {
			resSnap = pr.GetSnapshot()
		}
		pred, _ := s.predictor.GetLastPrediction(id)
		alerts := s.alerts.ByPlatform(id)

		s.json(w, 200, map[string]any{
			"battery":    snap,
			"resources":  resSnap,
			"prediction": pred,
			"alerts":     alerts,
		})
	case http.MethodDelete:
		if s.monitor.Remove(id) {
			s.tracker.Remove(id)
			s.predictor.Reset(id)
			s.json(w, 200, map[string]any{"message": "platform silindi", "id": id})
		} else {
			s.json(w, 404, map[string]any{"error": "platform bulunamadı"})
		}
	default:
		s.json(w, 405, map[string]any{"error": "Method Not Allowed"})
	}
}

// --- Battery ---

type sampleReq struct {
	PlatformID string  `json:"platform_id"`
	VoltageV   float64 `json:"voltage_v"`
	CurrentA   float64 `json:"current_a"`
	TempC      float64 `json:"temp_c"`
	Terrain    string  `json:"terrain,omitempty"`
	WindSpeedMS float64 `json:"wind_speed_ms,omitempty"`
	LoadFactor float64 `json:"load_factor,omitempty"`
}

func (s *Server) handleBatterySample(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]any{"error": "POST only"})
		return
	}
	var req sampleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]any{"error": "geçersiz JSON"})
		return
	}
	p, ok := s.monitor.Get(req.PlatformID)
	if !ok {
		s.json(w, 404, map[string]any{"error": "platform bulunamadı: " + req.PlatformID})
		return
	}

	// Koşulları güncelle
	if req.Terrain != "" {
		p.UpdateConditions(battery.TerrainType(req.Terrain), req.WindSpeedMS, req.LoadFactor)
	}

	// Ölçüm kaydet
	sample := p.RecordSample(req.VoltageV, req.CurrentA, req.TempC)

	// Tahmin motoruna ekle
	s.predictor.AddSample(req.PlatformID, p.GetSoC())

	// Uyarıları kontrol et
	s.alerts.CheckBattery(req.PlatformID, p.GetSoC(), s.criticalPct, s.warningPct, s.lowPct)
	s.alerts.CheckTemperature(req.PlatformID, req.TempC, s.tempWarnC, s.tempCritC)

	s.json(w, 200, map[string]any{
		"sample": sample,
		"status": p.GetStatus(),
		"soc":    p.GetSoC(),
	})
}

func (s *Server) handleBatteryStats(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	s.json(w, 200, s.monitor.GetStats())
}

func (s *Server) handleBatteryCritical(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	crits := s.monitor.CriticalPlatforms()
	s.json(w, 200, map[string]any{"critical_platforms": crits, "count": len(crits)})
}

// --- Resource ---

type resourceReq struct {
	PlatformID string `json:"platform_id"`
	CPU        *float64 `json:"cpu_percent,omitempty"`
	Memory     *float64 `json:"memory_percent,omitempty"`
	Disk       *float64 `json:"disk_percent,omitempty"`
	NetworkBps *float64 `json:"network_bps,omitempty"`
	TempC      *float64 `json:"temp_c,omitempty"`
}

func (s *Server) handleResourceMetric(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]any{"error": "POST only"})
		return
	}
	var req resourceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]any{"error": "geçersiz JSON"})
		return
	}
	if req.PlatformID == "" {
		s.json(w, 400, map[string]any{"error": "platform_id zorunlu"})
		return
	}

	pr := s.tracker.GetOrCreate(req.PlatformID)
	var metrics []resource.Metric
	if req.CPU != nil {
		metrics = append(metrics, pr.RecordMetric(resource.ResCPU, *req.CPU, "%"))
	}
	if req.Memory != nil {
		metrics = append(metrics, pr.RecordMetric(resource.ResMemory, *req.Memory, "%"))
	}
	if req.Disk != nil {
		metrics = append(metrics, pr.RecordMetric(resource.ResDisk, *req.Disk, "%"))
	}
	if req.NetworkBps != nil {
		metrics = append(metrics, pr.RecordMetric(resource.ResNetwork, *req.NetworkBps, "bps"))
	}
	if req.TempC != nil {
		metrics = append(metrics, pr.RecordMetric(resource.ResTemp, *req.TempC, "°C"))
	}

	s.json(w, 200, map[string]any{"metrics": metrics, "platform_id": req.PlatformID})
}

func (s *Server) handleResourceStats(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	s.json(w, 200, s.tracker.GetStats())
}

// --- Predict ---

type predictReq struct {
	PlatformID string `json:"platform_id"`
}

func (s *Server) handlePredict(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]any{"error": "POST only"})
		return
	}
	var req predictReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]any{"error": "geçersiz JSON"})
		return
	}
	pred, err := s.predictor.Predict(req.PlatformID)
	if err != nil {
		s.json(w, 422, map[string]any{"error": err.Error()})
		return
	}

	// Tahmine göre uyarı kontrolü
	s.alerts.CheckPrediction(req.PlatformID, pred.PredictedMinutes, pred.RTBFeasible)

	s.json(w, 200, pred)
}

type missionReq struct {
	PlatformID     string  `json:"platform_id"`
	MissionMinutes float64 `json:"mission_minutes"`
}

func (s *Server) handleMissionFeasibility(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]any{"error": "POST only"})
		return
	}
	var req missionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]any{"error": "geçersiz JSON"})
		return
	}
	result, err := s.predictor.CheckMissionFeasibility(req.PlatformID, req.MissionMinutes)
	if err != nil {
		s.json(w, 422, map[string]any{"error": err.Error()})
		return
	}
	s.json(w, 200, result)
}

// --- Alerts ---

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	severity := r.URL.Query().Get("severity")
	platformID := r.URL.Query().Get("platform_id")

	var alerts []*alert.Alert
	switch {
	case severity != "":
		alerts = s.alerts.BySeverity(alert.Severity(severity))
	case platformID != "":
		alerts = s.alerts.ByPlatform(platformID)
	default:
		alerts = s.alerts.Active()
	}
	s.json(w, 200, map[string]any{"alerts": alerts, "count": len(alerts)})
}

type ackReq struct {
	AlertID string `json:"alert_id"`
}

func (s *Server) handleAlertAck(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]any{"error": "POST only"})
		return
	}
	var req ackReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]any{"error": "geçersiz JSON"})
		return
	}
	if s.alerts.Acknowledge(req.AlertID) {
		s.json(w, 200, map[string]any{"message": "onaylandı", "id": req.AlertID})
	} else {
		s.json(w, 404, map[string]any{"error": "uyarı bulunamadı veya zaten onaylı"})
	}
}

func (s *Server) handleAlertResolve(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	if r.Method != http.MethodPost {
		s.json(w, 405, map[string]any{"error": "POST only"})
		return
	}
	var req ackReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, 400, map[string]any{"error": "geçersiz JSON"})
		return
	}
	if s.alerts.Resolve(req.AlertID) {
		s.json(w, 200, map[string]any{"message": "çözüldü", "id": req.AlertID})
	} else {
		s.json(w, 404, map[string]any{"error": "uyarı bulunamadı veya zaten çözümlü"})
	}
}

func (s *Server) handleAlertStats(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	s.json(w, 200, s.alerts.GetStats())
}

// --- RTB & Handoff ---

func (s *Server) handleRTB(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	recs := s.alerts.GetRTBRecommendations()
	s.json(w, 200, map[string]any{"rtb_recommendations": recs, "count": len(recs)})
}

func (s *Server) handleHandoff(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqCount, 1)
	recs := s.alerts.GetHandoffRecommendations()
	s.json(w, 200, map[string]any{"handoff_recommendations": recs, "count": len(recs)})
}

// --- Helpers ---

func (s *Server) json(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		fmt.Fprintf(w, `{"error":"encoding: %s"}`, err)
	}
}
