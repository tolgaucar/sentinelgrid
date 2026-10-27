# KÜTÜK — Anlık Durum Veritabanı

**ÖTÜKEN-MATRİS / ULAK Kümesi**

Savaş sahasındaki dinamik objelerin gerçek zamanlı durumlarının saklandığı dağıtık bellek-içi (in-memory) veritabanı.

## Mimari

```
┌─────────────────────────────────────────┐
│              KÜTÜK Sunucu               │
│                                         │
│  ┌──────────┐  ┌──────┐  ┌──────────┐  │
│  │  Sharded  │  │ RAFT │  │   WAL    │  │
│  │ KV Store  │←→│Engine│  │(Disk Log)│  │
│  │ (64 shard)│  └──────┘  └──────────┘  │
│  └──────────┘                           │
│       ↕                                 │
│  ┌──────────┐  ┌──────────────────┐     │
│  │   gRPC   │  │   HTTP Status    │     │
│  │  :9092   │  │     :9093        │     │
│  └──────────┘  └──────────────────┘     │
└─────────────────────────────────────────┘
```

## Özellikler

| Özellik | Detay |
|---------|-------|
| **Performans** | ~105ns GET, ~484ns PUT (Apple M1) |
| **Eşzamanlılık** | 64 shard ile lock contention minimizasyonu |
| **TTL** | Anahtar bazında yaşam süresi, otomatik temizlik |
| **Namespace** | `entity`, `sensor`, `track`, `alert`, `command`, `telemetry`, `system` |
| **Watch** | Gerçek zamanlı değişiklik izleme (prefix filtreli) |
| **RAFT** | Lider seçimi, log replikasyonu, term yönetimi |
| **WAL** | CRC32 checksum'lı write-ahead log, segment rotasyonu |
| **Snapshot** | JSON tabanlı yedekleme/geri yükleme |

## Çalıştırma

```bash
cd kutuk
go build ./cmd/kutuk/
./kutuk
```

## HTTP API

| Endpoint | Method | Açıklama |
|----------|--------|----------|
| `/health` | GET | Sağlık kontrolü |
| `/status` | GET | Detaylı durum bilgisi |
| `/put` | POST | Anahtar-değer yaz |
| `/get?ns=X&key=Y` | GET | Anahtar oku |
| `/delete?ns=X&key=Y` | DELETE | Anahtar sil |
| `/scan?ns=X&prefix=Y` | GET | Prefix ile tara |
| `/keys?ns=X` | GET | Anahtarları listele |
| `/namespaces` | GET | Namespace'leri listele |
| `/raft` | GET | RAFT durumu |

## Testler

```bash
go test ./... -v          # 33 test
go test ./internal/store/ -bench=. -benchmem  # Benchmark
```

## Portlar

- **gRPC**: 9092
- **HTTP**: 9093
