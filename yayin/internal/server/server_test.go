package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otukenmatris/yayin/internal/broadcast"
	"github.com/otukenmatris/yayin/internal/compress"
	"github.com/otukenmatris/yayin/internal/crypto"
	"github.com/otukenmatris/yayin/internal/fec"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	eng, _ := crypto.NewEngine(3)
	comp := compress.NewCompressor(6)
	fecEnc, _ := fec.NewEncoder(4, 256)
	bcast := broadcast.NewManager(eng, comp, fecEnc, 100, 100, 10*1024*1024, 100, 24*time.Hour)
	return NewServer(bcast, eng, comp, fecEnc)
}

func doJSON(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestHealth(t *testing.T) {
	s := newTestServer(t)
	rr := doJSON(t, s.Handler(), "GET", "/health", nil)
	if rr.Code != 200 {
		t.Errorf("Health status beklenen: 200, alınan: %d", rr.Code)
	}
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("Status beklenen: ok, alınan: %v", resp["status"])
	}
}

func TestStatus(t *testing.T) {
	s := newTestServer(t)
	rr := doJSON(t, s.Handler(), "GET", "/status", nil)
	if rr.Code != 200 {
		t.Errorf("Status code beklenen: 200, alınan: %d", rr.Code)
	}
}

func TestCreateAndListChannels(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler()

	// Kanal oluştur
	rr := doJSON(t, h, "POST", "/channels", map[string]any{
		"name": "emir", "description": "Emir kanalı", "priority": 3, "encrypted": true, "compressed": true, "fec": true,
	})
	if rr.Code != 201 {
		t.Fatalf("CreateChannel status: %d, body: %s", rr.Code, rr.Body.String())
	}

	var created map[string]any
	json.Unmarshal(rr.Body.Bytes(), &created)
	chID := created["id"].(string)

	// Listele
	rr = doJSON(t, h, "GET", "/channels", nil)
	if rr.Code != 200 {
		t.Fatalf("ListChannels status: %d", rr.Code)
	}

	// Detay
	rr = doJSON(t, h, "GET", "/channels/"+chID, nil)
	if rr.Code != 200 {
		t.Fatalf("GetChannel status: %d", rr.Code)
	}
}

func TestPublishAndHistory(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler()

	// Kanal
	rr := doJSON(t, h, "POST", "/channels", map[string]any{
		"name": "cmd", "encrypted": true, "compressed": true, "fec": true,
	})
	var ch map[string]any
	json.Unmarshal(rr.Body.Bytes(), &ch)
	chID := ch["id"].(string)

	// Abone
	doJSON(t, h, "POST", "/subscribe", map[string]any{"channel_id": chID, "unit_id": "drone-001"})

	// Yayınla
	rr = doJSON(t, h, "POST", "/publish", map[string]any{
		"channel_id": chID, "sender_id": "komutan", "content_type": "command",
		"payload": "Hedefe ilerle!", "priority": 3,
	})
	if rr.Code != 200 {
		t.Fatalf("Publish status: %d, body: %s", rr.Code, rr.Body.String())
	}
	var pub map[string]any
	json.Unmarshal(rr.Body.Bytes(), &pub)
	if pub["encrypted"] != true || pub["compressed"] != true {
		t.Errorf("Publish flag'leri yanlış: %v", pub)
	}

	// Geçmiş
	rr = doJSON(t, h, "GET", "/history/"+chID+"?n=10", nil)
	if rr.Code != 200 {
		t.Fatalf("History status: %d", rr.Code)
	}
}

func TestSubscribeUnsubscribe(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler()

	rr := doJSON(t, h, "POST", "/channels", map[string]any{"name": "test-sub"})
	var ch map[string]any
	json.Unmarshal(rr.Body.Bytes(), &ch)
	chID := ch["id"].(string)

	// Subscribe
	rr = doJSON(t, h, "POST", "/subscribe", map[string]any{"channel_id": chID, "unit_id": "tank-01"})
	if rr.Code != 200 {
		t.Fatalf("Subscribe status: %d", rr.Code)
	}

	// Subscribers
	rr = doJSON(t, h, "GET", "/channels/"+chID+"/subscribers", nil)
	if rr.Code != 200 {
		t.Fatalf("Subscribers status: %d", rr.Code)
	}

	// Unsubscribe
	rr = doJSON(t, h, "POST", "/unsubscribe", map[string]any{"channel_id": chID, "unit_id": "tank-01"})
	if rr.Code != 200 {
		t.Fatalf("Unsubscribe status: %d, body: %s", rr.Code, rr.Body.String())
	}
}

func TestCryptoEndpoints(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler()

	rr := doJSON(t, h, "POST", "/crypto/encrypt", map[string]string{"data": "gizli veri"})
	if rr.Code != 200 {
		t.Fatalf("Encrypt status: %d", rr.Code)
	}

	rr = doJSON(t, h, "GET", "/crypto/keys", nil)
	if rr.Code != 200 {
		t.Fatalf("Keys status: %d", rr.Code)
	}

	rr = doJSON(t, h, "POST", "/crypto/rotate", nil)
	if rr.Code != 200 {
		t.Fatalf("Rotate status: %d", rr.Code)
	}
}

func TestCompressEndpoint(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler()

	rr := doJSON(t, h, "POST", "/compress", map[string]string{"data": "sıkıştırılacak veri tekrar tekrar tekrar tekrar"})
	if rr.Code != 200 {
		t.Fatalf("Compress status: %d", rr.Code)
	}
}

func TestFECEndpoint(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler()

	rr := doJSON(t, h, "POST", "/fec/encode", map[string]string{"data": "ABCDEFGHIJKLMNOP"})
	if rr.Code != 200 {
		t.Fatalf("FEC encode status: %d", rr.Code)
	}
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["parity_blocks"].(float64) < 1 {
		t.Error("Parite bloğu bekleniyor")
	}
}

func TestDeleteChannel(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler()

	rr := doJSON(t, h, "POST", "/channels", map[string]any{"name": "silinecek"})
	var ch map[string]any
	json.Unmarshal(rr.Body.Bytes(), &ch)
	chID := ch["id"].(string)

	rr = doJSON(t, h, "DELETE", "/channels/"+chID, nil)
	if rr.Code != 200 {
		t.Fatalf("Delete status: %d", rr.Code)
	}

	rr = doJSON(t, h, "GET", "/channels/"+chID, nil)
	if rr.Code != 404 {
		t.Fatalf("Silinen kanal hâlâ var: %d", rr.Code)
	}
}
