// Package config — İZLEK yapılandırması.
package config

import "otukenmatris/ortam"

// ServerConfig — sunucu ayarları
type ServerConfig struct {
	Port     int
	GRPCPort int
}

// PredictorConfig — tahmin motoru parametreleri
type PredictorConfig struct {
	// Kalman filtre parametreleri
	ProcessNoise     float64 // süreç gürültüsü (Q matrisi çarpanı)
	MeasurementNoise float64 // ölçüm gürültüsü (R matrisi çarpanı)

	// Tahmin parametreleri
	DefaultHorizonSec float64 // varsayılan tahmin ufku (saniye)
	MaxHorizonSec     float64 // maksimum tahmin ufku
	StepSec           float64 // tahmin adım aralığı

	// İvme analizi
	MaxAccelMps2   float64 // max beklenen ivme (m/s²)
	ManeuverThresh float64 // manevra tespit eşiği (ivme değişim oranı)
}

// TrackerConfig — iz takip parametreleri
type TrackerConfig struct {
	MaxTracks      int     // eşzamanlı takip sayısı
	HistoryDepth   int     // her iz için saklanacak nokta sayısı
	StaleAfterSec  float64 // bu süredir güncellenmemişse iz bayat
	PruneInterval  int     // temizleme döngüsü (saniye)
}

// Config — İZLEK konfigürasyonu
type Config struct {
	Server    ServerConfig
	Predictor PredictorConfig
	Tracker   TrackerConfig
}

// Load — yapılandırmayı yükler.
func Load() Config {
	return Config{
		Server: ServerConfig{
			Port:     ortam.GetInt("IZLEK_PORT", 9126),
			GRPCPort: ortam.GetInt("IZLEK_GRPC_PORT", 9127),
		},
		Predictor: PredictorConfig{
			ProcessNoise:     ortam.GetFloat("IZLEK_PROCESS_NOISE", 0.1),
			MeasurementNoise: ortam.GetFloat("IZLEK_MEASUREMENT_NOISE", 0.5),
			DefaultHorizonSec: ortam.GetFloat("IZLEK_DEFAULT_HORIZON", 60.0),
			MaxHorizonSec:     ortam.GetFloat("IZLEK_MAX_HORIZON", 600.0),
			StepSec:           ortam.GetFloat("IZLEK_STEP_SEC", 5.0),
			MaxAccelMps2:      ortam.GetFloat("IZLEK_MAX_ACCEL", 15.0),
			ManeuverThresh:    ortam.GetFloat("IZLEK_MANEUVER_THRESH", 3.0),
		},
		Tracker: TrackerConfig{
			MaxTracks:     ortam.GetInt("IZLEK_MAX_TRACKS", 5000),
			HistoryDepth:  ortam.GetInt("IZLEK_HISTORY_DEPTH", 200),
			StaleAfterSec: ortam.GetFloat("IZLEK_STALE_SEC", 120.0),
			PruneInterval: ortam.GetInt("IZLEK_PRUNE_INTERVAL", 30),
		},
	}
}
