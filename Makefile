# ÖTÜKEN-MATRİS — Ana Makefile
# Tüm modülleri derleme, test ve çalıştırma komutları
#
# Kullanım:
#   make build-all    — Go modüllerini derle
#   make test-all     — Tüm testleri çalıştır
#   make run-dev      — Geliştirme ortamında başlat
#   make run-prod     — Üretim ortamında başlat
#   make clean        — Derleme çıktılarını temizle
#   make docker-build — Docker imajlarını oluştur

SHELL := /bin/bash
GO := go
PYTHON := python3
BIN_DIR := bin

GO_MODULES := bag kutuk dirlik bagdas akim yurt yuzey im yayin durak tutum tani koz sezi yanki duyum baskala sozgec izlek
ALL_MODULES := butun $(GO_MODULES)

# ——— Derleme ———
.PHONY: build-all build-go build-butun

build-all: build-go
	@echo "✅ Tüm Go modülleri derlendi → $(BIN_DIR)/"

build-go: $(addprefix build-,$(GO_MODULES))

build-%:
	@echo "🔨 Derleniyor: $*"
	@mkdir -p $(BIN_DIR)
	@cd $* && $(GO) build -o ../$(BIN_DIR)/$* ./cmd/$*/main.go
	@echo "   → $(BIN_DIR)/$*"

# ——— Test ———
.PHONY: test-all test-go test-python test-sim

test-all: test-go test-python test-sim
	@echo "✅ Tüm testler tamamlandı"

test-go:
	@echo "🧪 Go testleri çalıştırılıyor..."
	@for mod in $(GO_MODULES) ortam; do \
		echo "  ▸ $$mod"; \
		(cd $$mod && $(GO) test ./... -count=1 -timeout 60s); \
	done
	@echo "✅ Go testleri başarılı"

test-python:
	@echo "🧪 Python testleri çalıştırılıyor..."
	@cd butun && $(PYTHON) -m pytest tests/ -q --tb=short
	@echo "✅ Python testleri başarılı"

test-sim:
	@echo "🧪 Simülasyon testleri çalıştırılıyor..."
	@cd simulasyon && $(PYTHON) -m pytest tests/ -q --tb=short
	@echo "✅ Simülasyon testleri başarılı"

# ——— Çalıştırma ———
.PHONY: run-dev run-prod stop

run-dev:
	@echo "🚀 Geliştirme ortamı başlatılıyor..."
	@OTUKEN_ENV=dev $(PYTHON) simulasyon/baslatici.py

run-prod: build-all
	@echo "🚀 Üretim ortamı başlatılıyor..."
	@OTUKEN_ENV=prod $(PYTHON) simulasyon/baslatici.py --prod

stop:
	@echo "🛑 Modüller durduruluyor..."
	@$(PYTHON) simulasyon/baslatici.py --kill

# ——— Temizlik ———
.PHONY: clean

clean:
	@echo "🧹 Temizleniyor..."
	@rm -rf $(BIN_DIR)
	@for mod in $(GO_MODULES); do \
		cd $$mod && $(GO) clean && cd ..; \
	done
	@find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	@find . -type d -name ".pytest_cache" -exec rm -rf {} + 2>/dev/null || true
	@echo "✅ Temizlendi"

# ——— Docker ———
.PHONY: docker-build docker-up docker-down

docker-build:
	@echo "🐳 Docker imajları oluşturuluyor..."
	@for mod in $(ALL_MODULES); do \
		echo "  ▸ $$mod"; \
		docker build -t otukenmatris/$$mod:latest -f $$mod/Dockerfile . ; \
	done
	@echo "✅ Docker imajları hazır"

docker-up:
	docker compose up -d

docker-down:
	docker compose down

# ——— Yardım ———
.PHONY: help

help:
	@echo ""
	@echo "ÖTÜKEN-MATRİS Komutları:"
	@echo "  build-all     Go modüllerini derle"
	@echo "  test-all      Tüm testleri çalıştır"
	@echo "  test-go       Sadece Go testleri"
	@echo "  test-python   Sadece Python testleri"
	@echo "  run-dev       Geliştirme ortamında başlat"
	@echo "  run-prod      Üretim ortamında başlat (önce derler)"
	@echo "  stop          Çalışan modülleri durdur"
	@echo "  clean         Derleme çıktılarını temizle"
	@echo "  docker-build  Docker imajlarını oluştur"
	@echo "  docker-up     Docker Compose ile başlat"
	@echo "  docker-down   Docker Compose ile durdur"
	@echo ""
