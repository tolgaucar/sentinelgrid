package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/otukenmatris/tani/internal/classifier"
	"github.com/otukenmatris/tani/internal/detector"
	"github.com/otukenmatris/tani/internal/tracker"
)

func setupServer() *Server {
	det := detector.NewEngine(0.3, 0.5, 100, 640, 640, "YOLOv7-test")
	cls := classifier.NewEngine(true, 0.3)
	trk := tracker.NewEngine(100, 5, 0.3, 50)
	return NewServer(det, cls, trk)
}

func TestHandleHealth(t *testing.T) {
	srv := setupServer()
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("health status = %d, want 200", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["module"] != "TANI" {
		t.Errorf("module = %v, want TANI", resp["module"])
	}
}

func TestHandleStatus(t *testing.T) {
	srv := setupServer()
	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status code = %d, want 200", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["module"] != "TANI" {
		t.Errorf("module = %v", resp["module"])
	}
	if resp["detector_stats"] == nil {
		t.Error("detector_stats eksik")
	}
}

func TestHandleDetect(t *testing.T) {
	srv := setupServer()
	body, _ := json.Marshal(detectReq{
		FrameID:  "test-frame-001",
		Width:    640,
		Height:   480,
		SourceID: "cam-01",
	})
	req := httptest.NewRequest("POST", "/detect", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("detect status = %d, want 200", w.Code)
	}
	var resp detector.DetectionResult
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.FrameID != "test-frame-001" {
		t.Errorf("FrameID = %s", resp.FrameID)
	}
}

func TestHandleDetect_MethodNotAllowed(t *testing.T) {
	srv := setupServer()
	req := httptest.NewRequest("GET", "/detect", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 405 {
		t.Errorf("GET /detect = %d, want 405", w.Code)
	}
}

func TestHandleDetectLast_Empty(t *testing.T) {
	srv := setupServer()
	req := httptest.NewRequest("GET", "/detect/last", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("detect/last empty = %d, want 404", w.Code)
	}
}

func TestHandleDetectLast_AfterDetect(t *testing.T) {
	srv := setupServer()

	// İlk tespit
	body, _ := json.Marshal(detectReq{FrameID: "f1", Width: 640, Height: 640})
	req := httptest.NewRequest("POST", "/detect", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	// Son tespit
	req2 := httptest.NewRequest("GET", "/detect/last", nil)
	w2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Errorf("detect/last = %d, want 200", w2.Code)
	}
}

func TestHandleClassify(t *testing.T) {
	srv := setupServer()
	dets := classifyReq{
		Detections: []detector.Detection{
			{ID: "d1", Class: detector.ClassTank, Confidence: 0.9,
				BBox: detector.BoundingBox{10, 10, 50, 50}},
		},
	}
	body, _ := json.Marshal(dets)
	req := httptest.NewRequest("POST", "/classify", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("classify = %d, want 200", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["count"].(float64) != 1 {
		t.Errorf("classify count = %v, want 1", resp["count"])
	}
}

func TestHandleClassifyStats(t *testing.T) {
	srv := setupServer()
	req := httptest.NewRequest("GET", "/classify/stats", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("classify/stats = %d, want 200", w.Code)
	}
}

func TestHandleTracks_Empty(t *testing.T) {
	srv := setupServer()
	req := httptest.NewRequest("GET", "/tracks", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("tracks = %d, want 200", w.Code)
	}
}

func TestHandleTrackByID_NotFound(t *testing.T) {
	srv := setupServer()
	req := httptest.NewRequest("GET", "/tracks/TRK-999", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("tracks/TRK-999 = %d, want 404", w.Code)
	}
}

func TestHandleProcess(t *testing.T) {
	srv := setupServer()
	body, _ := json.Marshal(processReq{
		FrameID:  "pf-001",
		Width:    640,
		Height:   640,
		SourceID: "cam-01",
	})
	req := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("process = %d, want 200", w.Code)
	}
	var resp processResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.FrameID != "pf-001" {
		t.Errorf("process FrameID = %s", resp.FrameID)
	}
	if resp.TotalMs <= 0 {
		t.Error("TotalMs should be > 0")
	}
}

func TestHandleProcess_DefaultValues(t *testing.T) {
	srv := setupServer()
	body, _ := json.Marshal(processReq{})
	req := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("process empty = %d, want 200", w.Code)
	}
}

func TestHandleIFFRegister(t *testing.T) {
	srv := setupServer()
	body, _ := json.Marshal(iffReq{ID: "TR-001"})
	req := httptest.NewRequest("POST", "/iff/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("iff/register = %d, want 200", w.Code)
	}
}

func TestHandleIFFRegister_EmptyID(t *testing.T) {
	srv := setupServer()
	body, _ := json.Marshal(iffReq{})
	req := httptest.NewRequest("POST", "/iff/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("iff/register empty = %d, want 400", w.Code)
	}
}

func TestHandleIFFUnregister(t *testing.T) {
	srv := setupServer()
	body, _ := json.Marshal(iffReq{ID: "TR-001"})
	req := httptest.NewRequest("POST", "/iff/unregister", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("iff/unregister = %d, want 200", w.Code)
	}
}

func TestHandleFullStats(t *testing.T) {
	srv := setupServer()
	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("stats = %d, want 200", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["detector"] == nil || resp["classifier"] == nil || resp["tracker"] == nil {
		t.Error("Eksik istatistik alanı")
	}
}

func TestHandleHostile(t *testing.T) {
	srv := setupServer()
	req := httptest.NewRequest("GET", "/tracks/hostile", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("hostile = %d, want 200", w.Code)
	}
}

func TestHandleTrackStats(t *testing.T) {
	srv := setupServer()
	req := httptest.NewRequest("GET", "/tracks/stats", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("tracks/stats = %d, want 200", w.Code)
	}
}

func TestMultipleProcessCalls(t *testing.T) {
	srv := setupServer()
	handler := srv.Handler()

	for i := 0; i < 5; i++ {
		body, _ := json.Marshal(processReq{Width: 640, Height: 640})
		req := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("process[%d] = %d", i, w.Code)
		}
	}

	// Track sayısı artmış olmalı
	req := httptest.NewRequest("GET", "/tracks", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	// Birden fazla process çağrısından sonra track olması beklenir
	t.Logf("Track count after 5 process calls: %v", resp["count"])
}

// Endpoint yönlendirme doğruluğu
func TestRouting(t *testing.T) {
	srv := setupServer()
	handler := srv.Handler()

	endpoints := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/health", 200},
		{"GET", "/status", 200},
		{"GET", "/detect/last", 404}, // henüz tespit yok
		{"GET", "/classify/stats", 200},
		{"GET", "/tracks", 200},
		{"GET", "/tracks/hostile", 200},
		{"GET", "/tracks/stats", 200},
		{"GET", "/stats", 200},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != ep.status {
			t.Errorf("%s %s = %d, want %d", ep.method, ep.path, w.Code, ep.status)
		}
	}
}
