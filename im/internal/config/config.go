// Package config — İM modülü yapılandırması.
package config

import (
	"time"

	"otukenmatris/ortam"
)

// Config İM modülü yapılandırması.
type Config struct {
	Server ServerConfig `json:"server"`
	PKI    PKIConfig    `json:"pki"`
	Token  TokenConfig  `json:"token"`
}

// ServerConfig sunucu ayarları.
type ServerConfig struct {
	Host            string        `json:"host"`
	GRPCPort        int           `json:"grpc_port"`
	HTTPPort        int           `json:"http_port"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// PKIConfig açık anahtar altyapısı ayarları.
type PKIConfig struct {
	KeySize          int           `json:"key_size"`           // RSA anahtar boyutu (bit)
	CACertLifetime   time.Duration `json:"ca_cert_lifetime"`   // CA sertifika ömrü
	CertLifetime     time.Duration `json:"cert_lifetime"`      // Birim sertifika ömrü
	KeyRotationEvery time.Duration `json:"key_rotation_every"` // Anahtar rotasyon sıklığı
	MaxIssuedCerts   int           `json:"max_issued_certs"`
}

// TokenConfig JWT token ayarları.
type TokenConfig struct {
	Issuer          string        `json:"issuer"`
	AccessLifetime  time.Duration `json:"access_lifetime"`
	RefreshLifetime time.Duration `json:"refresh_lifetime"`
	MaxActiveSess   int           `json:"max_active_sessions"`
	SigningAlg      string        `json:"signing_alg"` // ES256, RS256
}

// DefaultConfig varsayılan yapılandırma.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			GRPCPort:        9104,
			HTTPPort:        9105,
			ShutdownTimeout: 10 * time.Second,
		},
		PKI: PKIConfig{
			KeySize:          4096,
			CACertLifetime:   10 * 365 * 24 * time.Hour, // 10 yıl
			CertLifetime:     365 * 24 * time.Hour,       // 1 yıl
			KeyRotationEvery: 24 * time.Hour,              // günlük
			MaxIssuedCerts:   100_000,
		},
		Token: TokenConfig{
			Issuer:          "otuken-matris-im",
			AccessLifetime:  15 * time.Minute,
			RefreshLifetime: 7 * 24 * time.Hour,
			MaxActiveSess:   5,
			SigningAlg:      "ES256",
		},
	}
}

// LoadFromEnv ortam değişkenlerinden yapılandırmayı günceller.
func (c *Config) LoadFromEnv() {
	c.Server.GRPCPort = ortam.GetInt("IM_GRPC_PORT", c.Server.GRPCPort)
	c.Server.HTTPPort = ortam.GetInt("IM_HTTP_PORT", c.Server.HTTPPort)
	c.PKI.KeySize = ortam.GetInt("IM_KEY_SIZE", c.PKI.KeySize)
	c.Token.Issuer = ortam.GetStr("IM_TOKEN_ISSUER", c.Token.Issuer)
}
