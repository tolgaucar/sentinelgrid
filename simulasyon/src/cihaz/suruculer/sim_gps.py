"""SimGPS — Sanal GPS/INS Konum Sensörü Sürücüsü.

Prod'da: u-blox, Trimble, askeri GPS/INS modülüyle değiştirilir.
Platform konumunu ve varlık telemetrisini yayınlar.
"""

from __future__ import annotations

import random
import time

from src.cihaz.base import BaseCihaz, CihazAyar, CihazKimlik, CihazTip


class SimGPS(BaseCihaz):
    """Simüle GPS/INS konum takip cihazı."""

    def __init__(self, kimlik: CihazKimlik, ayar: CihazAyar):
        kimlik.tip = CihazTip.GPS
        kimlik.kume = "GORGU"
        super().__init__(kimlik, ayar)
        self._entities: list[dict] = []

    @property
    def topic(self) -> str:
        return "telemetry.position"

    def set_entities(self, entities: list[dict]):
        self._entities = entities

    def on_tick(self) -> list[dict]:
        readings = []
        for entity in self._entities:
            lat = entity.get("lat", 0)
            lon = entity.get("lon", 0)

            # GPS doğruluk simülasyonu (CEP ~3m askeri GPS)
            gps_noise_lat = random.gauss(0, 0.00003)  # ~3m
            gps_noise_lon = random.gauss(0, 0.00003)

            readings.append({
                "sensor_id": self.cihaz_id,
                "sensor_type": "gps",
                "timestamp_ns": time.time_ns(),
                "entity_id": entity.get("id", ""),
                "lat": round(lat + gps_noise_lat, 7),
                "lon": round(lon + gps_noise_lon, 7),
                "altitude_m": entity.get("altitude_m", 0),
                "heading_deg": round(entity.get("heading", 0), 1),
                "speed_kmh": round(entity.get("speed", 0), 1),
                "fix_quality": random.choice(["3D_FIX", "3D_FIX", "3D_FIX", "DGPS", "RTK"]),
                "satellites": random.randint(6, 14),
                "hdop": round(random.uniform(0.5, 2.5), 1),
                "status": entity.get("status", "active"),
                "health": entity.get("health", 100),
                "fuel": entity.get("fuel", 100),
            })

        return readings


class SimGPSFactory:
    @staticmethod
    def create(config: dict) -> SimGPS:
        konum = config.get("konum", [39.925, 32.837])
        kimlik = CihazKimlik(
            cihaz_id=config.get("id", f"gps-{random.randint(1000,9999)}"),
            ad=config.get("ad", "GPS/INS Modülü"),
            surucu=config.get("surucu", "sim"),
            konum=tuple(konum),
        )
        ayar = CihazAyar(
            frekans_hz=config.get("frekans_hz", 1.0),
            oncelik=config.get("oncelik", 6),
            ttl_ms=config.get("ttl_ms", 3000),
        )
        return SimGPS(kimlik, ayar)
