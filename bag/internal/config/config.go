// Package config - BAĞ yapılandırma yönetimi
package config

import (
	"time"

	"otukenmatris/ortam"
)

// Config BAĞ modülünün tüm yapılandırması
type Config struct {
	Server  ServerConfig  `json:"server"`
	Broker  BrokerConfig  `json:"broker"`
	Transport TransportConfig `json:"transport"`
}

// ServerConfig gRPC sunucu ayarları
type ServerConfig struct {
	Host            string        `json:"host"`
	Port            int           `json:"port"`
	MaxRecvMsgSize  int           `json:"max_recv_msg_size"`  // byte
	MaxSendMsgSize  int           `json:"max_send_msg_size"`  // byte
	MaxConcurrent   int           `json:"max_concurrent"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// BrokerConfig mesaj broker ayarları
type BrokerConfig struct {
	DefaultPartitions    int           `json:"default_partitions"`
	DefaultRetention     time.Duration `json:"default_retention"`
	MaxTopics            int           `json:"max_topics"`
	MaxQueueSize         int           `json:"max_queue_size"`
	ConsumerPollInterval time.Duration `json:"consumer_poll_interval"`
}

// TransportConfig taşıma katmanı ayarları
type TransportConfig struct {
	MaxRetries     int           `json:"max_retries"`
	RetryBaseDelay time.Duration `json:"retry_base_delay"`
	RetryMaxDelay  time.Duration `json:"retry_max_delay"`
	AckTimeout     time.Duration `json:"ack_timeout"`
	BatchSize      int           `json:"batch_size"`
	FlushInterval  time.Duration `json:"flush_interval"`
}

// DefaultConfig varsayılan yapılandırma
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            9090,
			MaxRecvMsgSize:  64 * 1024 * 1024, // 64MB
			MaxSendMsgSize:  64 * 1024 * 1024,
			MaxConcurrent:   10000,
			ShutdownTimeout: 10 * time.Second,
		},
		Broker: BrokerConfig{
			DefaultPartitions:    4,
			DefaultRetention:     24 * time.Hour,
			MaxTopics:            1000,
			MaxQueueSize:         1_000_000,
			ConsumerPollInterval: 10 * time.Millisecond,
		},
		Transport: TransportConfig{
			MaxRetries:     5,
			RetryBaseDelay: 100 * time.Millisecond,
			RetryMaxDelay:  10 * time.Second,
			AckTimeout:     5 * time.Second,
			BatchSize:      100,
			FlushInterval:  50 * time.Millisecond,
		},
	}
}

// LoadFromEnv ortam değişkenlerinden yapılandırmayı günceller.
func (c *Config) LoadFromEnv() {
	c.Server.Port = ortam.GetInt("BAG_GRPC_PORT", c.Server.Port)
}

// HTTPPort — HTTP durum portu (gRPC port + 1).
func (c *Config) HTTPPort() int {
	return ortam.GetInt("BAG_HTTP_PORT", c.Server.Port+1)
}
