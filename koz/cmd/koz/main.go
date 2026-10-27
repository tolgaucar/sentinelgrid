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

	"github.com/otukenmatris/koz/internal/analyzer"
	"github.com/otukenmatris/koz/internal/config"
	"github.com/otukenmatris/koz/internal/server"
	"github.com/otukenmatris/koz/internal/thermal"
	"otukenmatris/ortam"
)

const banner = `
╔═══════════════════════════════════════════════════════════════╗
║   🔥  KÖZ — Termal İz ve Motor Sıcaklığı Analizi            ║
║        ÖTÜKEN-MATRİS  •  GÖRGÜ Kümesi                       ║
║                                                               ║
║   Kızılötesi algılama  •  Soğuma eğrisi tahmini              ║
║   Termal kümeleme      •  Anomali tespiti                    ║
╚═══════════════════════════════════════════════════════════════╝
`

func main() {
	fmt.Print(banner)

	ortam.AutoLoad("../..")

	cfg := config.Load()

	// Termal motor
	engine := thermal.NewEngine(
		cfg.Thermal.AmbientC,
		cfg.Thermal.MinDeltaC,
		cfg.Thermal.MaxAge,
		cfg.Thermal.Resolution,
		cfg.Thermal.FOV,
		cfg.Thermal.FPS,
	)

	// Analiz motoru
	az := analyzer.NewAnalyzer(
		cfg.Analyzer.ClusterRadiusM,
		cfg.Analyzer.CoolingThreshold,
	)

	// HTTP sunucu
	srv := server.New(engine, az)
	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: srv.Handler(),
	}

	// Periyodik temizlik
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				removed := engine.Prune()
				if removed > 0 {
					log.Printf("[KÖZ] 🧹 %d eski termal imza temizlendi", removed)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Başlat
	go func() {
		log.Printf("[KÖZ] 🔥 Termal analiz sunucusu başlatılıyor — port %d", cfg.Server.Port)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[KÖZ] ❌ Sunucu hatası: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[KÖZ] 🛑 Kapatılıyor...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[KÖZ] ⚠ Kapatma hatası: %v", err)
	}
	log.Println("[KÖZ] ✅ Termal analiz motoru durduruldu.")
}
