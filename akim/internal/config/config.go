// Package config — AKIM yapılandırması.
package config

import (
	"time"

	"otukenmatris/ortam"
)

// Config — AKIM konfigürasyonu
type Config struct {
	HTTPPort int
	GRPCPort int

	// Ağ topolojisi
	MaxNodes    int
	MaxLinks    int
	DefaultBW   float64 // Mbps varsayılan bant genişliği
	DefaultLat  float64 // ms varsayılan gecikme

	// Tıkanıklık eşikleri
	CongestionWarn     float64 // %75 — uyarı
	CongestionCritical float64 // %90 — kritik
	CongestionBlock    float64 // %98 — trafiği engelle

	// Yük dengeleme
	DefaultStrategy string // round_robin, least_conn, weighted, congestion_aware

	// İzleme
	MonitorInterval time.Duration
	HistorySize     int // link istatistik geçmişi boyutu

	// Akış yönetimi
	MaxFlowsPerLink int
	FlowTTL         time.Duration

	// Bağımlılık URL'leri
	ButunURL string
	BagURL   string
}

// DefaultConfig — varsayılan AKIM yapılandırması
func DefaultConfig() Config {
	return Config{
		HTTPPort:           9099,
		GRPCPort:           9098,
		MaxNodes:           1024,
		MaxLinks:           8192,
		DefaultBW:          1000.0,  // 1 Gbps
		DefaultLat:         2.0,     // 2ms
		CongestionWarn:     75.0,
		CongestionCritical: 90.0,
		CongestionBlock:    98.0,
		DefaultStrategy:    "congestion_aware",
		MonitorInterval:    time.Second,
		HistorySize:        120,
		MaxFlowsPerLink:    10000,
		FlowTTL:            5 * time.Minute,
		ButunURL:           "http://127.0.0.1:8000",
		BagURL:             "http://127.0.0.1:9091",
	}
}

// LoadFromEnv ortam değişkenlerinden yapılandırmayı günceller.
func (c *Config) LoadFromEnv() {
	c.HTTPPort = ortam.GetInt("AKIM_HTTP_PORT", c.HTTPPort)
	c.GRPCPort = ortam.GetInt("AKIM_GRPC_PORT", c.GRPCPort)
	c.ButunURL = ortam.GetStr("AKIM_BUTUN_URL", c.ButunURL)
	c.BagURL = ortam.GetStr("AKIM_BAG_URL", c.BagURL)
}

// QoSClass — trafik öncelik sınıfları
type QoSClass int

const (
	QoSCritical    QoSClass = 0 // Düşman ateş alarmı, hedef vurma onayı
	QoSRealtime    QoSClass = 1 // Canlı video, ses
	QoSCommand     QoSClass = 2 // Komuta emirleri
	QoSTelemetry   QoSClass = 3 // Sensör telemetri
	QoSBulk        QoSClass = 4 // Harita güncelleme, firmware
	QoSBestEffort  QoSClass = 5 // Diğer trafik
)

// String — QoS sınıf adı
func (q QoSClass) String() string {
	switch q {
	case QoSCritical:
		return "CRITICAL"
	case QoSRealtime:
		return "REALTIME"
	case QoSCommand:
		return "COMMAND"
	case QoSTelemetry:
		return "TELEMETRY"
	case QoSBulk:
		return "BULK"
	case QoSBestEffort:
		return "BEST_EFFORT"
	default:
		return "UNKNOWN"
	}
}

// BandwidthWeight — QoS sınıfına göre bant genişliği paylaşım oranı
func (q QoSClass) BandwidthWeight() float64 {
	switch q {
	case QoSCritical:
		return 0.30
	case QoSRealtime:
		return 0.25
	case QoSCommand:
		return 0.20
	case QoSTelemetry:
		return 0.15
	case QoSBulk:
		return 0.07
	case QoSBestEffort:
		return 0.03
	default:
		return 0.01
	}
}
