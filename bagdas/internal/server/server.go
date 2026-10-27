// Package server — BAĞDAŞ HTTP API sunucusu.
//
// Endpoints:
//   POST /api/convert          — Format dönüştür
//   POST /api/validate         — Şema doğrula
//   POST /api/pipeline/execute — Pipeline çalıştır
//   GET  /api/schemas          — Şema listele
//   POST /api/schemas          — Şema kaydet
//   GET  /api/schemas/:name    — Şema getir (en son)
//   GET  /api/schemas/:name/:ver — Belirli versiyon
//   DELETE /api/schemas/:name  — Şema sil
//   GET  /api/modules          — Modül listele
//   POST /api/modules          — Modül kaydet
//   POST /api/bridge           — Modül arası köprü
//   GET  /api/pipelines        — Pipeline listele
//   GET  /api/formats          — Desteklenen formatlar
//   GET  /api/stats            — İstatistikler
//   GET  /healthz              — Sağlık kontrolü
package server

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/bagdas/internal/adapter"
	"github.com/otukenmatris/bagdas/internal/codec"
	"github.com/otukenmatris/bagdas/internal/pipeline"
	"github.com/otukenmatris/bagdas/internal/schema"
)

// Server — BAĞDAŞ HTTP sunucusu
type Server struct {
	codecReg    *codec.Registry
	schemaReg   *schema.Registry
	adapter     *adapter.Adapter
	pipelineReg *pipeline.PipelineRegistry
	mux         *http.ServeMux
	httpServer  *http.Server
	requestCount int64
	startTime    time.Time
}

// NewServer — Yeni sunucu oluştur
func NewServer(
	codecReg *codec.Registry,
	schemaReg *schema.Registry,
	adpt *adapter.Adapter,
	pipeReg *pipeline.PipelineRegistry,
) *Server {
	s := &Server{
		codecReg:    codecReg,
		schemaReg:   schemaReg,
		adapter:     adpt,
		pipelineReg: pipeReg,
		mux:         http.NewServeMux(),
		startTime:   time.Now(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/health", s.handleHealth) // YÜZEY ve diğer modüller /health kullanır
	s.mux.HandleFunc("/api/convert", s.handleConvert)
	s.mux.HandleFunc("/api/validate", s.handleValidate)
	s.mux.HandleFunc("/api/formats", s.handleFormats)
	s.mux.HandleFunc("/api/stats", s.handleStats)
	s.mux.HandleFunc("/api/schemas", s.handleSchemas)
	s.mux.HandleFunc("/api/schemas/", s.handleSchemaByName)
	s.mux.HandleFunc("/api/modules", s.handleModules)
	s.mux.HandleFunc("/api/bridge", s.handleBridge)
	s.mux.HandleFunc("/api/pipelines", s.handlePipelines)
	s.mux.HandleFunc("/api/pipeline/execute", s.handlePipelineExecute)
}

// Handler — HTTP handler döndür (test için)
func (s *Server) Handler() http.Handler {
	return s.mux
}

// ListenAndServe — Sunucuyu başlat
func (s *Server) ListenAndServe(addr string) error {
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	log.Printf("[BAĞDAŞ] HTTP sunucu başlatılıyor: %s", addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown — Sunucuyu kapat
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// ──────────────────────────────────────────────
// Handlers
// ──────────────────────────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.requestCount, 1)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"module":  "bagdas",
		"uptime":  time.Since(s.startTime).String(),
		"formats": len(s.codecReg.Formats()),
		"schemas": s.schemaReg.Count(),
	})
}

// POST /api/convert
// Body: {"data": ..., "from": "json", "to": "msgpack"}
func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.requestCount, 1)
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "sadece POST")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 16*1024*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "gövde okunamadı")
		return
	}
	defer r.Body.Close()

	var req struct {
		Data interface{} `json:"data"`
		From string      `json:"from"`
		To   string      `json:"to"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "geçersiz JSON: "+err.Error())
		return
	}

	fromFmt, err := codec.ParseFormat(req.From)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	toFmt, err := codec.ParseFormat(req.To)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Veriyi kaynak formatta encode et
	srcCodec, err := s.codecReg.Get(fromFmt)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	srcBytes, err := srcCodec.Encode(req.Data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "kaynak encode hatası: "+err.Error())
		return
	}

	// Dönüştür
	result, err := s.codecReg.Convert(srcBytes, fromFmt, toFmt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "dönüşüm hatası: "+err.Error())
		return
	}

	// Sonucu JSON-uyumlu döndür
	dstCodec, err := s.codecReg.Get(toFmt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	decoded, err := dstCodec.Decode(result)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hedef decode hatası: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":      decoded,
		"from":      req.From,
		"to":        req.To,
		"src_bytes": len(srcBytes),
		"dst_bytes": len(result),
	})
}

// POST /api/validate
// Body: {"schema": "sensor_data", "version": 1, "data": {...}}
func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.requestCount, 1)
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "sadece POST")
		return
	}

	var req struct {
		Schema  string                 `json:"schema"`
		Version int                    `json:"version"`
		Data    map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "geçersiz JSON: "+err.Error())
		return
	}

	errs := s.schemaReg.Validate(req.Schema, req.Version, req.Data)
	valid := len(errs) == 0

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":  valid,
		"errors": errs,
	})
}

// GET /api/formats
func (s *Server) handleFormats(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.requestCount, 1)
	fmts := s.codecReg.Formats()
	names := make([]string, len(fmts))
	for i, f := range fmts {
		names[i] = string(f)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"formats": names,
	})
}

// GET /api/stats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.requestCount, 1)
	adapterStats := s.adapter.GetStats()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"uptime":          time.Since(s.startTime).String(),
		"total_requests":  atomic.LoadInt64(&s.requestCount),
		"adapter_stats":   adapterStats,
		"schema_count":    s.schemaReg.Count(),
		"pipeline_count":  s.pipelineReg.Count(),
		"module_count":    len(s.adapter.ListModules()),
		"format_count":    len(s.codecReg.Formats()),
	})
}

// GET/POST /api/schemas
func (s *Server) handleSchemas(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.requestCount, 1)
	switch r.Method {
	case "GET":
		names := s.schemaReg.List()
		schemas := make([]interface{}, 0)
		for _, name := range names {
			if latest, err := s.schemaReg.GetLatest(name); err == nil {
				schemas = append(schemas, latest)
			}
		}
		writeJSON(w, http.StatusOK, schemas)

	case "POST":
		var req struct {
			Name   string         `json:"name"`
			Fields []schema.Field `json:"fields"`
			Compat string         `json:"compat,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "geçersiz JSON: "+err.Error())
			return
		}
		if req.Compat != "" {
			mode, err := schema.ParseCompatMode(req.Compat)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			s.schemaReg.SetCompat(req.Name, mode)
		}
		sc, err := s.schemaReg.Register(req.Name, req.Fields)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, sc)

	default:
		writeError(w, http.StatusMethodNotAllowed, "GET veya POST")
	}
}

// GET/DELETE /api/schemas/{name} veya /api/schemas/{name}/{version}
func (s *Server) handleSchemaByName(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.requestCount, 1)
	path := strings.TrimPrefix(r.URL.Path, "/api/schemas/")
	parts := strings.SplitN(path, "/", 2)
	name := parts[0]

	if name == "" {
		writeError(w, http.StatusBadRequest, "şema adı gerekli")
		return
	}

	switch r.Method {
	case "GET":
		if len(parts) == 2 {
			// Belirli versiyon
			ver, err := strconv.Atoi(parts[1])
			if err != nil {
				writeError(w, http.StatusBadRequest, "geçersiz versiyon numarası")
				return
			}
			sc, err := s.schemaReg.Get(name, ver)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, sc)
		} else {
			// En son versiyon
			sc, err := s.schemaReg.GetLatest(name)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, sc)
		}

	case "DELETE":
		if err := s.schemaReg.Delete(name); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"deleted": name})

	default:
		writeError(w, http.StatusMethodNotAllowed, "GET veya DELETE")
	}
}

// GET/POST /api/modules
func (s *Server) handleModules(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.requestCount, 1)
	switch r.Method {
	case "GET":
		writeJSON(w, http.StatusOK, s.adapter.ListModules())

	case "POST":
		var info adapter.ModuleInfo
		if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
			writeError(w, http.StatusBadRequest, "geçersiz JSON: "+err.Error())
			return
		}
		if err := s.adapter.RegisterModule(info); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, info)

	default:
		writeError(w, http.StatusMethodNotAllowed, "GET veya POST")
	}
}

// POST /api/bridge
// Body: {"src": "butun", "dst": "bag", "path": "/api/publish", "data": {...}}
func (s *Server) handleBridge(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.requestCount, 1)
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "sadece POST")
		return
	}

	var req struct {
		Src  string      `json:"src"`
		Dst  string      `json:"dst"`
		Path string      `json:"path"`
		Data interface{} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "geçersiz JSON: "+err.Error())
		return
	}

	dataBytes, err := json.Marshal(req.Data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "veri encode hatası")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := s.adapter.Bridge(ctx, req.Src, req.Dst, req.Path, dataBytes)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	// JSON olarak döndür
	var decoded interface{}
	if err := json.Unmarshal(result, &decoded); err != nil {
		// Raw olarak döndür
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(result)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"result": decoded,
		"src":    req.Src,
		"dst":    req.Dst,
	})
}

// GET /api/pipelines
func (s *Server) handlePipelines(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.requestCount, 1)
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "sadece GET")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pipelines": s.pipelineReg.List(),
		"count":     s.pipelineReg.Count(),
	})
}

// POST /api/pipeline/execute
// Body: {"pipeline": "sensor_convert", "data": {...}}
func (s *Server) handlePipelineExecute(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.requestCount, 1)
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "sadece POST")
		return
	}

	var req struct {
		Pipeline string                 `json:"pipeline"`
		Data     map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "geçersiz JSON: "+err.Error())
		return
	}

	msg := pipeline.NewMessage(req.Data)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := s.pipelineReg.Execute(ctx, req.Pipeline, msg)
	if err != nil {
		status := http.StatusInternalServerError
		if result != nil {
			writeJSON(w, status, map[string]interface{}{
				"error":       err.Error(),
				"steps_run":   result.StepsRun,
				"duration_ms": result.Duration.Milliseconds(),
			})
			return
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"output":       result.Output,
		"steps_run":    result.StepsRun,
		"duration_ms":  result.Duration.Milliseconds(),
		"step_timings": result.StepTimings,
	})
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
