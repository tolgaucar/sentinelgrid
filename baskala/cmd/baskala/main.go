// BAŞKALA — Uydu Fotoğraflarındaki Değişimleri Bulma
//
// Optik ve SAR uydu görüntüleri üzerinde zaman serisi analizi yaparak
// siper kazılması, bina yıkımı, araç hareketi gibi stratejik değişimleri
// tespit eder. SAR+Optik füzyon ile yanlış alarmlar minimize edilir.
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

	"github.com/otukenmatris/baskala/internal/change"
	"github.com/otukenmatris/baskala/internal/config"
	"github.com/otukenmatris/baskala/internal/fusion"
	"github.com/otukenmatris/baskala/internal/imagery"
	"github.com/otukenmatris/baskala/internal/server"
)

const banner = `
╔═══════════════════════════════════════════════════════════════╗
║   🛰️  BAŞKALA — Uydu Değişim Tespiti                          ║
║        ÖTÜKEN-MATRİS  •  GÖRGÜ Kümesi                       ║
║                                                               ║
║   SAR+Optik füzyon     •  Zaman serisi analizi               ║
║   Değişim sınıflandırma •  Tehdit değerlendirmesi            ║
╚═══════════════════════════════════════════════════════════════╝
`

func main() {
	fmt.Print(banner)

	ortam.AutoLoad("../..")

	cfg := config.Load()

	imgEngine := imagery.NewEngine(
		cfg.Imagery.MaxImages,
		cfg.Imagery.MaxPixels,
		cfg.Change.DiffThreshold,
	)

	chgEngine := change.NewEngine(
		cfg.Change.MaxChanges,
		cfg.Change.ConfidenceMin,
		cfg.Change.DiffThreshold,
		cfg.Change.ClusterRadius,
		cfg.Change.ThreatDecayS,
	)

	fusEngine := fusion.NewEngine(
		cfg.Fusion.SARWeight,
		cfg.Fusion.OpticalWeight,
		cfg.Fusion.CrossRefBoost,
		cfg.Fusion.CloudPenalty,
	)

	srv := server.New(imgEngine, chgEngine, fusEngine, cfg)

	httpAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      srv.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// periyodik temizlik
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.Imagery.RetentionHours/2) * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			imgEngine.Prune(cfg.Imagery.RetentionHours)
			chgEngine.Prune()
		}
	}()

	go func() {
		log.Printf("BAŞKALA HTTP → %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP hata: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("BAŞKALA kapatılıyor...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
	log.Println("BAŞKALA durduruldu.")
}
