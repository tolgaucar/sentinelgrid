// AKIM — Veri Trafiğini Yoğunluğa Göre Yönlendirme
//
// Savaş anında ağ üzerindeki veri trafiğini tıkanıklık durumuna göre
// dinamik olarak yönlendirir. Yapay zeka destekli yük dengeleme mantığıyla
// en uygun uç noktalara (edge nodes) veya röle sunucularına yönlendirir.
//
// Port: gRPC=9098, HTTP=9099
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/otukenmatris/akim/internal/balancer"
	"github.com/otukenmatris/akim/internal/config"
	"github.com/otukenmatris/akim/internal/flow"
	"github.com/otukenmatris/akim/internal/monitor"
	"github.com/otukenmatris/akim/internal/router"
	"github.com/otukenmatris/akim/internal/server"

	"otukenmatris/ortam"
)

func main() {
	ortam.AutoLoad("../..")

	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()

	// 1. Yönlendirme Motoru
	eng := router.New(cfg)

	// Varsayılan topoloji: basit askeri ağ
	defaultNodes := []router.Node{
		{ID: "cmd-center", Type: "command", Capacity: 10000},
		{ID: "relay-north", Type: "relay", Capacity: 5000},
		{ID: "relay-south", Type: "relay", Capacity: 5000},
		{ID: "relay-east", Type: "relay", Capacity: 3000},
		{ID: "edge-alpha", Type: "edge", Capacity: 1000},
		{ID: "edge-bravo", Type: "edge", Capacity: 1000},
		{ID: "edge-charlie", Type: "edge", Capacity: 1000},
		{ID: "sensor-hub", Type: "sensor", Capacity: 2000},
	}
	for _, n := range defaultNodes {
		eng.AddNode(n)
	}
	log.Printf("🌐 Topoloji: %d düğüm yüklendi", len(defaultNodes))

	defaultLinks := []router.Link{
		{From: "cmd-center", To: "relay-north", Bandwidth: 10000, Latency: 1.0},
		{From: "cmd-center", To: "relay-south", Bandwidth: 10000, Latency: 1.5},
		{From: "cmd-center", To: "relay-east", Bandwidth: 5000, Latency: 2.0},
		{From: "relay-north", To: "edge-alpha", Bandwidth: 2000, Latency: 3.0},
		{From: "relay-north", To: "edge-bravo", Bandwidth: 2000, Latency: 4.0},
		{From: "relay-south", To: "edge-charlie", Bandwidth: 2000, Latency: 3.5},
		{From: "relay-south", To: "sensor-hub", Bandwidth: 3000, Latency: 2.0},
		{From: "relay-east", To: "edge-bravo", Bandwidth: 1500, Latency: 5.0},
		{From: "relay-east", To: "sensor-hub", Bandwidth: 2000, Latency: 3.0},
		{From: "edge-alpha", To: "edge-bravo", Bandwidth: 500, Latency: 8.0},
	}
	for _, l := range defaultLinks {
		eng.AddLink(l)
	}
	log.Printf("🔗 Topoloji: %d bağlantı yüklendi", len(defaultLinks))

	// 2. Akış Yöneticisi
	flowMgr := flow.NewManager(cfg)
	log.Printf("🌊 Akış yöneticisi hazır (TTL: %v)", cfg.FlowTTL)

	// 3. Yük Dengeleyici
	bal := balancer.New(balancer.CongestionAware)
	// Varsayılan backend'ler
	bal.AddBackend("edge-alpha", "192.168.1.10:9090", 30)
	bal.AddBackend("edge-bravo", "192.168.1.11:9090", 30)
	bal.AddBackend("edge-charlie", "192.168.1.12:9090", 20)
	bal.AddBackend("relay-north", "192.168.1.1:9090", 50)
	bal.AddBackend("relay-south", "192.168.1.2:9090", 50)
	log.Printf("⚖️ Yük dengeleyici hazır (strateji: %s, %d backend)", cfg.DefaultStrategy, 5)

	// 4. Ağ İzleyicisi
	mon := monitor.New(eng, cfg)
	mon.Start()
	log.Printf("📊 Ağ izleyicisi başlatıldı (aralık: %v)", cfg.MonitorInterval)

	// 5. HTTP Sunucusu
	srv := server.New(eng, flowMgr, bal, mon)
	addr := fmt.Sprintf(":%d", cfg.HTTPPort)

	go func() {
		log.Printf("🌐 AKIM HTTP sunucusu başlatıldı: http://127.0.0.1:%d", cfg.HTTPPort)
		if err := srv.ListenAndServe(addr); err != nil {
			log.Fatalf("HTTP sunucu hatası: %v", err)
		}
	}()

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║    AKIM — Veri Trafiği Yoğunluk Yönlendirmesi        ║")
	fmt.Println("║                                                      ║")
	fmt.Printf("║  HTTP     : http://127.0.0.1:%-5d                    ║\n", cfg.HTTPPort)
	fmt.Printf("║  gRPC     : :%d                                      ║\n", cfg.GRPCPort)
	fmt.Printf("║  Düğümler : %-3d                                       ║\n", len(defaultNodes))
	fmt.Printf("║  Bağlantı : %-3d                                       ║\n", len(defaultLinks))
	fmt.Printf("║  Strateji : %-20s                   ║\n", cfg.DefaultStrategy)
	fmt.Println("║                                                      ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("🛑 AKIM kapatılıyor...")
	mon.Stop()
	time.Sleep(100 * time.Millisecond)
	log.Println("✅ AKIM kapatıldı")
}
