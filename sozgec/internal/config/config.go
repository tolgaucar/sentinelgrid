// Package config — SÖZGEÇ yapılandırması.
package config

import (
	"os"
	"strconv"
)

type ServerConfig struct {
	Port     int
	GRPCPort int
}

type SpectrumConfig struct {
	FFTSize      int     // FFT pencere boyutu (2'nin kuvveti)
	SampleRate   float64 // örnekleme hızı (MHz)
	PeakMinDB    float64 // pik minimum genlik (dB)
	MaxBands     int     // azami frekans bandı
	WindowType   string  // pencere fonksiyonu (hanning, hamming, blackman)
	OverlapRatio float64 // FFT pencere örtüşme oranı
}

type FilterConfig struct {
	NoiseFloorDB    float64 // gürültü tabanı eşiği (dB)
	SNRMinDB        float64 // minimum sinyal-gürültü oranı (dB)
	MaxSignals      int     // azami eşzamanlı sinyal
	AdaptiveAlpha   float64 // uyarlanabilir filtre öğrenme katsayısı
	FriendlyBands   int     // dost frekans bandı sayısı (varsayılan)
	ThreatDecayS    int     // tehdit sönümlenme süresi (saniye)
}

type Config struct {
	Server   ServerConfig
	Spectrum SpectrumConfig
	Filter   FilterConfig
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
			Port:     envInt("SOZGEC_PORT", 9124),
			GRPCPort: envInt("SOZGEC_GRPC_PORT", 9125),
		},
		Spectrum: SpectrumConfig{
			FFTSize:      envInt("SOZGEC_FFT_SIZE", 4096),
			SampleRate:   envFloat("SOZGEC_SAMPLE_RATE", 100.0),
			PeakMinDB:    envFloat("SOZGEC_PEAK_MIN_DB", -60.0),
			MaxBands:     envInt("SOZGEC_MAX_BANDS", 256),
			WindowType:   "hanning",
			OverlapRatio: 0.5,
		},
		Filter: FilterConfig{
			NoiseFloorDB:  envFloat("SOZGEC_NOISE_FLOOR_DB", -80.0),
			SNRMinDB:      envFloat("SOZGEC_SNR_MIN_DB", 6.0),
			MaxSignals:    envInt("SOZGEC_MAX_SIGNALS", 500),
			AdaptiveAlpha: envFloat("SOZGEC_ADAPTIVE_ALPHA", 0.05),
			FriendlyBands: envInt("SOZGEC_FRIENDLY_BANDS", 8),
			ThreatDecayS:  envInt("SOZGEC_THREAT_DECAY", 3600),
		},
	}
}
