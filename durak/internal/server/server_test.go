package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otukenmatris/durak/internal/queue"
	"github.com/otukenmatris/durak/internal/scheduler"
)

func setupTestServer() *Server {
	sched := scheduler.New(scheduler.SchedulerConfig{
		BandwidthBps: 10_000_000,
		TickInterval: 100 * time.Millisecond,
		BurstFactor:  1.5,
	})

	// Varsayılan kuyruklar
	for _, q := range []struct{ id, name string }{
		{"komut", "Komut Kuyruğu"},
		{"sensor", "Sensör Kuyruğu"},
		{"telemetri", "Telemetri Kuyruğu"},
	} {
		pq := queue.NewPriorityQueue(queue.QueueInfo{
			ID: q.id, Name: q.name, MaxSize: 1000,
		}, 1000)
		sched.AddQueue(pq)
	}

	return New(sched)
}

func TestHealthEndpoint(t *testing.T) {
	srv := setupTestServer()
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("Health status: beklenen 200, alınan %d", rec.Code)
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["module"] != "durak" {
		t.Error("module alanı durak olmalı")
	}
}

func TestStatusEndpoint(t *testing.T) {
	srv := setupTestServer()
	req := httptest.NewRequest("GET", "/status", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("Status: beklenen 200, alınan %d", rec.Code)
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["module"] != "durak" {
		t.Error("module alanı durak olmalı")
	}
}

func TestQueuesEndpoint(t *testing.T) {
	srv := setupTestServer()
	req := httptest.NewRequest("GET", "/queues", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("Queues: beklenen 200, alınan %d", rec.Code)
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	queues := resp["queues"].([]any)
	if len(queues) != 3 {
		t.Errorf("3 kuyruk bekleniyor, alınan: %d", len(queues))
	}
}

func TestEnqueueDequeue(t *testing.T) {
	srv := setupTestServer()

	// Enqueue
	body, _ := json.Marshal(enqueueReq{
		QueueID:     "komut",
		Priority:    0,
		Source:      "merkez",
		Destination: "tim-1",
		ContentType: "application/json",
		Payload:     `{"cmd":"attack"}`,
		TTLSeconds:  60,
	})
	req := httptest.NewRequest("POST", "/enqueue", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("Enqueue: beklenen 200, alınan %d — %s", rec.Code, rec.Body.String())
	}
	var enqResp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &enqResp)
	if enqResp["queue"] != "komut" {
		t.Error("Kuyruk adı doğru değil")
	}

	// Dequeue
	body2, _ := json.Marshal(dequeueReq{QueueID: "komut"})
	req2 := httptest.NewRequest("POST", "/dequeue", bytes.NewReader(body2))
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)

	if rec2.Code != 200 {
		t.Fatalf("Dequeue: beklenen 200, alınan %d", rec2.Code)
	}
}

func TestEnqueueMissingQueue(t *testing.T) {
	srv := setupTestServer()
	body, _ := json.Marshal(enqueueReq{QueueID: "nonexistent"})
	req := httptest.NewRequest("POST", "/enqueue", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Errorf("Olmayan kuyruk: beklenen 404, alınan %d", rec.Code)
	}
}

func TestDequeueEmptyQueue(t *testing.T) {
	srv := setupTestServer()
	body, _ := json.Marshal(dequeueReq{QueueID: "komut"})
	req := httptest.NewRequest("POST", "/dequeue", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != 204 {
		t.Errorf("Boş kuyruktan dequeue: beklenen 204, alınan %d", rec.Code)
	}
}

func TestBandwidthEndpoint(t *testing.T) {
	srv := setupTestServer()

	// GET
	req := httptest.NewRequest("GET", "/scheduler/bandwidth", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET bandwidth: beklenen 200, alınan %d", rec.Code)
	}

	// POST
	body, _ := json.Marshal(bandwidthReq{Bps: 5_000_000})
	req2 := httptest.NewRequest("POST", "/scheduler/bandwidth", bytes.NewReader(body))
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != 200 {
		t.Fatalf("POST bandwidth: beklenen 200, alınan %d", rec2.Code)
	}

	// Invalid
	body3, _ := json.Marshal(bandwidthReq{Bps: -1})
	req3 := httptest.NewRequest("POST", "/scheduler/bandwidth", bytes.NewReader(body3))
	rec3 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec3, req3)
	if rec3.Code != 400 {
		t.Errorf("Negatif bps: beklenen 400, alınan %d", rec3.Code)
	}
}

func TestFlushEndpoint(t *testing.T) {
	srv := setupTestServer()

	// Paketler ekle
	body, _ := json.Marshal(enqueueReq{QueueID: "komut", Priority: 0, Payload: "test1"})
	req := httptest.NewRequest("POST", "/enqueue", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	body2, _ := json.Marshal(enqueueReq{QueueID: "komut", Priority: 0, Payload: "test2"})
	req2 := httptest.NewRequest("POST", "/enqueue", bytes.NewReader(body2))
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)

	// Flush
	flushBody, _ := json.Marshal(map[string]string{"queue_id": "komut"})
	req3 := httptest.NewRequest("POST", "/flush", bytes.NewReader(flushBody))
	rec3 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec3, req3)

	if rec3.Code != 200 {
		t.Fatalf("Flush: beklenen 200, alınan %d", rec3.Code)
	}
	var resp map[string]any
	json.Unmarshal(rec3.Body.Bytes(), &resp)
	if resp["flushed"].(float64) < 2 {
		t.Errorf("En az 2 paket flush bekleniyor, alınan: %v", resp["flushed"])
	}
}

func TestMethodNotAllowed(t *testing.T) {
	srv := setupTestServer()

	endpoints := []string{"/enqueue", "/dequeue", "/flush"}
	for _, ep := range endpoints {
		req := httptest.NewRequest("GET", ep, nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != 405 {
			t.Errorf("%s GET: beklenen 405, alınan %d", ep, rec.Code)
		}
	}
}

func TestSchedulerStatsEndpoint(t *testing.T) {
	srv := setupTestServer()
	req := httptest.NewRequest("GET", "/scheduler/stats", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("Stats: beklenen 200, alınan %d", rec.Code)
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if _, ok := resp["total_scheduled"]; !ok {
		t.Error("total_scheduled alanı eksik")
	}
}

func TestEnqueueInvalidJSON(t *testing.T) {
	srv := setupTestServer()
	req := httptest.NewRequest("POST", "/enqueue", bytes.NewReader([]byte("invalid")))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("Geçersiz JSON: beklenen 400, alınan %d", rec.Code)
	}
}
