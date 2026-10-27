// Package server — YÜZEY HTTP + WebSocket sunucusu.
//
// Taktiksel arayüz için gerçek zamanlı veri akışı, REST API ve
// tek sayfalık web dashboard sunar.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/otukenmatris/yuzey/internal/config"
	"github.com/otukenmatris/yuzey/internal/event"
	"github.com/otukenmatris/yuzey/internal/hub"
	"github.com/otukenmatris/yuzey/internal/layer"
)

// Server — YÜZEY HTTP sunucusu
type Server struct {
	hub       *hub.Hub
	bus       *event.Bus
	layers    *layer.Manager
	cfg       config.Config
	mux       *http.ServeMux
	startTime time.Time

	// WebSocket (basit polling tabanlı — gorilla/websocket olmadan)
	wsMu       sync.Mutex
	wsClients  map[string]chan []byte
	wsCounter  int64
}

// New — yeni sunucu
func New(h *hub.Hub, b *event.Bus, l *layer.Manager, cfg config.Config) *Server {
	srv := &Server{
		hub:       h,
		bus:       b,
		layers:    l,
		cfg:       cfg,
		mux:       http.NewServeMux(),
		startTime: time.Now(),
		wsClients: make(map[string]chan []byte),
	}
	srv.routes()

	// Event bus'tan gelen olayları WebSocket'e yönlendir
	b.Subscribe(func(evt config.TacticalEvent) {
		data, _ := json.Marshal(map[string]interface{}{
			"type": "event", "event": evt,
		})
		srv.broadcastWS(data)
	})

	return srv
}

// Handler — HTTP handler
func (s *Server) Handler() http.Handler { return s.mux }

// ListenAndServe — sunucuyu başlat
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	// Dashboard
	s.mux.HandleFunc("/", s.handleDashboard)
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/stats", s.handleStats)

	// Taktiksel Resim
	s.mux.HandleFunc("/tactical", s.handleTactical)
	s.mux.HandleFunc("/tactical/picture", s.handleTacticalPicture)

	// Modüller
	s.mux.HandleFunc("/modules", s.handleModules)
	s.mux.HandleFunc("/modules/", s.handleModuleByID)

	// Varlıklar
	s.mux.HandleFunc("/entities", s.handleEntities)
	s.mux.HandleFunc("/entities/", s.handleEntityByID)
	s.mux.HandleFunc("/entities/update", s.handleEntitiesUpdate)
	s.mux.HandleFunc("/entities/factions", s.handleEntityFactions)

	// Katmanlar
	s.mux.HandleFunc("/layers", s.handleLayers)
	s.mux.HandleFunc("/layers/", s.handleLayerByID)
	s.mux.HandleFunc("/layers/update", s.handleLayerUpdate)

	// Olaylar
	s.mux.HandleFunc("/events", s.handleEvents)
	s.mux.HandleFunc("/events/stream", s.handleEventStream)
	s.mux.HandleFunc("/events/emit", s.handleEventEmit)
	s.mux.HandleFunc("/events/stats", s.handleEventStats)

	// Hub
	s.mux.HandleFunc("/hub/stats", s.handleHubStats)
	s.mux.HandleFunc("/hub/summary", s.handleHubSummary)
	s.mux.HandleFunc("/hub/poll/", s.handleHubPoll)

	// Polling-based SSE (no gorilla)
	s.mux.HandleFunc("/stream", s.handleSSE)
}

// ── Dashboard ──

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

// ── Health & Stats ──

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	alive := 0
	for _, ms := range s.hub.GetModuleStatus() {
		if ms.Alive {
			alive++
		}
	}
	writeJSON(w, 200, map[string]interface{}{
		"status":        "ok",
		"module":        "YÜZEY",
		"version":       "1.0.0",
		"uptime":        time.Since(s.startTime).Seconds(),
		"modules_alive": alive,
		"modules_total": len(s.cfg.Modules),
		"entities":      s.hub.EntityCount(),
		"events":        s.bus.Count(),
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"hub":    s.hub.GetStats(),
		"events": s.bus.GetStats(),
		"layers": s.layers.GetStats(),
		"uptime": time.Since(s.startTime).Seconds(),
	})
}

// ── Taktiksel Resim ──

func (s *Server) handleTactical(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.hub.GetTacticalPicture())
}

func (s *Server) handleTacticalPicture(w http.ResponseWriter, r *http.Request) {
	pic := s.hub.GetTacticalPicture()
	pic["layers"] = s.layers.GetAllLayers()
	pic["recent_events"] = s.bus.GetEvents(50)
	writeJSON(w, 200, pic)
}

// ── Modüller ──

func (s *Server) handleModules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.hub.GetModuleStatus())
}

func (s *Server) handleModuleByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/modules/")
	if id == "" {
		writeJSON(w, 400, errMsg("modül ID gerekli"))
		return
	}

	parts := strings.SplitN(id, "/", 2)
	modID := parts[0]

	if len(parts) == 2 && parts[1] == "stats" {
		data, err := s.hub.PollModuleStats(modID)
		if err != nil {
			writeJSON(w, 502, errMsg(err.Error()))
			return
		}
		writeJSON(w, 200, data)
		return
	}

	ms, ok := s.hub.GetModuleByID(modID)
	if !ok {
		writeJSON(w, 404, errMsg("modül bulunamadı"))
		return
	}
	writeJSON(w, 200, ms)
}

// ── Varlıklar ──

func (s *Server) handleEntities(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		faction := r.URL.Query().Get("faction")
		if faction != "" {
			writeJSON(w, 200, s.hub.GetEntitiesByFaction(faction))
		} else {
			writeJSON(w, 200, s.hub.GetAllEntities())
		}
	case http.MethodDelete:
		n := s.hub.ClearEntities()
		writeJSON(w, 200, map[string]int{"cleared": n})
	default:
		writeJSON(w, 405, errMsg("GET veya DELETE"))
	}
}

func (s *Server) handleEntityByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/entities/")
	if id == "" || id == "update" || id == "factions" {
		return
	}
	e, ok := s.hub.GetEntity(id)
	if !ok {
		writeJSON(w, 404, errMsg("varlık bulunamadı"))
		return
	}
	writeJSON(w, 200, e)
}

func (s *Server) handleEntitiesUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var entities []config.Entity
	if err := json.NewDecoder(r.Body).Decode(&entities); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	changes := s.hub.UpdateEntities(entities)
	s.layers.UpdateEntityLayer(entities)

	// Düşman tespiti olayları
	for _, e := range changes.NewEnemies {
		typeName := e.Type
		if typeName == "" {
			typeName = "bilinmeyen"
		}
		s.bus.Emit("enemy_detected", "tani", event.SevCritical,
			"DÜŞMAN TESPİT: "+typeName,
			fmt.Sprintf("Yeni düşman varlık tespit edildi: %s [%.4f, %.4f]", e.ID, e.Lat, e.Lon),
			e.Lat, e.Lon)
	}

	// İmha olayları
	for _, e := range changes.Destroyed {
		sev := event.SevWarning
		prefix := "VARLIK İMHA: "
		if e.Faction == "friendly" {
			sev = event.SevCritical
			prefix = "DOST KAYIP: "
		}
		s.bus.Emit("entity_destroyed", "yuzey", sev,
			prefix+e.ID,
			fmt.Sprintf("%s imha edildi [%.4f, %.4f]", e.ID, e.Lat, e.Lon),
			e.Lat, e.Lon)
	}

	// Kritik hasar olayları
	for _, e := range changes.Critical {
		s.bus.Emit("health_critical", "dirlik", event.SevWarning,
			"KRİTİK HASAR: "+e.ID,
			fmt.Sprintf("Sağlık %%%.0f — acil destek gerekli [%.4f, %.4f]", e.Health, e.Lat, e.Lon),
			e.Lat, e.Lon)
	}

	// Hasar olayları (sadece ilk 5, spam önleme)
	dmgCount := len(changes.DamageEvents)
	if dmgCount > 5 {
		dmgCount = 5
	}
	for _, e := range changes.DamageEvents[:dmgCount] {
		s.bus.Emit("entity_damaged", "yuzey", event.SevInfo,
			"HASAR: "+e.ID,
			fmt.Sprintf("Sağlık %%%.0f [%.4f, %.4f]", e.Health, e.Lat, e.Lon),
			e.Lat, e.Lon)
	}

	// Dost kuvvet tespiti (ilk sefer)
	if len(changes.NewFriendlies) > 0 {
		s.bus.EmitSimple("friendly_detected", "yuzey", event.SevInfo,
			fmt.Sprintf("%d yeni dost varlık tespit edildi", len(changes.NewFriendlies)))
	}

	writeJSON(w, 200, map[string]int{"updated": len(entities)})
}

func (s *Server) handleEntityFactions(w http.ResponseWriter, r *http.Request) {
	entities := s.hub.GetAllEntities()
	factions := make(map[string]int)
	for _, e := range entities {
		factions[e.Faction]++
	}
	writeJSON(w, 200, factions)
}

// ── Katmanlar ──

func (s *Server) handleLayers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.layers.GetAllLayers())
}

func (s *Server) handleLayerByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/layers/")
	if id == "" {
		writeJSON(w, 400, errMsg("katman ID gerekli"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		l, ok := s.layers.GetLayer(layer.LayerType(id))
		if !ok {
			writeJSON(w, 404, errMsg("katman bulunamadı"))
			return
		}
		writeJSON(w, 200, l)
	case http.MethodPut:
		var body struct {
			Visible *bool    `json:"visible,omitempty"`
			Opacity *float64 `json:"opacity,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, errMsg(err.Error()))
			return
		}
		lt := layer.LayerType(id)
		if body.Visible != nil {
			s.layers.SetVisibility(lt, *body.Visible)
		}
		if body.Opacity != nil {
			s.layers.SetOpacity(lt, *body.Opacity)
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	default:
		writeJSON(w, 405, errMsg("GET veya PUT"))
	}
}

// ── Katman Veri Güncelleme ──

func (s *Server) handleLayerUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		Layer   string         `json:"layer"`
		Markers []layer.Marker `json:"markers,omitempty"`
		Zones   []layer.Zone   `json:"zones,omitempty"`
		Lines   []layer.Polyline `json:"lines,omitempty"`
		Mode    string         `json:"mode"` // "set" (üzerine yaz) veya "append" (ekle)
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	lt := layer.LayerType(body.Layer)
	if body.Layer == "" {
		writeJSON(w, 400, errMsg("layer alanı gerekli"))
		return
	}

	mode := body.Mode
	if mode == "" {
		mode = "set"
	}

	added := 0
	if mode == "set" {
		if body.Markers != nil {
			if err := s.layers.SetMarkers(lt, body.Markers); err != nil {
				writeJSON(w, 400, errMsg(err.Error()))
				return
			}
			added += len(body.Markers)
		}
		if body.Zones != nil {
			if err := s.layers.SetZones(lt, body.Zones); err != nil {
				writeJSON(w, 400, errMsg(err.Error()))
				return
			}
			added += len(body.Zones)
		}
		if body.Lines != nil {
			if err := s.layers.SetLines(lt, body.Lines); err != nil {
				writeJSON(w, 400, errMsg(err.Error()))
				return
			}
			added += len(body.Lines)
		}
	} else {
		for _, m := range body.Markers {
			if err := s.layers.AddMarker(lt, m); err != nil {
				writeJSON(w, 400, errMsg(err.Error()))
				return
			}
			added++
		}
		for _, z := range body.Zones {
			if err := s.layers.AddZone(lt, z); err != nil {
				writeJSON(w, 400, errMsg(err.Error()))
				return
			}
			added++
		}
		for _, l := range body.Lines {
			if err := s.layers.AddLine(lt, l); err != nil {
				writeJSON(w, 400, errMsg(err.Error()))
				return
			}
			added++
		}
	}

	// SSE'ye bildir
	data, _ := json.Marshal(map[string]interface{}{
		"type":  "layer_update",
		"layer": body.Layer,
		"items": added,
	})
	s.broadcastWS(data)

	writeJSON(w, 200, map[string]interface{}{"status": "ok", "layer": body.Layer, "items": added})
}

// ── Olaylar ──

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	n := 100
	if q := r.URL.Query().Get("n"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	source := r.URL.Query().Get("source")
	severity := r.URL.Query().Get("severity")
	evtType := r.URL.Query().Get("type")

	if source != "" {
		writeJSON(w, 200, s.bus.GetBySource(source, n))
	} else if severity != "" {
		writeJSON(w, 200, s.bus.GetBySeverity(severity, n))
	} else if evtType != "" {
		writeJSON(w, 200, s.bus.GetByType(evtType, n))
	} else {
		writeJSON(w, 200, s.bus.GetEvents(n))
	}
}

func (s *Server) handleEventStream(w http.ResponseWriter, r *http.Request) {
	sinceStr := r.URL.Query().Get("since")
	since := int64(0)
	if sinceStr != "" {
		if v, err := strconv.ParseInt(sinceStr, 10, 64); err == nil {
			since = v
		}
	}
	writeJSON(w, 200, s.bus.GetEventsSince(since))
}

func (s *Server) handleEventEmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, errMsg("POST gerekli"))
		return
	}
	var body struct {
		Type     string  `json:"type"`
		Source   string  `json:"source"`
		Severity string  `json:"severity"`
		Title    string  `json:"title"`
		Detail   string  `json:"detail"`
		Lat      float64 `json:"lat"`
		Lon      float64 `json:"lon"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, errMsg(err.Error()))
		return
	}
	evt := s.bus.Emit(body.Type, body.Source, body.Severity, body.Title, body.Detail, body.Lat, body.Lon)
	writeJSON(w, 201, evt)
}

func (s *Server) handleEventStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.bus.GetStats())
}

// ── Hub ──

func (s *Server) handleHubStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.hub.GetStats())
}

func (s *Server) handleHubSummary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.hub.Summary())
}

func (s *Server) handleHubPoll(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/hub/poll/")
	if id == "" {
		writeJSON(w, 400, errMsg("modül ID gerekli"))
		return
	}
	data, err := s.hub.PollModuleStats(id)
	if err != nil {
		writeJSON(w, 502, errMsg(err.Error()))
		return
	}
	writeJSON(w, 200, data)
}

// ── Server-Sent Events (SSE) ──

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, 500, errMsg("SSE desteklenmiyor"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher.Flush()

	ch := make(chan []byte, 256)
	clientID := fmt.Sprintf("sse-%d", time.Now().UnixNano())

	s.wsMu.Lock()
	s.wsClients[clientID] = ch
	s.wsMu.Unlock()

	defer func() {
		s.wsMu.Lock()
		delete(s.wsClients, clientID)
		s.wsMu.Unlock()
	}()

	ctx := r.Context()

	// İlk bağlantıda taktiksel resmi gönder
	pic, _ := json.Marshal(s.hub.GetTacticalPicture())
	fmt.Fprintf(w, "data: %s\n\n", pic)
	flusher.Flush()

	// Periyodik durum güncellemesi
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case data := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			update, _ := json.Marshal(map[string]interface{}{
				"type":    "update",
				"modules": s.hub.GetModuleStatus(),
				"entities": s.hub.EntityCount(),
				"events":  s.bus.Count(),
				"stats":   s.hub.GetStats(),
			})
			fmt.Fprintf(w, "data: %s\n\n", update)
			flusher.Flush()
		}
	}
}

// broadcastWS — tüm SSE istemcilerine gönder
func (s *Server) broadcastWS(data []byte) {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	for _, ch := range s.wsClients {
		select {
		case ch <- data:
		default:
			// Kanal dolu, at
		}
	}
}

// ── Yardımcı ──

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func errMsg(msg string) map[string]string {
	return map[string]string{"error": msg}
}
