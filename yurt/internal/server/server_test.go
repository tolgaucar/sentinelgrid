package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otukenmatris/yurt/internal/config"
	"github.com/otukenmatris/yurt/internal/partition"
	"github.com/otukenmatris/yurt/internal/proxy"
	"github.com/otukenmatris/yurt/internal/store"
	ysync "github.com/otukenmatris/yurt/internal/sync"
)

func testServer() *Server {
	cfg := config.DefaultConfig()
	cfg.NodeID = "test-node"
	s := store.New("test-node", 10000, time.Hour)
	se := ysync.New(s, cfg)
	h := proxy.NewHub(cfg)
	pm := partition.NewManager(cfg)
	return New(s, se, h, pm, cfg)
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
		t.Fatalf("health: %d", rr.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["module"] != "YURT" {
		t.Fatalf("module: %v", resp["module"])
	}
}

func TestStats(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/stats", nil)
	if rr.Code != 200 {
		t.Fatalf("stats: %d", rr.Code)
	}
}

func TestStorePutAndGet(t *testing.T) {
	s := testServer()
	rr := doReq(s, "POST", "/store/put", map[string]interface{}{
		"key": "test-key", "value": map[string]interface{}{"x": 1},
	})
	if rr.Code != 201 {
		t.Fatalf("put: %d — %s", rr.Code, rr.Body.String())
	}

	rr = doReq(s, "GET", "/store/get?key=test-key", nil)
	if rr.Code != 200 {
		t.Fatalf("get: %d", rr.Code)
	}
}

func TestStoreGetNotFound(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/store/get?key=missing", nil)
	if rr.Code != 404 {
		t.Fatalf("missing key should 404: %d", rr.Code)
	}
}

func TestStoreDelete(t *testing.T) {
	s := testServer()
	doReq(s, "POST", "/store/put", map[string]interface{}{
		"key": "del-key", "value": map[string]interface{}{},
	})
	rr := doReq(s, "POST", "/store/delete", map[string]interface{}{"key": "del-key"})
	if rr.Code != 200 {
		t.Fatalf("delete: %d", rr.Code)
	}
}

func TestStoreKeys(t *testing.T) {
	s := testServer()
	doReq(s, "POST", "/store/put", map[string]interface{}{"key": "a", "value": map[string]interface{}{}})
	doReq(s, "POST", "/store/put", map[string]interface{}{"key": "b", "value": map[string]interface{}{}})

	rr := doReq(s, "GET", "/store/keys", nil)
	if rr.Code != 200 {
		t.Fatalf("keys: %d", rr.Code)
	}
}

func TestStoreDirty(t *testing.T) {
	s := testServer()
	doReq(s, "POST", "/store/put", map[string]interface{}{"key": "d", "value": map[string]interface{}{}})

	rr := doReq(s, "GET", "/store/dirty", nil)
	if rr.Code != 200 {
		t.Fatalf("dirty: %d", rr.Code)
	}
}

func TestStoreSnapshot(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/store/snapshot", nil)
	if rr.Code != 200 {
		t.Fatalf("snapshot: %d", rr.Code)
	}
}

func TestStoreExpire(t *testing.T) {
	s := testServer()
	rr := doReq(s, "POST", "/store/expire", nil)
	if rr.Code != 200 {
		t.Fatalf("expire: %d", rr.Code)
	}
}

func TestStoreStats(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/store/stats", nil)
	if rr.Code != 200 {
		t.Fatalf("store stats: %d", rr.Code)
	}
}

func TestSyncPush(t *testing.T) {
	s := testServer()
	doReq(s, "POST", "/store/put", map[string]interface{}{"key": "sk", "value": map[string]interface{}{}})
	rr := doReq(s, "POST", "/sync/push", nil)
	if rr.Code != 200 {
		t.Fatalf("sync push: %d", rr.Code)
	}
}

func TestSyncPull(t *testing.T) {
	s := testServer()
	rr := doReq(s, "POST", "/sync/pull", nil)
	if rr.Code != 200 {
		t.Fatalf("sync pull: %d", rr.Code)
	}
}

func TestSyncFull(t *testing.T) {
	s := testServer()
	rr := doReq(s, "POST", "/sync/full", nil)
	if rr.Code != 200 {
		t.Fatalf("sync full: %d", rr.Code)
	}
}

func TestSyncState(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/sync/state", nil)
	if rr.Code != 200 {
		t.Fatalf("sync state: %d", rr.Code)
	}
}

func TestSyncConflicts(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/sync/conflicts?n=10", nil)
	if rr.Code != 200 {
		t.Fatalf("sync conflicts: %d", rr.Code)
	}
}

func TestSyncChangelog(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/sync/changelog", nil)
	if rr.Code != 200 {
		t.Fatalf("sync changelog: %d", rr.Code)
	}
}

func TestSyncStats(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/sync/stats", nil)
	if rr.Code != 200 {
		t.Fatalf("sync stats: %d", rr.Code)
	}
}

func TestProxyRequest(t *testing.T) {
	s := testServer()
	rr := doReq(s, "POST", "/proxy/request", map[string]interface{}{
		"method": "GET", "path": "/api/data", "priority": 2,
	})
	if rr.Code != 200 {
		t.Fatalf("proxy request: %d — %s", rr.Code, rr.Body.String())
	}
}

func TestProxyReplay(t *testing.T) {
	s := testServer()
	rr := doReq(s, "POST", "/proxy/replay", nil)
	if rr.Code != 200 {
		t.Fatalf("proxy replay: %d", rr.Code)
	}
}

func TestProxyBuffer(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/proxy/buffer", nil)
	if rr.Code != 200 {
		t.Fatalf("proxy buffer: %d", rr.Code)
	}
}

func TestProxyConnection(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/proxy/connection", nil)
	if rr.Code != 200 {
		t.Fatalf("get connection: %d", rr.Code)
	}

	rr = doReq(s, "PUT", "/proxy/connection", map[string]interface{}{"state": "OFFLINE"})
	if rr.Code != 200 {
		t.Fatalf("set connection: %d", rr.Code)
	}
}

func TestProxyHealth(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/proxy/health", nil)
	if rr.Code != 200 {
		t.Fatalf("proxy health: %d", rr.Code)
	}
}

func TestProxyClear(t *testing.T) {
	s := testServer()
	rr := doReq(s, "POST", "/proxy/clear", nil)
	if rr.Code != 200 {
		t.Fatalf("proxy clear: %d", rr.Code)
	}
}

func TestProxyStats(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/proxy/stats", nil)
	if rr.Code != 200 {
		t.Fatalf("proxy stats: %d", rr.Code)
	}
}

func TestProxySummary(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/proxy/summary", nil)
	if rr.Code != 200 {
		t.Fatalf("proxy summary: %d", rr.Code)
	}
}

func TestPartState(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/partition/state", nil)
	if rr.Code != 200 {
		t.Fatalf("part state: %d", rr.Code)
	}
}

func TestPartCheck(t *testing.T) {
	s := testServer()
	rr := doReq(s, "POST", "/partition/check", nil)
	if rr.Code != 200 {
		t.Fatalf("part check: %d", rr.Code)
	}
}

func TestPartPeers(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/partition/peers", nil)
	if rr.Code != 200 {
		t.Fatalf("list peers: %d", rr.Code)
	}

	rr = doReq(s, "POST", "/partition/peers", map[string]interface{}{
		"id": "new-peer", "address": "http://peer:9000", "role": "relay",
	})
	if rr.Code != 201 {
		t.Fatalf("add peer: %d", rr.Code)
	}

	rr = doReq(s, "DELETE", "/partition/peers/new-peer", nil)
	if rr.Code != 200 {
		t.Fatalf("remove peer: %d", rr.Code)
	}
}

func TestPartHeartbeat(t *testing.T) {
	s := testServer()
	doReq(s, "POST", "/partition/peers", map[string]interface{}{
		"id": "p1", "address": "addr", "role": "cloud",
	})
	rr := doReq(s, "POST", "/partition/heartbeat", map[string]interface{}{
		"peer_id": "p1", "latency_ms": 3.5,
	})
	if rr.Code != 200 {
		t.Fatalf("heartbeat: %d", rr.Code)
	}
}

func TestPartEvents(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/partition/events", nil)
	if rr.Code != 200 {
		t.Fatalf("events: %d", rr.Code)
	}
}

func TestPartStats(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/partition/stats", nil)
	if rr.Code != 200 {
		t.Fatalf("part stats: %d", rr.Code)
	}
}

func TestPartSummary(t *testing.T) {
	s := testServer()
	rr := doReq(s, "GET", "/partition/summary", nil)
	if rr.Code != 200 {
		t.Fatalf("part summary: %d", rr.Code)
	}
}

// unused import guard
var _ = http.StatusOK
