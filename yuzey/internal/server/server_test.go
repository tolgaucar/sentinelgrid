package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otukenmatris/yuzey/internal/config"
	"github.com/otukenmatris/yuzey/internal/event"
	"github.com/otukenmatris/yuzey/internal/hub"
	"github.com/otukenmatris/yuzey/internal/layer"
)

func setup() (*Server, *hub.Hub, *event.Bus, *layer.Manager) {
	cfg := config.Config{
		HTTPPort:     9103,
		PollInterval: 100 * time.Millisecond,
		TimeoutMs:    2000,
		MaxEntities:  1000,
		Modules:      make(map[string]config.ModuleEndpoint),
	}
	bus := event.NewBus(1000)
	layers := layer.NewManager()
	h := hub.New(cfg, bus)
	srv := New(h, bus, layers, cfg)
	return srv, h, bus, layers
}

func doReq(srv *Server, method, path string, body interface{}) *httptest.ResponseRecorder {
	var bodyReader *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(b)
	} else {
		bodyReader = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	return w
}

func parseJSON(w *httptest.ResponseRecorder) map[string]interface{} {
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	return result
}

// ── Dashboard ──

func TestDashboard(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/", nil)
	if w.Code != 200 {
		t.Errorf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
	if w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Error("Content-Type html olmalı")
	}
	if len(w.Body.String()) < 100 {
		t.Error("Dashboard HTML çok kısa")
	}
}

func TestDashboard404(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/nonexistent-page", nil)
	if w.Code != 404 {
		t.Errorf("Status=404 bekleniyordu, %d geldi", w.Code)
	}
}

// ── Health ──

func TestHealth(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/health", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
	data := parseJSON(w)
	if data["status"] != "ok" {
		t.Error("status=ok bekleniyordu")
	}
	if data["module"] != "YÜZEY" {
		t.Error("module=YÜZEY bekleniyordu")
	}
}

// ── Stats ──

func TestStats(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/stats", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
	data := parseJSON(w)
	if data["hub"] == nil {
		t.Error("hub alanı olmalı")
	}
	if data["events"] == nil {
		t.Error("events alanı olmalı")
	}
	if data["layers"] == nil {
		t.Error("layers alanı olmalı")
	}
}

// ── Tactical ──

func TestTactical(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/tactical", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
	data := parseJSON(w)
	if data["timestamp"] == nil {
		t.Error("timestamp alanı olmalı")
	}
}

func TestTacticalPicture(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/tactical/picture", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
	data := parseJSON(w)
	if data["layers"] == nil {
		t.Error("layers alanı olmalı")
	}
	if data["recent_events"] == nil {
		t.Error("recent_events alanı olmalı")
	}
}

// ── Modules ──

func TestModules(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/modules", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
}

func TestModuleByIDNotFound(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/modules/nonexistent", nil)
	if w.Code != 404 {
		t.Errorf("Status=404 bekleniyordu, %d geldi", w.Code)
	}
}

func TestModuleByIDEmpty(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/modules/", nil)
	if w.Code != 400 {
		t.Errorf("Status=400 bekleniyordu, %d geldi", w.Code)
	}
}

// ── Entities ──

func TestEntitiesEmpty(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/entities", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
}

func TestEntitiesUpdate(t *testing.T) {
	srv, _, _, _ := setup()
	ents := []config.Entity{
		{ID: "drone-1", Type: "drone", Faction: "friendly", Lat: 39.9, Lon: 32.8, Health: 100},
		{ID: "tank-1", Type: "tank", Faction: "enemy", Lat: 40.0, Lon: 33.0, Health: 85},
	}
	w := doReq(srv, "POST", "/entities/update", ents)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
	data := parseJSON(w)
	if data["updated"] != float64(2) {
		t.Errorf("updated=2 bekleniyordu, %v geldi", data["updated"])
	}

	// Varlıkları kontrol et
	w2 := doReq(srv, "GET", "/entities", nil)
	var entities []config.Entity
	json.Unmarshal(w2.Body.Bytes(), &entities)
	if len(entities) != 2 {
		t.Errorf("2 varlık bekleniyordu, %d geldi", len(entities))
	}
}

func TestEntitiesUpdateBadJSON(t *testing.T) {
	srv, _, _, _ := setup()
	req := httptest.NewRequest("POST", "/entities/update", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("Status=400 bekleniyordu, %d geldi", w.Code)
	}
}

func TestEntitiesUpdateMethodCheck(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/entities/update", nil)
	if w.Code != 405 {
		t.Errorf("Status=405 bekleniyordu, %d geldi", w.Code)
	}
}

func TestEntityByID(t *testing.T) {
	srv, h, _, _ := setup()
	h.UpdateEntity(config.Entity{ID: "drone-x", Type: "drone", Faction: "friendly"})

	w := doReq(srv, "GET", "/entities/drone-x", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
	data := parseJSON(w)
	if data["id"] != "drone-x" {
		t.Errorf("id=drone-x bekleniyordu, %v geldi", data["id"])
	}
}

func TestEntityByIDNotFound(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/entities/nonexistent", nil)
	if w.Code != 404 {
		t.Errorf("Status=404 bekleniyordu, %d geldi", w.Code)
	}
}

func TestEntityFactions(t *testing.T) {
	srv, h, _, _ := setup()
	h.UpdateEntities([]config.Entity{
		{ID: "a", Faction: "friendly"},
		{ID: "b", Faction: "friendly"},
		{ID: "c", Faction: "enemy"},
	})
	w := doReq(srv, "GET", "/entities/factions", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
	data := parseJSON(w)
	if data["friendly"] != float64(2) {
		t.Errorf("friendly=2 bekleniyordu, %v geldi", data["friendly"])
	}
}

func TestEntitiesByFactionQuery(t *testing.T) {
	srv, h, _, _ := setup()
	h.UpdateEntities([]config.Entity{
		{ID: "f1", Faction: "friendly"},
		{ID: "f2", Faction: "friendly"},
		{ID: "e1", Faction: "enemy"},
	})
	w := doReq(srv, "GET", "/entities?faction=friendly", nil)
	var ents []config.Entity
	json.Unmarshal(w.Body.Bytes(), &ents)
	if len(ents) != 2 {
		t.Errorf("2 dost varlık bekleniyordu, %d geldi", len(ents))
	}
}

func TestEntitiesClear(t *testing.T) {
	srv, h, _, _ := setup()
	h.UpdateEntities([]config.Entity{{ID: "a"}, {ID: "b"}})
	w := doReq(srv, "DELETE", "/entities", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
	data := parseJSON(w)
	if data["cleared"] != float64(2) {
		t.Errorf("cleared=2 bekleniyordu, %v geldi", data["cleared"])
	}
}

// ── Layers ──

func TestLayers(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/layers", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
	var layers []layer.Layer
	json.Unmarshal(w.Body.Bytes(), &layers)
	if len(layers) != 8 {
		t.Errorf("8 katman bekleniyordu, %d geldi", len(layers))
	}
}

func TestLayerByID(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/layers/entities", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
	data := parseJSON(w)
	if data["name"] != "Varlıklar" {
		t.Errorf("name=Varlıklar bekleniyordu, %v geldi", data["name"])
	}
}

func TestLayerByIDNotFound(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/layers/nonexistent", nil)
	if w.Code != 404 {
		t.Errorf("Status=404 bekleniyordu, %d geldi", w.Code)
	}
}

func TestLayerUpdate(t *testing.T) {
	srv, _, _, _ := setup()
	v := false
	o := 0.7
	body := struct {
		Visible *bool    `json:"visible"`
		Opacity *float64 `json:"opacity"`
	}{&v, &o}
	w := doReq(srv, "PUT", "/layers/entities", body)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}

	// Kontrol et
	w2 := doReq(srv, "GET", "/layers/entities", nil)
	data := parseJSON(w2)
	if data["visible"] != false {
		t.Error("visible=false olmalıydı")
	}
}

func TestLayerEmpty(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/layers/", nil)
	if w.Code != 400 {
		t.Errorf("Status=400 bekleniyordu, %d geldi", w.Code)
	}
}

// ── Events ──

func TestEvents(t *testing.T) {
	srv, _, bus, _ := setup()
	bus.EmitSimple("test1", "src", "info", "Test 1")
	bus.EmitSimple("test2", "src", "warning", "Test 2")

	w := doReq(srv, "GET", "/events", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
	var events []config.TacticalEvent
	json.Unmarshal(w.Body.Bytes(), &events)
	if len(events) < 2 {
		t.Errorf("En az 2 olay bekleniyordu, %d geldi", len(events))
	}
}

func TestEventsWithN(t *testing.T) {
	srv, _, bus, _ := setup()
	for i := 0; i < 10; i++ {
		bus.EmitSimple("test", "src", "info", "Olay")
	}
	w := doReq(srv, "GET", "/events?n=3", nil)
	var events []config.TacticalEvent
	json.Unmarshal(w.Body.Bytes(), &events)
	if len(events) != 3 {
		t.Errorf("3 olay bekleniyordu, %d geldi", len(events))
	}
}

func TestEventsFilterBySource(t *testing.T) {
	srv, _, bus, _ := setup()
	bus.EmitSimple("a", "dirlik", "info", "1")
	bus.EmitSimple("b", "bag", "info", "2")
	bus.EmitSimple("c", "dirlik", "info", "3")

	w := doReq(srv, "GET", "/events?source=dirlik", nil)
	var events []config.TacticalEvent
	json.Unmarshal(w.Body.Bytes(), &events)
	if len(events) != 2 {
		t.Errorf("2 dirlik olay bekleniyordu, %d geldi", len(events))
	}
}

func TestEventsFilterBySeverity(t *testing.T) {
	srv, _, bus, _ := setup()
	bus.EmitSimple("a", "src", "info", "1")
	bus.EmitSimple("b", "src", "critical", "2")
	bus.EmitSimple("c", "src", "critical", "3")

	w := doReq(srv, "GET", "/events?severity=critical", nil)
	var events []config.TacticalEvent
	json.Unmarshal(w.Body.Bytes(), &events)
	if len(events) != 2 {
		t.Errorf("2 critical olay bekleniyordu, %d geldi", len(events))
	}
}

func TestEventsFilterByType(t *testing.T) {
	srv, _, bus, _ := setup()
	bus.EmitSimple("explosion", "src", "info", "1")
	bus.EmitSimple("ping", "src", "info", "2")
	bus.EmitSimple("explosion", "src", "info", "3")

	w := doReq(srv, "GET", "/events?type=explosion", nil)
	var events []config.TacticalEvent
	json.Unmarshal(w.Body.Bytes(), &events)
	if len(events) != 2 {
		t.Errorf("2 explosion olay bekleniyordu, %d geldi", len(events))
	}
}

func TestEventEmit(t *testing.T) {
	srv, _, _, _ := setup()
	body := map[string]interface{}{
		"type": "alert", "source": "sensor1", "severity": "critical",
		"title": "Tehdit Algılandı", "detail": "Bilinmeyen hedef", "lat": 39.9, "lon": 32.8,
	}
	w := doReq(srv, "POST", "/events/emit", body)
	if w.Code != 201 {
		t.Fatalf("Status=201 bekleniyordu, %d geldi", w.Code)
	}
	data := parseJSON(w)
	if data["type"] != "alert" {
		t.Error("type=alert bekleniyordu")
	}
}

func TestEventEmitMethodCheck(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/events/emit", nil)
	if w.Code != 405 {
		t.Errorf("Status=405 bekleniyordu, %d geldi", w.Code)
	}
}

func TestEventStats(t *testing.T) {
	srv, _, bus, _ := setup()
	bus.EmitSimple("a", "src", "info", "1")
	bus.EmitSimple("b", "src", "warning", "2")

	w := doReq(srv, "GET", "/events/stats", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
	data := parseJSON(w)
	if data["total_emitted"] != float64(2) {
		t.Errorf("total_emitted=2 bekleniyordu, %v geldi", data["total_emitted"])
	}
}

func TestEventStream(t *testing.T) {
	srv, _, bus, _ := setup()
	bus.EmitSimple("old", "src", "info", "1")
	time.Sleep(10 * time.Millisecond)
	since := time.Now().UnixMilli()
	time.Sleep(10 * time.Millisecond)
	bus.EmitSimple("new", "src", "info", "2")

	w := doReq(srv, "GET", "/events/stream?since="+json.Number(itoa64(since)).String(), nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
}

// ── Hub endpoints ──

func TestHubStats(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/hub/stats", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
}

func TestHubSummary(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/hub/summary", nil)
	if w.Code != 200 {
		t.Fatalf("Status=200 bekleniyordu, %d geldi", w.Code)
	}
}

func TestHubPollEmpty(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/hub/poll/", nil)
	if w.Code != 400 {
		t.Errorf("Status=400 bekleniyordu, %d geldi", w.Code)
	}
}

func TestHubPollUnknown(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/hub/poll/unknown-module", nil)
	if w.Code != 502 {
		t.Errorf("Status=502 bekleniyordu, %d geldi", w.Code)
	}
}

// ── Integration: Entity Update + Layer ──

func TestEntityUpdateUpdatesLayers(t *testing.T) {
	srv, _, _, lm := setup()
	ents := []config.Entity{
		{ID: "drone-1", Type: "drone", Faction: "friendly", Lat: 39.9, Lon: 32.8, Health: 100},
	}
	w := doReq(srv, "POST", "/entities/update", ents)
	if w.Code != 200 {
		t.Fatal("Entity update başarısız")
	}

	// Layer kontrolü
	l, ok := lm.GetLayer(layer.LayerEntities)
	if !ok {
		t.Fatal("entities katmanı bulunamadı")
	}
	if len(l.Markers) != 1 {
		t.Errorf("1 marker bekleniyordu, %d geldi", len(l.Markers))
	}
}

// ── Integration: Event Emit + Bus ──

func TestEventEmitAddsToBus(t *testing.T) {
	srv, _, bus, _ := setup()
	body := map[string]interface{}{
		"type": "alert", "source": "test", "severity": "info",
		"title": "Test Olay",
	}
	doReq(srv, "POST", "/events/emit", body)
	// Bus'ta olayı kontrol et
	evts := bus.GetByType("alert", 10)
	if len(evts) == 0 {
		t.Error("Bus'ta alert olayı olmalı")
	}
}

// ── CORS ──

func TestCORSHeaders(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/health", nil)
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS header eksik")
	}
}

// ── JSON Content-Type ──

func TestJSONContentType(t *testing.T) {
	srv, _, _, _ := setup()
	w := doReq(srv, "GET", "/health", nil)
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type=application/json bekleniyordu, %s geldi", ct)
	}
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	b := make([]byte, 0, 20)
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
