// DUYUM — Açık Kaynak İstihbarat (OSINT) Modülü
//
// Sosyal medya, haber siteleri, forumlar, uydu metadatası ve resmi yayınlar
// üzerinden NLP/semantik analiz yaparak eyleme dönüştürülebilir istihbarat
// raporları üretir. Kaynak yönetimi CRUD API ile dinamik olarak yapılır.
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

	"github.com/otukenmatris/duyum/internal/analyzer"
	"github.com/otukenmatris/duyum/internal/collector"
	"github.com/otukenmatris/duyum/internal/config"
	"github.com/otukenmatris/duyum/internal/server"
)

const banner = `
╔═══════════════════════════════════════════════════════════════╗
║   🔍  DUYUM — Açık Kaynak İstihbarat (OSINT)                 ║
║        ÖTÜKEN-MATRİS  •  GÖRGÜ Kümesi                       ║
║                                                               ║
║   NLP / Semantik analiz  •  Kaynak yönetimi (CRUD)           ║
║   Çapraz doğrulama       •  İstihbarat raporu üretimi        ║
╚═══════════════════════════════════════════════════════════════╝
`

func main() {
	fmt.Print(banner)

	ortam.AutoLoad("../..")

	cfg := config.Load()

	coll := collector.NewEngine(
		cfg.Collector.MaxSources,
		cfg.Collector.MaxItems,
		cfg.Collector.MinCredibility,
		cfg.Collector.PruneAge,
	)

	anal := analyzer.NewEngine(
		cfg.Analyzer.MaxReports,
		cfg.Analyzer.ConfidenceMin,
		cfg.Analyzer.CrossRefMin,
		cfg.Analyzer.ThreatDecayS,
	)

	srv := server.New(coll, anal, cfg)

	httpAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      srv.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// periyodik temizlik
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.Collector.PruneAge/2) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			coll.Prune()
			anal.Prune()
		}
	}()

	go func() {
		log.Printf("DUYUM HTTP → %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP hata: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("DUYUM kapatılıyor...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
	log.Println("DUYUM durduruldu.")
}
