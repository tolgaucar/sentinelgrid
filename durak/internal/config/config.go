// Package config — DURAK modülü yapılandırması.
package config

import (
	"time"

	"otukenmatris/ortam"
)

// Config DURAK modülü yapılandırması.
type Config struct {
	Server    ServerConfig    `json:"server"`
	Queue     QueueConfig     `json:"queue"`
	Scheduler SchedulerConfig `json:"scheduler"`
}

// ServerConfig sunucu ayarları.
type ServerConfig struct {
	Host            string        `json:"host"`
	GRPCPort        int           `json:"grpc_port"`
	HTTPPort        int           `json:"http_port"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// QueueConfig kuyruk ayarları.
type QueueConfig struct {
	MaxQueues     int `json:"max_queues"`
	MaxQueueSize  int `json:"max_queue_size"`   // Kuyruk başına maks paket
	MaxPacketSize int `json:"max_packet_size"`  // Byte
}

// SchedulerConfig zamanlayıcı ayarları.
type SchedulerConfig struct {
	BandwidthBps    int64         `json:"bandwidth_bps"`     // Toplam bant genişliği (bit/sn)
	TickInterval    time.Duration `json:"tick_interval"`     // Zamanlama döngüsü
	StarvationLimit int           `json:"starvation_limit"`  // Açlık eşiği (tick)
	DropPolicy      string        `json:"drop_policy"`       // tail, head, priority
}

// DefaultConfig varsayılan yapılandırma.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			GRPCPort:        9108,
			HTTPPort:        9109,
			ShutdownTimeout: 10 * time.Second,
		},
		Queue: QueueConfig{
			MaxQueues:     64,
			MaxQueueSize:  10000,
			MaxPacketSize: 1024 * 1024, // 1 MB
		},
		Scheduler: SchedulerConfig{
			BandwidthBps:    10_000_000, // 10 Mbps
			TickInterval:    10 * time.Millisecond,
			StarvationLimit: 100,
			DropPolicy:      "tail",
		},
	}
}

// LoadFromEnv ortam değişkenlerinden günceller.
func (c *Config) LoadFromEnv() {
	c.Server.GRPCPort = ortam.GetInt("DURAK_GRPC_PORT", c.Server.GRPCPort)
	c.Server.HTTPPort = ortam.GetInt("DURAK_HTTP_PORT", c.Server.HTTPPort)
	c.Queue.MaxQueues = ortam.GetInt("DURAK_MAX_QUEUES", c.Queue.MaxQueues)
	c.Queue.MaxQueueSize = ortam.GetInt("DURAK_MAX_QUEUE_SIZE", c.Queue.MaxQueueSize)

	if bw := ortam.GetInt("DURAK_BANDWIDTH_BPS", 0); bw > 0 {
		c.Scheduler.BandwidthBps = int64(bw)
	}
	if dp := ortam.GetStr("DURAK_DROP_POLICY", ""); dp != "" {
		c.Scheduler.DropPolicy = dp
	}
}
