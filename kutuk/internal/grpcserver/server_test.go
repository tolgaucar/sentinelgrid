package grpcserver

import (
	"context"
	"encoding/json"
	"github.com/otukenmatris/kutuk/internal/raft"
	"github.com/otukenmatris/kutuk/internal/store"
	"testing"
)

func newTestServer() *Server {
	s := store.New()
	r := raft.NewNode("test-1", nil, nil)
	r.Start()
	return NewServer(s, r, nil)
}

func TestServerPutGet(t *testing.T) {
	srv := newTestServer()
	ctx := context.Background()

	// Put
	putResp, err := srv.Put(ctx, &KVRequest{
		Namespace: "entity",
		Key:       "tank-001",
		Value:     []byte(`{"health":100,"lat":39.9}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if putResp.Error != "" {
		t.Fatal(putResp.Error)
	}
	if putResp.Version == 0 {
		t.Fatal("versiyon 0 olmamalı")
	}

	// Get
	getResp, err := srv.Get(ctx, &KVRequest{
		Namespace: "entity",
		Key:       "tank-001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !getResp.Found {
		t.Fatal("kayıt bulunamadı")
	}

	var data map[string]interface{}
	json.Unmarshal(getResp.Value, &data)
	if data["health"] != float64(100) {
		t.Fatalf("yanlış değer: %v", data)
	}
}

func TestServerDelete(t *testing.T) {
	srv := newTestServer()
	ctx := context.Background()

	srv.Put(ctx, &KVRequest{Namespace: "entity", Key: "e1", Value: []byte("v")})
	delResp, _ := srv.Delete(ctx, &KVRequest{Namespace: "entity", Key: "e1"})
	if !delResp.Found {
		t.Fatal("silme başarısız")
	}

	getResp, _ := srv.Get(ctx, &KVRequest{Namespace: "entity", Key: "e1"})
	if getResp.Found {
		t.Fatal("silinen kayıt hala mevcut")
	}
}

func TestServerScan(t *testing.T) {
	srv := newTestServer()
	ctx := context.Background()

	srv.Put(ctx, &KVRequest{Namespace: "sensor", Key: "radar-001", Value: []byte(`"r1"`)})
	srv.Put(ctx, &KVRequest{Namespace: "sensor", Key: "radar-002", Value: []byte(`"r2"`)})
	srv.Put(ctx, &KVRequest{Namespace: "sensor", Key: "thermal-001", Value: []byte(`"t1"`)})

	resp, _ := srv.Scan(ctx, &ScanRequest{Namespace: "sensor", Prefix: "radar-", Limit: 10})
	if resp.Count != 2 {
		t.Fatalf("beklenen 2 sonuç, alınan: %d", resp.Count)
	}
}

func TestServerGetNotFound(t *testing.T) {
	srv := newTestServer()
	resp, _ := srv.Get(context.Background(), &KVRequest{Key: "nonexistent"})
	if resp.Found {
		t.Fatal("olmayan kayıt bulunmamalı")
	}
}

func TestServerPutWithTTL(t *testing.T) {
	srv := newTestServer()
	ctx := context.Background()

	srv.Put(ctx, &KVRequest{
		Namespace: "track",
		Key:       "t1",
		Value:     []byte("v"),
		TTLMs:     50,
	})

	resp, _ := srv.Get(ctx, &KVRequest{Namespace: "track", Key: "t1"})
	if !resp.Found {
		t.Fatal("kayıt hemen bulunmalı")
	}
}
