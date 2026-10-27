package config

import (
	"time"

	"otukenmatris/ortam"
)

// Format — Desteklenen serileştirme formatları
type Format string

const (
	FormatJSON    Format = "json"
	FormatMsgPack Format = "msgpack"
	FormatCBOR    Format = "cbor"
)

// Config — BAĞDAŞ yapılandırması
type Config struct {
	HTTPPort       int
	GRPCPort       int
	DefaultFormat  Format
	MaxPayloadSize int           // Maks mesaj boyutu (byte)
	ConvertTimeout time.Duration // Dönüşüm zaman aşımı
	ButunURL       string
	BagURL         string
}

// DefaultConfig — Varsayılan yapılandırma
func DefaultConfig() Config {
	return Config{
		HTTPPort:       9097,
		GRPCPort:       9096,
		DefaultFormat:  FormatJSON,
		MaxPayloadSize: 16 * 1024 * 1024, // 16MB
		ConvertTimeout: 5 * time.Second,
		ButunURL:       "http://127.0.0.1:8000",
		BagURL:         "http://127.0.0.1:9091",
	}
}

// LoadFromEnv ortam değişkenlerinden yapılandırmayı günceller.
func (c *Config) LoadFromEnv() {
	c.HTTPPort = ortam.GetInt("BAGDAS_HTTP_PORT", c.HTTPPort)
	c.ButunURL = ortam.GetStr("BAGDAS_BUTUN_URL", c.ButunURL)
	c.BagURL = ortam.GetStr("BAGDAS_BAG_URL", c.BagURL)
}
