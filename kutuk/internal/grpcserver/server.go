// Package grpcserver KÜTÜK gRPC sunucusu ve HTTP durum API'si.
// Put/Get/Delete/Scan/Watch operasyonlarını dışarı açar.
package grpcserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/otukenmatris/kutuk/internal/raft"
	"github.com/otukenmatris/kutuk/internal/store"
	"github.com/otukenmatris/kutuk/internal/wal"

	"google.golang.org/grpc"
)

// Server KÜTÜK sunucusu - store, raft ve wal'ı birleştirir.
type Server struct {
	store    *store.Store
	raft     *raft.Node
	wal      *wal.WAL
	grpcSrv  *grpc.Server
	httpAddr string
}

// NewServer yeni sunucu oluştur.
func NewServer(s *store.Store, r *raft.Node, w *wal.WAL) *Server {
	return &Server{
		store: s,
		raft:  r,
		wal:   w,
	}
}

// ── gRPC Tanımları (protobuf yerine doğrudan struct) ──

// KVRequest anahtar-değer isteği.
type KVRequest struct {
	Namespace string
	Key       string
	Value     []byte
	TTLMs     int64 // Milisaniye
	Metadata  map[string]string
}

// KVResponse anahtar-değer yanıtı.
type KVResponse struct {
	Key       string
	Value     []byte
	Namespace string
	Version   uint64
	Found     bool
	Error     string
}

// ScanRequest tarama isteği.
type ScanRequest struct {
	Namespace string
	Prefix    string
	Limit     int
}

// ScanResponse tarama yanıtı.
type ScanResponse struct {
	Entries []EntryInfo
	Count   int
}

// EntryInfo giriş bilgisi.
type EntryInfo struct {
	Key       string            `json:"key"`
	Value     json.RawMessage   `json:"value"`
	Namespace string            `json:"namespace"`
	Version   uint64            `json:"version"`
	CreatedAt int64             `json:"created_at"`
	UpdatedAt int64             `json:"updated_at"`
	ExpiresAt int64             `json:"expires_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// StatusInfo durum bilgisi.
type StatusInfo struct {
	Module     string      `json:"module"`
	Version    string      `json:"version"`
	Uptime     string      `json:"uptime"`
	Store      store.Stats `json:"store"`
	Raft       raft.State  `json:"raft"`
	WALWrites  int64       `json:"wal_writes"`
	WALBytes   int64       `json:"wal_bytes"`
}

// ── gRPC Servis İmplementasyonu ──

// Put anahtar yaz.
func (s *Server) Put(ctx context.Context, req *KVRequest) (*KVResponse, error) {
	if !s.raft.IsLeader() {
		return &KVResponse{Error: "düğüm lider değil"}, nil
	}

	ttl := time.Duration(req.TTLMs) * time.Millisecond

	// WAL'a yaz
	if s.wal != nil {
		var ttlNano int64
		if ttl > 0 {
			ttlNano = int64(ttl)
		}
		if err := s.wal.AppendPut(req.Namespace, req.Key, req.Value, ttlNano); err != nil {
			log.Printf("[KÜTÜK] WAL yazma hatası: %v", err)
		}
	}

	// RAFT log'a ekle
	cmd, _ := json.Marshal(map[string]interface{}{
		"op": "put", "ns": req.Namespace, "key": req.Key,
		"value": req.Value, "ttl_ms": req.TTLMs,
	})
	if _, err := s.raft.Propose(cmd); err != nil {
		log.Printf("[KÜTÜK] RAFT propose hatası: %v", err)
		return &KVResponse{Error: fmt.Sprintf("raft propose: %v", err)}, nil
	}

	// Store'a yaz
	entry := s.store.Put(req.Namespace, req.Key, req.Value, ttl, req.Metadata)

	return &KVResponse{
		Key:       entry.Key,
		Namespace: entry.Namespace,
		Version:   entry.Version,
		Found:     true,
	}, nil
}

// Get anahtar oku.
func (s *Server) Get(ctx context.Context, req *KVRequest) (*KVResponse, error) {
	entry, ok := s.store.Get(req.Namespace, req.Key)
	if !ok {
		return &KVResponse{Key: req.Key, Found: false}, nil
	}
	return &KVResponse{
		Key:       entry.Key,
		Value:     entry.Value,
		Namespace: entry.Namespace,
		Version:   entry.Version,
		Found:     true,
	}, nil
}

// Delete anahtar sil.
func (s *Server) Delete(ctx context.Context, req *KVRequest) (*KVResponse, error) {
	if !s.raft.IsLeader() {
		return &KVResponse{Error: "düğüm lider değil"}, nil
	}

	// WAL'a yaz
	if s.wal != nil {
		s.wal.AppendDelete(req.Namespace, req.Key)
	}

	ok := s.store.Delete(req.Namespace, req.Key)
	return &KVResponse{Key: req.Key, Found: ok}, nil
}

// Scan tarama yap.
func (s *Server) Scan(ctx context.Context, req *ScanRequest) (*ScanResponse, error) {
	entries := s.store.Scan(req.Namespace, req.Prefix, req.Limit)
	infos := make([]EntryInfo, len(entries))
	for i, e := range entries {
		infos[i] = EntryInfo{
			Key:       e.Key,
			Value:     e.Value,
			Namespace: e.Namespace,
			Version:   e.Version,
			CreatedAt: e.CreatedAt,
			UpdatedAt: e.UpdatedAt,
			ExpiresAt: e.ExpiresAt,
			Metadata:  e.Metadata,
		}
	}
	return &ScanResponse{Entries: infos, Count: len(infos)}, nil
}

// ── HTTP Status Sunucusu ──

// StartHTTP HTTP durum sunucusunu başlat.
func (s *Server) StartHTTP(addr string) error {
	s.httpAddr = addr
	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/keys", s.handleKeys)
	mux.HandleFunc("/get", s.handleGet)
	mux.HandleFunc("/put", s.handlePut)
	mux.HandleFunc("/delete", s.handleDelete)
	mux.HandleFunc("/scan", s.handleScan)
	mux.HandleFunc("/namespaces", s.handleNamespaces)
	mux.HandleFunc("/raft", s.handleRaft)

	log.Printf("[KÜTÜK] HTTP durum sunucusu: %s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{
		"status": "healthy",
		"module": "KÜTÜK",
		"leader": s.raft.IsLeader(),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	walW, walB := int64(0), int64(0)
	if s.wal != nil {
		walW, walB = s.wal.Stats()
	}
	writeJSON(w, StatusInfo{
		Module:    "KÜTÜK",
		Version:   "1.0.0",
		Store:     s.store.GetStats(),
		Raft:      s.raft.GetState(),
		WALWrites: walW,
		WALBytes:  walB,
	})
}

func (s *Server) handleKeys(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns")
	keys := s.store.Keys(ns)
	writeJSON(w, map[string]interface{}{"keys": keys, "count": len(keys)})
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns")
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, `{"error":"key parametresi gerekli"}`, 400)
		return
	}
	entry, ok := s.store.Get(ns, key)
	if !ok {
		http.Error(w, `{"error":"bulunamadı"}`, 404)
		return
	}
	writeJSON(w, EntryInfo{
		Key:       entry.Key,
		Value:     entry.Value,
		Namespace: entry.Namespace,
		Version:   entry.Version,
		CreatedAt: entry.CreatedAt,
		UpdatedAt: entry.UpdatedAt,
		ExpiresAt: entry.ExpiresAt,
		Metadata:  entry.Metadata,
	})
}

func (s *Server) handlePut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"POST gerekli"}`, 405)
		return
	}
	var body struct {
		Namespace string            `json:"namespace"`
		Key       string            `json:"key"`
		Value     json.RawMessage   `json:"value"`
		TTLMs     int64             `json:"ttl_ms"`
		Metadata  map[string]string `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), 400)
		return
	}

	resp, err := s.Put(r.Context(), &KVRequest{
		Namespace: body.Namespace,
		Key:       body.Key,
		Value:     body.Value,
		TTLMs:     body.TTLMs,
		Metadata:  body.Metadata,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), 500)
		return
	}
	if resp.Error != "" {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, resp.Error), 500)
		return
	}
	writeJSON(w, resp)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, `{"error":"DELETE veya POST gerekli"}`, 405)
		return
	}
	ns := r.URL.Query().Get("ns")
	key := r.URL.Query().Get("key")
	resp, err := s.Delete(r.Context(), &KVRequest{Namespace: ns, Key: key})
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), 500)
		return
	}
	writeJSON(w, resp)
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns")
	prefix := r.URL.Query().Get("prefix")
	resp, err := s.Scan(r.Context(), &ScanRequest{
		Namespace: ns,
		Prefix:    prefix,
		Limit:     1000,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), 500)
		return
	}
	writeJSON(w, resp)
}

func (s *Server) handleNamespaces(w http.ResponseWriter, r *http.Request) {
	ns := s.store.Namespaces()
	writeJSON(w, map[string]interface{}{"namespaces": ns, "count": len(ns)})
}

func (s *Server) handleRaft(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.raft.GetState())
}

// StartGRPC gRPC sunucusunu başlat.
func (s *Server) StartGRPC(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("gRPC port dinlenemedi: %w", err)
	}
	s.grpcSrv = grpc.NewServer()
	log.Printf("[KÜTÜK] gRPC sunucusu: :%d", port)
	return s.grpcSrv.Serve(lis)
}

// Stop sunucuyu durdur.
func (s *Server) Stop() {
	if s.grpcSrv != nil {
		s.grpcSrv.GracefulStop()
	}
	s.raft.Stop()
	if s.wal != nil {
		s.wal.Close()
	}
	s.store.Close()
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
