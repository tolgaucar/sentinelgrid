# AKIM — Veri Trafiğini Yoğunluğa Göre Yönlendirme

**ÖTÜKEN-MATRİS / ULAK Kümesi / Faz 1**

Savaş anında ağ üzerinde devasa boyutlarda veri paketleri dolaşır. AKIM modülü,
sahadan gelen yüksek çözünürlüklü video akışlarının, sensör telemetrilerinin ve
şifreli ses iletişimlerinin bant genişliğini dinamik olarak optimize eder.

## Mimari

```
┌──────────────────────────────────────────────┐
│                  AKIM                         │
│                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │  Router   │  │   Flow   │  │ Balancer │  │
│  │  Engine   │  │  Manager │  │          │  │
│  └──────────┘  └──────────┘  └──────────┘  │
│       │              │              │        │
│  ┌──────────┐        │              │        │
│  │ Monitor  │◄───────┴──────────────┘        │
│  └──────────┘                                │
│       │                                      │
│  ┌──────────┐                                │
│  │  Server   │ HTTP :9099                    │
│  └──────────┘                                │
└──────────────────────────────────────────────┘
```

## Paketler

| Paket | Açıklama |
|-------|----------|
| `config` | Yapılandırma, QoS sınıfları (6 seviye), bant genişliği ağırlıkları |
| `router` | Dijkstra tabanlı tıkanıklık-farkında yönlendirme, ağ topoloji grafı |
| `flow` | Veri akışı yaşam döngüsü, QoS-tabanlı bant genişliği tahsisi |
| `balancer` | 5 stratejili yük dengeleyici (round-robin, least-conn, weighted, congestion-aware, random) |
| `monitor` | Link kullanım izleme, tıkanıklık tespiti, alarm üretimi |
| `server` | 20+ HTTP endpoint |

## QoS Sınıfları

| Sınıf | Öncelik | BW Payı | Kullanım |
|-------|---------|---------|----------|
| CRITICAL | 0 | %30 | Düşman ateş alarmı, hedef vurma onayı |
| REALTIME | 1 | %25 | Canlı video, ses |
| COMMAND | 2 | %20 | Komuta emirleri |
| TELEMETRY | 3 | %15 | Sensör telemetri |
| BULK | 4 | %7 | Harita güncelleme, firmware |
| BEST_EFFORT | 5 | %3 | Diğer trafik |

## HTTP Endpoint'leri

| Yol | Metod | Açıklama |
|-----|-------|----------|
| `/health` | GET | Sağlık kontrolü |
| `/stats` | GET | Tüm istatistikler |
| `/topology` | GET | Topoloji özeti |
| `/nodes` | GET/POST | Düğüm listele/ekle |
| `/nodes/{id}` | GET/DELETE | Düğüm getir/sil |
| `/links` | GET/POST | Bağlantı listele/ekle |
| `/links/{id}` | GET/PUT | Bağlantı getir/güncelle |
| `/route` | POST | Tıkanıklık-farkında rota bul |
| `/routes/alternate` | POST | Alternatif rotalar |
| `/flows` | GET/POST | Akış listele/oluştur |
| `/flows/{id}` | GET/PUT/DELETE | Akış yönet |
| `/flows/allocate` | POST | BW yeniden tahsis |
| `/flows/expire` | POST | Süresi dolan akışları kapat |
| `/balancer` | GET | Yük dengeleyici durumu |
| `/balancer/select` | POST | Backend seç |
| `/balancer/backends` | GET/POST | Backend yönet |
| `/balancer/strategy` | GET/PUT | Strateji değiştir |
| `/monitor/snapshots` | GET | Link snapshot'ları |
| `/monitor/alerts` | GET | Tıkanıklık alarmları |
| `/monitor/congestion` | GET | Tıkanıklık raporu |
| `/monitor/history/{id}` | GET | Link geçmişi |

## Portlar

- HTTP: 9099
- gRPC: 9098

## Testler

```bash
cd akim && go test ./... -v
```

64 test, 6 benchmark.
