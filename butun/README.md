# BÜTÜN - ÖTÜKEN-MATRİS Ana Çatı Sistemi

## Genel Bakış

BÜTÜN, ÖTÜKEN-MATRİS sisteminin kalbidir. Tüm 60 modülü birleştiren mikroservis 
mimarisinin ana kapsayıcısı olarak **Service Mesh**, **API Gateway** ve 
**Orkestrasyon** işlevlerini üstlenir.

## Mimari Bileşenler

```
butun/
├── src/
│   ├── api_gateway/          # API Gateway - Merkezi istek yönlendirme
│   │   ├── gateway.py        # FastAPI tabanlı ana gateway
│   │   └── middleware.py     # Zero Trust middleware
│   ├── service_mesh/         # Service Mesh
│   │   ├── registry.py       # Servis kayıt defteri
│   │   ├── discovery.py      # Servis keşfi (DNS-benzeri)
│   │   ├── load_balancer.py  # Yük dengeleme (RR/LC/Weighted)
│   │   └── circuit_breaker.py # Devre kesici deseni
│   ├── orchestrator/         # Orkestrasyon
│   │   ├── lifecycle.py      # Yaşam döngüsü yönetimi
│   │   ├── scheduler.py      # Topolojik başlatma sıralaması
│   │   └── scaler.py         # Otomatik ölçekleme (HPA benzeri)
│   ├── resource_manager/     # Kaynak Yönetimi
│   │   ├── monitor.py        # CPU/RAM/Disk izleme
│   │   └── allocator.py      # Adil kaynak dağıtımı
│   ├── dependency/           # Bağımlılık Yönetimi
│   │   └── graph.py          # DAG tabanlı bağımlılık grafiği
│   ├── models/               # Veri Modelleri
│   │   └── service.py        # Pydantic modelleri
│   └── main.py               # Ana giriş noktası
├── config/
│   └── default.yaml          # Varsayılan yapılandırma
├── tests/                    # Birim testleri
├── Dockerfile
└── requirements.txt
```

## Küme Bağımlılık Grafiği

```
ULAK (Faz 1) ──┬──> GÖRGÜ (Faz 2) ──> BUYRUK (Faz 3) ──> KOVAN (Faz 4) ──> KIN (Faz 5)
               └──> BUYRUK (Faz 3)
               └──> KOVAN (Faz 4)
               └──> KIN (Faz 5)
```

## Kurulum

```bash
cd butun
pip install -r requirements.txt
```

## Çalıştırma

```bash
python -m src.main
```

Sunucu `http://localhost:8000` adresinde başlar.

## API Endpoints

| Endpoint | Method | Açıklama |
|----------|--------|----------|
| `/` | GET | Sistem durumu |
| `/health` | GET | Sağlık kontrolü |
| `/docs` | GET | Swagger UI |
| `/api/v1/services/register` | POST | Servis kaydı |
| `/api/v1/services/{name}` | GET | Servis bilgisi |
| `/api/v1/services/{name}` | DELETE | Servis kaldırma |
| `/api/v1/services/{name}/heartbeat` | POST | Kalp atışı |
| `/api/v1/services` | GET | Tüm servisleri listele |
| `/api/v1/discover/{name}` | GET | Servis keşfi |
| `/api/v1/orchestrator/start/{name}` | POST | Servis başlat |
| `/api/v1/orchestrator/stop/{name}` | POST | Servis durdur |
| `/api/v1/orchestrator/restart/{name}` | POST | Servis yeniden başlat |
| `/api/v1/orchestrator/schedule` | POST | Başlatma sırası hesapla |
| `/api/v1/scaler/check` | POST | Ölçekleme kontrolü |
| `/api/v1/resources` | GET | Kaynak kullanımı |
| `/api/v1/resources/alerts` | GET | Kaynak alarmları |
| `/api/v1/resources/allocate` | POST | Kaynak tahsisi |
| `/api/v1/dependencies` | GET | Bağımlılık grafiği |
| `/api/v1/dependencies/add` | POST | Bağımlılık ekle |
| `/api/v1/dependencies/{node}/impact` | GET | Etki analizi |
| `/api/v1/circuits` | GET | Circuit breaker durumları |
| `/api/v1/circuits/{name}/reset` | POST | Circuit breaker sıfırla |
| `/api/v1/cluster/{name}` | GET | Küme bilgisi |
| `/api/v1/clusters` | GET | Tüm kümeleri listele |

## Testler

```bash
pip install pytest
pytest tests/ -v
```

## Sıfır Güven (Zero Trust) Mimarisi

Her istek:
- Benzersiz Request ID ile izlenir
- Rate limiting'e tabi tutulur
- İstek süresi ölçülür ve loglanır
- `X-Request-ID` ve `X-Response-Time` header'ları eklenir
