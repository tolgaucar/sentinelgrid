"""
Senaryo Temel Sınıfı

Tüm simülasyon senaryoları bu sınıftan türer. Yeni senaryo
eklemek için BaseScenario'yu subclass'layıp gerekli metodları override edin.

Cihaz Soyutlama: Sensör ve cihaz verileri CihazYoneticisi aracılığıyla
üretilir. Prod'a geçişte sadece YAML konfigürasyonu değişir.
"""

from __future__ import annotations

import logging
import time
from abc import ABC, abstractmethod
from pathlib import Path
from typing import Any

from src.generators.sensor import SensorGenerator
from src.generators.battlefield import BattlefieldGenerator
from src.generators.comms import CommsGenerator
from src.cihaz.yonetici import CihazYoneticisi
from src.clients.butun_client import ButunClient
from src.clients.bag_client import BagClient
from src.clients.kutuk_client import KutukClient
from src.clients.dirlik_client import DirlikClient
from src.clients.bagdas_client import BagdasClient
from src.clients.akim_client import AkimClient
from src.clients.yurt_client import YurtClient
from src.clients.yuzey_client import YuzeyClient
from src.clients.koz_client import KozClient
from src.clients.im_client import ImClient
from src.clients.yayin_client import YayinClient
from src.clients.durak_client import DurakClient
from src.clients.tutum_client import TutumClient
from src.clients.tani_client import TaniClient
from src.clients.sezi_client import SeziClient
from src.clients.yanki_client import YankiClient
from src.clients.duyum_client import DuyumClient
from src.clients.baskala_client import BaskalaClient
from src.clients.sozgec_client import SozgecClient
from src.clients.izlek_client import IzlekClient

logger = logging.getLogger("otuken.senaryo")


class BaseScenario(ABC):
    """Senaryo temel sınıfı."""

    name: str = "Temel Senaryo"
    description: str = ""

    def __init__(self, config: dict = None):
        self.config = config or {}
        self.tick_count = 0
        self.max_ticks = self.config.get("max_ticks", 100)
        # tick_interval_ms → saniyeye çevir (YAML'da ms olarak tanımlı)
        tick_ms = self.config.get("tick_interval_ms", self.config.get("tick_interval", 500))
        if isinstance(tick_ms, (int, float)) and tick_ms > 10:
            self.tick_interval = tick_ms / 1000.0
        else:
            self.tick_interval = tick_ms if tick_ms <= 10 else 0.5
        self.events: list[dict] = []
        self.stats = {
            "messages_sent": 0,
            "errors": 0,
            "start_time": 0.0,
            "end_time": 0.0,
        }
        # Detaylı modül istatistikleri
        self.module_stats = {
            "butun": {"requests": 0, "errors": 0, "registrations": 0, "heartbeats": 0, "discoveries": 0},
            "bag": {"publishes": 0, "errors": 0, "sensor_msgs": 0, "c2_msgs": 0, "alert_msgs": 0, "telemetry_msgs": 0, "bytes_sent": 0},
            "kutuk": {"puts": 0, "gets": 0, "deletes": 0, "errors": 0, "entity_writes": 0, "sensor_writes": 0, "track_writes": 0},
            "dirlik": {"checks": 0, "alerts": 0, "probes": 0, "metrics": 0, "errors": 0},
            "bagdas": {"converts": 0, "validates": 0, "pipelines": 0, "bridges": 0, "errors": 0},
            "akim": {"routes": 0, "flows": 0, "balancer_selects": 0, "congestion_checks": 0, "errors": 0},
            "yurt": {"store_puts": 0, "store_gets": 0, "syncs": 0, "proxy_reqs": 0, "partition_checks": 0, "errors": 0},
            "yuzey": {"tactical_updates": 0, "entity_updates": 0, "events_emitted": 0, "layer_updates": 0, "health_checks": 0, "errors": 0},
            "koz": {"scans": 0, "signatures": 0, "clusters": 0, "anomalies": 0, "errors": 0},
            "im": {"enrolls": 0, "tokens_issued": 0, "verifications": 0, "errors": 0},
            "yayin": {"encryptions": 0, "channels": 0, "publishes": 0, "errors": 0},
            "durak": {"prioritized": 0, "queued": 0, "dropped": 0, "errors": 0},
            "tutum": {"checks": 0, "alerts": 0, "predictions": 0, "errors": 0},
            "tani": {"detections": 0, "classifications": 0, "tracks": 0, "errors": 0},
            "sezi": {"scans": 0, "camo_detected": 0, "threats": 0, "errors": 0},
            "yanki": {"detections": 0, "localized": 0, "gunshots": 0, "threats": 0, "errors": 0},
            "duyum": {"reports": 0, "sources": 0, "threats": 0, "trends": 0, "errors": 0},
            "baskala": {"changes": 0, "images": 0, "threats": 0, "fused": 0, "errors": 0},
            "sozgec": {"signals": 0, "bands": 0, "friendly": 0, "hostile": 0, "jamming": 0, "errors": 0},
            "izlek": {"predictions": 0, "intercepts": 0, "tracks": 0, "maneuvers": 0, "errors": 0},
            "cihaz": {"toplam_cihaz": 0, "toplam_veri": 0, "toplam_yayın": 0, "hatalar": 0, "aktif_birim": 0},
        }
        # Sensör tipi bazında sayaçlar
        self.sensor_stats = {"radar": 0, "thermal": 0, "acoustic": 0, "optical": 0, "osint": 0}
        # C2 mesaj tipi bazında sayaçlar
        self.c2_stats = {"order": 0, "report": 0, "intel": 0, "alert": 0, "request": 0, "logistics": 0, "situation": 0, "ack": 0}
        # Savaş alanı olayları
        self.battle_events = {"damage": 0, "destroyed": 0, "course_change": 0, "low_fuel": 0, "alerts": 0}
        # Tick bazında mesaj sayıları (throughput grafiği için)
        self.tick_message_counts: list[int] = []
        self.running = False

        # Üreticiler
        battlefield_cfg = self.config.get("battlefield", {})
        self.sensor_gen = SensorGenerator(
            center_lat=battlefield_cfg.get("center_lat", 39.925),
            center_lon=battlefield_cfg.get("center_lon", 32.837),
        )
        self.battlefield_gen = BattlefieldGenerator(
            center_lat=battlefield_cfg.get("center_lat", 39.925),
            center_lon=battlefield_cfg.get("center_lon", 32.837),
            radius_km=battlefield_cfg.get("radius_km", 50),
        )
        self.comms_gen = CommsGenerator()

        # İstemciler
        modules = self.config.get("modules", {})
        butun_cfg = modules.get("butun", {})
        bag_cfg = modules.get("bag", {})
        kutuk_cfg = modules.get("kutuk", {})
        self.butun = ButunClient(
            host=butun_cfg.get("host", "http://127.0.0.1"),
            port=butun_cfg.get("port", 8000),
        )
        self.bag = BagClient(
            host=bag_cfg.get("host", "http://127.0.0.1"),
            port=bag_cfg.get("port", 9091),
        )
        self.kutuk = KutukClient(
            host=kutuk_cfg.get("host", "http://127.0.0.1"),
            port=kutuk_cfg.get("port", 9093),
        )
        dirlik_cfg = modules.get("dirlik", {})
        self.dirlik = DirlikClient(
            host=dirlik_cfg.get("host", "http://127.0.0.1"),
            port=dirlik_cfg.get("port", 9095),
        )
        bagdas_cfg = modules.get("bagdas", {})
        self.bagdas = BagdasClient(
            host=bagdas_cfg.get("host", "127.0.0.1"),
            port=bagdas_cfg.get("port", 9097),
        )
        akim_cfg = modules.get("akim", {})
        self.akim = AkimClient(
            host=akim_cfg.get("host", "http://127.0.0.1"),
            port=akim_cfg.get("port", 9099),
        )
        yurt_cfg = modules.get("yurt", {})
        self.yurt = YurtClient(
            host=yurt_cfg.get("host", "http://127.0.0.1"),
            port=yurt_cfg.get("port", 9101),
        )
        yuzey_cfg = modules.get("yuzey", {})
        self.yuzey = YuzeyClient(
            host=yuzey_cfg.get("host", "http://127.0.0.1"),
            port=yuzey_cfg.get("port", 9103),
        )
        koz_cfg = modules.get("koz", {})
        self.koz = KozClient(
            host=koz_cfg.get("host", "http://127.0.0.1"),
            port=koz_cfg.get("port", 9115),
        )
        im_cfg = modules.get("im", {})
        self.im = ImClient(
            host=im_cfg.get("host", "http://127.0.0.1"),
            port=im_cfg.get("port", 9105),
        )
        yayin_cfg = modules.get("yayin", {})
        self.yayin = YayinClient(
            host=yayin_cfg.get("host", "http://127.0.0.1"),
            port=yayin_cfg.get("port", 9107),
        )
        durak_cfg = modules.get("durak", {})
        self.durak = DurakClient(
            host=durak_cfg.get("host", "http://127.0.0.1"),
            port=durak_cfg.get("port", 9109),
        )
        tutum_cfg = modules.get("tutum", {})
        self.tutum = TutumClient(
            host=tutum_cfg.get("host", "http://127.0.0.1"),
            port=tutum_cfg.get("port", 9111),
        )
        tani_cfg = modules.get("tani", {})
        self.tani = TaniClient(
            host=tani_cfg.get("host", "http://127.0.0.1"),
            port=tani_cfg.get("port", 9113),
        )
        sezi_cfg = modules.get("sezi", {})
        self.sezi = SeziClient(
            host=sezi_cfg.get("host", "http://127.0.0.1"),
            port=sezi_cfg.get("port", 9116),
        )
        yanki_cfg = modules.get("yanki", {})
        self.yanki = YankiClient(
            host=yanki_cfg.get("host", "http://127.0.0.1"),
            port=yanki_cfg.get("port", 9118),
        )
        duyum_cfg = modules.get("duyum", {})
        self.duyum = DuyumClient(
            host=duyum_cfg.get("host", "http://127.0.0.1"),
            port=duyum_cfg.get("port", 9120),
        )
        baskala_cfg = modules.get("baskala", {})
        self.baskala = BaskalaClient(
            host=baskala_cfg.get("host", "http://127.0.0.1"),
            port=baskala_cfg.get("port", 9122),
        )
        sozgec_cfg = modules.get("sozgec", {})
        self.sozgec = SozgecClient(
            host=sozgec_cfg.get("host", "http://127.0.0.1"),
            port=sozgec_cfg.get("port", 9124),
        )
        izlek_cfg = modules.get("izlek", {})
        self.izlek = IzlekClient(
            host=izlek_cfg.get("host", "http://127.0.0.1"),
            port=izlek_cfg.get("port", 9126),
        )

        # ── Cihaz Yöneticisi ──
        cihaz_yaml = self.config.get("cihaz_yaml", "")
        if not cihaz_yaml:
            # Varsayılan: simulasyon/config/cihazlar.yaml
            default_path = Path(__file__).parent.parent.parent / "config" / "cihazlar.yaml"
            if default_path.exists():
                cihaz_yaml = str(default_path)

        if cihaz_yaml:
            self.cihaz_yoneticisi = CihazYoneticisi.from_yaml(cihaz_yaml)
        else:
            self.cihaz_yoneticisi = CihazYoneticisi()

        # Modül istemcilerini cihazlara enjekte et — HER ZAMAN
        # Simülasyonda da prod'da da aynı akış: cihaz → İM auth → BAĞ publish → YAYIN encrypt
        # Modül ayakta değilse client graceful fail eder (hata loglar, devam eder)
        self.cihaz_yoneticisi.inject_clients(
            butun=self.butun,
            bag=self.bag,
            kutuk=self.kutuk,
            im=self.im,
            yayin=self.yayin,
        )

        # Prod katmanlarını başlat — İM auth + YAYIN şifreleme + zaman sync + sensör füzyon
        # Tüm katmanlar graceful: modül ayakta değilse atlanır, simülasyon durmaz
        prod_config = self.config.get("prod", {})
        self.cihaz_yoneticisi.init_prod_layers(
            im_client=self.im,
            yayin_client=self.yayin,
            zaman_mode=prod_config.get("zaman_mode", "local"),
            zaman_server=prod_config.get("zaman_server", ""),
        )

    @abstractmethod
    def setup(self) -> None:
        """Senaryo başlangıç hazırlıkları."""

    @abstractmethod
    def on_tick(self, tick: int) -> list[dict]:
        """Her tick'te çalışır. Üretilen olayları döndür."""

    def on_finish(self) -> dict:
        """Senaryo bittiğinde çalışır. Kapsamlı rapor döndür."""
        duration = self.stats.get("end_time", 0) - self.stats.get("start_time", 0)
        msg_rate = self.stats["messages_sent"] / duration if duration > 0 else 0
        avg_per_tick = self.stats["messages_sent"] / self.tick_count if self.tick_count > 0 else 0
        peak_tick = max(self.tick_message_counts) if self.tick_message_counts else 0
        min_tick = min(self.tick_message_counts) if self.tick_message_counts else 0

        return {
            "scenario": self.name,
            "description": self.description,
            "ticks": self.tick_count,
            "max_ticks": self.max_ticks,
            "tick_interval": self.tick_interval,
            "stats": self.stats,
            "event_count": len(self.events),
            "duration_sec": round(duration, 2),
            "throughput": {
                "messages_per_sec": round(msg_rate, 1),
                "avg_per_tick": round(avg_per_tick, 1),
                "peak_per_tick": peak_tick,
                "min_per_tick": min_tick,
                "total_events": len(self.events),
            },
            "modules": self.module_stats,
            "sensors": self.sensor_stats,
            "c2_messages": self.c2_stats,
            "battle_events": self.battle_events,
            "tick_history": self.tick_message_counts,
            "cihaz_raporu": self.cihaz_yoneticisi.stats(),
            "prod_katmanlar": self.cihaz_yoneticisi.prod_stats(),
        }

    def run(self, callback=None) -> dict:
        """Senaryoyu çalıştır."""
        self.stats["start_time"] = time.time()
        self.running = True
        self.setup()

        for tick in range(self.max_ticks):
            if not self.running:
                break
            self.tick_count = tick + 1
            msgs_before = self.stats["messages_sent"]

            try:
                tick_events = self.on_tick(tick)
                self.events.extend(tick_events)
            except Exception as e:
                self.stats["errors"] += 1
                self.events.append({"type": "error", "tick": tick, "error": str(e)})
                tick_events = []

            self.tick_message_counts.append(self.stats["messages_sent"] - msgs_before)

            if callback:
                callback(tick, self.tick_count, tick_events)

            time.sleep(self.tick_interval)

        self.running = False
        self.stats["end_time"] = time.time()
        return self.on_finish()

    def stop(self) -> None:
        """Senaryoyu durdur."""
        self.running = False

    def close(self) -> None:
        """Kaynakları kapat."""
        self.cihaz_yoneticisi.stop_all()
        self.butun.close()
        self.bag.close()
        self.kutuk.close()
        self.dirlik.close()
        self.bagdas.close()
        self.akim.close()
        self.yurt.close()
        self.yuzey.close()
        self.koz.close()
        self.im.close()
        self.yayin.close()
        self.durak.close()
        self.tutum.close()
        self.tani.close()
        self.sezi.close()
        self.yanki.close()
        self.duyum.close()
        self.baskala.close()
        self.sozgec.close()
        self.izlek.close()
