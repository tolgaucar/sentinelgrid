// Package server — YURT HTTP sunucusu.
//
// Yerel depolama, senkronizasyon, proxy ve bölünme yönetimi endpoint'leri.
package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/otukenmatris/yurt/internal/config"
	"github.com/otukenmatris/yurt/internal/partition"
	"github.com/otukenmatris/yurt/internal/proxy"
	"github.com/otukenmatris/yurt/internal/store"
	ysync "github.com/otukenmatris/yurt/internal/sync"
)

// Server — YURT HTTP sunucusu
type Server struct {
	store     *store.Store
	syncEng   *ysync.Engine
	hub       *proxy.HubProxy
	partMgr   *partition.Manager
	cfg       config.Config
	mux       *http.ServeMux
	startTime time.Time
}

// New — yeni sunucu
func New(s *store.Store, se *ysync.Engine, h *proxy.HubProxy, pm *partition.Manager, cfg config.Config) *Server {
	srv := &Server{
		store:     s,
		syncEng:   se,
		hub:       h,
		partMgr:   pm,
		cfg:       cfg,
		mux:       http.NewServeMux(),
		startTime: time.Now(),
	}
	srv.routes()
	return srv
}

// Handler — HTTP handler
func (s *Server) Handler() http.Handler { return s.mux }

// ListenAndServe — sunucuyu başlat
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/stats", s.handleStats)

	// Yerel Depolama
	s.mux.HandleFunc("/store/get", s.handleStoreGet)
	s.mux.HandleFunc("/store/put", s.handleStorePut)
	s.mux.HandleFunc("/store/delete", s.handleStoreDelete)
	s.mux.HandleFunc("/store/keys", s.handleStoreKeys)
	s.mux.HandleFunc("/store/dirty", s.handleStoreDirty)
	s.mux.HandleFunc("/store/snapshot", s.handleStoreSnapshot)
	s.mux.HandleFunc("/store/expire", s.handleStoreExpire)
	s.mux.HandleFunc("/store/stats", s.handleStoreStats)

	// Senkronizasyon
	s.mux.HandleFunc("/sync/push", s.handleSyncPush)
	s.mux.HandleFunc("/sync/pull", s.handleSyncPull)
	s.mux.HandleFunc("/sync/full", s.handleSyncFull)
	s.mux.HandleFunc("/sync/state", s.handleSyncState)
	s.mux.HandleFunc("/sync/conflicts", s.handleSyncConflicts)
	s.mux.HandleFunc("/sync/changelog", s.handleSyncChangelog)
	s.mux.HandleFunc("/sync/stats", s.handleSyncStats)

	// Proxy
	s.mux.HandleFunc("/proxy/request", s.handleProxyRequest)
	s.mux.HandleFunc("/proxy/replay", s.handleProxyReplay)
	s.mux.HandleFunc("/proxy/buffer", s.handleProxyBuffer)
	s.mux.HandleFunc("/proxy/connection", s.handleProxyConnection)
	s.mux.HandleFunc("/proxy/health", s.handleProxyHealth)
	s.mux.HandleFunc("/proxy/clear", s.handleProxyClear)
	s.mux.HandleFunc("/proxy/stats", s.handleProxyStats)
	s.mux.HandleFunc("/proxy/summary", s.handleProxySummary)

	// Partition
	s.mux.HandleFunc("/partition/state", s.handlePartState)
	s.mux.HandleFunc("/partition/check", s.handlePartCheck)
	s.mux.HandleFunc("/partition/peers", s.handlePartPeers)
	s.mux.HandleFunc("/partition/peers/", s.handlePartPeerByID)
	s.mux.HandleFunc("/partition/heartbeat", s.handlePartHeartbeat)
	s.mux.HandleFunc("/partition/events", s.handlePartEvents)
	s.mux.HandleFunc("/partition/stats", s.handlePartStats)
	s.mux.HandleFunc("/partition/summary", s.handlePartSummary)
}

// ── Health & Stats ──

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"status":     "ok",
		"module":     "YURT",
		"node_id":    s.cfg.NodeID,
		"uptime":     time.Since(s.startTime).Seconds(),
		"connection": s.hub.GetConnectionState().String(),
		"partition":  s.partMgr.GetState().String(),
		"version":    "1.0.0",
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"store":     s.store.GetStats(),
		"sync":      s.syncEng.GetStats(),
		"proxy":     s.hub.GetStats(),
		"partition": s.partMgr.GetStats(),
		"node_id":   s.cfg.NodeID,
		"uptime":    time.Since(s.startTime).Seconds(),
	})
}

// ── Yerel Depolama ──

func (s *Server) handleStorePut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		Key   string                 `json:"key"`
		Value map[string]interface{} `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	if err := s.store.Put(body.Key, body.Value); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	writeJSON(w, 201, map[string]string{"status": "ok", "key": body.Key})
}

func (s *Server) handleStoreGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		writeJSON(w, 400, errMsg("key parametresi gerekli"))
		return
	}
	entry, ok := s.store.Get(key)
	if !ok {
		writeJSON(w, 404, errMsg("anahtar bulunamadı"))
		return
	}
	writeJSON(w, 200, entry)
}

func (s *Server) handleStoreDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeJSON(w, 405, errMsg("POST veya DELETE gerekli"))
		return
	}
	var body struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	if err := s.store.Delete(body.Key); err != nil {
		writeJSON(w, 404, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleStoreKeys(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	keys := s.store.ListKeys(prefix)
	writeJSON(w, 200, keys)
}

func (s *Server) handleStoreDirty(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if q := r.URL.Query().Get("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			limit = v
		}
	}
	writeJSON(w, 200, s.store.GetDirty(limit))
}

func (s *Server) handleStoreSnapshot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.store.Snapshot())
}

func (s *Server) handleStoreExpire(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	expired := s.store.ExpireOld()
	writeJSON(w, 200, map[string]int{"expired": expired})
}

func (s *Server) handleStoreStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.store.GetStats())
}

// ── Senkronizasyon ──

func (s *Server) handleSyncPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	n, err := s.syncEng.Push()
	if err != nil {
		writeJSON(w, 500, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, map[string]int{"pushed": n})
}

func (s *Server) handleSyncPull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	n, err := s.syncEng.Pull()
	if err != nil {
		writeJSON(w, 500, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, map[string]int{"pulled": n})
}

func (s *Server) handleSyncFull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	pushed, pulled, err := s.syncEng.Sync()
	if err != nil {
		writeJSON(w, 500, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"pushed": pushed, "pulled": pulled,
	})
}

func (s *Server) handleSyncState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"state":     s.syncEng.GetState().String(),
		"last_sync": s.syncEng.GetLastSync(),
	})
}

func (s *Server) handleSyncConflicts(w http.ResponseWriter, r *http.Request) {
	n := 50
	if q := r.URL.Query().Get("n"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	writeJSON(w, 200, s.syncEng.GetConflicts(n))
}

func (s *Server) handleSyncChangelog(w http.ResponseWriter, r *http.Request) {
	since := time.Time{}
	if q := r.URL.Query().Get("since"); q != "" {
		if t, err := time.Parse(time.RFC3339, q); err == nil {
			since = t
		}
	}
	writeJSON(w, 200, s.store.GetChangeLog(since))
}

func (s *Server) handleSyncStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.syncEng.GetStats())
}

// ── Proxy ──

func (s *Server) handleProxyRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		Method   string                 `json:"method"`
		Path     string                 `json:"path"`
		Body     map[string]interface{} `json:"body"`
		Priority int                    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	reqID, err := s.hub.HandleRequest(body.Method, body.Path, body.Body, body.Priority)
	if err != nil {
		writeJSON(w, 503, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, map[string]string{"request_id": reqID})
}

func (s *Server) handleProxyReplay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	n := s.hub.Replay()
	writeJSON(w, 200, map[string]int{"replayed": n})
}

func (s *Server) handleProxyBuffer(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.hub.GetBuffer())
}

func (s *Server) handleProxyConnection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, map[string]string{
			"state": s.hub.GetConnectionState().String(),
		})
	case http.MethodPut:
		var body struct {
			State string `json:"state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, errMsg(err.Error()))
			return
		}
		state := config.ConnOnline
		switch strings.ToUpper(body.State) {
		case "OFFLINE":
			state = config.ConnOffline
		case "DEGRADED":
			state = config.ConnDegraded
		}
		s.hub.SetConnectionState(state)
		writeJSON(w, 200, map[string]string{"status": "ok", "state": state.String()})
	default:
		writeJSON(w, 405, errMsg("GET veya PUT gerekli"))
	}
}

func (s *Server) handleProxyHealth(w http.ResponseWriter, r *http.Request) {
	healthy := s.hub.HealthCheck()
	writeJSON(w, 200, map[string]bool{"healthy": healthy})
}

func (s *Server) handleProxyClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	n := s.hub.ClearBuffer()
	writeJSON(w, 200, map[string]int{"cleared": n})
}

func (s *Server) handleProxyStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.hub.GetStats())
}

func (s *Server) handleProxySummary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.hub.Summary())
}

// ── Partition ──

func (s *Server) handlePartState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{
		"state": s.partMgr.GetState().String(),
	})
}

func (s *Server) handlePartCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	state := s.partMgr.CheckPartition()
	writeJSON(w, 200, map[string]string{"state": state.String()})
}

func (s *Server) handlePartPeers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, s.partMgr.ListPeers())
	case http.MethodPost:
		var body struct {
			ID      string `json:"id"`
			Address string `json:"address"`
			Role    string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, errMsg(err.Error()))
			return
		}
		s.partMgr.AddPeer(body.ID, body.Address, body.Role)
		writeJSON(w, 201, map[string]string{"status": "ok"})
	default:
		writeJSON(w, 405, errMsg("GET veya POST gerekli"))
	}
}

func (s *Server) handlePartPeerByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/partition/peers/")
	if id == "" {
		writeJSON(w, 400, errMsg("peer ID gerekli"))
		return
	}
	if r.Method == http.MethodDelete {
		s.partMgr.RemovePeer(id)
		writeJSON(w, 200, map[string]string{"status": "ok"})
	} else {
		writeJSON(w, 405, errMsg("DELETE gerekli"))
	}
}

func (s *Server) handlePartHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		PeerID  string  `json:"peer_id"`
		Latency float64 `json:"latency_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	s.partMgr.RecordHeartbeat(body.PeerID, body.Latency)
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handlePartEvents(w http.ResponseWriter, r *http.Request) {
	n := 50
	if q := r.URL.Query().Get("n"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	writeJSON(w, 200, s.partMgr.GetEvents(n))
}

func (s *Server) handlePartStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.partMgr.GetStats())
}

func (s *Server) handlePartSummary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.partMgr.Summary())
}

// ── Yardımcı ──

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func errMsg(msg string) map[string]string {
	return map[string]string{"error": msg}
}
