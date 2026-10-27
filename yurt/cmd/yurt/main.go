// YURT — Uç Bilişim ve Yerel Depolama
//
// İletişimin veya GPS bağlantısının koptuğu zorlu ortamlarda sistemin
// çalışmaya devam etmesini sağlayan uç bilişim (Edge Computing) mimarisi.
// Verileri yerel cihazlarda tutarak asenkron senkronizasyon çalıştırır.
//
// Port: gRPC=9100, HTTP=9101
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/otukenmatris/yurt/internal/config"
	"github.com/otukenmatris/yurt/internal/partition"
	"github.com/otukenmatris/yurt/internal/proxy"
	"github.com/otukenmatris/yurt/internal/server"
	"github.com/otukenmatris/yurt/internal/store"
	ysync "github.com/otukenmatris/yurt/internal/sync"

	"otukenmatris/ortam"
)

func main() {
	ortam.AutoLoad("../..")

	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()

	// 1. Yerel Depolama
	edgeStore := store.New(cfg.NodeID, cfg.MaxEntries, cfg.DefaultTTL)
	log.Printf("🗄️  Yerel depo hazır (maks: %d kayıt, TTL: %v)", cfg.MaxEntries, cfg.DefaultTTL)

	// 2. Senkronizasyon Motoru
	syncEngine := ysync.New(edgeStore, cfg)
	log.Printf("🔄 Senkronizasyon motoru hazır (aralık: %v, politika: %s)",
		cfg.SyncInterval, cfg.ConflictPolicy)

	// 3. Hub Proxy
	hub := proxy.NewHub(cfg)
	log.Printf("🔌 YurtHub proxy hazır (buffer: %d, cloud: %s)", cfg.BufferSize, cfg.CloudURL)

	// 4. Bölünme Yöneticisi
	partMgr := partition.NewManager(cfg)
	// Varsayılan düğümler
	partMgr.AddPeer("cloud-main", cfg.CloudURL, "cloud")
	partMgr.AddPeer("relay-alpha", "http://192.168.1.10:9101", "relay")
	partMgr.AddPeer("relay-bravo", "http://192.168.1.11:9101", "relay")
	partMgr.Start()
	log.Printf("🌐 Bölünme yöneticisi hazır (quorum: %d, %d düğüm)", cfg.QuorumSize, 3)

	// 5. HTTP Sunucusu
	srv := server.New(edgeStore, syncEngine, hub, partMgr, cfg)
	addr := fmt.Sprintf(":%d", cfg.HTTPPort)

	go func() {
		log.Printf("🌐 YURT HTTP sunucusu başlatıldı: http://127.0.0.1:%d", cfg.HTTPPort)
		if err := srv.ListenAndServe(addr); err != nil {
			log.Fatalf("HTTP sunucu hatası: %v", err)
		}
	}()

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║      YURT — Uç Bilişim ve Yerel Depolama             ║")
	fmt.Println("║                                                      ║")
	fmt.Printf("║  HTTP     : http://127.0.0.1:%-5d                    ║\n", cfg.HTTPPort)
	fmt.Printf("║  gRPC     : :%d                                      ║\n", cfg.GRPCPort)
	fmt.Printf("║  Düğüm ID : %-20s                   ║\n", cfg.NodeID)
	fmt.Printf("║  Politika : %-20s                   ║\n", cfg.ConflictPolicy)
	fmt.Printf("║  Cloud    : %-20s                   ║\n", cfg.CloudURL)
	fmt.Println("║                                                      ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("🛑 YURT kapatılıyor...")
	partMgr.Stop()
	time.Sleep(100 * time.Millisecond)
	log.Println("✅ YURT kapatıldı")
}
