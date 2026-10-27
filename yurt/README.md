# YURT — Uç Bilişim ve Yerel Depolama

ÖTÜKEN-MATRİS ULAK kümesinin 8. modülü. İletişimin veya GPS bağlantısının koptuğu zorlu ortamlarda sistemin çalışmaya devam etmesini sağlayan uç bilişim (Edge Computing) mimarisi.

## Mimari

```
┌─────────────────────────────────────────────┐
│              YURT Edge Node                 │
│                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐ │
│  │  Store    │  │  Sync    │  │  Proxy   │ │
│  │ (32-shard│  │ (LWW /   │  │ (YurtHub │ │
│  │  KV +    │  │  Cloud / │  │  buffer  │ │
│  │  VClock) │  │  Edge /  │  │  replay) │ │
│  │          │  │  Merge)  │  │          │ │
│  └──────────┘  └──────────┘  └──────────┘ │
│                                             │
│  ┌──────────┐  ┌──────────────────────────┐ │
│  │Partition │  │     HTTP Server          │ │
│  │(split-   │  │  (35+ endpoint)         │ │
│  │ brain)   │  │  :9101                  │ │
│  └──────────┘  └──────────────────────────┘ │
└─────────────────────────────────────────────┘
```

## Bileşenler

### 1. Store — Yerel Depolama
- 32-shard concurrent KV store (FNV32a hash)
- Vector Clock ile dağıtık versiyonlama
- Dirty tracking (senkronize edilmemiş veriler)
- TTL tabanlı otomatik temizlik
- Tombstone ile yumuşak silme
- Change log ile delta senkronizasyon desteği

### 2. Sync — Senkronizasyon Motoru
- Çift yönlü asenkron senkronizasyon (Push/Pull)
- 4 çatışma çözüm politikası:
  - **LWW** (Last Write Wins): Zaman damgasına göre
  - **CloudWins**: Bulut verisi öncelikli
  - **EdgeWins**: Edge verisi öncelikli
  - **MergeAll**: Tüm alanları birleştir
- Vector clock tabanlı çatışma tespiti
- Çatışma geçmişi kaydı

### 3. Proxy — YurtHub Sidecar
- Bağlantı durumu yönetimi (Online/Offline/Degraded)
- Offline modda istek tamponlama
- Öncelik tabanlı eviction (buffer dolu olduğunda)
- Bağlantı geldiğinde otomatik replay
- Sağlık kontrolü

### 4. Partition — Ağ Bölünmesi Yönetimi
- Peer yönetimi ve heartbeat izleme
- Durum makinesi: Normal → Partitioned → SplitBrain → Isolated
- Quorum tabanlı split-brain tespiti
- Periyodik otomatik kontrol
- Bölünme olay geçmişi

## API Endpoint'leri

| Kategori | Endpoint | Metod | Açıklama |
|----------|----------|-------|----------|
| Genel | `/health` | GET | Sağlık durumu |
| Genel | `/stats` | GET | Tüm istatistikler |
| Store | `/store/put` | POST | Veri yaz |
| Store | `/store/get` | GET | Veri oku |
| Store | `/store/delete` | POST | Veri sil |
| Store | `/store/keys` | GET | Anahtar listesi |
| Store | `/store/dirty` | GET | Dirty kayıtlar |
| Store | `/store/snapshot` | GET | Tam snapshot |
| Store | `/store/expire` | POST | TTL temizliği |
| Sync | `/sync/push` | POST | Push (edge→cloud) |
| Sync | `/sync/pull` | POST | Pull (cloud→edge) |
| Sync | `/sync/full` | POST | Tam senkronizasyon |
| Sync | `/sync/state` | GET | Sync durumu |
| Sync | `/sync/conflicts` | GET | Çatışma geçmişi |
| Proxy | `/proxy/request` | POST | İstek gönder |
| Proxy | `/proxy/replay` | POST | Buffer replay |
| Proxy | `/proxy/connection` | GET/PUT | Bağlantı durumu |
| Partition | `/partition/state` | GET | Bölünme durumu |
| Partition | `/partition/check` | POST | Kontrol yap |
| Partition | `/partition/peers` | GET/POST | Peer yönetimi |
| Partition | `/partition/heartbeat` | POST | Heartbeat gönder |

## Test Sonuçları

```
ok  otukenmatris/yurt/internal/store      — 28 test (VClock, CRUD, TTL, dirty, snapshot, concurrent)
ok  otukenmatris/yurt/internal/sync       — 13 test (Push, Pull, 4 conflict policy, delta sync)
ok  otukenmatris/yurt/internal/proxy      — 13 test (online/offline, buffer, eviction, replay)
ok  otukenmatris/yurt/internal/partition   — 14 test (state machine, heartbeat, split-brain)
ok  otukenmatris/yurt/internal/server      — 25 test (tüm HTTP endpoint'ler)
───────────────────────────────────────────
TOPLAM: 93 test ✅
```

## Çalıştırma

```bash
cd yurt
go build -o yurt ./cmd/yurt
./yurt

# Veya ortam değişkenleri ile
YURT_HTTP_PORT=9101 YURT_NODE_ID=edge-alpha ./yurt
```

## Portlar

- **HTTP**: 9101
- **gRPC**: 9100 (ayrılmış)
