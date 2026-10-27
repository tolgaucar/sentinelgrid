// YÜZEY — ÖTÜKEN-MATRİS 3D Taktiksel Arayüz
//
// Tüm modülleri gerçek zamanlı izleyen, taktiksel resim oluşturan
// ve web tabanlı komuta kontrol panosu sunan merkezi görselleştirme sistemi.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/otukenmatris/yuzey/internal/config"
	"github.com/otukenmatris/yuzey/internal/event"
	"github.com/otukenmatris/yuzey/internal/hub"
	"github.com/otukenmatris/yuzey/internal/layer"
	"github.com/otukenmatris/yuzey/internal/server"

	"otukenmatris/ortam"
)

const banner = `
╔═══════════════════════════════════════════════════╗
║          YÜZEY — TAKTİKSEL İZLEME SİSTEMİ        ║
║         ÖTÜKEN-MATRİS C4ISR PLATFORMU             ║
║    3D Görselleştirme • Durumsal Farkındalık        ║
╚═══════════════════════════════════════════════════╝
`

func main() {
	fmt.Print(banner)

	ortam.AutoLoad("../..")

	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()
	fmt.Printf("[YÜZEY] Yapılandırma yüklendi — %d modül tanımlı\n", len(cfg.Modules))

	bus := event.NewBus(10000)
	fmt.Println("[YÜZEY] Olay veriyolu başlatıldı (10000 tampon)")

	layers := layer.NewManager()
	fmt.Printf("[YÜZEY] Katman yöneticisi başlatıldı — %d katman\n", len(layers.GetAllLayers()))

	h := hub.New(cfg, bus)
	h.Start()
	fmt.Println("[YÜZEY] Merkez toplayıcı başlatıldı — modüller yoklanıyor")

	bus.EmitSimple("system_start", "yuzey", "info", "YÜZEY sistemi başlatıldı")

	srv := server.New(h, bus, layers, cfg)

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	fmt.Printf("[YÜZEY] HTTP sunucu başlatılıyor → http://localhost%s\n", addr)
	fmt.Println("[YÜZEY] Dashboard → http://localhost" + addr + "/")
	fmt.Println("[YÜZEY] API → http://localhost" + addr + "/health")
	fmt.Println("[YÜZEY] SSE → http://localhost" + addr + "/stream")

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n[YÜZEY] Kapatılıyor...")
		h.Stop()
		os.Exit(0)
	}()

	if err := srv.ListenAndServe(addr); err != nil {
		fmt.Fprintf(os.Stderr, "[YÜZEY] Hata: %v\n", err)
		os.Exit(1)
	}
}
