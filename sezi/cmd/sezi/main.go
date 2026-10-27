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

	"github.com/otukenmatris/sezi/internal/config"
	"github.com/otukenmatris/sezi/internal/detector"
	"github.com/otukenmatris/sezi/internal/server"
	"github.com/otukenmatris/sezi/internal/texture"
	"otukenmatris/ortam"
)

const banner = `
╔═══════════════════════════════════════════════════════════════╗
║   🎨  SEZİ — Doku Analiziyle Kamuflaj Bulma                  ║
║        ÖTÜKEN-MATRİS  •  GÖRGÜ Kümesi                       ║
║                                                               ║
║   Gabor dalgacıkları     •  Doku varyans analizi             ║
║   Konveks/konkav tespit  •  Kamuflaj sınıflandırma           ║
╚═══════════════════════════════════════════════════════════════╝
`

func main() {
	fmt.Print(banner)

	ortam.AutoLoad("../..")

	cfg := config.Load()

	// Doku analiz motoru
	texEngine := texture.NewEngine(
		cfg.Texture.GaborScales,
		cfg.Texture.GaborOrients,
		cfg.Texture.GaborMinFreq,
		cfg.Texture.GaborMaxFreq,
		cfg.Texture.BlockSize,
		cfg.Texture.VarianceThreshold,
	)

	// Kamuflaj tespit motoru
	det := detector.NewEngine(
		texEngine,
		cfg.Detector.MaxAge,
		cfg.Detector.ConfidenceMin,
		cfg.Detector.ConvexThreshold,
		cfg.Detector.ConcaveThreshold,
		cfg.Detector.MaxDetections,
	)

	// HTTP sunucu
	srv := server.New(texEngine, det)
	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: srv.Handler(),
	}

	// Periyodik temizlik
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ticker := time.NewTicker(time.Duration(cfg.Detector.PruneInterval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				removed := det.Prune()
				if removed > 0 {
					log.Printf("[SEZİ] 🧹 %d eski kamuflaj tespiti temizlendi", removed)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Başlat
	go func() {
		log.Printf("[SEZİ] 🎨 Kamuflaj tespit sunucusu başlatılıyor — port %d", cfg.Server.Port)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[SEZİ] ❌ Sunucu hatası: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[SEZİ] 🛑 Kapatılıyor...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[SEZİ] ⚠ Kapatma hatası: %v", err)
	}
	log.Println("[SEZİ] ✅ Kamuflaj tespit motoru durduruldu.")
}
