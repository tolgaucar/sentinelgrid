// Package config — YURT yapılandırması.
package config

import (
	"time"

	"otukenmatris/ortam"
)

// Config — YURT konfigürasyonu
type Config struct {
	HTTPPort int
	GRPCPort int
	NodeID   string // bu edge düğümün benzersiz kimliği

	// Yerel Depolama
	MaxEntries    int           // maks kayıt sayısı
	DefaultTTL    time.Duration // varsayılan veri ömrü
	WALEnabled    bool
	WALDir        string
	SnapshotEvery int // her N yazma işleminde snapshot al

	// Senkronizasyon
	SyncInterval    time.Duration // cloud ile senkronizasyon aralığı
	SyncBatchSize   int           // tek seferde senkronize edilecek kayıt
	SyncTimeout     time.Duration
	ConflictPolicy  ConflictPolicy
	DeltaSyncEnabled bool         // sadece değişiklikleri gönder

	// Proxy (YurtHub)
	CloudURL        string
	BufferSize      int           // offline iken buffer kapasitesi
	RetryInterval   time.Duration // yeniden bağlanma aralığı
	MaxRetries      int
	HealthCheckFreq time.Duration

	// Partition
	PartitionTimeout time.Duration // bağlantı kopma tespiti eşiği
	HeartbeatFreq    time.Duration
	QuorumSize       int           // split-brain çözümü için minimum düğüm
}

// ConflictPolicy — çatışma çözüm politikası
type ConflictPolicy int

const (
	LWW        ConflictPolicy = iota // Last-Writer-Wins (zaman damgası)
	CloudWins                        // Bulut versiyonu öncelikli
	EdgeWins                         // Yerel versiyon öncelikli
	MergeAll                         // İki veriyi birleştir (map merge)
)

// String — politika adı
func (c ConflictPolicy) String() string {
	switch c {
	case LWW:
		return "LWW"
	case CloudWins:
		return "CLOUD_WINS"
	case EdgeWins:
		return "EDGE_WINS"
	case MergeAll:
		return "MERGE_ALL"
	default:
		return "UNKNOWN"
	}
}

// SyncState — senkronizasyon durumu
type SyncState int

const (
	SyncIdle     SyncState = iota
	SyncPushing            // yerel → bulut
	SyncPulling            // bulut → yerel
	SyncMerging            // çatışma çözümü
	SyncError
)

func (s SyncState) String() string {
	switch s {
	case SyncIdle:
		return "IDLE"
	case SyncPushing:
		return "PUSHING"
	case SyncPulling:
		return "PULLING"
	case SyncMerging:
		return "MERGING"
	case SyncError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ConnState — bağlantı durumu
type ConnState int

const (
	ConnOnline    ConnState = iota
	ConnOffline
	ConnDegraded
)

func (c ConnState) String() string {
	switch c {
	case ConnOnline:
		return "ONLINE"
	case ConnOffline:
		return "OFFLINE"
	case ConnDegraded:
		return "DEGRADED"
	default:
		return "UNKNOWN"
	}
}

// DefaultConfig — varsayılan YURT yapılandırması
func DefaultConfig() Config {
	return Config{
		HTTPPort:         9101,
		GRPCPort:         9100,
		NodeID:           "edge-001",
		MaxEntries:       100000,
		DefaultTTL:       24 * time.Hour,
		WALEnabled:       true,
		WALDir:           "/tmp/yurt-wal",
		SnapshotEvery:    1000,
		SyncInterval:     5 * time.Second,
		SyncBatchSize:    100,
		SyncTimeout:      10 * time.Second,
		ConflictPolicy:   LWW,
		DeltaSyncEnabled: true,
		CloudURL:         "http://127.0.0.1:8000",
		BufferSize:       10000,
		RetryInterval:    3 * time.Second,
		MaxRetries:       10,
		HealthCheckFreq:  2 * time.Second,
		PartitionTimeout: 10 * time.Second,
		HeartbeatFreq:    2 * time.Second,
		QuorumSize:       2,
	}
}

// LoadFromEnv ortam değişkenlerinden yapılandırmayı günceller.
func (c *Config) LoadFromEnv() {
	c.HTTPPort = ortam.GetInt("YURT_HTTP_PORT", c.HTTPPort)
	c.GRPCPort = ortam.GetInt("YURT_GRPC_PORT", c.GRPCPort)
	c.NodeID = ortam.GetStr("YURT_NODE_ID", c.NodeID)
	c.CloudURL = ortam.GetStr("YURT_CLOUD_URL", c.CloudURL)
	c.WALDir = ortam.GetStr("YURT_WAL_DIR", c.WALDir)
}
