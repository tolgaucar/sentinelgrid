package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/otukenmatris/bagdas/internal/adapter"
	"github.com/otukenmatris/bagdas/internal/codec"
	"github.com/otukenmatris/bagdas/internal/pipeline"
	"github.com/otukenmatris/bagdas/internal/schema"
)

func newTestServer() *Server {
	codecReg := codec.NewRegistry()
	schemaReg := schema.NewRegistry()
	schemaReg.Register("test_schema", []schema.Field{
		{Name: "id", Type: schema.TypeString, Required: true},
		{Name: "value", Type: schema.TypeFloat, Required: false},
	})

	adpt := adapter.NewAdapter(codecReg)
	pipeReg := pipeline.NewPipelineRegistry()
	pipeReg.Register(
		pipeline.NewPipeline("test_pipe").
			AddStep(pipeline.NewTimestampStep("ts")),
	)

	return NewServer(codecReg, schemaReg, adpt, pipeReg)
}

func doRequest(s *Server, method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

func parseResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("JSON parse: %v\nBody: %s", err, w.Body.String())
	}
	return result
}

// ──────────────────────────────────────────────
// Healthz
// ──────────────────────────────────────────────

func TestHealthz(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "GET", "/healthz", nil)
	if w.Code != 200 {
		t.Errorf("status = %d", w.Code)
	}
	r := parseResponse(t, w)
	if r["status"] != "ok" {
		t.Errorf("status = %v", r["status"])
	}
	if r["module"] != "bagdas" {
		t.Errorf("module = %v", r["module"])
	}
}

// ──────────────────────────────────────────────
// Convert
// ──────────────────────────────────────────────

func TestConvert_JSONtoMsgPack(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "POST", "/api/convert", map[string]interface{}{
		"data": map[string]interface{}{"key": "value", "num": 42},
		"from": "json",
		"to":   "msgpack",
	})
	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	r := parseResponse(t, w)
	if r["from"] != "json" || r["to"] != "msgpack" {
		t.Errorf("from/to: %v/%v", r["from"], r["to"])
	}
}

func TestConvert_JSONtoCBOR(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "POST", "/api/convert", map[string]interface{}{
		"data": map[string]interface{}{"sensor": "radar", "lat": 39.925},
		"from": "json",
		"to":   "cbor",
	})
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestConvert_InvalidFormat(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "POST", "/api/convert", map[string]interface{}{
		"data": "test",
		"from": "xml",
		"to":   "json",
	})
	if w.Code != 400 {
		t.Errorf("status = %d, beklenen 400", w.Code)
	}
}

func TestConvert_MethodNotAllowed(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "GET", "/api/convert", nil)
	if w.Code != 405 {
		t.Errorf("status = %d", w.Code)
	}
}

// ──────────────────────────────────────────────
// Validate
// ──────────────────────────────────────────────

func TestValidate_Valid(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "POST", "/api/validate", map[string]interface{}{
		"schema":  "test_schema",
		"version": 1,
		"data":    map[string]interface{}{"id": "ok", "value": 42.5},
	})
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	r := parseResponse(t, w)
	if r["valid"] != true {
		t.Errorf("valid = %v", r["valid"])
	}
}

func TestValidate_Invalid(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "POST", "/api/validate", map[string]interface{}{
		"schema":  "test_schema",
		"version": 1,
		"data":    map[string]interface{}{"value": 42.5},
	})
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	r := parseResponse(t, w)
	if r["valid"] != false {
		t.Error("doğrulama başarısız olmalıydı")
	}
}

// ──────────────────────────────────────────────
// Schemas
// ──────────────────────────────────────────────

func TestSchemas_List(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "GET", "/api/schemas", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestSchemas_Create(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "POST", "/api/schemas", map[string]interface{}{
		"name": "new_schema",
		"fields": []map[string]interface{}{
			{"name": "id", "type": "string", "required": true},
		},
	})
	if w.Code != 201 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestSchemas_CreateWithCompat(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "POST", "/api/schemas", map[string]interface{}{
		"name":   "compat_schema",
		"compat": "backward",
		"fields": []map[string]interface{}{
			{"name": "id", "type": "string", "required": true},
		},
	})
	if w.Code != 201 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestSchemas_GetByName(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "GET", "/api/schemas/test_schema", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	r := parseResponse(t, w)
	if r["name"] != "test_schema" {
		t.Errorf("name = %v", r["name"])
	}
}

func TestSchemas_GetByNameVersion(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "GET", "/api/schemas/test_schema/1", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestSchemas_GetNotFound(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "GET", "/api/schemas/nope", nil)
	if w.Code != 404 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestSchemas_Delete(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "DELETE", "/api/schemas/test_schema", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	// Tekrar getirme → 404
	w2 := doRequest(s, "GET", "/api/schemas/test_schema", nil)
	if w2.Code != 404 {
		t.Errorf("silme sonrası status = %d", w2.Code)
	}
}

func TestSchemas_DeleteNotFound(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "DELETE", "/api/schemas/nope", nil)
	if w.Code != 404 {
		t.Errorf("status = %d", w.Code)
	}
}

// ──────────────────────────────────────────────
// Formats
// ──────────────────────────────────────────────

func TestFormats(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "GET", "/api/formats", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	r := parseResponse(t, w)
	fmts := r["formats"].([]interface{})
	if len(fmts) != 3 {
		t.Errorf("format count = %d", len(fmts))
	}
}

// ──────────────────────────────────────────────
// Stats
// ──────────────────────────────────────────────

func TestStats(t *testing.T) {
	s := newTestServer()
	// Birkaç istek yap
	doRequest(s, "GET", "/healthz", nil)
	doRequest(s, "GET", "/api/formats", nil)

	w := doRequest(s, "GET", "/api/stats", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	r := parseResponse(t, w)
	reqs := r["total_requests"].(float64)
	if reqs < 3 {
		t.Errorf("total_requests = %v", reqs)
	}
}

// ──────────────────────────────────────────────
// Modules
// ──────────────────────────────────────────────

func TestModules_List(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "GET", "/api/modules", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestModules_Register(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "POST", "/api/modules", map[string]interface{}{
		"name":     "new_module",
		"base_url": "http://localhost:9999",
		"type":     "go",
		"format":   "json",
	})
	if w.Code != 201 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

// ──────────────────────────────────────────────
// Pipelines
// ──────────────────────────────────────────────

func TestPipelines_List(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "GET", "/api/pipelines", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	r := parseResponse(t, w)
	if r["count"].(float64) != 1 {
		t.Errorf("pipeline count = %v", r["count"])
	}
}

func TestPipeline_Execute(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "POST", "/api/pipeline/execute", map[string]interface{}{
		"pipeline": "test_pipe",
		"data":     map[string]interface{}{"id": "test"},
	})
	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	r := parseResponse(t, w)
	if r["steps_run"].(float64) != 1 {
		t.Errorf("steps_run = %v", r["steps_run"])
	}
}

func TestPipeline_Execute_NotFound(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "POST", "/api/pipeline/execute", map[string]interface{}{
		"pipeline": "nope",
		"data":     map[string]interface{}{},
	})
	if w.Code != 500 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestPipeline_Execute_MethodNotAllowed(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "GET", "/api/pipeline/execute", nil)
	if w.Code != 405 {
		t.Errorf("status = %d", w.Code)
	}
}

// ──────────────────────────────────────────────
// Bridge — sadece hata yolları (gerçek modül gerek)
// ──────────────────────────────────────────────

func TestBridge_ModuleNotFound(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "POST", "/api/bridge", map[string]interface{}{
		"src":  "butun",
		"dst":  "bag",
		"path": "/api/publish",
		"data": map[string]interface{}{"msg": "test"},
	})
	// Modüller kayıtlı değil → 502
	if w.Code != 502 {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestBridge_MethodNotAllowed(t *testing.T) {
	s := newTestServer()
	w := doRequest(s, "GET", "/api/bridge", nil)
	if w.Code != 405 {
		t.Errorf("status = %d", w.Code)
	}
}
