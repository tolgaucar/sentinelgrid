// Package config — DUYUM modülü yapılandırması.
package config

import (
	"otukenmatris/ortam"
)

// Config — DUYUM modülü yapılandırması.
type Config struct {
	Server    ServerConfig
	Collector CollectorConfig
	Analyzer  AnalyzerConfig
}

// ServerConfig — HTTP sunucu ayarları.
type ServerConfig struct {
	Port     int
	GRPCPort int
}

// CollectorConfig — OSINT veri toplama ayarları.
type CollectorConfig struct {
	MaxSources      int     // maksimum kaynak sayısı
	RefreshInterval int     // varsayılan yenileme aralığı (saniye)
	MaxItems        int     // bellekte tutulacak maks kayıt
	MinCredibility  float64 // minimum güvenilirlik eşiği
	PruneAge        int     // temizlik yaşı (saniye)
}

// AnalyzerConfig — semantik analiz ayarları.
type AnalyzerConfig struct {
	MaxReports       int     // bellekte tutulacak maks rapor
	ConfidenceMin    float64 // minimum güven eşiği
	CrossRefMin      int     // çapraz doğrulama için min kaynak
	TrendWindowTicks int     // trend penceresi (tick)
	ThreatDecayS     int     // tehdit bozunma süresi (saniye)
}

// Load — yapılandırma yükle (varsayılan + ortam değişkenleri).
func Load() Config {
	c := Config{
		Server: ServerConfig{
			Port:     9120,
			GRPCPort: 9121,
		},
		Collector: CollectorConfig{
			MaxSources:      100,
			RefreshInterval: 300,
			MaxItems:        5000,
			MinCredibility:  0.2,
			PruneAge:        3600,
		},
		Analyzer: AnalyzerConfig{
			MaxReports:       1000,
			ConfidenceMin:    0.3,
			CrossRefMin:      2,
			TrendWindowTicks: 50,
			ThreatDecayS:     1800,
		},
	}
	c.Server.Port = ortam.GetInt("DUYUM_PORT", c.Server.Port)
	c.Server.GRPCPort = ortam.GetInt("DUYUM_GRPC_PORT", c.Server.GRPCPort)
	c.Collector.MaxSources = ortam.GetInt("DUYUM_MAX_SOURCES", c.Collector.MaxSources)
	c.Collector.MaxItems = ortam.GetInt("DUYUM_MAX_ITEMS", c.Collector.MaxItems)
	c.Collector.MinCredibility = ortam.GetFloat("DUYUM_MIN_CREDIBILITY", c.Collector.MinCredibility)
	c.Analyzer.MaxReports = ortam.GetInt("DUYUM_MAX_REPORTS", c.Analyzer.MaxReports)
	c.Analyzer.ConfidenceMin = ortam.GetFloat("DUYUM_CONFIDENCE_MIN", c.Analyzer.ConfidenceMin)
	return c
}
