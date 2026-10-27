// YANKI — Akustik Konumlandırma Modülü
//
// Mikrofon dizilimlerinden gelen akustik verileri işleyerek silah atışı,
// patlama, araç motoru gibi sesleri tespit eder. Çapraz korelasyon ve
// eşleştirilmiş filtre algoritmalarıyla şok dalgası/namlu ağzı patlaması
// varış zamanlarını (TOA) hesaplar ve üçgenlemeyle düşman konumunu belirler.
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

	"github.com/otukenmatris/yanki/internal/acoustic"
	"github.com/otukenmatris/yanki/internal/config"
	"github.com/otukenmatris/yanki/internal/server"
)

const banner = `
╔═══════════════════════════════════════════════════════════════╗
║   🔊  YANKI — Akustik Konumlandırma                          ║
║        ÖTÜKEN-MATRİS  •  GÖRGÜ Kümesi                       ║
║                                                               ║
║   Çapraz korelasyon  •  Eşleştirilmiş filtre                ║
║   TOA üçgenleme      •  Şok dalgası / namlu patlaması       ║
╚═══════════════════════════════════════════════════════════════╝
`

func main() {
	fmt.Print(banner)

	ortam.AutoLoad("../..")

	cfg := config.Load()

	engine := acoustic.NewEngine(
		cfg.Acoustic.MinAmplitudeDB,
		cfg.Acoustic.MaxRange,
		cfg.Acoustic.NoiseFloorDB,
		cfg.Locator.SpeedOfSound,
		cfg.Locator.MinSensors,
		cfg.Locator.TOATolerance,
		cfg.Locator.ConfidenceMin,
		cfg.Locator.MaxAge,
		cfg.Locator.MaxDetections,
	)

	srv := server.New(engine, cfg)

	httpAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      srv.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// periyodik temizlik
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.Locator.PruneInterval) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			engine.Prune()
		}
	}()

	go func() {
		log.Printf("YANKI HTTP → %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP hata: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("YANKI kapatılıyor...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
	log.Println("YANKI durduruldu.")
}
