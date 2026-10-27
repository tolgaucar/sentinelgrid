// SÖZGEÇ — Karmaşık Sinyal Ayıklama
//
// Elektronik harp ortamındaki karmaşık sinyal çorbasından dost frekansları
// FFT ve dalgacık dönüşümleriyle ayrıştırıp, DSP filtreleriyle gürültü
// temizleyerek istihbarat birimlerine sunar.
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

	"github.com/otukenmatris/sozgec/internal/config"
	"github.com/otukenmatris/sozgec/internal/filter"
	"github.com/otukenmatris/sozgec/internal/server"
	"github.com/otukenmatris/sozgec/internal/spectrum"
)

const banner = `
╔═══════════════════════════════════════════════════════════════╗
║   📶  SÖZGEÇ — Karmaşık Sinyal Ayıklama                      ║
║        ÖTÜKEN-MATRİS  •  GÖRGÜ Kümesi                       ║
║                                                               ║
║   FFT / Dalgacık dönüşümü  •  DSP filtreleme                ║
║   Dost/düşman ayrımı       •  Gürültü bastırma              ║
╚═══════════════════════════════════════════════════════════════╝
`

func main() {
	fmt.Print(banner)

	ortam.AutoLoad("../..")

	cfg := config.Load()

	specEngine := spectrum.NewEngine(
		cfg.Spectrum.FFTSize,
		cfg.Spectrum.SampleRate,
		cfg.Spectrum.PeakMinDB,
		cfg.Spectrum.MaxBands,
	)

	filtEngine := filter.NewEngine(
		cfg.Filter.NoiseFloorDB,
		cfg.Filter.SNRMinDB,
		cfg.Filter.MaxSignals,
		cfg.Filter.AdaptiveAlpha,
		cfg.Filter.ThreatDecayS,
	)

	srv := server.New(specEngine, filtEngine, cfg)

	httpAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      srv.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// periyodik temizlik
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.Filter.ThreatDecayS/2) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			specEngine.Prune(cfg.Filter.ThreatDecayS)
			filtEngine.Prune()
		}
	}()

	go func() {
		log.Printf("SÖZGEÇ HTTP → %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP hata: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("SÖZGEÇ kapatılıyor...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
	log.Println("SÖZGEÇ durduruldu.")
}
