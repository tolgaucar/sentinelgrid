// Package config — TANI modülü yapılandırması.
package config

import (
	"time"

	"otukenmatris/ortam"
)

// Config TANI modülü yapılandırması.
type Config struct {
	Server    ServerConfig    `json:"server"`
	Detector  DetectorConfig  `json:"detector"`
	Classifier ClassifierConfig `json:"classifier"`
	Tracker   TrackerConfig   `json:"tracker"`
}

// ServerConfig sunucu ayarları.
type ServerConfig struct {
	Host            string        `json:"host"`
	GRPCPort        int           `json:"grpc_port"`
	HTTPPort        int           `json:"http_port"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// DetectorConfig nesne tespit ayarları.
type DetectorConfig struct {
	ConfidenceThreshold float64       `json:"confidence_threshold"`
	NMSThreshold        float64       `json:"nms_threshold"`
	MaxDetections       int           `json:"max_detections"`
	InputWidth          int           `json:"input_width"`
	InputHeight         int           `json:"input_height"`
	ModelName           string        `json:"model_name"`
	InferenceTimeout    time.Duration `json:"inference_timeout"`
}

// ClassifierConfig sınıflandırma ayarları.
type ClassifierConfig struct {
	MaxClasses       int     `json:"max_classes"`
	IFFEnabled       bool    `json:"iff_enabled"`
	MinConfidence    float64 `json:"min_confidence"`
	HistorySize      int     `json:"history_size"`
}

// TrackerConfig izleme ayarları.
type TrackerConfig struct {
	MaxTracks        int           `json:"max_tracks"`
	MaxLostFrames    int           `json:"max_lost_frames"`
	IOUThreshold     float64       `json:"iou_threshold"`
	HistorySize      int           `json:"history_size"`
	PruneInterval    time.Duration `json:"prune_interval"`
}

// DefaultConfig varsayılan yapılandırma.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			GRPCPort:        9112,
			HTTPPort:        9113,
			ShutdownTimeout: 10 * time.Second,
		},
		Detector: DetectorConfig{
			ConfidenceThreshold: 0.45,
			NMSThreshold:        0.50,
			MaxDetections:       256,
			InputWidth:          640,
			InputHeight:         640,
			ModelName:           "YOLOv7-military",
			InferenceTimeout:    500 * time.Millisecond,
		},
		Classifier: ClassifierConfig{
			MaxClasses:    64,
			IFFEnabled:    true,
			MinConfidence: 0.40,
			HistorySize:   10000,
		},
		Tracker: TrackerConfig{
			MaxTracks:     1024,
			MaxLostFrames: 30,
			IOUThreshold:  0.35,
			HistorySize:   500,
			PruneInterval: 10 * time.Second,
		},
	}
}

// LoadFromEnv ortam değişkenlerinden günceller.
func (c *Config) LoadFromEnv() {
	c.Server.GRPCPort = ortam.GetInt("TANI_GRPC_PORT", c.Server.GRPCPort)
	c.Server.HTTPPort = ortam.GetInt("TANI_HTTP_PORT", c.Server.HTTPPort)
	c.Detector.MaxDetections = ortam.GetInt("TANI_MAX_DETECTIONS", c.Detector.MaxDetections)
	c.Tracker.MaxTracks = ortam.GetInt("TANI_MAX_TRACKS", c.Tracker.MaxTracks)

	if ct := ortam.GetInt("TANI_CONFIDENCE", 0); ct > 0 {
		c.Detector.ConfidenceThreshold = float64(ct) / 100.0
	}
	if iff := ortam.GetStr("TANI_IFF_ENABLED", ""); iff == "false" {
		c.Classifier.IFFEnabled = false
	}
}
