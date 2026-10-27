package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/otukenmatris/izlek/internal/config"
	"github.com/otukenmatris/izlek/internal/predictor"
	"github.com/otukenmatris/izlek/internal/server"
	"github.com/otukenmatris/izlek/internal/tracker"
	"otukenmatris/ortam"
)

const banner = `
╔══════════════════════════════════════════════════════════╗
║  ██╗███████╗██╗     ███████╗██╗  ██╗                    ║
║  ██║╚══███╔╝██║     ██╔════╝██║ ██╔╝                    ║
║  ██║  ███╔╝ ██║     █████╗  █████╔╝                     ║
║  ██║ ███╔╝  ██║     ██╔══╝  ██╔═██╗                     ║
║  ██║███████╗███████╗███████╗██║  ██╗                    ║
║  ╚═╝╚══════╝╚══════╝╚══════╝╚═╝  ╚═╝                    ║
║  ÖTÜKEN-MATRİS » İZLEK — Rota Tahmini                  ║
║  Kalman Filtre · Kinematik Analiz · Önleme Atışı        ║
╚══════════════════════════════════════════════════════════╝
`

func main() {
	fmt.Print(banner)

	ortam.AutoLoad("../..")
	cfg := config.Load()

	// Tracker
	tm := tracker.NewManager(
		cfg.Tracker.MaxTracks,
		cfg.Tracker.HistoryDepth,
		cfg.Tracker.StaleAfterSec,
		cfg.Predictor.ManeuverThresh,
	)

	// Predictor
	pe := predictor.NewEngine(
		cfg.Predictor.ProcessNoise,
		cfg.Predictor.MeasurementNoise,
		cfg.Predictor.DefaultHorizonSec,
		cfg.Predictor.MaxHorizonSec,
		cfg.Predictor.StepSec,
		cfg.Predictor.MaxAccelMps2,
		cfg.Predictor.ManeuverThresh,
	)

	// Server
	srv := server.New(tm, pe)
	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      srv.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Periyodik temizlik
	pruneCtx, pruneCancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.Tracker.PruneInterval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				tm.Prune()
			case <-pruneCtx.Done():
				return
			}
		}
	}()

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Println("\n🛑 İZLEK kapatılıyor...")
		pruneCancel()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpSrv.Shutdown(ctx)
	}()

	fmt.Printf("🚀 İZLEK dinlemede → :%d\n", cfg.Server.Port)
	if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "❌ Sunucu hatası: %v\n", err)
		os.Exit(1)
	}
}
