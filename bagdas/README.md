# BAĞDAŞ — Polyglot Köprü & Format Dönüşüm Motoru

ÖTÜKEN-MATRİS'in **Foreign Function Interface (FFI)** modülüdür. Farklı dillerdeki bileşenleri (Go, Python, C++) kayıpsız bir şekilde birbirine bağlar.

## Mimari

```
┌────────────────────────────────────────────┐
│              BAĞDAŞ HTTP :9097             │
│                                            │
│  ┌──────────┐  ┌────────┐  ┌───────────┐ │
│  │  Codec   │  │ Schema │  │  Pipeline  │ │
│  │ Registry │  │Registry│  │  Registry  │ │
│  └─────┬────┘  └───┬────┘  └─────┬─────┘ │
│        │           │             │        │
│  ┌─────┴───────────┴─────────────┴─────┐  │
│  │           Adapter Layer             │  │
│  │  BÜTÜN ↔ BAĞ ↔ KÜTÜK ↔ DİRLİK    │  │
│  └─────────────────────────────────────┘  │
└────────────────────────────────────────────┘
```

## Paketler

| Paket | Açıklama |
|-------|----------|
| `codec` | JSON, MessagePack, CBOR — saf Go, bağımlılıksız |
| `schema` | Şema versiyonlama, forward/backward uyumluluk |
| `adapter` | Modül arası protokol köprüsü, retry, format dönüşüm |
| `pipeline` | Composable dönüşüm pipeline: validate → enrich → filter |
| `server` | 16 HTTP endpoint |
| `config` | Merkezi yapılandırma |

## Codec'ler (Saf Go, Dış Bağımlılık Yok)

- **JSON** — `encoding/json` + `UseNumber()` hassasiyeti
- **MessagePack** — RFC 8949 uyumlu saf Go encoder/decoder
- **CBOR** — RFC 8949 uyumlu saf Go encoder/decoder

Tüm codec'ler round-trip güvenlidir: `decode(encode(data)) == data`

## HTTP API

| Endpoint | Metod | Açıklama |
|----------|-------|----------|
| `/healthz` | GET | Sağlık kontrolü |
| `/api/convert` | POST | Format dönüştürme |
| `/api/validate` | POST | Şema doğrulama |
| `/api/schemas` | GET/POST | Şema CRUD |
| `/api/schemas/{name}` | GET/DELETE | Şema detay/silme |
| `/api/schemas/{name}/{ver}` | GET | Belirli versiyon |
| `/api/modules` | GET/POST | Modül yönetimi |
| `/api/bridge` | POST | Modül arası köprü |
| `/api/pipelines` | GET | Pipeline listesi |
| `/api/pipeline/execute` | POST | Pipeline çalıştır |
| `/api/formats` | GET | Desteklenen formatlar |
| `/api/stats` | GET | İstatistikler |

## Testler

```bash
go test ./... -v
# 153 test, 5 benchmark
```

## Çalıştırma

```bash
cd bagdas && go run cmd/bagdas/main.go
# HTTP :9097
```
