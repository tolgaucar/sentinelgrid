package config

import (
	"time"

	"otukenmatris/ortam"
)
type Config struct {
	// Sunucu
	GRPCPort int
	HTTPPort int

	// Depolama
	MaxKeys       int
	DefaultTTL    time.Duration
	CleanupPeriod time.Duration

	// WAL
	WALDir         string
	WALSegmentSize int64
	WALSyncOnWrite bool

	// RAFT
	RaftNodeID     string
	RaftPeers      []string
	ElectionMin    time.Duration
	ElectionMax    time.Duration
	HeartbeatTick  time.Duration

	// Snapshot
	SnapshotDir      string
	SnapshotInterval time.Duration
}

// DefaultConfig varsayılan konfigürasyon.
func DefaultConfig() *Config {
	return &Config{
		GRPCPort:         9092,
		HTTPPort:         9093,
		MaxKeys:          10_000_000,
		DefaultTTL:       0, // Sonsuz
		CleanupPeriod:    5 * time.Second,
		WALDir:           "/tmp/kutuk/wal",
		WALSegmentSize:   64 * 1024 * 1024, // 64MB
		WALSyncOnWrite:   true,
		RaftNodeID:       "node-1",
		RaftPeers:        nil,
		ElectionMin:      150 * time.Millisecond,
		ElectionMax:      300 * time.Millisecond,
		HeartbeatTick:    50 * time.Millisecond,
		SnapshotDir:      "/tmp/kutuk/snapshot",
		SnapshotInterval: 60 * time.Second,
	}
}

// LoadFromEnv ortam değişkenlerinden yapılandırmayı günceller.
func (c *Config) LoadFromEnv() {
	c.GRPCPort = ortam.GetInt("KUTUK_GRPC_PORT", c.GRPCPort)
	c.HTTPPort = ortam.GetInt("KUTUK_HTTP_PORT", c.HTTPPort)
	c.RaftNodeID = ortam.GetStr("KUTUK_NODE_ID", c.RaftNodeID)
	c.WALDir = ortam.GetStr("KUTUK_WAL_DIR", c.WALDir)
	c.SnapshotDir = ortam.GetStr("KUTUK_SNAPSHOT_DIR", c.SnapshotDir)
}
