# DİRLİK — Sistem Sağlık Taraması

**ÖTÜKEN-MATRİS ULAK Kümesi — Faz 1**

Tüm servislerin CPU, RAM, disk I/O ve ağ gecikme değerlerini milisaniye bazında ölçerek
anomali tespit eden ve BÜTÜN modülüne geri bildirim gönderen telemetri izleme sistemi.

## Mimari

```
┌─────────────┐    ┌──────────────┐    ┌──────────────┐
│  Collector   │───▶│   Detector   │───▶│   Alerter    │
│  (metrikler) │    │  (anomali)   │    │ (BÜTÜN+BAĞ)  │
└─────────────┘    └──────────────┘    └──────────────┘
                          │
┌─────────────┐           │
│    Probe     │───────────┘
│  (sağlık)   │
└─────────────┘
```

## Alt Sistemler

| Paket | Dosya | Görev |
|-------|-------|-------|
| **collector** | `internal/collector/` | CPU, RAM, disk, GC metrikleri toplama |
| **anomaly** | `internal/anomaly/` | Sliding window + eşik tabanlı anomali tespiti |
| **probe** | `internal/probe/` | HTTP/TCP/gRPC servis sağlık kontrolü |
| **alerter** | `internal/alerter/` | BÜTÜN'e restart bildirimi, BAĞ'a alarm yayını |
| **server** | `internal/server/` | HTTP API endpoint'leri |
| **config** | `internal/config/` | Yapılandırma |

## Port Yapısı

- **HTTP:** `:9095`
- **gRPC:** `:9094`

## API Endpoint'leri

```
GET  /health              — Sağlık kontrolü
GET  /status              — Detaylı durum raporu
GET  /metrics/current     — Anlık sistem metrikleri
GET  /metrics/history?n=  — Metrik geçmişi
GET  /metrics/services    — Servis bazlı metrikler
GET  /probes              — Tüm probe durumları
POST /probes              — Yeni probe hedefi ekle
GET  /probes/{id}         — Tek probe durumu
GET  /probes/{id}/check   — Tek seferlik probe
DEL  /probes/{id}         — Probe kaldır
GET  /alerts?n=           — Anomali alarmları
GET  /alerts/summary      — Alarm özeti (severity bazlı)
GET  /alerts/records?n=   — Gönderilmiş alarm kayıtları
```

## Çalıştırma

```bash
cd dirlik
go run cmd/dirlik/main.go
```

## Test

```bash
go test ./... -v
# 56 test
```

## Ortam Değişkenleri

| Değişken | Varsayılan | Açıklama |
|----------|-----------|----------|
| `DIRLIK_HTTP_PORT` | 9095 | HTTP port |
| `DIRLIK_GRPC_PORT` | 9094 | gRPC port |
| `BUTUN_URL` | http://127.0.0.1:8000 | BÜTÜN bağlantısı |
| `BAG_URL` | http://127.0.0.1:9091 | BAĞ bağlantısı |

## Alarm Eşikleri

| Metrik | Eşik | Şiddet |
|--------|------|--------|
| CPU | >85% | WARNING, >95% CRITICAL |
| RAM | >90% | WARNING, >98% CRITICAL |
| Disk | >95% | WARNING, >99% FATAL |
| GC Pause | >10ms | INFO |
| Goroutine | >10000 | WARNING |
| Hata oranı | >5% | WARNING, >20% CRITICAL |
| Ağ gecikme | >200ms | WARNING, >400ms CRITICAL |
