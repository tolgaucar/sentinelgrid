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

	"github.com/otukenmatris/tani/internal/classifier"
	"github.com/otukenmatris/tani/internal/config"
	"github.com/otukenmatris/tani/internal/detector"
	"github.com/otukenmatris/tani/internal/server"
	"github.com/otukenmatris/tani/internal/tracker"
)

const banner = `
╔══════════════════════════════════════════════════════╗
║  ████████╗ █████╗ ███╗   ██╗██╗                     ║
║  ╚══██╔══╝██╔══██╗████╗  ██║██║                     ║
║     ██║   ███████║██╔██╗ ██║██║                     ║
║     ██║   ██╔══██║██║╚██╗██║██║                     ║
║     ██║   ██║  ██║██║ ╚████║██║                     ║
║     ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝                     ║
║                                                      ║
║  ÖTÜKEN-MATRİS  —  Nesne Tespit ve Sınıflandırma    ║
║  Faz 2 GÖRGÜ Kümesi — Görsel Korteks                ║
║                                                      ║
║  • YOLOv7 tabanlı nesne tespiti (simülasyon)         ║
║  • Çok sınıflı sınıflandırma (14 sınıf)             ║
║  • IFF (Dost/Düşman Tanıma) sistemi                 ║
║  • Çoklu nesne izleme (multi-object tracking)        ║
║  • Non-Maximum Suppression (NMS)                     ║
║  • Tehdit seviyesi hesaplama                         ║
║  • Hız vektörü ve rota tahmini                       ║
╚══════════════════════════════════════════════════════╝
`

func main() {
	fmt.Print(banner)
	ortam.AutoLoad("../..")

	// Yapılandırma
	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()

	log.Printf("[TANI] Yapılandırma yüklendi: HTTP=%d, GRPC=%d", cfg.Server.HTTPPort, cfg.Server.GRPCPort)

	// Tespit motoru
	det := detector.NewEngine(
		cfg.Detector.ConfidenceThreshold,
		cfg.Detector.NMSThreshold,
		cfg.Detector.MaxDetections,
		cfg.Detector.InputWidth,
		cfg.Detector.InputHeight,
		cfg.Detector.ModelName,
	)
	log.Printf("[TANI] Tespit motoru: %s, güven=%.2f, NMS=%.2f, max=%d, giriş=%dx%d",
		cfg.Detector.ModelName, cfg.Detector.ConfidenceThreshold,
		cfg.Detector.NMSThreshold, cfg.Detector.MaxDetections,
		cfg.Detector.InputWidth, cfg.Detector.InputHeight)

	// Sınıflandırma motoru
	cls := classifier.NewEngine(cfg.Classifier.IFFEnabled, cfg.Classifier.MinConfidence)
	log.Printf("[TANI] Sınıflandırma motoru: IFF=%v, min güven=%.2f, %d sınıf",
		cfg.Classifier.IFFEnabled, cfg.Classifier.MinConfidence, len(detector.AllClasses))

	// Varsayılan dost kimlikler (simülasyon)
	friendlies := []string{
		"TR-KARTAL-01", "TR-KARTAL-02", "TR-KURT-01",
		"TR-BORAN-01", "TR-ATAK-01",
	}
	for _, f := range friendlies {
		cls.RegisterFriendly(f)
	}
	log.Printf("[TANI] %d dost transponder kaydedildi (IFF)", len(friendlies))

	// İzleme motoru
	trk := tracker.NewEngine(
		cfg.Tracker.MaxTracks,
		cfg.Tracker.MaxLostFrames,
		cfg.Tracker.IOUThreshold,
		cfg.Tracker.HistorySize,
	)
	log.Printf("[TANI] İzleme motoru: max %d iz, kayıp eşik=%d kare, IoU=%.2f",
		cfg.Tracker.MaxTracks, cfg.Tracker.MaxLostFrames, cfg.Tracker.IOUThreshold)

	// HTTP sunucu
	srv := server.NewServer(det, cls, trk)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.HTTPPort)
	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      srv.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Periyodik iz temizleme
	go func() {
		ticker := time.NewTicker(cfg.Tracker.PruneInterval)
		defer ticker.Stop()
		for range ticker.C {
			pruned := trk.Prune()
			if pruned > 0 {
				log.Printf("[TANI] Silinmiş izler temizlendi: %d", pruned)
			}
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("[TANI] HTTP sunucu başlatılıyor: %s", addr)
		log.Printf("[TANI] Endpoints: /health /status /detect /classify /tracks /process /iff/* /stats")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[TANI] HTTP sunucu hatası: %v", err)
		}
	}()

	<-quit
	log.Println("[TANI] Kapatılıyor...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("[TANI] Shutdown hatası: %v", err)
	}
	log.Println("[TANI] Kapatıldı.")
}
