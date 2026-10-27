# BAĞ - Veri Transfer Katmanı

**ÖTÜKEN-MATRİS ULAK Kümesi | Faz 1**

Modüller arası düşük gecikmeli (low-latency), asenkron veri iletişim altyapısı. Apache Kafka benzeri olay güdümlü mesajlaşma ve gRPC protokol desteği sunar.

## Dil & Teknoloji

- **Go 1.26** - goroutine tabanlı eşzamanlılık, GC duraklamaları < 1ms
- Lock-free ring buffer kuyruk
- Min-heap priority queue (QoS - DURAK modülü entegrasyonu)
- Exponential backoff retry mekanizması
- Consumer group destekli topic/partition sistemi

## Mimari

```
bag/
├── cmd/bag/main.go              # Sunucu giriş noktası
├── internal/
│   ├── broker/                  # Kafka-benzeri mesaj broker
│   │   ├── broker.go            # Merkezi broker (topic CRUD, publish/consume)
│   │   └── topic.go             # Topic, Partition, ConsumerGroup
│   ├── grpcserver/
│   │   └── server.go            # gRPC servis implementasyonu
│   ├── queue/
│   │   ├── queue.go             # Lock-free ring buffer (FIFO)
│   │   └── priority.go          # Min-heap priority queue
│   ├── transport/
│   │   └── transport.go         # Sender (retry), Receiver (ACK), Dispatcher
│   └── config/
│       └── config.go            # Yapılandırma
├── proto/
│   └── bag.proto                # gRPC protobuf tanımları
└── go.mod
```

## Çalıştırma

```bash
cd bag
go build -o bag ./cmd/bag && ./bag
```

- **gRPC**: `0.0.0.0:9090`
- **HTTP Status**: `0.0.0.0:9091`

## HTTP Endpoints (BÜTÜN Entegrasyonu)

| Endpoint | Method | Açıklama |
|----------|--------|----------|
| `/health` | GET | Sağlık kontrolü (self-test) |
| `/status` | GET | Sistem durumu + istatistikler |
| `/topics` | GET | Topic listesi |
| `/publish?topic=xxx` | POST | HTTP üzerinden mesaj yayınla |

## Varsayılan Topic'ler

```
sensor.radar, sensor.thermal, sensor.acoustic, sensor.optical, sensor.osint
command.buyruk, command.kovan
alert.critical, alert.warning
telemetry.dirlik, telemetry.tutum
```

## Testler

```bash
go test ./... -v
```

## Temel Kavramlar

- **Topic**: Mesaj kategorisi (Kafka topic)
- **Partition**: Topic'in paralel veri bölümü (consistent hashing ile dağıtım)
- **Consumer Group**: Birden fazla tüketicinin aynı topic'i paylaşması
- **Priority Queue**: Kritik veriler (hedef onayı, saldırı alarmı) düşük öncelikli verilerden (video akışı) önce iletilir
- **Retry + Backoff**: Başarısız gönderimler exponential backoff ile tekrar denenir
