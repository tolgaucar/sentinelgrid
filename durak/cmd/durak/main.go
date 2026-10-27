// DURAK — Veri Paketlerinin Bekletilmesi (QoS)
//
// Askeri haberleşmede sınırlı bant genişliğinde öncelik tabanlı
// paket kuyruklama ve zamanlama modülü. Kritik paketler (NBC alarm,
// hedef vurma) düşük öncelikli verilerin (video akışı, log)
// önüne geçer.
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

	"github.com/otukenmatris/durak/internal/config"
	"github.com/otukenmatris/durak/internal/queue"
	"github.com/otukenmatris/durak/internal/scheduler"
	"github.com/otukenmatris/durak/internal/server"
	"otukenmatris/ortam"
)

func main() {
	fmt.Println(`
╔══════════════════════════════════════════════╗
║  DURAK — Veri Paketi Önceliklendirme (QoS)  ║
║  ÖTÜKEN-MATRİS / Faz 1 ULAK Kümesi         ║
╚══════════════════════════════════════════════╝`)

	// Ortam yükle
	ortam.AutoLoad("../..")

	// Yapılandırma
	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()
	fmt.Printf("[DURAK] HTTP port: %d | Bant genişliği: %d bps\n", cfg.Server.HTTPPort, cfg.Scheduler.BandwidthBps)
	fmt.Printf("[DURAK] Maks kuyruk: %d | Maks boyut: %d\n", cfg.Queue.MaxQueues, cfg.Queue.MaxQueueSize)

	// Zamanlayıcı
	sched := scheduler.New(scheduler.SchedulerConfig{
		BandwidthBps: cfg.Scheduler.BandwidthBps,
		TickInterval: cfg.Scheduler.TickInterval,
		BurstFactor:  1.5,
	})

	// Varsayılan askeri kuyruklar
	defaultQueues := []struct {
		id, name, desc string
	}{
		{"komut", "Komut Kuyruğu", "Komuta kontrol mesajları (hedef onayı, geri çekilme)"},
		{"sensor", "Sensör Kuyruğu", "Radar, EW, SIGINT sensör verileri"},
		{"telemetri", "Telemetri Kuyruğu", "Platform durum, konum, hız bilgileri"},
		{"video", "Video Kuyruğu", "İHA/kamera video akışları"},
		{"bulk", "Toplu Veri Kuyruğu", "Güncellemeler, loglar, harita verileri"},
	}

	for _, q := range defaultQueues {
		pq := queue.NewPriorityQueue(queue.QueueInfo{
			ID:          q.id,
			Name:        q.name,
			Description: q.desc,
			MaxSize:     cfg.Queue.MaxQueueSize,
			CreatedAt:   time.Now(),
		}, cfg.Queue.MaxQueueSize)
		sched.AddQueue(pq)
		fmt.Printf("[DURAK] Kuyruk oluşturuldu: %s — %s\n", q.id, q.desc)
	}

	// Zamanlayıcıyı başlat
	if err := sched.Start(); err != nil {
		log.Fatalf("[DURAK] Zamanlayıcı başlatılamadı: %v", err)
	}
	fmt.Println("[DURAK] Zamanlayıcı aktif (WFQ modu)")

	// Zamanlanan paketleri tüket (arka plan)
	go func() {
		for pkt := range sched.Output() {
			_ = pkt // Gerçek sistemde BAĞ modülüne iletilir
		}
	}()

	// HTTP sunucu
	srv := server.New(sched)
	addr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
	httpSrv := &http.Server{Addr: addr, Handler: srv.Handler()}

	go func() {
		fmt.Printf("[DURAK] HTTP sunucu başlatıldı → %s\n", addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[DURAK] Sunucu hatası: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n[DURAK] Kapatılıyor...")
	sched.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpSrv.Shutdown(ctx)
	fmt.Println("[DURAK] Kapatıldı.")
}
