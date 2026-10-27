# YÜZEY — 3D Taktiksel İzleme Sistemi

ÖTÜKEN-MATRİS C4ISR platformunun merkezi görselleştirme ve durumsal farkındalık modülü.

## Özellikler

- **Gerçek Zamanlı Modül İzleme** — 7 modülü (BÜTÜN, BAĞ, KÜTÜK, DİRLİK, BAĞDAŞ, AKIM, YURT) periyodik olarak yoklar
- **Taktiksel Resim** — Tüm varlıkları, modül durumlarını ve olayları tek bir görünümde birleştirir
- **Varlık Yönetimi** — 5000+ varlık takibi, taraf/tip/durum bazlı filtreleme
- **8 Katmanlı Harita** — Varlıklar, sensörler, rotalar, tehditler, bölünmeler, kapsama, trafik, notlar
- **Olay Veriyolu** — 10000 olay tamponu, tip/kaynak/önem bazlı filtreleme
- **Server-Sent Events (SSE)** — Gerçek zamanlı veri akışı (WebSocket olmadan)
- **Web Dashboard** — Canvas tabanlı taktik harita, modül durumu, olay akışı, performans grafiği

## Mimari

```
yuzey/
├── cmd/yuzey/main.go           # Ana giriş noktası
├── internal/
│   ├── config/config.go        # Yapılandırma ve veri modelleri
│   ├── event/event.go          # Olay veriyolu (pub/sub)
│   ├── hub/hub.go              # Merkezi veri toplayıcı
│   ├── layer/layer.go          # Harita katman yönetimi
│   └── server/
│       ├── server.go           # HTTP + SSE sunucusu
│       └── dashboard.go        # Web arayüzü HTML
└── go.mod
```

## API Endpointleri

| Endpoint | Metod | Açıklama |
|----------|-------|----------|
| `/` | GET | Web dashboard |
| `/health` | GET | Sistem sağlık durumu |
| `/stats` | GET | Genel istatistikler |
| `/tactical` | GET | Taktiksel resim |
| `/tactical/picture` | GET | Genişletilmiş taktiksel resim |
| `/modules` | GET | Modül durumları |
| `/modules/{id}` | GET | Modül detayı |
| `/modules/{id}/stats` | GET | Modül istatistikleri |
| `/entities` | GET/DELETE | Varlık listesi / temizle |
| `/entities/{id}` | GET | Varlık detayı |
| `/entities/update` | POST | Toplu varlık güncelleme |
| `/entities/factions` | GET | Taraf bazlı sayılar |
| `/layers` | GET | Katman listesi |
| `/layers/{id}` | GET/PUT | Katman detayı / güncelle |
| `/events` | GET | Olay listesi (filtreli) |
| `/events/emit` | POST | Olay yayınla |
| `/events/stats` | GET | Olay istatistikleri |
| `/events/stream` | GET | Zamana göre olaylar |
| `/stream` | GET | SSE canlı akış |
| `/hub/stats` | GET | Hub istatistikleri |
| `/hub/summary` | GET | Hub özet |

## Çalıştırma

```bash
cd yuzey
go build ./cmd/yuzey
./yuzey
# Dashboard: http://localhost:9103
```

## Test

```bash
cd yuzey
go test ./... -v -count=1
# 126 test
```

## Port

- HTTP: **9103**

## Bağımlılıklar

Sadece Go standart kütüphanesi. Harici bağımlılık yok.
