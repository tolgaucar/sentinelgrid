"""
CihazYoneticisi — Cihaz Yaşam Döngüsü Yöneticisi.

YAML konfigürasyonundan cihazları yükler, başlatır, durdurur.
Prod'a geçişte: sadece YAML'daki "surucu" alanı değişir.

Kullanım:
    yonetici = CihazYoneticisi.from_yaml("config/cihazlar.yaml")
    yonetici.inject_clients(butun, bag, kutuk)
    yonetici.register_all()
    yonetici.connect_all()
    yonetici.start_all()
    ...
    yonetici.stop_all()
"""

from __future__ import annotations

import logging
import os
from pathlib import Path
from typing import Any

import yaml

from src.cihaz.base import BaseCihaz, CihazDurum

# Sürücü kayıt defteri — yeni sürücü eklemek için buraya ekle
from src.cihaz.suruculer.sim_radar import SimRadarFactory
from src.cihaz.suruculer.sim_flir import SimFLIRFactory
from src.cihaz.suruculer.sim_optik import SimOptikFactory
from src.cihaz.suruculer.sim_akustik import SimAkustikFactory
from src.cihaz.suruculer.sim_gps import SimGPSFactory
from src.cihaz.suruculer.sim_komuta import SimKomutaFactory
from src.cihaz.suruculer.sim_platform import SimPlatformFactory
from src.cihaz.suruculer.sim_birim import SimBirimFactory

logger = logging.getLogger("otuken.cihaz.yonetici")

# Tip → Factory eşlemesi
# Prod'da gerçek sürücü factory'leri de buraya eklenir
DRIVER_REGISTRY: dict[str, dict[str, Any]] = {
    "radar": {
        "sim": SimRadarFactory,
    },
    "flir": {
        "sim": SimFLIRFactory,
    },
    "optik_kamera": {
        "sim": SimOptikFactory,
    },
    "akustik": {
        "sim": SimAkustikFactory,
    },
    "gps": {
        "sim": SimGPSFactory,
    },
    "komuta_terminali": {
        "sim": SimKomutaFactory,
    },
    "platform": {
        "sim": SimPlatformFactory,
    },
    "birim": {
        "sim": SimBirimFactory,
    },
}


def register_driver(tip: str, surucu_adi: str, factory):
    """
    Yeni sürücü kaydet — plug-in sistemi.

    Örnek:
        from my_real_radar import ThalesRadarFactory
        register_driver("radar", "thales_rdr_4000", ThalesRadarFactory)
    """
    if tip not in DRIVER_REGISTRY:
        DRIVER_REGISTRY[tip] = {}
    DRIVER_REGISTRY[tip][surucu_adi] = factory
    logger.info(f"Sürücü kaydedildi: {tip}/{surucu_adi}")


class CihazYoneticisi:
    """Cihaz yaşam döngüsü yöneticisi."""

    def __init__(self):
        self._cihazlar: dict[str, BaseCihaz] = {}
        self._butun_client = None
        self._bag_client = None
        self._kutuk_client = None
        self._im_client = None
        self._yayin_client = None
        self._config: dict = {}

        # Prod katmanları
        self._zaman_sync = None    # ZamanSync instance
        self._sensor_fuzyon = None # SensorFuzyon instance

    @classmethod
    def from_yaml(cls, yaml_path: str | Path) -> "CihazYoneticisi":
        """YAML dosyasından cihaz yöneticisi oluştur."""
        ym = cls()
        path = Path(yaml_path)
        if not path.exists():
            logger.warning(f"YAML bulunamadı: {path}, boş yönetici oluşturuluyor")
            return ym

        with open(path, "r", encoding="utf-8") as f:
            ym._config = yaml.safe_load(f) or {}

        cihaz_listesi = ym._config.get("cihazlar", [])
        for cihaz_cfg in cihaz_listesi:
            tip = cihaz_cfg.get("tip", "")
            surucu = cihaz_cfg.get("surucu", "sim")
            aktif = cihaz_cfg.get("aktif", True)

            if not aktif:
                logger.info(f"Atlanıyor (deaktif): {tip}/{surucu}")
                continue

            cihaz = ym._create_device(tip, surucu, cihaz_cfg)
            if cihaz:
                ym._cihazlar[cihaz.cihaz_id] = cihaz

        logger.info(f"YAML'dan {len(ym._cihazlar)} cihaz yüklendi: {path}")
        return ym

    @classmethod
    def from_config(cls, config: dict) -> "CihazYoneticisi":
        """Python dict konfigürasyonundan cihaz yöneticisi oluştur."""
        ym = cls()
        ym._config = config
        cihaz_listesi = config.get("cihazlar", [])
        for cihaz_cfg in cihaz_listesi:
            tip = cihaz_cfg.get("tip", "")
            surucu = cihaz_cfg.get("surucu", "sim")
            aktif = cihaz_cfg.get("aktif", True)

            if not aktif:
                continue

            cihaz = ym._create_device(tip, surucu, cihaz_cfg)
            if cihaz:
                ym._cihazlar[cihaz.cihaz_id] = cihaz

        return ym

    def _create_device(self, tip: str, surucu: str, config: dict) -> BaseCihaz | None:
        """Sürücü factory'den cihaz oluştur."""
        drivers = DRIVER_REGISTRY.get(tip, {})
        factory = drivers.get(surucu)
        if not factory:
            logger.error(f"Bilinmeyen sürücü: tip={tip}, surucu={surucu}")
            logger.error(f"  Kayıtlı sürücüler: {list(drivers.keys())}")
            return None
        try:
            cihaz = factory.create(config)
            logger.info(f"Cihaz oluşturuldu: {cihaz.cihaz_id} ({tip}/{surucu})")
            return cihaz
        except Exception as e:
            logger.error(f"Cihaz oluşturma hatası: {tip}/{surucu} → {e}")
            return None

    # ── Bağımlılık Enjeksiyonu ──

    def inject_clients(self, butun=None, bag=None, kutuk=None, im=None, yayin=None):
        """Modül istemcilerini tüm cihazlara enjekte et."""
        self._butun_client = butun
        self._bag_client = bag
        self._kutuk_client = kutuk
        if im is not None:
            self._im_client = im
        if yayin is not None:
            self._yayin_client = yayin
        for cihaz in self._cihazlar.values():
            cihaz.inject_clients(butun=butun, bag=bag, kutuk=kutuk, im=im, yayin=yayin)

    # ── Yaşam Döngüsü ──

    def register_all(self) -> int:
        """Tüm cihazları BÜTÜN'e kayıt et."""
        ok = 0
        for cihaz in self._cihazlar.values():
            if cihaz.register():
                ok += 1
        logger.info(f"Kayıt tamamlandı: {ok}/{len(self._cihazlar)}")
        return ok

    def connect_all(self) -> int:
        """Tüm cihazları BAĞ'a bağla."""
        ok = 0
        for cihaz in self._cihazlar.values():
            if cihaz.connect():
                ok += 1
        return ok

    def start_all(self, threaded: bool = False):
        """
        Tüm cihazları başlat.

        threaded=True: Her cihaz kendi thread'inde çalışır (bağımsız frekans).
        threaded=False: tick_once() ile dışarıdan çağrılır (senaryo döngüsü).
        """
        for cihaz in self._cihazlar.values():
            if threaded:
                cihaz.start()
            else:
                cihaz.durum = CihazDurum.CALISIYOR
                cihaz._running = True
        mode = "thread" if threaded else "manual-tick"
        logger.info(f"Tüm cihazlar başlatıldı ({mode}): {len(self._cihazlar)} cihaz")

    def stop_all(self):
        """Tüm cihazları durdur."""
        for cihaz in self._cihazlar.values():
            cihaz.stop()
        logger.info(f"Tüm cihazlar durduruldu: {len(self._cihazlar)} cihaz")

    def tick_all(self) -> dict[str, list[dict]]:
        """
        Tüm cihazlara tek tick çalıştır (threaded=False modu).
        Dönüş: {cihaz_id: [payload, ...]}
        """
        results = {}
        for cid, cihaz in self._cihazlar.items():
            payloads = cihaz.tick_once()
            if payloads:
                results[cid] = payloads
        return results

    # ── Hedef Güncelleme ──

    def update_targets(self, entities: list[dict]):
        """Tüm sensör cihazlarına hedef listesini güncelle."""
        for cihaz in self._cihazlar.values():
            if hasattr(cihaz, "set_targets"):
                cihaz.set_targets(entities)
            if hasattr(cihaz, "set_entities"):
                cihaz.set_entities(entities)

    # ── Sorgulama ──

    def get_device(self, cihaz_id: str) -> BaseCihaz | None:
        return self._cihazlar.get(cihaz_id)

    def get_devices_by_type(self, tip: str) -> list[BaseCihaz]:
        return [c for c in self._cihazlar.values() if c.kimlik.tip.value == tip]

    @property
    def cihaz_listesi(self) -> list[BaseCihaz]:
        return list(self._cihazlar.values())

    @property
    def cihaz_sayisi(self) -> int:
        return len(self._cihazlar)

    def health_all(self) -> dict[str, dict]:
        """Tüm cihazların sağlık raporu."""
        return {cid: c.health() for cid, c in self._cihazlar.items()}

    def stats(self) -> dict:
        """Genel istatistikler."""
        total_data = sum(c.stats["uretilen_veri"] for c in self._cihazlar.values())
        total_published = sum(c.stats["yayinlanan_mesaj"] for c in self._cihazlar.values())
        total_errors = sum(c.stats["hatalar"] for c in self._cihazlar.values())

        by_type: dict[str, int] = {}
        for c in self._cihazlar.values():
            t = c.kimlik.tip.value
            by_type[t] = by_type.get(t, 0) + 1

        return {
            "toplam_cihaz": len(self._cihazlar),
            "tipe_gore": by_type,
            "toplam_uretilen_veri": total_data,
            "toplam_yayinlanan": total_published,
            "toplam_hata": total_errors,
            "calisan": sum(1 for c in self._cihazlar.values() if c.calisiyor),
            "birim_sayisi": self.birim_sayisi,
            "sensor_sayisi": self.sensor_sayisi,
        }

    # ── Prod Katman Yönetimi ──

    def init_prod_layers(self, im_client=None, yayin_client=None,
                         zaman_mode: str = "local", zaman_server: str = ""):
        """Prod katmanlarını başlat (opsiyonel — yoksa simülasyon modu).

        Bu metod çağrılmasa bile sistem çalışır (simülasyon modu).
        Prod'da: scenario veya main bu metodu client'larla çağırır.
        """
        # İM Auth
        if im_client:
            self._im_client = im_client
            for cihaz in self._cihazlar.values():
                cihaz._im_client = im_client
                cihaz.authenticate()

        # YAYIN Şifreleme
        if yayin_client:
            self._yayin_client = yayin_client
            for cihaz in self._cihazlar.values():
                cihaz._yayin_client = yayin_client
                cihaz.enable_encryption()

        # Zaman Senkronizasyonu
        from src.cihaz.zaman_sync import ZamanSync, SyncMode
        try:
            mode = SyncMode(zaman_mode)
        except ValueError:
            mode = SyncMode.LOCAL
        self._zaman_sync = ZamanSync(mode=mode, server=zaman_server)
        offset = self._zaman_sync.measure_offset()
        for cihaz in self._cihazlar.values():
            cihaz.sync_time(offset)

        # Sensör Füzyon
        from src.cihaz.sensor_fuzyon import SensorFuzyon
        self._sensor_fuzyon = SensorFuzyon()

        logger.info(f"Prod katmanlar başlatıldı: İM={'✓' if im_client else '✗'}, "
                     f"YAYIN={'✓' if yayin_client else '✗'}, "
                     f"Zaman={mode.value}, Füzyon=✓")

    def fuse_tick(self, device_results: dict) -> dict:
        """Bir tick'in cihaz verilerini sensör füzyonundan geçir.

        Returns: fused track dict (entity_id → FusedTrack.to_dict())
        """
        if not self._sensor_fuzyon:
            return {}
        count = self._sensor_fuzyon.ingest_device_results(device_results, self)
        tracks = self._sensor_fuzyon.fuse()
        return {eid: t.to_dict() for eid, t in tracks.items()}

    def prod_stats(self) -> dict:
        """Prod katman istatistikleri."""
        result = {}
        if self._zaman_sync:
            result["zaman_sync"] = self._zaman_sync.to_dict()
        if self._sensor_fuzyon:
            result["sensor_fuzyon"] = self._sensor_fuzyon.to_dict()
        # Transport istatistikleri
        transport_stats = {}
        for cid, c in self._cihazlar.items():
            if c._transport and c._transport.is_connected:
                transport_stats[cid] = c._transport.stats.to_dict()
        if transport_stats:
            result["transport"] = transport_stats
        return result

    def add_device(self, cihaz: BaseCihaz):
        """Çalışma zamanında cihaz ekle."""
        cihaz.inject_clients(
            butun=self._butun_client,
            bag=self._bag_client,
            kutuk=self._kutuk_client,
        )
        self._cihazlar[cihaz.cihaz_id] = cihaz
        logger.info(f"Cihaz eklendi: {cihaz.cihaz_id}")

    def remove_device(self, cihaz_id: str) -> bool:
        """Cihaz kaldır."""
        cihaz = self._cihazlar.pop(cihaz_id, None)
        if cihaz:
            cihaz.stop()
            return True
        return False

    # ── Dost Birim Yönetimi ──

    def sync_friendly_entities(self, entities: list[dict], threaded: bool = False) -> int:
        """
        Dost birimler için otomatik cihaz oluştur/güncelle/kaldır.

        BattlefieldGenerator'dan gelen entity listesini alır,
        her dost birim için bir SimBirimModulu cihazı oluşturur.
        Zaten varsa günceller, yok edilmişse kaldırır.

        Returns: aktif birim cihazı sayısı
        """
        friendly_entities = [e for e in entities if e.get("faction") == "friendly"]
        active_birim_ids = set()

        for entity in friendly_entities:
            eid = entity.get("id", "")
            birim_id = f"birim-{eid}"
            active_birim_ids.add(birim_id)

            existing = self._cihazlar.get(birim_id)

            if entity.get("status") == "destroyed":
                # İmha edilen birimin cihazı kapatılır
                if existing:
                    existing.stop()
                    self._cihazlar.pop(birim_id, None)
                    logger.info(f"Birim imha — cihaz kapatıldı: {birim_id}")
                active_birim_ids.discard(birim_id)
                continue

            if existing:
                # Güncelle — konum ve durum
                if hasattr(existing, "set_entity"):
                    existing.set_entity(entity)
            else:
                # Yeni birim → yeni cihaz oluştur
                cihaz = self._create_birim_device(entity)
                if cihaz:
                    cihaz.inject_clients(
                        butun=self._butun_client,
                        bag=self._bag_client,
                        kutuk=self._kutuk_client,
                    )
                    cihaz.register()
                    cihaz.connect()
                    if threaded:
                        cihaz.start()
                    else:
                        cihaz.durum = CihazDurum.CALISIYOR
                        cihaz._running = True
                    self._cihazlar[birim_id] = cihaz

        # Artık sahada olmayan birimlerin cihazlarını kaldır
        birim_keys = [k for k in self._cihazlar if k.startswith("birim-")]
        for bk in birim_keys:
            if bk not in active_birim_ids:
                old = self._cihazlar.pop(bk, None)
                if old:
                    old.stop()

        return len(active_birim_ids)

    def _create_birim_device(self, entity: dict) -> BaseCihaz | None:
        """Bir dost birim için SimBirimModulu oluştur."""
        drivers = DRIVER_REGISTRY.get("birim", {})
        factory = drivers.get("sim")
        if not factory:
            return None
        try:
            config = {"entity": entity, "surucu": "sim", "frekans_hz": 2.0}
            return factory.create(config)
        except Exception as e:
            logger.error(f"Birim cihazı oluşturma hatası: {entity.get('id')} → {e}")
            return None

    @property
    def birim_sayisi(self) -> int:
        """Aktif birim cihaz sayısı."""
        return sum(1 for k in self._cihazlar if k.startswith("birim-"))

    @property
    def sensor_sayisi(self) -> int:
        """Sabit sensör cihaz sayısı (birim hariç)."""
        return sum(1 for k in self._cihazlar if not k.startswith("birim-"))

    def __repr__(self):
        return f"<CihazYoneticisi cihaz={len(self._cihazlar)}>"
