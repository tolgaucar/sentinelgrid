// Package config — SEZİ modülü yapılandırması.
package config

import (
	"otukenmatris/ortam"
)

// Config — SEZİ modülü yapılandırması.
type Config struct {
	Server   ServerConfig
	Texture  TextureConfig
	Detector DetectorConfig
}

// ServerConfig — HTTP sunucu ayarları.
type ServerConfig struct {
	Port     int
	GRPCPort int
}

// TextureConfig — doku analiz motoru ayarları.
type TextureConfig struct {
	GaborScales      int     // Gabor filtre ölçek sayısı
	GaborOrients     int     // Gabor filtre yönelim sayısı
	GaborMinFreq     float64 // minimum Gabor frekansı
	GaborMaxFreq     float64 // maksimum Gabor frekansı
	BlockSize        int     // doku analiz blok boyutu (piksel)
	VarianceThreshold float64 // doğal/yapay ayrım eşiği
}

// DetectorConfig — kamuflaj tespit motoru ayarları.
type DetectorConfig struct {
	MaxAge           int     // tespit kayıt süresi (saniye)
	ConfidenceMin    float64 // minimum güven eşiği
	ConvexThreshold  float64 // dışbükey nesne anormallik eşiği
	ConcaveThreshold float64 // içbükey nesne anormallik eşiği
	MaxDetections    int     // maksimum takip edilen tespit sayısı
	PruneInterval    int     // temizlik aralığı (saniye)
}

// Load — yapılandırma yükle (varsayılan + ortam değişkenleri).
func Load() Config {
	c := Config{
		Server: ServerConfig{
			Port:     9116,
			GRPCPort: 9117,
		},
		Texture: TextureConfig{
			GaborScales:      4,
			GaborOrients:     6,
			GaborMinFreq:     0.05,
			GaborMaxFreq:     0.40,
			BlockSize:        16,
			VarianceThreshold: 0.35,
		},
		Detector: DetectorConfig{
			MaxAge:           600,
			ConfidenceMin:    0.3,
			ConvexThreshold:  0.60,
			ConcaveThreshold: 0.55,
			MaxDetections:    500,
			PruneInterval:    30,
		},
	}
	c.Server.Port = ortam.GetInt("SEZI_PORT", c.Server.Port)
	c.Server.GRPCPort = ortam.GetInt("SEZI_GRPC_PORT", c.Server.GRPCPort)
	c.Texture.GaborScales = ortam.GetInt("SEZI_GABOR_SCALES", c.Texture.GaborScales)
	c.Texture.GaborOrients = ortam.GetInt("SEZI_GABOR_ORIENTS", c.Texture.GaborOrients)
	c.Texture.VarianceThreshold = ortam.GetFloat("SEZI_VARIANCE_THRESHOLD", c.Texture.VarianceThreshold)
	c.Detector.MaxDetections = ortam.GetInt("SEZI_MAX_DETECTIONS", c.Detector.MaxDetections)
	c.Detector.ConfidenceMin = ortam.GetFloat("SEZI_CONFIDENCE_MIN", c.Detector.ConfidenceMin)
	return c
}
