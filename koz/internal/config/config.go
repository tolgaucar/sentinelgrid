// Package config — KÖZ modülü yapılandırması.
package config

import (
	"otukenmatris/ortam"
)

// Config — KÖZ modülü yapılandırması.
type Config struct {
	Server   ServerConfig
	Thermal  ThermalConfig
	Analyzer AnalyzerConfig
}

// ServerConfig — HTTP sunucu ayarları.
type ServerConfig struct {
	Port     int
	GRPCPort int
}

// ThermalConfig — termal iz motoru ayarları.
type ThermalConfig struct {
	AmbientC   float64 // ortam sıcaklığı (°C)
	MinDeltaC  float64 // minimum tespit eşiği (°C fark)
	MaxAge     int     // iz kayıt süresi (saniye)
	Resolution int     // sensör çözünürlüğü (piksel)
	FOV        float64 // sensör görüş alanı (derece)
	FPS        int     // sensör kare hızı
}

// AnalyzerConfig — analiz motoru ayarları.
type AnalyzerConfig struct {
	CoolingThreshold float64 // soğuma hızı eşiği (°C/s)
	MaxSignatures    int     // maksimum takip edilen iz sayısı
	ClusterRadiusM   float64 // kümeleme yarıçapı (metre)
	HistoryDepth     int     // geçmiş ölçüm derinliği
}

// Load — yapılandırma yükle (varsayılan + ortam değişkenleri).
func Load() Config {
	c := Config{
		Server: ServerConfig{
			Port:     9115,
			GRPCPort: 9114,
		},
		Thermal: ThermalConfig{
			AmbientC:   22.0,
			MinDeltaC:  2.0,
			MaxAge:     300,
			Resolution: 640,
			FOV:        24.0,
			FPS:        60,
		},
		Analyzer: AnalyzerConfig{
			CoolingThreshold: 0.5,
			MaxSignatures:    500,
			ClusterRadiusM:   500.0,
			HistoryDepth:     60,
		},
	}
	c.Server.Port = ortam.GetInt("KOZ_PORT", c.Server.Port)
	c.Server.GRPCPort = ortam.GetInt("KOZ_GRPC_PORT", c.Server.GRPCPort)
	c.Thermal.AmbientC = ortam.GetFloat("KOZ_AMBIENT_TEMP", c.Thermal.AmbientC)
	c.Thermal.MinDeltaC = ortam.GetFloat("KOZ_MIN_DELTA", c.Thermal.MinDeltaC)
	c.Analyzer.MaxSignatures = ortam.GetInt("KOZ_MAX_SIGNATURES", c.Analyzer.MaxSignatures)
	return c
}
