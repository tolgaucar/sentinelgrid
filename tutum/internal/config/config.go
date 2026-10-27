// Package config — TUTUM modülü yapılandırması.
package config

import (
	"time"

	"otukenmatris/ortam"
)

// Config TUTUM modülü yapılandırması.
type Config struct {
	Server    ServerConfig    `json:"server"`
	Battery   BatteryConfig   `json:"battery"`
	Resource  ResourceConfig  `json:"resource"`
	Predictor PredictorConfig `json:"predictor"`
	Alert     AlertConfig     `json:"alert"`
}

// ServerConfig sunucu ayarları.
type ServerConfig struct {
	Host            string        `json:"host"`
	GRPCPort        int           `json:"grpc_port"`
	HTTPPort        int           `json:"http_port"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// BatteryConfig batarya izleme ayarları.
type BatteryConfig struct {
	MaxPlatforms       int           `json:"max_platforms"`
	SampleInterval     time.Duration `json:"sample_interval"`
	HistorySize        int           `json:"history_size"`
	CriticalPercent    float64       `json:"critical_percent"`
	WarningPercent     float64       `json:"warning_percent"`
	LowPercent         float64       `json:"low_percent"`
	DischargeRateAlpha float64       `json:"discharge_rate_alpha"` // EMA katsayısı
}

// ResourceConfig kaynak izleme ayarları.
type ResourceConfig struct {
	CPUWarning     float64       `json:"cpu_warning"`
	CPUCritical    float64       `json:"cpu_critical"`
	MemWarning     float64       `json:"mem_warning"`
	MemCritical    float64       `json:"mem_critical"`
	TempWarning    float64       `json:"temp_warning"`    // °C
	TempCritical   float64       `json:"temp_critical"`
	SampleInterval time.Duration `json:"sample_interval"`
	HistorySize    int           `json:"history_size"`
}

// PredictorConfig tahmin motoru ayarları.
type PredictorConfig struct {
	MinSamples       int           `json:"min_samples"`        // Tahmin için min örnek
	WindowSize       int           `json:"window_size"`        // Regresyon pencere boyutu
	PredictInterval  time.Duration `json:"predict_interval"`   // Tahmin periyodu
	RTBReserveMin    float64       `json:"rtb_reserve_min"`    // Geri dönüş için min % batarya
	MissionCheckSec  int           `json:"mission_check_sec"`  // Görev kontrol periyodu
}

// AlertConfig uyarı yönetimi ayarları.
type AlertConfig struct {
	MaxAlerts        int           `json:"max_alerts"`
	RetentionPeriod  time.Duration `json:"retention_period"`
	DeduplicateWindow time.Duration `json:"deduplicate_window"`
	AutoRTB          bool          `json:"auto_rtb"`           // Otomatik geri dönüş önerisi
}

// DefaultConfig varsayılan yapılandırma.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			GRPCPort:        9110,
			HTTPPort:        9111,
			ShutdownTimeout: 10 * time.Second,
		},
		Battery: BatteryConfig{
			MaxPlatforms:       256,
			SampleInterval:     5 * time.Second,
			HistorySize:        1000,
			CriticalPercent:    10.0,
			WarningPercent:     25.0,
			LowPercent:        40.0,
			DischargeRateAlpha: 0.3, // EMA: 0.3 yeni + 0.7 eski
		},
		Resource: ResourceConfig{
			CPUWarning:     70.0,
			CPUCritical:    90.0,
			MemWarning:     75.0,
			MemCritical:    90.0,
			TempWarning:    65.0,
			TempCritical:   80.0,
			SampleInterval: 5 * time.Second,
			HistorySize:    500,
		},
		Predictor: PredictorConfig{
			MinSamples:      10,
			WindowSize:      50,
			PredictInterval: 10 * time.Second,
			RTBReserveMin:   15.0,
			MissionCheckSec: 30,
		},
		Alert: AlertConfig{
			MaxAlerts:         10000,
			RetentionPeriod:   24 * time.Hour,
			DeduplicateWindow: 30 * time.Second,
			AutoRTB:           true,
		},
	}
}

// LoadFromEnv ortam değişkenlerinden günceller.
func (c *Config) LoadFromEnv() {
	c.Server.GRPCPort = ortam.GetInt("TUTUM_GRPC_PORT", c.Server.GRPCPort)
	c.Server.HTTPPort = ortam.GetInt("TUTUM_HTTP_PORT", c.Server.HTTPPort)
	c.Battery.MaxPlatforms = ortam.GetInt("TUTUM_MAX_PLATFORMS", c.Battery.MaxPlatforms)

	if cp := ortam.GetInt("TUTUM_BATTERY_CRITICAL", 0); cp > 0 {
		c.Battery.CriticalPercent = float64(cp)
	}
	if wp := ortam.GetInt("TUTUM_BATTERY_WARNING", 0); wp > 0 {
		c.Battery.WarningPercent = float64(wp)
	}
	if ar := ortam.GetStr("TUTUM_AUTO_RTB", ""); ar == "false" {
		c.Alert.AutoRTB = false
	}
}
