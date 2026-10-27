"""SimPlatform — Sanal Araç/Birlik Platformu Sürücüsü.

Her fiziksel birim (tank, İHA, gemi vb.) bir Platform cihazıdır.
Platform kendi üzerindeki sensörleri, konum bilgisini ve durumunu
YÜZEY ve KÜTÜK'e yayınlar.

Prod'da: Gerçek araç bilgisayarı (VMS) veya birlik takip sistemiyle değiştirilir.
"""

from __future__ import annotations

import random
import time

from src.cihaz.base import BaseCihaz, CihazAyar, CihazKimlik, CihazTip


class SimPlatform(BaseCihaz):
    """Simüle araç/birlik platformu — varlık durumunu yayınlar."""

    def __init__(self, kimlik: CihazKimlik, ayar: CihazAyar):
        kimlik.tip = CihazTip.PLATFORM
        kimlik.kume = "GORGU"
        super().__init__(kimlik, ayar)
        self._entities: list[dict] = []

    @property
    def topic(self) -> str:
        return "entity.update"

    def set_entities(self, entities: list[dict]):
        """Takip edilecek varlıkları ayarla."""
        self._entities = entities

    def on_tick(self) -> list[dict]:
        """Her varlık için durum güncellemesi yayınla."""
        updates = []
        for entity in self._entities:
            updates.append({
                "entity_id": entity.get("id", ""),
                "type": entity.get("type", "unknown"),
                "faction": entity.get("faction", "unknown"),
                "callsign": entity.get("callsign", entity.get("name", "")),
                "lat": entity.get("lat", 0),
                "lon": entity.get("lon", 0),
                "altitude_m": entity.get("altitude_m", 0),
                "heading": entity.get("heading", 0),
                "speed": entity.get("speed", 0),
                "status": entity.get("status", "active"),
                "health": entity.get("health", 100),
                "fuel": entity.get("fuel", 100),
                "ammo": entity.get("ammo", 100),
                "threat_level": entity.get("threat_level", 0),
                "intel_level": entity.get("intel_level", 4),
                "last_update": time.time_ns() // 1_000_000,
                "timestamp_ns": time.time_ns(),
            })
        return updates


class SimPlatformFactory:
    @staticmethod
    def create(config: dict) -> SimPlatform:
        konum = config.get("konum", [39.925, 32.837])
        kimlik = CihazKimlik(
            cihaz_id=config.get("id", f"platform-{random.randint(1000,9999)}"),
            ad=config.get("ad", "Araç Platformu"),
            surucu=config.get("surucu", "sim"),
            konum=tuple(konum),
        )
        ayar = CihazAyar(
            frekans_hz=config.get("frekans_hz", 2.0),  # Her 0.5 saniye
            oncelik=config.get("oncelik", 3),
            ttl_ms=config.get("ttl_ms", 5000),
        )
        return SimPlatform(kimlik, ayar)
