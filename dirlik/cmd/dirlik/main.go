// DİRLİK — Sistem Sağlık Taraması
//
// Tüm servislerin CPU, RAM, Disk I/O ve ağ gecikme değerlerini
// milisaniye bazında ölçer, anomali tespit eder, BÜTÜN modülüne
// geri bildirim göndererek çöken servislerin yeniden başlatılmasını tetikler.
//
// Port: gRPC=9094, HTTP=9095
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/otukenmatris/dirlik/internal/alerter"
	"github.com/otukenmatris/dirlik/internal/anomaly"
	"github.com/otukenmatris/dirlik/internal/collector"
	"github.com/otukenmatris/dirlik/internal/config"
	"github.com/otukenmatris/dirlik/internal/probe"
	"github.com/otukenmatris/dirlik/internal/server"

	"otukenmatris/ortam"
)

func main() {
	ortam.AutoLoad("../..")

	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()

	// 1. Metrik Toplayıcı
	coll := collector.New(cfg.CollectInterval, cfg.WindowSize*60)
	coll.Start()
	log.Printf("📊 Metrik toplayıcı başlatıldı (aralık: %v)", cfg.CollectInterval)

	// 2. Anomali Dedektörü
	det := anomaly.NewDetector(cfg)
	log.Printf("🔍 Anomali dedektörü hazır (pencere: %d, CPU eşik: %.0f%%, RAM eşik: %.0f%%)",
		cfg.WindowSize, cfg.Thresholds.CPUPercent, cfg.Thresholds.MemPercent)

	// 3. Probe Yöneticisi
	probeMgr := probe.NewManager()

	// Varsayılan probe hedefleri
	defaultTargets := []probe.ProbeTarget{
		{ServiceID: "butun", Type: probe.ProbeHTTP, Endpoint: cfg.ButunURL + "/health", Interval: cfg.ProbeInterval, Timeout: cfg.ProbeTimeout, MaxFails: cfg.Thresholds.ProbeFailures},
		{ServiceID: "bag", Type: probe.ProbeHTTP, Endpoint: cfg.BagURL + "/health", Interval: cfg.ProbeInterval, Timeout: cfg.ProbeTimeout, MaxFails: cfg.Thresholds.ProbeFailures},
		{ServiceID: "kutuk", Type: probe.ProbeHTTP, Endpoint: cfg.KutukURL + "/health", Interval: cfg.ProbeInterval, Timeout: cfg.ProbeTimeout, MaxFails: cfg.Thresholds.ProbeFailures},
	}
	for _, t := range defaultTargets {
		probeMgr.AddTarget(t)
	}
	log.Printf("🏥 Probe yöneticisi hazır (%d hedef)", len(defaultTargets))

	// 4. Alarm Yöneticisi
	alt := alerter.New(cfg.ButunURL, cfg.BagURL)

	// Bağlantılar: collector → detector → alerter
	sub := coll.Subscribe()
	go func() {
		for snap := range sub {
			alerts := det.Analyze(snap)
			for _, a := range alerts {
				alt.HandleAnomaly(a)
			}
		}
	}()

	// Probe durum değişikliği → alerter
	probeMgr.OnStatusChange(func(serviceID string, oldStatus, newStatus probe.ServiceStatus) {
		log.Printf("⚠️  Servis durum değişikliği: %s %s → %s", serviceID, oldStatus, newStatus)
		alt.HandleServiceDown(serviceID, oldStatus, newStatus)
	})

	probeMgr.Start()
	log.Printf("🏥 Probe'lar başlatıldı")

	// 5. HTTP Sunucusu
	srv := server.New(coll, det, probeMgr, alt)
	addr := fmt.Sprintf(":%d", cfg.HTTPPort)

	go func() {
		log.Printf("🌐 DİRLİK HTTP sunucusu başlatıldı: http://127.0.0.1:%d", cfg.HTTPPort)
		if err := srv.ListenAndServe(addr); err != nil {
			log.Fatalf("HTTP sunucu hatası: %v", err)
		}
	}()

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════╗")
	fmt.Println("║        DİRLİK — Sistem Sağlık Taraması        ║")
	fmt.Println("║                                                ║")
	fmt.Printf("║  HTTP   : http://127.0.0.1:%-5d               ║\n", cfg.HTTPPort)
	fmt.Printf("║  gRPC   : :%d                                 ║\n", cfg.GRPCPort)
	fmt.Printf("║  Hedef  : %d servis izleniyor                  ║\n", len(defaultTargets))
	fmt.Println("║                                                ║")
	fmt.Println("╚════════════════════════════════════════════════╝")
	fmt.Println()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("🛑 DİRLİK kapatılıyor...")
	probeMgr.Stop()
	coll.Stop()
	time.Sleep(100 * time.Millisecond)
	log.Println("✅ DİRLİK kapatıldı")
}
