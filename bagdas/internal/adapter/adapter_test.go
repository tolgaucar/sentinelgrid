package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otukenmatris/bagdas/internal/codec"
)

func TestNewAdapter(t *testing.T) {
	reg := codec.NewRegistry()
	a := NewAdapter(reg)
	if a == nil {
		t.Fatal("adaptör nil")
	}
}

func TestRegisterModule(t *testing.T) {
	a := NewAdapter(codec.NewRegistry())
	err := a.RegisterModule(ModuleInfo{
		Name:    "test",
		BaseURL: "http://localhost:9999",
		Format:  codec.JSON,
	})
	if err != nil {
		t.Fatal(err)
	}

	mod, err := a.GetModule("test")
	if err != nil {
		t.Fatal(err)
	}
	if mod.Name != "test" {
		t.Errorf("name = %s", mod.Name)
	}
	if mod.Timeout != 5*time.Second {
		t.Errorf("default timeout = %v", mod.Timeout)
	}
	if mod.MaxRetries != 3 {
		t.Errorf("default retries = %d", mod.MaxRetries)
	}
}

func TestRegisterModule_EmptyName(t *testing.T) {
	a := NewAdapter(codec.NewRegistry())
	err := a.RegisterModule(ModuleInfo{BaseURL: "http://localhost"})
	if err == nil {
		t.Error("boş isim için hata beklendi")
	}
}

func TestRegisterModule_EmptyURL(t *testing.T) {
	a := NewAdapter(codec.NewRegistry())
	err := a.RegisterModule(ModuleInfo{Name: "test"})
	if err == nil {
		t.Error("boş URL için hata beklendi")
	}
}

func TestUnregisterModule(t *testing.T) {
	a := NewAdapter(codec.NewRegistry())
	a.RegisterModule(ModuleInfo{Name: "test", BaseURL: "http://localhost"})
	a.UnregisterModule("test")

	_, err := a.GetModule("test")
	if err == nil {
		t.Error("silinen modül bulunamazsınız")
	}
}

func TestGetModule_NotFound(t *testing.T) {
	a := NewAdapter(codec.NewRegistry())
	_, err := a.GetModule("nope")
	if err == nil {
		t.Error("bulunamayan modül hatası beklendi")
	}
}

func TestListModules(t *testing.T) {
	a := NewAdapter(codec.NewRegistry())
	a.RegisterModule(ModuleInfo{Name: "a", BaseURL: "http://a"})
	a.RegisterModule(ModuleInfo{Name: "b", BaseURL: "http://b"})

	mods := a.ListModules()
	if len(mods) != 2 {
		t.Errorf("module count = %d", len(mods))
	}
}

func TestRegisterDefaults(t *testing.T) {
	a := NewAdapter(codec.NewRegistry())
	a.RegisterDefaults()

	mods := a.ListModules()
	if len(mods) != 4 {
		t.Errorf("default module count = %d, beklenen 4", len(mods))
	}

	// Her modülün varlığını kontrol et
	for _, name := range []string{"butun", "bag", "kutuk", "dirlik"} {
		if _, err := a.GetModule(name); err != nil {
			t.Errorf("default modül %s bulunamadı", name)
		}
	}
}

func TestSend_JSONModule(t *testing.T) {
	// Test HTTP sunucusu
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer ts.Close()

	a := NewAdapter(codec.NewRegistry())
	a.RegisterModule(ModuleInfo{
		Name:    "test",
		BaseURL: ts.URL,
		Format:  codec.JSON,
	})

	data := []byte(`{"key":"value"}`)
	body, status, err := a.Send(context.Background(), "test", "/api/echo", data, codec.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 {
		t.Errorf("status = %d", status)
	}
	if len(body) == 0 {
		t.Error("boş yanıt")
	}
}

func TestSend_FormatConversion(t *testing.T) {
	var receivedContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	a := NewAdapter(codec.NewRegistry())
	a.RegisterModule(ModuleInfo{
		Name:    "msgpack_module",
		BaseURL: ts.URL,
		Format:  codec.MsgPack,
	})

	// JSON gönder → MsgPack'e dönüşmeli
	data := []byte(`{"sensor":"radar"}`)
	_, _, err := a.Send(context.Background(), "msgpack_module", "/data", data, codec.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if receivedContentType != "application/msgpack" {
		t.Errorf("content-type = %s, beklenen application/msgpack", receivedContentType)
	}

	// Dönüşüm sayısı artmalı
	stats := a.GetStats()
	if stats.TotalConverts != 1 {
		t.Errorf("converts = %d", stats.TotalConverts)
	}
}

func TestSend_ModuleNotFound(t *testing.T) {
	a := NewAdapter(codec.NewRegistry())
	_, _, err := a.Send(context.Background(), "nope", "/", nil, codec.JSON)
	if err == nil {
		t.Error("bulunamayan modül hatası beklendi")
	}
}

func TestSend_GETRequest(t *testing.T) {
	var method string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(200)
	}))
	defer ts.Close()

	a := NewAdapter(codec.NewRegistry())
	a.RegisterModule(ModuleInfo{Name: "test", BaseURL: ts.URL, Format: codec.JSON})

	// Boş body → GET olmalı
	_, _, err := a.Send(context.Background(), "test", "/health", nil, codec.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if method != "GET" {
		t.Errorf("method = %s, beklenen GET", method)
	}
}

func TestGetStats(t *testing.T) {
	a := NewAdapter(codec.NewRegistry())
	stats := a.GetStats()
	if stats.TotalRequests != 0 {
		t.Errorf("başlangıç requests = %d", stats.TotalRequests)
	}
}

func TestBridge_ModuleNotFound(t *testing.T) {
	a := NewAdapter(codec.NewRegistry())
	_, err := a.Bridge(context.Background(), "nope", "nope2", "/", nil)
	if err == nil {
		t.Error("hata beklendi")
	}
}

func TestSend_Headers(t *testing.T) {
	var headers http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header
		w.WriteHeader(200)
	}))
	defer ts.Close()

	a := NewAdapter(codec.NewRegistry())
	a.RegisterModule(ModuleInfo{Name: "test", BaseURL: ts.URL, Format: codec.JSON})

	a.Send(context.Background(), "test", "/api/data", []byte(`{"x":1}`), codec.MsgPack)

	if headers.Get("X-Bagdas-Module") != "test" {
		t.Error("X-Bagdas-Module header eksik")
	}
	if headers.Get("X-Source-Format") != "msgpack" {
		t.Errorf("X-Source-Format = %s", headers.Get("X-Source-Format"))
	}
}
