// Package server — AKIM HTTP/gRPC sunucusu.
//
// Yönlendirme, akış yönetimi, yük dengeleme ve tıkanıklık izleme
// endpoint'leri sunar.
package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/otukenmatris/akim/internal/balancer"
	"github.com/otukenmatris/akim/internal/config"
	"github.com/otukenmatris/akim/internal/flow"
	"github.com/otukenmatris/akim/internal/monitor"
	"github.com/otukenmatris/akim/internal/router"
)

// Server — AKIM HTTP sunucusu
type Server struct {
	engine   *router.Engine
	flowMgr  *flow.Manager
	balancer *balancer.Balancer
	monitor  *monitor.Monitor
	mux      *http.ServeMux
	start    time.Time
}

// New — yeni sunucu
func New(eng *router.Engine, fm *flow.Manager, bal *balancer.Balancer, mon *monitor.Monitor) *Server {
	s := &Server{
		engine:   eng,
		flowMgr:  fm,
		balancer: bal,
		monitor:  mon,
		mux:      http.NewServeMux(),
		start:    time.Now(),
	}
	s.routes()
	return s
}

// Handler — HTTP handler
func (s *Server) Handler() http.Handler {
	return s.mux
}

// ListenAndServe — sunucuyu başlat
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/stats", s.handleStats)

	// Topoloji
	s.mux.HandleFunc("/topology", s.handleTopology)
	s.mux.HandleFunc("/nodes", s.handleNodes)
	s.mux.HandleFunc("/nodes/", s.handleNodeByID)
	s.mux.HandleFunc("/links", s.handleLinks)
	s.mux.HandleFunc("/links/", s.handleLinkByID)

	// Yönlendirme
	s.mux.HandleFunc("/route", s.handleRoute)
	s.mux.HandleFunc("/routes/alternate", s.handleAlternateRoutes)

	// Akış yönetimi
	s.mux.HandleFunc("/flows", s.handleFlows)
	s.mux.HandleFunc("/flows/", s.handleFlowByID)
	s.mux.HandleFunc("/flows/allocate", s.handleAllocate)
	s.mux.HandleFunc("/flows/expire", s.handleExpire)

	// Yük dengeleme
	s.mux.HandleFunc("/balancer", s.handleBalancer)
	s.mux.HandleFunc("/balancer/select", s.handleBalancerSelect)
	s.mux.HandleFunc("/balancer/backends", s.handleBackends)
	s.mux.HandleFunc("/balancer/strategy", s.handleStrategy)

	// İzleme
	s.mux.HandleFunc("/monitor/snapshots", s.handleSnapshots)
	s.mux.HandleFunc("/monitor/alerts", s.handleAlerts)
	s.mux.HandleFunc("/monitor/congestion", s.handleCongestion)
	s.mux.HandleFunc("/monitor/history/", s.handleLinkHistory)
}

// ── Health & Stats ──

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"status":  "ok",
		"module":  "AKIM",
		"uptime":  time.Since(s.start).Seconds(),
		"version": "1.0.0",
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"router":   s.engine.GetStats(),
		"flows":    s.flowMgr.GetStats(),
		"balancer": s.balancer.GetStats(),
		"monitor":  s.monitor.GetStats(),
		"uptime":   time.Since(s.start).Seconds(),
	})
}

// ── Topoloji ──

func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.engine.Topology())
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, s.engine.ListNodes())
	case http.MethodPost:
		var n router.Node
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if err := s.engine.AddNode(n); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, map[string]string{"status": "ok", "id": n.ID})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleNodeByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/nodes/")
	if id == "" {
		writeJSON(w, 400, map[string]string{"error": "node ID gerekli"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		n, ok := s.engine.GetNode(id)
		if !ok {
			writeJSON(w, 404, map[string]string{"error": "düğüm bulunamadı"})
			return
		}
		writeJSON(w, 200, n)
	case http.MethodDelete:
		if err := s.engine.RemoveNode(id); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleLinks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, s.engine.ListLinks())
	case http.MethodPost:
		var l router.Link
		if err := json.NewDecoder(r.Body).Decode(&l); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if err := s.engine.AddLink(l); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, map[string]string{"status": "ok", "id": l.ID})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleLinkByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/links/")
	if id == "" {
		writeJSON(w, 400, map[string]string{"error": "link ID gerekli"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		l, ok := s.engine.GetLink(id)
		if !ok {
			writeJSON(w, 404, map[string]string{"error": "link bulunamadı"})
			return
		}
		writeJSON(w, 200, l)
	case http.MethodPut:
		var body struct {
			UsedBW *float64 `json:"used_bw_mbps"`
			Active *bool    `json:"active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if body.UsedBW != nil {
			if err := s.engine.UpdateLinkTraffic(id, *body.UsedBW); err != nil {
				writeJSON(w, 400, map[string]string{"error": err.Error()})
				return
			}
		}
		if body.Active != nil {
			if err := s.engine.SetLinkActive(id, *body.Active); err != nil {
				writeJSON(w, 400, map[string]string{"error": err.Error()})
				return
			}
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

// ── Yönlendirme ──

func (s *Server) handleRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "POST gerekli"})
		return
	}
	var body struct {
		From string `json:"from"`
		To   string `json:"to"`
		QoS  int    `json:"qos"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	route, err := s.engine.FindRoute(body.From, body.To, config.QoSClass(body.QoS))
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, route)
}

func (s *Server) handleAlternateRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "POST gerekli"})
		return
	}
	var body struct {
		From  string `json:"from"`
		To    string `json:"to"`
		QoS   int    `json:"qos"`
		Count int    `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if body.Count <= 0 {
		body.Count = 3
	}
	routes, err := s.engine.FindAlternateRoutes(body.From, body.To, config.QoSClass(body.QoS), body.Count)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, routes)
}

// ── Akış Yönetimi ──

func (s *Server) handleFlows(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		activeOnly := r.URL.Query().Get("active") == "true"
		writeJSON(w, 200, s.flowMgr.ListFlows(activeOnly))
	case http.MethodPost:
		var body struct {
			From     string   `json:"from"`
			To       string   `json:"to"`
			QoS      int      `json:"qos"`
			BW       float64  `json:"bw_mbps"`
			DataType string   `json:"data_type"`
			Links    []string `json:"links"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		f, err := s.flowMgr.CreateFlow(body.From, body.To, config.QoSClass(body.QoS), body.BW, body.DataType, body.Links)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, f)
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleFlowByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/flows/")
	if id == "" || id == "allocate" || id == "expire" {
		return
	}
	switch r.Method {
	case http.MethodGet:
		f, ok := s.flowMgr.GetFlow(id)
		if !ok {
			writeJSON(w, 404, map[string]string{"error": "akış bulunamadı"})
			return
		}
		writeJSON(w, 200, f)
	case http.MethodPut:
		var body struct {
			Bytes   int64 `json:"bytes"`
			Packets int64 `json:"packets"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if err := s.flowMgr.UpdateFlow(id, body.Bytes, body.Packets); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if err := s.flowMgr.CloseFlow(id); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAllocate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "POST gerekli"})
		return
	}
	var body struct {
		TotalBW float64 `json:"total_bw_mbps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if body.TotalBW <= 0 {
		body.TotalBW = 1000.0
	}
	allocs := s.flowMgr.AllocateBandwidth(body.TotalBW)
	writeJSON(w, 200, allocs)
}

func (s *Server) handleExpire(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "POST gerekli"})
		return
	}
	expired := s.flowMgr.ExpireFlows()
	writeJSON(w, 200, map[string]interface{}{"expired": expired})
}

// ── Yük Dengeleme ──

func (s *Server) handleBalancer(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"strategy": s.balancer.GetStrategy(),
		"stats":    s.balancer.GetStats(),
		"backends": s.balancer.ListBackends(),
	})
}

func (s *Server) handleBalancerSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "POST gerekli"})
		return
	}
	be, err := s.balancer.Select()
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, be)
}

func (s *Server) handleBackends(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, s.balancer.ListBackends())
	case http.MethodPost:
		var body struct {
			ID      string `json:"id"`
			Address string `json:"address"`
			Weight  int    `json:"weight"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if err := s.balancer.AddBackend(body.ID, body.Address, body.Weight); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, map[string]string{"status": "ok"})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleStrategy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, map[string]string{"strategy": string(s.balancer.GetStrategy())})
	case http.MethodPut:
		var body struct {
			Strategy string `json:"strategy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		s.balancer.SetStrategy(balancer.Strategy(body.Strategy))
		writeJSON(w, 200, map[string]string{"status": "ok", "strategy": body.Strategy})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

// ── İzleme ──

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.monitor.AllSnapshots())
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	n := 100
	if q := r.URL.Query().Get("n"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	writeJSON(w, 200, s.monitor.GetAlerts(n))
}

func (s *Server) handleCongestion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.monitor.CongestionReport())
}

func (s *Server) handleLinkHistory(w http.ResponseWriter, r *http.Request) {
	parts := strings.TrimPrefix(r.URL.Path, "/monitor/history/")
	if parts == "" {
		writeJSON(w, 400, map[string]string{"error": "link ID gerekli"})
		return
	}
	n := 60
	if q := r.URL.Query().Get("n"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	writeJSON(w, 200, s.monitor.LinkHistory(parts, n))
}

// ── Yardımcı ──

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
