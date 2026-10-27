// Package config — YAYIN modülü yapılandırması.
package config

import (
	"time"

	"otukenmatris/ortam"
)

// Config YAYIN modülü yapılandırması.
type Config struct {
	Server   ServerConfig   `json:"server"`
	Crypto   CryptoConfig   `json:"crypto"`
	Compress CompressConfig `json:"compress"`
	FEC      FECConfig      `json:"fec"`
	Bcast    BroadcastConfig `json:"broadcast"`
}

// ServerConfig sunucu ayarları.
type ServerConfig struct {
	Host            string        `json:"host"`
	GRPCPort        int           `json:"grpc_port"`
	HTTPPort        int           `json:"http_port"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// CryptoConfig şifreleme ayarları.
type CryptoConfig struct {
	Algorithm        string        `json:"algorithm"`          // AES-256-GCM
	KeyRotationEvery time.Duration `json:"key_rotation_every"` // Anahtar döndürme sıklığı
	MaxKeys          int           `json:"max_keys"`           // Saklanan eski anahtar sayısı
}

// CompressConfig sıkıştırma ayarları.
type CompressConfig struct {
	Algorithm string `json:"algorithm"` // flate, zlib, none
	Level     int    `json:"level"`     // 1-9 (1=hız, 9=oran)
}

// FECConfig ileri hata düzeltme ayarları.
type FECConfig struct {
	Enabled     bool `json:"enabled"`
	DataShards  int  `json:"data_shards"`   // Veri parça sayısı
	ParityPct   int  `json:"parity_pct"`    // Parite yüzdesi (ör: 25 → her 4 blokta 1 parite)
	BlockSize   int  `json:"block_size"`    // Blok boyutu (byte)
}

// BroadcastConfig yayın ayarları.
type BroadcastConfig struct {
	MaxChannels      int           `json:"max_channels"`
	MaxSubscribers   int           `json:"max_subscribers"`    // Kanal başına
	MaxMessageSize   int           `json:"max_message_size"`   // Byte
	HistorySize      int           `json:"history_size"`       // Kanal başına saklanacak mesaj
	RetryCount       int           `json:"retry_count"`
	RetryInterval    time.Duration `json:"retry_interval"`
	MessageTTL       time.Duration `json:"message_ttl"`
}

// DefaultConfig varsayılan yapılandırma.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			GRPCPort:        9106,
			HTTPPort:        9107,
			ShutdownTimeout: 10 * time.Second,
		},
		Crypto: CryptoConfig{
			Algorithm:        "AES-256-GCM",
			KeyRotationEvery: 6 * time.Hour,
			MaxKeys:          5,
		},
		Compress: CompressConfig{
			Algorithm: "flate",
			Level:     6, // Dengeli
		},
		FEC: FECConfig{
			Enabled:    true,
			DataShards: 4,
			ParityPct:  25,
			BlockSize:  1024,
		},
		Bcast: BroadcastConfig{
			MaxChannels:    256,
			MaxSubscribers: 1024,
			MaxMessageSize: 10 * 1024 * 1024, // 10 MB
			HistorySize:    1000,
			RetryCount:     3,
			RetryInterval:  500 * time.Millisecond,
			MessageTTL:     24 * time.Hour,
		},
	}
}

// LoadFromEnv ortam değişkenlerinden günceller.
func (c *Config) LoadFromEnv() {
	c.Server.GRPCPort = ortam.GetInt("YAYIN_GRPC_PORT", c.Server.GRPCPort)
	c.Server.HTTPPort = ortam.GetInt("YAYIN_HTTP_PORT", c.Server.HTTPPort)
	c.Compress.Level = ortam.GetInt("YAYIN_COMPRESS_LEVEL", c.Compress.Level)
	c.FEC.DataShards = ortam.GetInt("YAYIN_FEC_DATA_SHARDS", c.FEC.DataShards)
	c.FEC.ParityPct = ortam.GetInt("YAYIN_FEC_PARITY_PCT", c.FEC.ParityPct)
	c.Bcast.MaxChannels = ortam.GetInt("YAYIN_MAX_CHANNELS", c.Bcast.MaxChannels)
	c.Bcast.HistorySize = ortam.GetInt("YAYIN_HISTORY_SIZE", c.Bcast.HistorySize)

	if alg := ortam.GetStr("YAYIN_COMPRESS_ALG", ""); alg != "" {
		c.Compress.Algorithm = alg
	}
	if ortam.GetStr("YAYIN_FEC_ENABLED", "") == "false" {
		c.FEC.Enabled = false
	}
}
