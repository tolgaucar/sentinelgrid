// Package config — YANKI modülü yapılandırması.
package config

import (
	"otukenmatris/ortam"
)

// Config — YANKI modülü yapılandırması.
type Config struct {
	Server   ServerConfig
	Acoustic AcousticConfig
	Locator  LocatorConfig
}

// ServerConfig — HTTP sunucu ayarları.
type ServerConfig struct {
	Port     int
	GRPCPort int
}

// AcousticConfig — akustik algılama motoru ayarları.
type AcousticConfig struct {
	SampleRate       int     // örnekleme hızı (Hz)
	FFTSize          int     // FFT pencere boyutu
	MinAmplitudeDB   float64 // minimum tespit eşiği (dB)
	MaxRange         float64 // maksimum tespit menzili (metre)
	MatchedFilterLen int     // eşleştirilmiş filtre uzunluğu
	NoiseFloorDB     float64 // gürültü taban seviyesi (dB)
}

// LocatorConfig — konumlandırma motoru ayarları.
type LocatorConfig struct {
	MinSensors       int     // minimum sensör sayısı (üçgenleme için)
	SpeedOfSound     float64 // ses hızı (m/s, ortam koşullarına göre)
	MaxAge           int     // tespit kayıt süresi (saniye)
	MaxDetections    int     // maksimum takip edilen tespit
	TOATolerance     float64 // varış zamanı toleransı (milisaniye)
	PruneInterval    int     // temizlik aralığı (saniye)
	ConfidenceMin    float64 // minimum güven eşiği
}

// Load — yapılandırma yükle (varsayılan + ortam değişkenleri).
func Load() Config {
	c := Config{
		Server: ServerConfig{
			Port:     9118,
			GRPCPort: 9119,
		},
		Acoustic: AcousticConfig{
			SampleRate:       44100,
			FFTSize:          4096,
			MinAmplitudeDB:   30.0,
			MaxRange:         10000.0,
			MatchedFilterLen: 512,
			NoiseFloorDB:     20.0,
		},
		Locator: LocatorConfig{
			MinSensors:    3,
			SpeedOfSound:  343.0,
			MaxAge:        600,
			MaxDetections: 500,
			TOATolerance:  2.0,
			PruneInterval: 30,
			ConfidenceMin: 0.3,
		},
	}
	c.Server.Port = ortam.GetInt("YANKI_PORT", c.Server.Port)
	c.Server.GRPCPort = ortam.GetInt("YANKI_GRPC_PORT", c.Server.GRPCPort)
	c.Acoustic.MinAmplitudeDB = ortam.GetFloat("YANKI_MIN_AMPLITUDE", c.Acoustic.MinAmplitudeDB)
	c.Acoustic.MaxRange = ortam.GetFloat("YANKI_MAX_RANGE", c.Acoustic.MaxRange)
	c.Locator.SpeedOfSound = ortam.GetFloat("YANKI_SPEED_OF_SOUND", c.Locator.SpeedOfSound)
	c.Locator.MaxDetections = ortam.GetInt("YANKI_MAX_DETECTIONS", c.Locator.MaxDetections)
	c.Locator.ConfidenceMin = ortam.GetFloat("YANKI_CONFIDENCE_MIN", c.Locator.ConfidenceMin)
	return c
}
