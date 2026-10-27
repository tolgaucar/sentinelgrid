// İM — Dijital İmza ve Kimlik Modülü
// ÖTÜKEN-MATRİS Sıfır Güven (Zero Trust) kimlik doğrulama platformu
//
// Ana giriş noktası: PKI, token ve kimlik servislerini başlatır.
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

	"github.com/otukenmatris/im/internal/config"
	"github.com/otukenmatris/im/internal/identity"
	"github.com/otukenmatris/im/internal/pki"
	"github.com/otukenmatris/im/internal/server"
	"github.com/otukenmatris/im/internal/token"

	"otukenmatris/ortam"
)

const banner = `
======================================================
  İM — ÖTÜKEN-MATRİS Dijital İmza ve Kimlik v0.1.0
  Sıfır Güven Kimlik Doğrulama Platformu
  PKI • JWT • Zero Trust Identity
======================================================
`

func main() {
	fmt.Print(banner)

	ortam.AutoLoad("../..")

	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()

	// ── PKI (Sertifika Otoritesi) ──
	log.Println("[İM] PKI başlatılıyor — ECDSA P-256 anahtar çifti üretiliyor...")
	ca, err := pki.NewCA(cfg.PKI.CertLifetime, cfg.PKI.MaxIssuedCerts)
	if err != nil {
		log.Fatalf("[İM] PKI başlatma hatası: %v", err)
	}
	log.Printf("[İM] PKI hazır — Anahtar boyutu: P-256, Sertifika ömrü: %v", cfg.PKI.CertLifetime)

	// ── Token Yöneticisi ──
	log.Println("[İM] Token yöneticisi başlatılıyor — HMAC-SHA256...")
	tokens, err := token.NewManager(
		cfg.Token.Issuer,
		cfg.Token.AccessLifetime,
		cfg.Token.RefreshLifetime,
		cfg.Token.MaxActiveSess,
	)
	if err != nil {
		log.Fatalf("[İM] Token yöneticisi hatası: %v", err)
	}
	log.Printf("[İM] Token hazır — Erişim: %v, Yenileme: %v", cfg.Token.AccessLifetime, cfg.Token.RefreshLifetime)

	// ── Kimlik Kaydı ──
	registry := identity.NewRegistry()
	log.Println("[İM] Kimlik kaydı hazır — Zero Trust modeli aktif")

	// ── HTTP Sunucu ──
	srv := server.NewServer(ca, tokens, registry)
	httpAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.HTTPPort)

	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      srv.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("[İM] HTTP sunucu başlatıldı: %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("[İM] HTTP sunucu hatası: %v", err)
		}
	}()

	// ── Periyodik Anahtar Rotasyonu (dev ortamında kısa) ──
	rotationInterval := cfg.PKI.KeyRotationEvery
	if ortam.IsDev() {
		rotationInterval = 1 * time.Hour
	}
	rotationTicker := time.NewTicker(rotationInterval)
	go func() {
		for range rotationTicker.C {
			if err := tokens.RotateKey(); err != nil {
				log.Printf("[İM] Token anahtar rotasyon hatası: %v", err)
			} else {
				log.Println("[İM] Token imzalama anahtarı döndürüldü")
			}
			cleaned := tokens.CleanExpired()
			if cleaned > 0 {
				log.Printf("[İM] %d süresi dolmuş token temizlendi", cleaned)
			}
		}
	}()

	log.Printf("[İM] Sistem hazır — HTTP: %s", httpAddr)
	log.Printf("[İM] Endpoint'ler: /health /status /enroll /pki/* /token/* /identity/*")

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("[İM] Sinyal alındı: %v, kapatılıyor...", sig)

	rotationTicker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("[İM] HTTP kapatma hatası: %v", err)
	}

	log.Println("[İM] Sistem kapatıldı")
}
