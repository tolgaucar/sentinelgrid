package config

import (
	"time"

	"otukenmatris/ortam"
)

// Config — DİRLİK yapılandırması
type Config struct {
	// Metrik toplama aralığı
	CollectInterval time.Duration
	// Anomali pencere boyutu
	WindowSize int
	// Probe aralığı
	ProbeInterval time.Duration
	// Probe timeout
	ProbeTimeout time.Duration
	// Alarm eşikleri
	Thresholds Thresholds
	// HTTP port
	HTTPPort int
	// gRPC port
	GRPCPort int
	// BÜTÜN bağlantısı
	ButunURL string
	// BAĞ bağlantısı
	BagURL string
	// KÜTÜK bağlantısı
	KutukURL string
}

// Thresholds — Alarm eşik değerleri
type Thresholds struct {
	CPUPercent    float64 // CPU yüzde eşik (varsayılan 85)
	MemPercent    float64 // RAM yüzde eşik (varsayılan 90)
	DiskPercent   float64 // Disk doluluk eşik (varsayılan 95)
	DiskIOMs      float64 // Disk IO gecikme ms (varsayılan 100)
	NetLatencyMs  float64 // Ağ gecikme ms (varsayılan 200)
	ErrorRate     float64 // Hata oranı eşik (varsayılan 0.05 = %5)
	ProbeFailures int     // Ardışık probe hata sayısı (varsayılan 3)
}

// DefaultConfig — Varsayılan yapılandırma
func DefaultConfig() Config {
	return Config{
		CollectInterval: 1 * time.Second,
		WindowSize:      60,
		ProbeInterval:   5 * time.Second,
		ProbeTimeout:    3 * time.Second,
		Thresholds: Thresholds{
			CPUPercent:    85.0,
			MemPercent:    90.0,
			DiskPercent:   95.0,
			DiskIOMs:      100.0,
			NetLatencyMs:  200.0,
			ErrorRate:     0.05,
			ProbeFailures: 3,
		},
		HTTPPort: 9095,
		GRPCPort: 9094,
		ButunURL: "http://127.0.0.1:8000",
		BagURL:   "http://127.0.0.1:9091",
		KutukURL: "http://127.0.0.1:9093",
	}
}

// LoadFromEnv ortam değişkenlerinden yapılandırmayı günceller.
func (c *Config) LoadFromEnv() {
	c.HTTPPort = ortam.GetInt("DIRLIK_HTTP_PORT", c.HTTPPort)
	c.GRPCPort = ortam.GetInt("DIRLIK_GRPC_PORT", c.GRPCPort)
	c.ButunURL = ortam.GetStr("DIRLIK_BUTUN_URL", c.ButunURL)
	c.BagURL = ortam.GetStr("DIRLIK_BAG_URL", c.BagURL)
	c.KutukURL = ortam.GetStr("DIRLIK_KUTUK_URL", c.KutukURL)
}
