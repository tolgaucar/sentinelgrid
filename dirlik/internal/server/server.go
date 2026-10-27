// Package server — DİRLİK HTTP/gRPC sunucusu.
// Metrik sorgu, probe yönetimi, alarm geçmişi, durum endpoint'leri.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/otukenmatris/dirlik/internal/alerter"
	"github.com/otukenmatris/dirlik/internal/anomaly"
	"github.com/otukenmatris/dirlik/internal/collector"
	"github.com/otukenmatris/dirlik/internal/probe"
)

// Server — DİRLİK HTTP sunucusu
type Server struct {
	collector *collector.Collector
	detector  *anomaly.Detector
	probes    *probe.Manager
	alerter   *alerter.Alerter
	mux       *http.ServeMux
	startTime time.Time
}

// New — Yeni sunucu oluştur
func New(c *collector.Collector, d *anomaly.Detector, p *probe.Manager, a *alerter.Alerter) *Server {
	s := &Server{
		collector: c,
		detector:  d,
		probes:    p,
		alerter:   a,
		mux:       http.NewServeMux(),
		startTime: time.Now(),
	}
	s.routes()
	return s
}

// Handler — HTTP handler döndür
func (s *Server) Handler() http.Handler {
	return s.mux
}

// ListenAndServe — Sunucuyu başlat
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/status", s.handleStatus)

	// Metrikler
	s.mux.HandleFunc("/metrics/current", s.handleMetricsCurrent)
	s.mux.HandleFunc("/metrics/history", s.handleMetricsHistory)
	s.mux.HandleFunc("/metrics/services", s.handleMetricsServices)

	// Probe'lar
	s.mux.HandleFunc("/probes", s.handleProbes)
	s.mux.HandleFunc("/probes/", s.handleProbeAction)

	// Alarmlar
	s.mux.HandleFunc("/alerts", s.handleAlerts)
	s.mux.HandleFunc("/alerts/summary", s.handleAlertsSummary)
	s.mux.HandleFunc("/alerts/records", s.handleAlertRecords)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"status":  "healthy",
		"service": "dirlik",
		"uptime":  time.Since(s.startTime).Seconds(),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	snap, hasSnap := s.collector.Latest()

	downServices := s.probes.DownServices()
	alertStats := s.alerter.Stats()
	alertBySev := s.detector.AlertsBySeverity()

	status := map[string]interface{}{
		"service":        "dirlik",
		"version":        "1.0.0",
		"uptime_sec":     time.Since(s.startTime).Seconds(),
		"collecting":     s.collector.IsRunning(),
		"snapshot_count": s.collector.SnapshotCount(),
		"probe_targets":  s.probes.TargetCount(),
		"down_services":  len(downServices),
		"alerts": map[string]interface{}{
			"total":       s.detector.AlertCount(),
			"by_severity": alertBySev,
			"sent":        alertStats["sent"],
			"failed":      alertStats["failed"],
		},
	}

	if hasSnap {
		status["system"] = map[string]interface{}{
			"cpu_percent":  snap.CPUPercent,
			"mem_percent":  snap.MemPercent,
			"mem_used_mb":  snap.MemUsed / 1024 / 1024,
			"disk_percent": snap.DiskPercent,
			"goroutines":   snap.Goroutines,
			"heap_mb":      snap.HeapAlloc / 1024 / 1024,
		}
	}

	writeJSON(w, 200, status)
}

func (s *Server) handleMetricsCurrent(w http.ResponseWriter, r *http.Request) {
	snap := s.collector.Collect()
	writeJSON(w, 200, snap)
}

func (s *Server) handleMetricsHistory(w http.ResponseWriter, r *http.Request) {
	n := 60
	if v := r.URL.Query().Get("n"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			n = parsed
		}
	}
	history := s.collector.History(n)
	writeJSON(w, 200, map[string]interface{}{
		"count":   len(history),
		"metrics": history,
	})
}

func (s *Server) handleMetricsServices(w http.ResponseWriter, r *http.Request) {
	metrics := s.collector.GetServiceMetrics()
	writeJSON(w, 200, map[string]interface{}{
		"count":    len(metrics),
		"services": metrics,
	})
}

func (s *Server) handleProbes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		states := s.probes.AllStates()
		writeJSON(w, 200, map[string]interface{}{
			"count":  len(states),
			"probes": states,
		})

	case "POST":
		var target probe.ProbeTarget
		if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
			writeJSON(w, 400, map[string]string{"error": "geçersiz JSON: " + err.Error()})
			return
		}
		if target.ServiceID == "" || target.Endpoint == "" {
			writeJSON(w, 400, map[string]string{"error": "service_id ve endpoint zorunlu"})
			return
		}
		if target.Type == "" {
			target.Type = probe.ProbeHTTP
		}
		s.probes.AddTarget(target)
		writeJSON(w, 201, map[string]string{"status": "eklendi", "service_id": target.ServiceID})

	default:
		w.WriteHeader(405)
	}
}

func (s *Server) handleProbeAction(w http.ResponseWriter, r *http.Request) {
	// /probes/{serviceID} veya /probes/{serviceID}/check
	path := strings.TrimPrefix(r.URL.Path, "/probes/")
	parts := strings.SplitN(path, "/", 2)
	serviceID := parts[0]

	if serviceID == "" {
		writeJSON(w, 400, map[string]string{"error": "service_id gerekli"})
		return
	}

	if len(parts) > 1 && parts[1] == "check" {
		// Tek seferlik probe
		result, err := s.probes.ProbeOnce(serviceID)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, result)
		return
	}

	switch r.Method {
	case "GET":
		state, ok := s.probes.GetState(serviceID)
		if !ok {
			writeJSON(w, 404, map[string]string{"error": "servis bulunamadı"})
			return
		}
		writeJSON(w, 200, state)

	case "DELETE":
		s.probes.RemoveTarget(serviceID)
		writeJSON(w, 200, map[string]string{"status": "kaldırıldı"})

	default:
		w.WriteHeader(405)
	}
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	n := 100
	if v := r.URL.Query().Get("n"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			n = parsed
		}
	}
	alerts := s.detector.Alerts(n)
	writeJSON(w, 200, map[string]interface{}{
		"count":  len(alerts),
		"alerts": alerts,
	})
}

func (s *Server) handleAlertsSummary(w http.ResponseWriter, r *http.Request) {
	bySev := s.detector.AlertsBySeverity()
	stats := s.alerter.Stats()
	writeJSON(w, 200, map[string]interface{}{
		"total":       s.detector.AlertCount(),
		"by_severity": bySev,
		"sent":        stats["sent"],
		"failed":      stats["failed"],
	})
}

func (s *Server) handleAlertRecords(w http.ResponseWriter, r *http.Request) {
	n := 100
	if v := r.URL.Query().Get("n"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			n = parsed
		}
	}
	records := s.alerter.Records(n)
	writeJSON(w, 200, map[string]interface{}{
		"count":   len(records),
		"records": records,
	})
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Service", "dirlik")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		fmt.Fprintf(w, `{"error":"json encode hatası"}`)
	}
}
