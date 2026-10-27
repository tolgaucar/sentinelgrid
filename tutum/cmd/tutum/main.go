package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"otukenmatris/ortam"

	"github.com/otukenmatris/tutum/internal/alert"
	"github.com/otukenmatris/tutum/internal/battery"
	"github.com/otukenmatris/tutum/internal/config"
	"github.com/otukenmatris/tutum/internal/predictor"
	"github.com/otukenmatris/tutum/internal/resource"
	"github.com/otukenmatris/tutum/internal/server"
)

const banner = `
╔══════════════════════════════════════════════════════╗
║  ████████╗██╗   ██╗████████╗██╗   ██╗███╗   ███╗   ║
║  ╚══██╔══╝██║   ██║╚══██╔══╝██║   ██║████╗ ████║   ║
║     ██║   ██║   ██║   ██║   ██║   ██║██╔████╔██║   ║
║     ██║   ██║   ██║   ██║   ██║   ██║██║╚██╔╝██║   ║
║     ██║   ╚██████╔╝   ██║   ╚██████╔╝██║ ╚═╝ ██║   ║
║     ╚═╝    ╚═════╝    ╚═╝    ╚═════╝ ╚═╝     ╚═╝   ║
║                                                      ║
║  ÖTÜKEN-MATRİS  —  Batarya & Kaynak İzleyicisi      ║
║  Faz 1 ULAK Kümesi                                   ║
║                                                      ║
║  • Batarya SoC/SoH takibi & Coulomb sayımı           ║
║  • Doğrusal regresyon ile ömür tahmini                ║
║  • Rüzgar/arazi/yük çevresel faktör analizi           ║
║  • Otomatik RTB (geri dönüş) önerisi                 ║
║  • Görev yapılabilirlik değerlendirmesi               ║
║  • Çok seviyeli uyarı sistemi                         ║
║  • CPU/Bellek/Sıcaklık kaynak izleme                  ║
╚══════════════════════════════════════════════════════╝
`

func main() {
	fmt.Print(banner)
	ortam.AutoLoad("../..")

	// Yapılandırma
	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()

	log.Printf("[TUTUM] Yapılandırma yüklendi: HTTP=%d, GRPC=%d", cfg.Server.HTTPPort, cfg.Server.GRPCPort)

	// Batarya monitörü
	mon := battery.NewMonitor(cfg.Battery.MaxPlatforms)
	log.Printf("[TUTUM] Batarya monitörü: max %d platform, kritik %%%0.f, uyarı %%%0.f",
		cfg.Battery.MaxPlatforms, cfg.Battery.CriticalPercent, cfg.Battery.WarningPercent)

	// Kaynak izleyici
	trk := resource.NewTracker(cfg.Resource.HistorySize, nil)
	log.Printf("[TUTUM] Kaynak izleyici: CPU uyarı %%%0.f, kritik %%%0.f",
		cfg.Resource.CPUWarning, cfg.Resource.CPUCritical)

	// Tahmin motoru
	pred := predictor.NewEngine(cfg.Predictor.MinSamples, cfg.Predictor.WindowSize, cfg.Predictor.RTBReserveMin)
	log.Printf("[TUTUM] Tahmin motoru: min %d örnek, pencere %d, RTB yedek %%%0.f",
		cfg.Predictor.MinSamples, cfg.Predictor.WindowSize, cfg.Predictor.RTBReserveMin)

	// Uyarı yöneticisi
	alm := alert.NewManager(cfg.Alert.MaxAlerts, cfg.Alert.RetentionPeriod,
		cfg.Alert.DeduplicateWindow, cfg.Alert.AutoRTB)
	log.Printf("[TUTUM] Uyarı yöneticisi: max %d, dedup %s, RTB=%v",
		cfg.Alert.MaxAlerts, cfg.Alert.DeduplicateWindow, cfg.Alert.AutoRTB)

	// Varsayılan platformlar (simülasyon için)
	defaults := []struct {
		id, name string
		ptype    battery.PlatformType
		capacity float64
	}{
		{"iha-kartal-01", "Kartal-1 İHA", battery.PlatformDrone, 500},
		{"iha-kartal-02", "Kartal-2 İHA", battery.PlatformDrone, 500},
		{"ugv-kurt-01", "Kurt-1 Otonom", battery.PlatformUGV, 2000},
		{"radyo-tr-01", "TR-01 Taktik Radyo", battery.PlatformRadio, 50},
		{"sensor-alan-01", "Alan Sensörü-1", battery.PlatformSensor, 100},
	}

	for _, d := range defaults {
		p := battery.NewPlatform(d.id, d.name, d.ptype, d.capacity)
		p.SetThresholds(cfg.Battery.CriticalPercent, cfg.Battery.WarningPercent, cfg.Battery.LowPercent)
		if err := mon.Register(p); err != nil {
			log.Printf("[TUTUM] UYARI: platform kaydedilemedi: %s — %v", d.id, err)
		} else {
			log.Printf("[TUTUM] Platform kaydedildi: %s (%s, %.0f Wh)", d.name, d.ptype, d.capacity)
		}
	}

	// HTTP sunucu
	srv := server.NewServer(mon, trk, pred, alm,
		cfg.Battery.CriticalPercent, cfg.Battery.WarningPercent, cfg.Battery.LowPercent,
		cfg.Resource.TempWarning, cfg.Resource.TempCritical)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.HTTPPort)
	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      srv.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Uyarı temizleme goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			removed := alm.Cleanup()
			if removed > 0 {
				log.Printf("[TUTUM] Eski uyarılar temizlendi: %d", removed)
			}
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("[TUTUM] HTTP sunucu başlatılıyor: %s", addr)
		log.Printf("[TUTUM] Endpoints: /health /status /platforms /battery/* /resource/* /predict/* /alerts/* /rtb /handoff")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[TUTUM] HTTP sunucu hatası: %v", err)
		}
	}()

	<-quit
	log.Println("[TUTUM] Kapatılıyor...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("[TUTUM] Shutdown hatası: %v", err)
	}
	log.Println("[TUTUM] Kapatıldı.")
}
