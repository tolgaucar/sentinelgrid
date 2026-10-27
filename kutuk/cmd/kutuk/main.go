// KÜTÜK: Anlık Durum Veritabanı
//
// ÖTÜKEN-MATRİS platformunun dağıtık bellek-içi durum deposu.
// Savaş alanı varlıklarının gerçek zamanlı durumlarını mikrosaniye
// seviyesinde okuma/yazma ile saklar.
//
// Bileşenler:
//   - Sharded KV Store: 64 parçalı eşzamanlı bellek deposu
//   - RAFT Konsensüs: Lider seçimi ve log replikasyonu
//   - WAL: Çökme kurtarma için disk kalıcılığı
//   - gRPC + HTTP: Çift protokol erişim
//
// Portlar:
//   - gRPC: 9092
//   - HTTP:  9093
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/otukenmatris/kutuk/internal/config"
	"github.com/otukenmatris/kutuk/internal/grpcserver"
	"github.com/otukenmatris/kutuk/internal/raft"
	"github.com/otukenmatris/kutuk/internal/store"
	"github.com/otukenmatris/kutuk/internal/wal"

	"otukenmatris/ortam"
)

func main() {
	ortam.AutoLoad("../..")

	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()

	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Println("╔══════════════════════════════════════╗")
	log.Println("║  KÜTÜK - Anlık Durum Veritabanı      ║")
	log.Println("║  ÖTÜKEN-MATRİS / ULAK Kümesi         ║")
	log.Println("╚══════════════════════════════════════╝")

	// 1. Store oluştur
	kvStore := store.New()
	kvStore.StartCleanup(cfg.CleanupPeriod)
	log.Printf("[KÜTÜK] Bellek deposu hazır (parça=%d)", 64)

	// 2. RAFT başlat
	raftNode := raft.NewNode(cfg.RaftNodeID, cfg.RaftPeers, func(entry raft.LogEntry) error {
		// RAFT log kaydını store'a uygula
		var cmd map[string]interface{}
		if err := json.Unmarshal(entry.Command, &cmd); err != nil {
			log.Printf("[KÜTÜK] RAFT apply parse hatası: %v", err)
			return err
		}
		op, _ := cmd["op"].(string)
		ns, _ := cmd["ns"].(string)
		key, _ := cmd["key"].(string)
		switch op {
		case "put":
			var val []byte
			if v, ok := cmd["value"].(string); ok {
				val = []byte(v)
			}
			ttlMs, _ := cmd["ttl_ms"].(float64)
			ttl := time.Duration(int64(ttlMs)) * time.Millisecond
			kvStore.Put(ns, key, val, ttl, nil)
		case "delete":
			kvStore.Delete(ns, key)
		default:
			log.Printf("[KÜTÜK] RAFT bilinmeyen op: %s", op)
		}
		return nil
	})
	raftNode.Start()
	log.Printf("[KÜTÜK] RAFT düğümü başlatıldı (id=%s, peers=%d)", cfg.RaftNodeID, len(cfg.RaftPeers))

	// 3. WAL başlat
	walEngine, err := wal.New(cfg.WALDir, cfg.WALSegmentSize, cfg.WALSyncOnWrite)
	if err != nil {
		log.Printf("[KÜTÜK] WAL başlatılamadı (devam ediliyor): %v", err)
		walEngine = nil
	} else {
		// WAL'dan kurtarma
		count, err := walEngine.Replay(func(rec wal.Record) error {
			switch rec.Type {
			case wal.RecordPut:
				ttl := time.Duration(0)
				if rec.TTLNano > 0 {
					ttl = time.Duration(rec.TTLNano)
				}
				kvStore.Put(rec.Namespace, rec.Key, rec.Value, ttl, nil)
			case wal.RecordDelete:
				kvStore.Delete(rec.Namespace, rec.Key)
			}
			return nil
		})
		if err != nil {
			log.Printf("[KÜTÜK] WAL replay hatası: %v", err)
		} else if count > 0 {
			log.Printf("[KÜTÜK] WAL'dan %d kayıt kurtarıldı", count)
		}
		log.Printf("[KÜTÜK] WAL hazır (dir=%s, sync=%v)", cfg.WALDir, cfg.WALSyncOnWrite)
	}

	// 4. Varsayılan namespace'leri oluştur
	defaultNS := []string{"entity", "sensor", "track", "alert", "command", "telemetry", "system"}
	for _, ns := range defaultNS {
		kvStore.Put(ns, "_ns_marker", []byte(`{"created":true}`), 0, nil)
	}
	log.Printf("[KÜTÜK] %d varsayılan namespace oluşturuldu", len(defaultNS))

	// 5. Sunucu
	srv := grpcserver.NewServer(kvStore, raftNode, walEngine)

	// HTTP
	go func() {
		addr := fmt.Sprintf(":%d", cfg.HTTPPort)
		if err := srv.StartHTTP(addr); err != nil {
			log.Fatalf("[KÜTÜK] HTTP sunucu hatası: %v", err)
		}
	}()

	// gRPC
	go func() {
		if err := srv.StartGRPC(cfg.GRPCPort); err != nil {
			log.Fatalf("[KÜTÜK] gRPC sunucu hatası: %v", err)
		}
	}()

	log.Printf("[KÜTÜK] Hazır! gRPC=:%d HTTP=:%d", cfg.GRPCPort, cfg.HTTPPort)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("[KÜTÜK] Sinyal alındı: %v, kapatılıyor...", sig)

	srv.Stop()
	log.Println("[KÜTÜK] Kapatıldı.")
}
