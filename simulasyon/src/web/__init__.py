"""
ÖTÜKEN-MATRİS Simülasyon Web Arayüzü

FastAPI + WebSocket tabanlı canlı simülasyon dashboard'u.
Senaryo başlat/durdur, canlı olay akışı, modül durumu, varlık haritası.
"""

from __future__ import annotations

import asyncio
import json
import os
import threading
import time
import uuid
from pathlib import Path
from typing import Optional

from fastapi import FastAPI, WebSocket, WebSocketDisconnect
from fastapi.responses import HTMLResponse, JSONResponse
from fastapi.staticfiles import StaticFiles

from src.runner import SimulationRunner, load_config, SCENARIO_REGISTRY
from src.generators.sensor import SensorGenerator
from src.generators.battlefield import BattlefieldGenerator
from src.generators.comms import CommsGenerator
from src.clients.butun_client import ButunClient
from src.clients.bag_client import BagClient
from src.clients.kutuk_client import KutukClient
from src.clients.dirlik_client import DirlikClient
from src.clients.bagdas_client import BagdasClient
from src.clients.akim_client import AkimClient
from src.clients.yurt_client import YurtClient
from src.clients.yuzey_client import YuzeyClient
from src.clients.im_client import ImClient
from src.clients.yayin_client import YayinClient
from src.clients.durak_client import DurakClient
from src.clients.tutum_client import TutumClient
from src.clients.tani_client import TaniClient
from src.clients.koz_client import KozClient

STATIC_DIR = Path(__file__).parent / "static"

# Modül açıklamaları — eğitici bilgi
MODULE_INFO = {
    "BÜTÜN": {
        "icon": "🏛️", "port": 8000,
        "role": "Ana Çatı ve Orkestrasyon Sistemi",
        "desc": "Tüm modülleri başlatır, durdurur ve izler. Servis kayıt defteri tutar, heartbeat ile canlılık kontrol eder. Bir modül çökerse yeniden başlatır.",
        "realworld": "Gerçek ortamda: Savaş alanındaki tüm yazılım bileşenlerini tek merkezden yönetir."
    },
    "BAĞ": {
        "icon": "📡", "port": 9091,
        "role": "Mesaj Kuyruğu ve Veri Transfer Katmanı",
        "desc": "Modüller arası veri taşıyıcı. Topic-based publish/subscribe mimarisi kullanır. Sensör verisi, komuta mesajı, alarm — her şey BAĞ üzerinden akar.",
        "realworld": "Gerçek ortamda: Milisaniye gecikmeyle güvenli, öncelikli mesajlaşma sağlar."
    },
    "KÜTÜK": {
        "icon": "🗄️", "port": 9093,
        "role": "Dağıtık Anahtar-Değer Veritabanı",
        "desc": "Anlık durum deposu. RAFT konsensüs ile tutarlı. Varlık konumları, sensör okumaları, sistem durumları burada saklanır.",
        "realworld": "Gerçek ortamda: 3+ düğümde tutarlılık garanti eder, tek düğüm çökse bile veri kaybolmaz."
    },
    "DİRLİK": {
        "icon": "🏥", "port": 9095,
        "role": "Sistem Sağlık İzleme ve Anomali Tespiti",
        "desc": "CPU, RAM, disk kullanımını ölçer. Servis probe'ları ile her modülün canlılığını kontrol eder. Anomali tespit ederse alarm üretir.",
        "realworld": "Gerçek ortamda: Donanım arızalarını erken tespit eder, otomatik müdahale başlatır."
    },
    "BAĞDAŞ": {
        "icon": "🔄", "port": 9097,
        "role": "Polyglot Veri Dönüştürücü ve Şema Doğrulayıcı",
        "desc": "Farklı veri formatları arasında köprü kurar. JSON ↔ MsgPack ↔ CBOR dönüşümü yapar. Şema doğrulama ve veri pipeline'ları çalıştırır.",
        "realworld": "Gerçek ortamda: NATO STANAG formatlarıyla uyumluluk, müttefik sistemlerle entegrasyon."
    },
    "AKIM": {
        "icon": "🌊", "port": 9099,
        "role": "Ağ Trafiği Yönlendirme ve QoS Yönetimi",
        "desc": "Veri trafiğini yönetir. QoS bazlı önceliklendirme yapar. Kritik alarm → düşük gecikme, telemetri → normal kuyruk. Tıkanıklık yönetimi sağlar.",
        "realworld": "Gerçek ortamda: Bant genişliği kısıtlı taktik ağlarda verimli iletişim."
    },
    "YURT": {
        "icon": "🏕️", "port": 9101,
        "role": "Uç Bilişim Düğümü ve Yerel Depolama",
        "desc": "Sahada çalışan edge düğüm. Merkeze bağlanamadığında bile yerel depolama ile çalışmaya devam eder. Bağlantı gelince senkronize olur.",
        "realworld": "Gerçek ortamda: Bölünme (partition) toleransı, çevrimdışı operasyon desteği."
    },
    "YÜZEY": {
        "icon": "🗺️", "port": 9103,
        "role": "Taktiksel Görüntüleme ve Durum Resmi",
        "desc": "Savaş alanı haritası. Tüm varlıkları, sensör verilerini, alarm bölgelerini katmanlı olarak görüntüler. Komutanın 'büyük resmi' görmesini sağlar.",
        "realworld": "Gerçek ortamda: Katmanlı harita ile dost/düşman/tarafsız görüntüleme."
    },
    "İM": {
        "icon": "🔐", "port": 9105,
        "role": "Dijital Kimlik ve İmza Yönetimi (Zero Trust PKI)",
        "desc": "Sıfır Güven mimarisinin temeli. Her birime ECDSA P-256 sertifika üretir, JWT token çifti verir (erişim 15dk + yenileme 7gün). Anahtar döndürme, sertifika iptali ve oturum limiti yönetir.",
        "realworld": "Gerçek ortamda: Sahte düğüm engelleme, iletişim şifrelemesi, birim doğrulama, rol bazlı erişim kontrolü (RBAC)."
    },
    "YAYIN": {
        "icon": "📡", "port": 9107,
        "role": "Şifreli ve Sıkıştırılmış Veri Yayımı",
        "desc": "Komuta emirlerinin, koordinatların ve güncellemelerin sahadaki birimlere güvenli yayınını yönetir. AES-256-GCM uçtan uca şifreleme, DEFLATE kayıpsız sıkıştırma ve FEC (İleri Hata Düzeltme) ile paket kaybına karşı koruma sağlar.",
        "realworld": "Gerçek ortamda: Dar bant genişliğinde güvenli emir iletimi, karıştırılmış RF ortamında hata toleranslı veri aktarımı."
    },
    "DURAK": {
        "icon": "⏸️", "port": 9109,
        "role": "Veri Paketi Önceliklendirme (QoS)",
        "desc": "Sınırlı bant genişliğinde öncelik tabanlı paket kuyruklama. Min-heap ile 6 öncelik seviyesi (FLASH→BULK), WFQ zamanlayıcı, TTL bazlı paket süresi dolma ve anlık bant genişliği ayarlama. Komut kuyrukları, sensör, telemetri, video ve toplu veri için ayrı kuyruklar.",
        "realworld": "Gerçek ortamda: NBC alarm tüm trafiğin önüne geçer, video akışı komut trafiğini engellemez, düşük bant genişliğinde kritik veri kaybı önlenir."
    },
    "TUTUM": {
        "icon": "🔋", "port": 9111,
        "role": "Batarya ve Kaynak Tüketimi İzleyicisi",
        "desc": "İHA, otonom araç ve taktik radyo batarya durumunu izler. Coulomb sayımı ile SoC/SoH takibi, doğrusal regresyon ile ömür tahmini, rüzgar/arazi/yük çevresel faktör analizi. Otomatik RTB (geri dönüş) önerisi ve görev yapılabilirlik değerlendirmesi. CPU/Bellek/Sıcaklık kaynak izleme ve çok seviyeli uyarı sistemi.",
        "realworld": "Gerçek ortamda: İHA'lar batarya bittiğinde düşmez, otonom araçlar görev yarıda kalmaz, kritik enerji seviyesinde otomatik geri çağırma."
    },
    "TANI": {
        "icon": "🎯", "port": 9113,
        "role": "Nesne Tespit ve Sınıflandırma",
        "desc": "YOLOv7 tabanlı simüle nesne tespiti, 14 askeri sınıf (tank, APC, helikopter, İHA, radar, SAM vb.). Non-Maximum Suppression, çoklu nesne izleme (IoU tabanlı), IFF (Dost/Düşman Tanıma) sistemi, 5 kademeli tehdit seviyesi hesaplama ve hız vektörü tahmini.",
        "realworld": "Gerçek ortamda: Termal/optik kameralardan canlı görüntü işleme, düşman zırhlı araç tespiti, İHA sürüsü takibi, SAM mevzii konumlama ve otomatik ateş kontrol entegrasyonu."
    },
    "KÖZ": {
        "icon": "🔥", "port": 9115,
        "role": "Termal İz ve Motor Sıcaklığı Analizi",
        "desc": "Kızılötesi algılayıcı simülasyonu. 16 araç tipi için termal profil, soğuma eğrisi tahmini (Newton soğuma), termal kümeleme (birlik/konvoy tespiti), anomali tespiti (ani ısınma, kamuflaj). Motor durumu analizi ve hareket tahmini.",
        "realworld": "Gerçek ortamda: FLIR/termal kamera verileriyle düşman araç tespiti, motor çalışma durumu analizi, gizli mevzi keşfi, termal kamuflaj geçme."
    },
}

app = FastAPI(title="ÖTÜKEN-MATRİS Simülasyon", version="1.0.0")
app.mount("/static", StaticFiles(directory=str(STATIC_DIR)), name="static")

# ── Global State ──
sim_state = {
    "running": False,
    "scenario": None,
    "tick": 0,
    "max_ticks": 0,
    "events": [],
    "stats": {"messages_sent": 0, "errors": 0},
    "entities": [],
    "recent_events": [],
    "results": None,
    "devices": {},
}
sim_lock = threading.Lock()
ws_clients: list[WebSocket] = []
_stop_event = threading.Event()


async def broadcast(data: dict):
    """Tüm WebSocket istemcilerine gönder."""
    dead = []
    for ws in ws_clients:
        try:
            await ws.send_json(data)
        except Exception:
            dead.append(ws)
    for ws in dead:
        ws_clients.remove(ws)


def _run_simulation_thread(scenario_name: str, config: dict, loop: asyncio.AbstractEventLoop):
    """Ayrı thread'de simülasyon çalıştır."""
    global sim_state
    _stop_event.clear()

    runner = SimulationRunner(config=config)
    scenario = runner.select_scenario(scenario_name)

    with sim_lock:
        sim_state["running"] = True
        sim_state["scenario"] = scenario.name
        sim_state["max_ticks"] = scenario.max_ticks
        sim_state["tick"] = 0
        sim_state["events"] = []
        sim_state["recent_events"] = []
        sim_state["stats"] = {"messages_sent": 0, "errors": 0}
        sim_state["results"] = None
        sim_state["devices"] = {}

    scenario.setup()
    scenario.stats["start_time"] = time.time()

    for tick in range(scenario.max_ticks):
        if _stop_event.is_set():
            break

        msgs_before = scenario.stats["messages_sent"]

        try:
            tick_events = scenario.on_tick(tick)
            scenario.events.extend(tick_events)
            scenario.tick_count = tick + 1
        except Exception as e:
            tick_events = [{"type": "error", "tick": tick, "error": str(e)}]
            scenario.stats["errors"] += 1

        # Tick başına mesaj sayısı (throughput grafiği için)
        scenario.tick_message_counts.append(scenario.stats["messages_sent"] - msgs_before)

        # Entity snapshot
        entities = [e.to_dict() for e in scenario.battlefield_gen.entities.values()]

        # Cihaz durumu snapshot
        device_snapshot = {}
        try:
            ym = scenario.cihaz_yoneticisi
            device_snapshot = {
                "summary": ym.stats(),
                "devices": {
                    cid: {
                        "id": c.cihaz_id,
                        "tip": c.kimlik.tip.value,
                        "surucu": c.kimlik.surucu,
                        "durum": c.durum.value,
                        "konum": c.kimlik.konum,
                        "uretilen": c.stats["uretilen_veri"],
                        "yayinlanan": c.stats["yayinlanan_mesaj"],
                        "hatalar": c.stats["hatalar"],
                        "topic": c.topic,
                    }
                    for cid, c in ym._cihazlar.items()
                },
                "prod": ym.prod_stats(),
            }
        except Exception:
            pass

        with sim_lock:
            sim_state["tick"] = tick + 1
            sim_state["stats"] = dict(scenario.stats)
            sim_state["module_stats"] = dict(scenario.module_stats)
            sim_state["recent_events"] = tick_events[-20:]
            sim_state["entities"] = entities
            sim_state["devices"] = device_snapshot

        # WebSocket broadcast
        msg = {
            "type": "tick",
            "tick": tick + 1,
            "max_ticks": scenario.max_ticks,
            "stats": dict(scenario.stats),
            "module_stats": dict(scenario.module_stats),
            "events": tick_events[-10:],
            "entity_count": len(entities),
            "devices": device_snapshot.get("summary", {}),
        }
        try:
            asyncio.run_coroutine_threadsafe(broadcast(msg), loop)
        except RuntimeError:
            pass  # Event loop kapalı (test ortamı)

        time.sleep(scenario.tick_interval)

    scenario.stats["end_time"] = time.time()
    results = scenario.on_finish()

    # Simülasyon bitti — YÜZEY'e "simülasyon sona erdi" olayı gönder ve varlıkları temizle
    try:
        scenario.yuzey.emit_event(
            event_type="simulation_end",
            source="SİMÜLASYON",
            severity="info",
            title="Simülasyon Tamamlandı",
            detail=f"{scenario.name} senaryosu sona erdi",
        )
        scenario.yuzey.clear_entities()
    except Exception:
        pass

    scenario.close()

    with sim_lock:
        sim_state["running"] = False
        sim_state["results"] = results

    try:
        asyncio.run_coroutine_threadsafe(
            broadcast({"type": "finished", "results": results}), loop
        )
    except RuntimeError:
        pass


# ── Routes ──

@app.get("/", response_class=HTMLResponse)
async def index():
    return (STATIC_DIR / "index.html").read_text(encoding="utf-8")


@app.get("/api/state")
async def get_state():
    with sim_lock:
        return JSONResponse(sim_state)


@app.get("/api/scenarios")
async def list_scenarios():
    return [{"name": k, "description": v.description} for k, v in SCENARIO_REGISTRY.items()]


@app.post("/api/start")
async def start_simulation(body: dict = None):
    body = body or {}
    env = os.environ.get("OTUKEN_ENV", "development").lower()
    if env not in ("development", "dev", "test", "local"):
        return JSONResponse(
            {"error": f"Simülasyon sadece geliştirme ortamında çalışır (OTUKEN_ENV={env})"},
            status_code=403,
        )
    if sim_state["running"]:
        return JSONResponse({"error": "Simülasyon zaten çalışıyor"}, status_code=409)

    config = load_config()
    sim_cfg = config.setdefault("simulation", {})
    sim_cfg["max_ticks"] = body.get("ticks", 100)
    sim_cfg["tick_interval"] = body.get("interval", 0.3)
    sim_cfg["connect_modules"] = True  # Her zaman gerçek modüllerle konuş
    # Modül bilgilerini simülasyon config'ine aktar
    sim_cfg["modules"] = config.get("modules", {})
    bf = sim_cfg.setdefault("battlefield", {})
    bf["friendly_units"] = body.get("friendly", 20)
    bf["enemy_units"] = body.get("enemy", 15)
    bf["drone_count"] = body.get("drones", 10)
    bf["sensor_count"] = body.get("sensors", 30)

    scenario_name = body.get("scenario", "temel")
    loop = asyncio.get_event_loop()
    t = threading.Thread(target=_run_simulation_thread, args=(scenario_name, config, loop), daemon=True)
    t.start()

    return {"status": "started", "scenario": scenario_name}


@app.post("/api/stop")
async def stop_simulation():
    if not sim_state["running"]:
        return JSONResponse({"error": "Çalışan simülasyon yok"}, status_code=400)
    _stop_event.set()
    return {"status": "stopping"}


@app.get("/api/entities")
async def get_entities():
    with sim_lock:
        return sim_state["entities"]


@app.get("/api/devices")
async def get_devices():
    """Cihaz durumları — aktif cihazların anlık bilgisi."""
    with sim_lock:
        return sim_state.get("devices", {})


@app.get("/api/modules/status")
async def check_modules():
    results = {}
    for name, cls in [("BÜTÜN", ButunClient), ("BAĞ", BagClient), ("KÜTÜK", KutukClient), ("DİRLİK", DirlikClient), ("BAĞDAŞ", BagdasClient), ("AKIM", AkimClient), ("YURT", YurtClient), ("YÜZEY", YuzeyClient), ("İM", ImClient), ("YAYIN", YayinClient), ("DURAK", DurakClient), ("TUTUM", TutumClient), ("TANI", TaniClient), ("KÖZ", KozClient)]:
        client = cls()
        info = MODULE_INFO.get(name, {})
        results[name] = {
            "alive": client.is_alive(),
            "url": client.base_url,
            "icon": info.get("icon", ""),
            "role": info.get("role", ""),
            "desc": info.get("desc", ""),
            "realworld": info.get("realworld", ""),
        }
        client.close()
    return results


@app.get("/api/modules/info")
async def module_info():
    """Modül açıklamaları — eğitici bilgi."""
    return MODULE_INFO


@app.get("/api/generate/{gen_type}")
async def generate_data(gen_type: str, count: int = 5):
    if gen_type == "sensor":
        gen = SensorGenerator(seed=int(time.time()))
        return [r.to_bag_payload() for r in gen.generate_burst(count)]
    elif gen_type == "battlefield":
        gen = BattlefieldGenerator(seed=int(time.time()))
        entities = gen.generate_battlefield(friendly=count//3 or 1, enemy=count//3 or 1)
        return [e.to_dict() for e in entities]
    elif gen_type == "comms":
        gen = CommsGenerator(seed=int(time.time()))
        return [m.to_dict() for m in gen.generate_burst(count)]
    return JSONResponse({"error": "Bilinmeyen tip"}, status_code=400)


@app.websocket("/ws")
async def websocket_endpoint(ws: WebSocket):
    await ws.accept()
    ws_clients.append(ws)
    try:
        # İlk bağlantıda mevcut durumu gönder
        with sim_lock:
            await ws.send_json({"type": "state", **sim_state})
        while True:
            data = await ws.receive_text()
            if data == "ping":
                await ws.send_json({"type": "pong"})
    except WebSocketDisconnect:
        if ws in ws_clients:
            ws_clients.remove(ws)


def start_server(host: str = "0.0.0.0", port: int = 8080):
    """Web sunucusunu başlat."""
    import uvicorn
    print(f"\n🌐 ÖTÜKEN-MATRİS Web Arayüzü: http://localhost:{port}")
    uvicorn.run(app, host=host, port=port, log_level="warning")
