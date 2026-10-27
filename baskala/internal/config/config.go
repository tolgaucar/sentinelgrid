// Package config — BAŞKALA yapılandırması.
package config

import (
	"os"
	"strconv"
)

type ServerConfig struct {
	Port     int
	GRPCPort int
}

type ImageryConfig struct {
	MaxImages      int     // azami depolanacak görüntü
	MaxPixels      int     // simülasyon grid boyutu (NxN)
	MinResolution  float64 // minimum çözünürlük (m/piksel)
	RetentionHours int     // görüntü saklama süresi (saat)
}

type ChangeConfig struct {
	MaxChanges     int     // azami değişim kaydı
	ConfidenceMin  float64 // minimum güven eşiği
	DiffThreshold  float64 // piksel fark eşiği (0-1)
	ClusterRadius  float64 // coğrafi kümeleme yarıçapı (km)
	ThreatDecayS   int     // tehdit sönümlenme süresi (saniye)
}

type FusionConfig struct {
	SARWeight     float64 // SAR füzyon ağırlığı
	OpticalWeight float64 // optik füzyon ağırlığı
	CrossRefBoost float64 // çapraz doğrulama güven artışı
	CloudPenalty  float64 // bulutlu optik ceza katsayısı
}

type Config struct {
	Server  ServerConfig
	Imagery ImageryConfig
	Change  ChangeConfig
	Fusion  FusionConfig
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func Load() Config {
	return Config{
		Server: ServerConfig{
			Port:     envInt("BASKALA_PORT", 9122),
			GRPCPort: envInt("BASKALA_GRPC_PORT", 9123),
		},
		Imagery: ImageryConfig{
			MaxImages:      envInt("BASKALA_MAX_IMAGES", 500),
			MaxPixels:      envInt("BASKALA_GRID_SIZE", 64),
			MinResolution:  envFloat("BASKALA_MIN_RESOLUTION", 0.5),
			RetentionHours: envInt("BASKALA_RETENTION_HOURS", 48),
		},
		Change: ChangeConfig{
			MaxChanges:    envInt("BASKALA_MAX_CHANGES", 1000),
			ConfidenceMin: envFloat("BASKALA_CONFIDENCE_MIN", 0.3),
			DiffThreshold: envFloat("BASKALA_DIFF_THRESHOLD", 0.15),
			ClusterRadius: envFloat("BASKALA_CLUSTER_RADIUS", 2.0),
			ThreatDecayS:  envInt("BASKALA_THREAT_DECAY", 3600),
		},
		Fusion: FusionConfig{
			SARWeight:     envFloat("BASKALA_SAR_WEIGHT", 0.5),
			OpticalWeight: envFloat("BASKALA_OPTICAL_WEIGHT", 0.5),
			CrossRefBoost: envFloat("BASKALA_CROSSREF_BOOST", 0.30),
			CloudPenalty:  envFloat("BASKALA_CLOUD_PENALTY", 0.40),
		},
	}
}
