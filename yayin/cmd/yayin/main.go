// YAYIN — Şifreli ve Sıkıştırılmış Veri Yayımı
// ÖTÜKEN-MATRİS cephe haberleşme yayın platformu
//
// Ana giriş noktası: Crypto, Compress, FEC ve Broadcast servislerini başlatır.
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

	"github.com/otukenmatris/yayin/internal/broadcast"
	"github.com/otukenmatris/yayin/internal/compress"
	"github.com/otukenmatris/yayin/internal/config"
	"github.com/otukenmatris/yayin/internal/crypto"
	"github.com/otukenmatris/yayin/internal/fec"
	"github.com/otukenmatris/yayin/internal/server"

	"otukenmatris/ortam"
)

const banner = `
======================================================
  YAYIN — ÖTÜKEN-MATRİS Şifreli Veri Yayımı v0.1.0
  Uçtan Uca Şifreleme • Kayıpsız Sıkıştırma • FEC
  AES-256-GCM • DEFLATE • XOR Parite
======================================================
`

func main() {
	fmt.Print(banner)

	ortam.AutoLoad("../..")

	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()

	// ── Şifreleme Motoru (AES-256-GCM) ──
	log.Println("[YAYIN] Şifreleme motoru başlatılıyor — AES-256-GCM...")
	cryptoEng, err := crypto.NewEngine(cfg.Crypto.MaxKeys)
	if err != nil {
		log.Fatalf("[YAYIN] Şifreleme motoru hatası: %v", err)
	}
	log.Printf("[YAYIN] Şifreleme hazır — %s, Anahtar rotasyonu: %v", cfg.Crypto.Algorithm, cfg.Crypto.KeyRotationEvery)

	// ── Sıkıştırma (DEFLATE) ──
	compressor := compress.NewCompressor(cfg.Compress.Level)
	log.Printf("[YAYIN] Sıkıştırma hazır — %s seviye %d", cfg.Compress.Algorithm, cfg.Compress.Level)

	// ── FEC (İleri Hata Düzeltme) ──
	var fecEncoder *fec.Encoder
	if cfg.FEC.Enabled {
		fecEncoder, err = fec.NewEncoder(cfg.FEC.DataShards, cfg.FEC.BlockSize)
		if err != nil {
			log.Fatalf("[YAYIN] FEC başlatma hatası: %v", err)
		}
		log.Printf("[YAYIN] FEC hazır — %d veri parçası, %d byte blok, %%%d parite", cfg.FEC.DataShards, cfg.FEC.BlockSize, cfg.FEC.ParityPct)
	} else {
		log.Println("[YAYIN] FEC devre dışı")
	}

	// ── Yayın Yöneticisi ──
	bcastMgr := broadcast.NewManager(
		cryptoEng, compressor, fecEncoder,
		cfg.Bcast.MaxChannels,
		cfg.Bcast.MaxSubscribers,
		cfg.Bcast.MaxMessageSize,
		cfg.Bcast.HistorySize,
		cfg.Bcast.MessageTTL,
	)
	log.Printf("[YAYIN] Yayın yöneticisi hazır — Maks kanal: %d, Geçmiş: %d mesaj/kanal", cfg.Bcast.MaxChannels, cfg.Bcast.HistorySize)

	// Varsayılan kanallar oluştur
	defaultChannels := []struct {
		name, desc string
		prio       broadcast.Priority
		enc, comp  bool
	}{
		{"komut", "Komuta emirleri (şifreli)", broadcast.PriorityCritical, true, true},
		{"koordinat", "Hedef ve mevzi koordinatları", broadcast.PriorityHigh, true, true},
		{"telemetri", "Sensör ve telemetri verileri", broadcast.PriorityNormal, false, true},
		{"alarm", "NBC/Saldırı/Geri çekilme alarmları", broadcast.PriorityFlash, true, false},
		{"guncelleme", "Yazılım ve konfigürasyon güncellemeleri", broadcast.PriorityLow, true, true},
	}
	for _, dc := range defaultChannels {
		ch, err := bcastMgr.CreateChannel(dc.name, dc.desc, dc.prio, dc.enc, dc.comp, cfg.FEC.Enabled)
		if err != nil {
			log.Printf("[YAYIN] Varsayılan kanal hatası (%s): %v", dc.name, err)
		} else {
			log.Printf("[YAYIN]   📢 %s kanalı hazır (ID: %s)", ch.Name, ch.ID)
		}
	}

	// ── HTTP Sunucu ──
	srv := server.NewServer(bcastMgr, cryptoEng, compressor, fecEncoder)
	httpAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.HTTPPort)

	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      srv.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("[YAYIN] HTTP sunucu başlatıldı: %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("[YAYIN] HTTP sunucu hatası: %v", err)
		}
	}()

	// ── Periyodik Görevler ──
	rotationInterval := cfg.Crypto.KeyRotationEvery
	if ortam.IsDev() {
		rotationInterval = 1 * time.Hour
	}
	rotationTicker := time.NewTicker(rotationInterval)
	cleanTicker := time.NewTicker(5 * time.Minute)

	go func() {
		for {
			select {
			case <-rotationTicker.C:
				if err := cryptoEng.RotateKey(); err != nil {
					log.Printf("[YAYIN] Anahtar rotasyon hatası: %v", err)
				} else {
					log.Printf("[YAYIN] Şifreleme anahtarı döndürüldü → ID=%d", cryptoEng.ActiveKeyID())
				}
			case <-cleanTicker.C:
				cleaned := bcastMgr.CleanExpired()
				if cleaned > 0 {
					log.Printf("[YAYIN] %d süresi dolmuş mesaj temizlendi", cleaned)
				}
			}
		}
	}()

	log.Printf("[YAYIN] Sistem hazır — HTTP: %s", httpAddr)
	log.Printf("[YAYIN] Endpoint'ler: /health /status /channels /publish /subscribe /history /crypto/* /compress /fec/*")

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("[YAYIN] Sinyal alındı: %v, kapatılıyor...", sig)

	rotationTicker.Stop()
	cleanTicker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("[YAYIN] HTTP kapatma hatası: %v", err)
	}

	log.Println("[YAYIN] Sistem kapatıldı")
}
