// Package ortam — ÖTÜKEN-MATRİS ortak ortam yapılandırması.
//
// Her Go modülü bu paketi kopyalayarak kullanır.
// Env dosyası (.env.dev / .env.prod) root dizinden okunur.
package ortam

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// Env ortam türü
type Env string

const (
	Dev  Env = "dev"
	Prod Env = "prod"
)

// Current mevcut ortamı döndürür
func Current() Env {
	e := os.Getenv("OTUKEN_ENV")
	if e == "prod" || e == "production" {
		return Prod
	}
	return Dev
}

// IsProd üretim ortamı mı
func IsProd() bool { return Current() == Prod }

// IsDev geliştirme ortamı mı
func IsDev() bool { return Current() == Dev }

// LogLevel ortama göre log seviyesi
func LogLevel() string {
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		return v
	}
	if IsProd() {
		return "warn"
	}
	return "debug"
}

// CORSOrigins izin verilen CORS origin'leri
func CORSOrigins() string {
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		return v
	}
	if IsProd() {
		return ""
	}
	return "*"
}

// ── Env helpers ──

// GetStr ortam değişkenini okur, yoksa varsayılan döner
func GetStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// GetInt ortam değişkenini int olarak okur
func GetInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// GetBool ortam değişkenini bool olarak okur
func GetBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

// GetFloat ortam değişkeninden float64 okur; yoksa fallback döner.
func GetFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

// LoadEnvFile .env dosyasını okuyup os.Setenv yapar.
// Zaten tanımlı olan env var'ları ezmez.
func LoadEnvFile(paths ...string) {
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// Mevcut env var'ı ezme
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
		f.Close()
		return // ilk bulunan dosyayı yükle
	}
}

// AutoLoad ortama göre .env dosyasını otomatik yükler.
// rootDir projenin kök dizini olmalı.
func AutoLoad(rootDir string) {
	env := Current()
	// Önce ortama özel, sonra genel
	LoadEnvFile(
		fmt.Sprintf("%s/.env.%s", rootDir, env),
		fmt.Sprintf("%s/.env", rootDir),
	)
	log.Printf("[ORTAM] ortam=%s, log=%s, cors=%s", env, LogLevel(), CORSOrigins())
}
