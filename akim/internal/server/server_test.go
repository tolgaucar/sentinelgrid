package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/otukenmatris/akim/internal/balancer"
	"github.com/otukenmatris/akim/internal/config"
	"github.com/otukenmatris/akim/internal/flow"
	"github.com/otukenmatris/akim/internal/monitor"
	"github.com/otukenmatris/akim/internal/router"
)

func testServer() *Server {
	cfg := config.DefaultConfig()
	eng := router.New(cfg)

	eng.AddNode(router.Node{ID: "A", Type: "command", Capacity: 10000})
	eng.AddNode(router.Node{ID: "B", Type: "relay", Capacity: 5000})
	eng.AddNode(router.Node{ID: "C", Type: "edge", Capacity: 2000})
	eng.AddLink(router.Link{From: "A", To: "B", Bandwidth: 1000, Latency: 2.0})
	eng.AddLink(router.Link{From: "B", To: "C", Bandwidth: 500, Latency: 3.0})

	fm := flow.NewManager(cfg)
	bal := balancer.New(balancer.CongestionAware)
	bal.AddBackend("be1", "10.0.0.1:8080", 30)
	bal.AddBackend("be2", "10.0.0.2:8080", 20)
	mon := monitor.New(eng, cfg)

	return New(eng, fm, bal, mon)
}

func doReq(s *Server, method, path string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	return rr
}

func TestHealth(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/health", nil)
	if rr.Code != 200 {
		t.Fatalf("beklenen: 200, alınan: %d", rr.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["module"] != "AKIM" {
		t.Fatalf("yanlış modül: %v", resp["module"])
	}
}

func TestGetStats(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/stats", nil)
	if rr.Code != 200 {
		t.Fatalf("beklenen: 200, alınan: %d", rr.Code)
	}
}

func TestGetTopology(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/topology", nil)
	if rr.Code != 200 {
		t.Fatalf("beklenen: 200, alınan: %d", rr.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["total_nodes"].(float64) != 3 {
		t.Fatalf("beklenen: 3 düğüm, alınan: %v", resp["total_nodes"])
	}
}

func TestNodesCRUD(t *testing.T) {
	s := testServer()

	// List
	rr := doReq(s, "GET", "/nodes", nil)
	if rr.Code != 200 {
		t.Fatalf("list: %d", rr.Code)
	}

	// Create
	rr = doReq(s, "POST", "/nodes", map[string]interface{}{
		"id": "D", "type": "sensor", "capacity_mbps": 500,
	})
	if rr.Code != 201 {
		t.Fatalf("create: %d — %s", rr.Code, rr.Body.String())
	}

	// Get
	rr = doReq(s, "GET", "/nodes/D", nil)
	if rr.Code != 200 {
		t.Fatalf("get: %d", rr.Code)
	}

	// Delete
	rr = doReq(s, "DELETE", "/nodes/D", nil)
	if rr.Code != 200 {
		t.Fatalf("delete: %d", rr.Code)
	}

	// Get after delete
	rr = doReq(s, "GET", "/nodes/D", nil)
	if rr.Code != 404 {
		t.Fatalf("silinen düğüm 404 olmalı: %d", rr.Code)
	}
}

func TestLinksCRUD(t *testing.T) {
	s := testServer()

	// List
	rr := doReq(s, "GET", "/links", nil)
	if rr.Code != 200 {
		t.Fatalf("list: %d", rr.Code)
	}

	// Create
	rr = doReq(s, "POST", "/links", map[string]interface{}{
		"from": "A", "to": "C", "bandwidth_mbps": 800, "latency_ms": 5.0,
	})
	if rr.Code != 201 {
		t.Fatalf("create: %d — %s", rr.Code, rr.Body.String())
	}

	// Update traffic
	rr = doReq(s, "PUT", "/links/A->C", map[string]interface{}{
		"used_bw_mbps": 400.0,
	})
	if rr.Code != 200 {
		t.Fatalf("update: %d — %s", rr.Code, rr.Body.String())
	}
}

func TestRouting(t *testing.T) {
	s := testServer()
	rr := doReq(s, "POST", "/route", map[string]interface{}{
		"from": "A", "to": "C", "qos": 3,
	})
	if rr.Code != 200 {
		t.Fatalf("route: %d — %s", rr.Code, rr.Body.String())
	}
	var route map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &route)
	if route["from"] != "A" {
		t.Fatalf("yanlış kaynak: %v", route["from"])
	}
}

func TestAlternateRoutes(t *testing.T) {
	s := testServer()
	rr := doReq(s, "POST", "/routes/alternate", map[string]interface{}{
		"from": "A", "to": "C", "qos": 3, "count": 2,
	})
	if rr.Code != 200 {
		t.Fatalf("alternate: %d — %s", rr.Code, rr.Body.String())
	}
}

func TestFlowsCRUD(t *testing.T) {
	s := testServer()

	// Create
	rr := doReq(s, "POST", "/flows", map[string]interface{}{
		"from": "A", "to": "C", "qos": 3, "bw_mbps": 100, "data_type": "sensor",
	})
	if rr.Code != 201 {
		t.Fatalf("create flow: %d — %s", rr.Code, rr.Body.String())
	}
	var f map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &f)
	fid := f["id"].(string)

	// Get
	rr = doReq(s, "GET", "/flows/"+fid, nil)
	if rr.Code != 200 {
		t.Fatalf("get flow: %d", rr.Code)
	}

	// Update
	rr = doReq(s, "PUT", "/flows/"+fid, map[string]interface{}{
		"bytes": 2048, "packets": 20,
	})
	if rr.Code != 200 {
		t.Fatalf("update flow: %d", rr.Code)
	}

	// Delete
	rr = doReq(s, "DELETE", "/flows/"+fid, nil)
	if rr.Code != 200 {
		t.Fatalf("close flow: %d", rr.Code)
	}
}

func TestFlowAllocate(t *testing.T) {
	s := testServer()
	doReq(s, "POST", "/flows", map[string]interface{}{
		"from": "A", "to": "C", "qos": 0, "bw_mbps": 200, "data_type": "alert",
	})
	rr := doReq(s, "POST", "/flows/allocate", map[string]interface{}{
		"total_bw_mbps": 1000,
	})
	if rr.Code != 200 {
		t.Fatalf("allocate: %d", rr.Code)
	}
}

func TestFlowExpire(t *testing.T) {
	s := testServer()
	rr := doReq(s, "POST", "/flows/expire", nil)
	if rr.Code != 200 {
		t.Fatalf("expire: %d", rr.Code)
	}
}

func TestBalancerEndpoints(t *testing.T) {
	s := testServer()

	// Overview
	rr := doReq(s, "GET", "/balancer", nil)
	if rr.Code != 200 {
		t.Fatalf("balancer: %d", rr.Code)
	}

	// Select
	rr = doReq(s, "POST", "/balancer/select", nil)
	if rr.Code != 200 {
		t.Fatalf("select: %d — %s", rr.Code, rr.Body.String())
	}

	// Backends
	rr = doReq(s, "GET", "/balancer/backends", nil)
	if rr.Code != 200 {
		t.Fatalf("backends: %d", rr.Code)
	}

	// Add backend
	rr = doReq(s, "POST", "/balancer/backends", map[string]interface{}{
		"id": "be3", "address": "10.0.0.3:8080", "weight": 15,
	})
	if rr.Code != 201 {
		t.Fatalf("add backend: %d", rr.Code)
	}

	// Strategy
	rr = doReq(s, "GET", "/balancer/strategy", nil)
	if rr.Code != 200 {
		t.Fatalf("get strategy: %d", rr.Code)
	}

	// Change strategy
	rr = doReq(s, "PUT", "/balancer/strategy", map[string]interface{}{
		"strategy": "round_robin",
	})
	if rr.Code != 200 {
		t.Fatalf("set strategy: %d", rr.Code)
	}
}

func TestMonitorEndpoints(t *testing.T) {
	s := testServer()

	rr := doReq(s, "GET", "/monitor/snapshots", nil)
	if rr.Code != 200 {
		t.Fatalf("snapshots: %d", rr.Code)
	}

	rr = doReq(s, "GET", "/monitor/alerts?n=10", nil)
	if rr.Code != 200 {
		t.Fatalf("alerts: %d", rr.Code)
	}

	rr = doReq(s, "GET", "/monitor/congestion", nil)
	if rr.Code != 200 {
		t.Fatalf("congestion: %d", rr.Code)
	}

	rr = doReq(s, "GET", "/monitor/history/A->B?n=5", nil)
	if rr.Code != 200 {
		t.Fatalf("history: %d", rr.Code)
	}
}

func TestRouteNotFound(t *testing.T) {
	s := testServer()
	rr := doReq(s, "POST", "/route", map[string]interface{}{
		"from": "A", "to": "MISSING", "qos": 0,
	})
	if rr.Code != 404 {
		t.Fatalf("missing route should 404: %d", rr.Code)
	}
}

func TestNodeNotFound(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/nodes/MISSING", nil)
	if rr.Code != 404 {
		t.Fatalf("missing node should 404: %d", rr.Code)
	}
}

func TestFlowNotFound(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/flows/missing-id", nil)
	if rr.Code != 404 {
		t.Fatalf("missing flow should 404: %d", rr.Code)
	}
}

// suppress unused import
var _ = http.StatusOK
