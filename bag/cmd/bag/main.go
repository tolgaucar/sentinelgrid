// BAĞ - Veri Transfer Katmanı
// ÖTÜKEN-MATRİS modüller arası düşük gecikmeli iletişim platformu
//
// Ana giriş noktası: gRPC sunucusunu ve broker'ı başlatır.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/otukenmatris/bag/internal/broker"
	"github.com/otukenmatris/bag/internal/config"
	"github.com/otukenmatris/bag/internal/grpcserver"
	"github.com/otukenmatris/bag/internal/transport"

	"otukenmatris/ortam"
)

const banner = `
======================================================
  BAG - OTUKEN-MATRIS Veri Transfer Katmani v0.1.0
  Dusuk gecikmeli asenkron mesajlasma platformu
======================================================
`

func main() {
	fmt.Print(banner)

	ortam.AutoLoad("../..")

	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()

	// Broker oluştur
	brk := broker.NewBroker(broker.BrokerOpts{
		DefaultPartitions: cfg.Broker.DefaultPartitions,
		MaxQueueSize:      cfg.Broker.MaxQueueSize,
		MaxTopics:         cfg.Broker.MaxTopics,
		RetentionDuration: cfg.Broker.DefaultRetention,
	})

	// Transport bileşenleri
	sender := transport.NewSender(
		cfg.Transport.MaxRetries,
		cfg.Transport.RetryBaseDelay,
		cfg.Transport.RetryMaxDelay,
		cfg.Transport.AckTimeout,
	)
	receiver := transport.NewReceiver()

	// gRPC sunucu
	srv := grpcserver.NewServer(brk, sender, receiver)
	srv.Start()

	// Varsayılan topic'leri oluştur
	defaultTopics := []string{
		"sensor.radar", "sensor.thermal", "sensor.acoustic",
		"sensor.optical", "sensor.osint",
		"command.buyruk", "command.kovan",
		"alert.critical", "alert.warning",
		"telemetry.dirlik", "telemetry.tutum",
	}
	for _, t := range defaultTopics {
		if _, err := brk.CreateTopic(t, cfg.Broker.DefaultPartitions); err != nil {
			log.Printf("[BAĞ] Varsayılan topic oluşturma: %v", err)
		}
	}

	// HTTP durum sunucusu (BÜTÜN ile entegrasyon için)
	httpAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port+1) // 9091
	httpSrv := startHTTPServer(httpAddr, srv)

	// gRPC listener
	grpcAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port) // 9090
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("[BAĞ] TCP dinleme hatası: %v", err)
	}

	log.Printf("[BAĞ] gRPC sunucu başlatıldı: %s", grpcAddr)
	log.Printf("[BAĞ] HTTP durum sunucusu: %s", httpAddr)
	log.Printf("[BAĞ] %d varsayılan topic oluşturuldu", len(defaultTopics))

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	var connWg sync.WaitGroup

	// gRPC bağlantı kabul döngüsü (basitleştirilmiş)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := lis.Accept()
			if err != nil {
				return // listener kapatıldığında çık
			}
			connWg.Add(1)
			go func() {
				defer connWg.Done()
				handleConnection(conn, srv)
			}()
		}
	}()

	// Sinyal bekle
	sig := <-sigCh
	log.Printf("[BAĞ] Sinyal alındı: %v, kapatılıyor...", sig)

	// Temiz kapatma
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	_ = httpSrv.Shutdown(ctx)
	_ = lis.Close()
	srv.Stop()

	connWg.Wait()
	wg.Wait()
	log.Println("[BAĞ] Sistem kapatıldı")
}

// handleConnection basit bağlantı işleyici (ileride gRPC olacak)
func handleConnection(conn net.Conn, srv *grpcserver.Server) {
	defer conn.Close()
	log.Printf("[BAĞ] Yeni bağlantı: %s", conn.RemoteAddr())
	// Gerçek gRPC implementasyonu protoc ile generate edilecek.
	// Şu an TCP bağlantısını kabul edip loglayıp kapatıyoruz.
	_, _ = conn.Write([]byte("BAĞ mesaj servisi: gRPC protobuf ile entegrasyon bekleniyor\n"))
}

// startHTTPServer BÜTÜN ile entegrasyon için HTTP durum servisi
func startHTTPServer(addr string, srv *grpcserver.Server) *http.Server {
	mux := http.NewServeMux()

	// Durum endpoint'i
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		status := srv.GetStatus()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	// Sağlık kontrolü
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		healthy, status, latency := srv.HealthCheck()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"healthy":    healthy,
			"status":     status,
			"latency_ns": latency,
			"module":     "BAĞ",
		})
	})

	// Topic listesi
	mux.HandleFunc("/topics", func(w http.ResponseWriter, r *http.Request) {
		topics := srv.ListTopics()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"topics": topics,
			"count":  len(topics),
		})
	})

	// Mesaj gönder (POST /publish?topic=xxx)
	mux.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST gerekli", http.StatusMethodNotAllowed)
			return
		}

		topic := r.URL.Query().Get("topic")
		if topic == "" {
			http.Error(w, "topic parametresi gerekli", http.StatusBadRequest)
			return
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "geçersiz JSON", http.StatusBadRequest)
			return
		}

		payload, _ := json.Marshal(body)
		env := grpcserver.Envelope{
			ID:          fmt.Sprintf("http-%d", time.Now().UnixNano()),
			Topic:       topic,
			Source:      "http",
			Payload:     payload,
			TimestampNs: time.Now().UnixNano(),
			Priority:    5,
		}

		ack := srv.Send(r.Context(), env)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ack)
	})

	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("[BAĞ] HTTP sunucu hatası: %v", err)
		}
	}()

	return httpServer
}
