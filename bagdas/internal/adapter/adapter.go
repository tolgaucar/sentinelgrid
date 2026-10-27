// Package adapter — Polyglot servis adaptörleri.
//
// Farklı modüller arasında protokol köprüsü kurar:
// - Go modülleri (BAĞ, KÜTÜK, DİRLİK) ↔ Python modülleri (BÜTÜN, Simülasyon)
// - Format dönüştürme (JSON↔MsgPack↔CBOR)
// - Header/metadata propagation
// - Otomatik retry ve timeout
package adapter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/otukenmatris/bagdas/internal/codec"
)

// ModuleType — Modül dil tipi
type ModuleType string

const (
	ModulePython ModuleType = "python" // BÜTÜN, simülasyon
	ModuleGo     ModuleType = "go"     // BAĞ, KÜTÜK, DİRLİK
	ModuleCPP    ModuleType = "cpp"    // İleride NATO donanımı
)

// ModuleInfo — Modül bilgisi
type ModuleInfo struct {
	Name       string       `json:"name"`
	Type       ModuleType   `json:"type"`
	BaseURL    string       `json:"base_url"`
	Format     codec.Format `json:"format"`     // Modülün beklediği format
	Timeout    time.Duration `json:"timeout"`
	MaxRetries int          `json:"max_retries"`
}

// Adapter — Modül arası protokol adaptörü
type Adapter struct {
	mu       sync.RWMutex
	modules  map[string]*ModuleInfo
	registry *codec.Registry
	client   *http.Client
	stats    AdapterStats
}

// AdapterStats — İstatistikler
type AdapterStats struct {
	mu             sync.Mutex
	TotalRequests  int64            `json:"total_requests"`
	TotalErrors    int64            `json:"total_errors"`
	TotalConverts  int64            `json:"total_converts"`
	ModuleRequests map[string]int64 `json:"module_requests"`
}

// NewAdapter — Yeni adaptör oluştur
func NewAdapter(reg *codec.Registry) *Adapter {
	return &Adapter{
		modules:  make(map[string]*ModuleInfo),
		registry: reg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		stats: AdapterStats{
			ModuleRequests: make(map[string]int64),
		},
	}
}

// RegisterModule — Modül kaydet
func (a *Adapter) RegisterModule(info ModuleInfo) error {
	if info.Name == "" {
		return fmt.Errorf("modül adı boş")
	}
	if info.BaseURL == "" {
		return fmt.Errorf("modül URL'si boş")
	}
	if info.Timeout == 0 {
		info.Timeout = 5 * time.Second
	}
	if info.MaxRetries == 0 {
		info.MaxRetries = 3
	}
	if info.Format == "" {
		info.Format = codec.JSON
	}

	a.mu.Lock()
	a.modules[info.Name] = &info
	a.mu.Unlock()
	return nil
}

// UnregisterModule — Modül kaydını sil
func (a *Adapter) UnregisterModule(name string) {
	a.mu.Lock()
	delete(a.modules, name)
	a.mu.Unlock()
}

// GetModule — Modül bilgisi getir
func (a *Adapter) GetModule(name string) (*ModuleInfo, error) {
	a.mu.RLock()
	m, ok := a.modules[name]
	a.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("modül bulunamadı: %s", name)
	}
	return m, nil
}

// ListModules — Kayıtlı modülleri listele
func (a *Adapter) ListModules() []ModuleInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]ModuleInfo, 0, len(a.modules))
	for _, m := range a.modules {
		out = append(out, *m)
	}
	return out
}

// Send — Modüle format-uyumlu veri gönder
// data herhangi bir formatta olabilir, hedef modülün formatına otomatik dönüştürülür
func (a *Adapter) Send(ctx context.Context, moduleName, path string, data []byte, srcFormat codec.Format) ([]byte, int, error) {
	mod, err := a.GetModule(moduleName)
	if err != nil {
		return nil, 0, err
	}

	// Format dönüştürme
	var body []byte
	if srcFormat != mod.Format && len(data) > 0 {
		body, err = a.registry.Convert(data, srcFormat, mod.Format)
		if err != nil {
			return nil, 0, fmt.Errorf("format dönüşüm hatası: %w", err)
		}
		a.stats.mu.Lock()
		a.stats.TotalConverts++
		a.stats.mu.Unlock()
	} else {
		body = data
	}

	// HTTP isteği
	url := strings.TrimRight(mod.BaseURL, "/") + "/" + strings.TrimLeft(path, "/")

	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt <= mod.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			wait := time.Duration(1<<uint(attempt-1)) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(wait):
			}
		}

		var req *http.Request
		if len(body) > 0 {
			req, err = http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		} else {
			req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
		}
		if err != nil {
			return nil, 0, err
		}

		// Content-Type header
		switch mod.Format {
		case codec.JSON:
			req.Header.Set("Content-Type", "application/json")
		case codec.MsgPack:
			req.Header.Set("Content-Type", "application/msgpack")
		case codec.CBOR:
			req.Header.Set("Content-Type", "application/cbor")
		}

		req.Header.Set("X-Source-Format", string(srcFormat))
		req.Header.Set("X-Bagdas-Module", moduleName)

		resp, lastErr = a.client.Do(req)
		if lastErr == nil {
			break
		}
	}

	a.stats.mu.Lock()
	a.stats.TotalRequests++
	a.stats.ModuleRequests[moduleName]++
	a.stats.mu.Unlock()

	if lastErr != nil {
		a.stats.mu.Lock()
		a.stats.TotalErrors++
		a.stats.mu.Unlock()
		return nil, 0, fmt.Errorf("modül %s'e ulaşılamadı: %w", moduleName, lastErr)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("yanıt okunamadı: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// Bridge — Kaynak modülden hedef modüle veri köprüleme
func (a *Adapter) Bridge(ctx context.Context, srcModule, dstModule, path string, data []byte) ([]byte, error) {
	src, err := a.GetModule(srcModule)
	if err != nil {
		return nil, fmt.Errorf("kaynak modül hatası: %w", err)
	}

	respBody, statusCode, err := a.Send(ctx, dstModule, path, data, src.Format)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, fmt.Errorf("hedef modül %s hata döndü: HTTP %d", dstModule, statusCode)
	}

	// Yanıtı kaynak modülün formatına geri çevir
	dst, err := a.GetModule(dstModule)
	if err != nil {
		return nil, err
	}

	if dst.Format != src.Format && len(respBody) > 0 {
		respBody, err = a.registry.Convert(respBody, dst.Format, src.Format)
		if err != nil {
			return nil, fmt.Errorf("yanıt format dönüşüm hatası: %w", err)
		}
	}

	return respBody, nil
}

// GetStats — İstatistikleri döndür
func (a *Adapter) GetStats() AdapterStats {
	a.stats.mu.Lock()
	defer a.stats.mu.Unlock()
	cp := AdapterStats{
		TotalRequests:  a.stats.TotalRequests,
		TotalErrors:    a.stats.TotalErrors,
		TotalConverts:  a.stats.TotalConverts,
		ModuleRequests: make(map[string]int64),
	}
	for k, v := range a.stats.ModuleRequests {
		cp.ModuleRequests[k] = v
	}
	return cp
}

// RegisterDefaults — Varsayılan ÖTÜKEN-MATRİS modüllerini kaydet
func (a *Adapter) RegisterDefaults() {
	defaults := []ModuleInfo{
		{Name: "butun", Type: ModulePython, BaseURL: "http://127.0.0.1:8000", Format: codec.JSON, Timeout: 5 * time.Second},
		{Name: "bag", Type: ModuleGo, BaseURL: "http://127.0.0.1:9091", Format: codec.JSON, Timeout: 3 * time.Second},
		{Name: "kutuk", Type: ModuleGo, BaseURL: "http://127.0.0.1:9093", Format: codec.JSON, Timeout: 3 * time.Second},
		{Name: "dirlik", Type: ModuleGo, BaseURL: "http://127.0.0.1:9095", Format: codec.JSON, Timeout: 3 * time.Second},
	}
	for _, d := range defaults {
		a.RegisterModule(d)
	}
}
