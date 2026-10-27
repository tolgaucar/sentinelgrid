// Package config — YÜZEY yapılandırması.
package config

import (
	"time"

	"otukenmatris/ortam"
)

// ModuleEndpoint — bir modülün bağlantı bilgisi
type ModuleEndpoint struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Enabled bool   `json:"enabled"`
}

// URL — modülün tam adresi
func (m ModuleEndpoint) URL() string {
	return m.Host + ":" + itoa(m.Port)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	b := make([]byte, 0, 5)
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// Config — YÜZEY konfigürasyonu
type Config struct {
	HTTPPort int
	WSPort   int
	NodeID   string

	// Harita merkezi (varsayılan: Ankara)
	CenterLat float64
	CenterLon float64
	RadiusKm  float64

	// Modül yoklama
	PollInterval    time.Duration // modülleri yoklama sıklığı
	HealthCheckFreq time.Duration // sağlık kontrolü sıklığı
	TimeoutMs       int           // HTTP istek timeout

	// Görselleştirme
	MaxEntities    int // haritada gösterilecek maks varlık
	MaxEvents      int // tutulacak maks olay
	MaxTrailPoints int // varlık iz noktası sayısı

	// WebSocket
	WSPingInterval time.Duration
	WSMaxClients   int

	// Modüller
	Modules map[string]ModuleEndpoint
}

// DefaultConfig — varsayılan konfigürasyon
func DefaultConfig() Config {
	return Config{
		HTTPPort:        9103,
		WSPort:          9103,
		NodeID:          "yuzey-main",
		CenterLat:       39.925,
		CenterLon:       32.837,
		RadiusKm:        50.0,
		PollInterval:    2 * time.Second,
		HealthCheckFreq: 5 * time.Second,
		TimeoutMs:       3000,
		MaxEntities:     5000,
		MaxEvents:       10000,
		MaxTrailPoints:  100,
		WSPingInterval:  15 * time.Second,
		WSMaxClients:    50,
		Modules: map[string]ModuleEndpoint{
			"butun":   {Name: "BÜTÜN", Host: "http://127.0.0.1", Port: 8000, Enabled: true},
			"bag":     {Name: "BAĞ", Host: "http://127.0.0.1", Port: 9091, Enabled: true},
			"kutuk":   {Name: "KÜTÜK", Host: "http://127.0.0.1", Port: 9093, Enabled: true},
			"dirlik":  {Name: "DİRLİK", Host: "http://127.0.0.1", Port: 9095, Enabled: true},
			"bagdas":  {Name: "BAĞDAŞ", Host: "http://127.0.0.1", Port: 9097, Enabled: true},
			"akim":    {Name: "AKIM", Host: "http://127.0.0.1", Port: 9099, Enabled: true},
			"yurt":    {Name: "YURT", Host: "http://127.0.0.1", Port: 9101, Enabled: true},
			"im":      {Name: "İM", Host: "http://127.0.0.1", Port: 9105, Enabled: true},
			"yayin":   {Name: "YAYIN", Host: "http://127.0.0.1", Port: 9107, Enabled: true},
			"durak":   {Name: "DURAK", Host: "http://127.0.0.1", Port: 9109, Enabled: true},
			"tutum":   {Name: "TUTUM", Host: "http://127.0.0.1", Port: 9111, Enabled: true},
			"tani":    {Name: "TANI", Host: "http://127.0.0.1", Port: 9113, Enabled: true},
			"koz":     {Name: "KÖZ", Host: "http://127.0.0.1", Port: 9115, Enabled: true},
			"sezi":    {Name: "SEZİ", Host: "http://127.0.0.1", Port: 9116, Enabled: true},
			"yanki":   {Name: "YANKI", Host: "http://127.0.0.1", Port: 9118, Enabled: true},
			"duyum":   {Name: "DUYUM", Host: "http://127.0.0.1", Port: 9120, Enabled: true},
			"baskala": {Name: "BAŞKALA", Host: "http://127.0.0.1", Port: 9122, Enabled: true},
			"sozgec":  {Name: "SÖZGEÇ", Host: "http://127.0.0.1", Port: 9124, Enabled: true},
			"izlek":   {Name: "İZLEK", Host: "http://127.0.0.1", Port: 9126, Enabled: true},
		},
	}
}

// LoadFromEnv ortam değişkenlerinden yapılandırmayı günceller.
func (c *Config) LoadFromEnv() {
	c.HTTPPort = ortam.GetInt("YUZEY_HTTP_PORT", c.HTTPPort)
	c.NodeID = ortam.GetStr("YUZEY_NODE_ID", c.NodeID)
}

// ModuleStatus — modül durum bilgisi
type ModuleStatus struct {
	Name      string  `json:"name"`
	ID        string  `json:"id"`
	URL       string  `json:"url"`
	Alive     bool    `json:"alive"`
	LatencyMs float64 `json:"latency_ms"`
	Details   map[string]interface{} `json:"details,omitempty"`
	LastCheck int64   `json:"last_check"` // unix ms
}

// Entity — haritada görüntülenen varlık
type Entity struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Faction      string  `json:"faction"`
	Status       string  `json:"status"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	Heading      float64 `json:"heading"`
	Speed        float64 `json:"speed"`
	Health       float64 `json:"health"`
	Fuel         float64 `json:"fuel"`
	Altitude     float64 `json:"altitude,omitempty"`
	LastUpdate   int64   `json:"last_update"`
	IntelLevel   int     `json:"intel_level,omitempty"` // 0-4: 0=yok, 1=konum, 2=tür, 3=detay, 4=tam
}

// TacticalEvent — taktiksel olay
type TacticalEvent struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Source    string `json:"source"` // hangi modülden
	Severity  string `json:"severity"` // info, warning, critical
	Title     string `json:"title"`
	Detail    string `json:"detail,omitempty"`
	Lat       float64 `json:"lat,omitempty"`
	Lon       float64 `json:"lon,omitempty"`
	Timestamp int64   `json:"timestamp"`
}
