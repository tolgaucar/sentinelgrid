"""SimOptik — Sanal Optik Kamera Sürücüsü.

Prod'da: Axis, Hikvision vb. gerçek IP kamera sürücüsüyle değiştirilir.
"""

from __future__ import annotations

import math
import random
import time

from src.cihaz.base import BaseCihaz, CihazAyar, CihazKimlik, CihazTip

MILITARY_CLASSES = [
    "tank", "apc", "truck", "humvee", "helicopter", "fighter_jet",
    "uav", "ucav", "soldier", "sniper", "civilian",
    "radar", "sam_launcher", "artillery", "command_post",
]


class SimOptik(BaseCihaz):
    """Simüle optik kamera — nesne tespiti verisi üretir."""

    def __init__(self, kimlik: CihazKimlik, ayar: CihazAyar):
        kimlik.tip = CihazTip.OPTIK_KAMERA
        kimlik.kume = "GORGU"
        super().__init__(kimlik, ayar)
        self._targets: list[dict] = []
        self._frame_counter = 0

    @property
    def topic(self) -> str:
        return "sensor.optical"

    def set_targets(self, entities: list[dict]):
        self._targets = entities

    def on_tick(self) -> list[dict]:
        readings = []
        lat, lon = self.kimlik.konum
        menzil_m = self.ayar.menzil_km * 1000
        self._frame_counter += 1

        for target in self._targets:
            t_lat = target.get("lat", 0)
            t_lon = target.get("lon", 0)
            dist_m = _haversine_m(lat, lon, t_lat, t_lon)

            if dist_m > menzil_m:
                continue
            if target.get("status") == "destroyed":
                if random.random() > 0.3:  # Enkazı bazen göremeyiz
                    continue

            # Mesafe arttıkça güven düşer
            confidence = max(0.1, 1.0 - (dist_m / menzil_m) ** 0.7)
            # Gece/kötü hava simülasyonu
            confidence *= random.uniform(0.7, 1.0)

            t_type = target.get("type", "unknown")
            bbox_size = max(10, int(200 * (1 - dist_m / menzil_m)))

            readings.append({
                "sensor_id": self.cihaz_id,
                "sensor_type": "optical",
                "timestamp_ns": time.time_ns(),
                "target_id": target.get("id", ""),
                "frame_id": f"FRM-{self._frame_counter:08d}",
                "bbox_x": random.randint(50, 500),
                "bbox_y": random.randint(50, 400),
                "bbox_w": bbox_size,
                "bbox_h": int(bbox_size * random.uniform(0.6, 1.2)),
                "class_label": t_type,
                "confidence": round(confidence, 2),
                "range_km": round(dist_m / 1000, 2),
                "lat": round(t_lat, 6),
                "lon": round(t_lon, 6),
            })

        return readings


class SimOptikFactory:
    @staticmethod
    def create(config: dict) -> SimOptik:
        konum = config.get("konum", [39.925, 32.837])
        kimlik = CihazKimlik(
            cihaz_id=config.get("id", f"optik-{random.randint(1000,9999)}"),
            ad=config.get("ad", "Optik Kamera"),
            surucu=config.get("surucu", "sim"),
            konum=tuple(konum),
        )
        ayar = CihazAyar(
            frekans_hz=config.get("frekans_hz", 25.0),
            menzil_km=config.get("menzil_km", 30.0),
            cozunurluk=config.get("cozunurluk", 1920),
            fov_derece=config.get("fov_derece", 60.0),
            oncelik=config.get("oncelik", 5),
            ttl_ms=config.get("ttl_ms", 3000),
        )
        return SimOptik(kimlik, ayar)


def _haversine_m(lat1, lon1, lat2, lon2) -> float:
    R = 6_371_000
    dLat = math.radians(lat2 - lat1)
    dLon = math.radians(lon2 - lon1)
    a = math.sin(dLat / 2) ** 2 + math.cos(math.radians(lat1)) * math.cos(math.radians(lat2)) * math.sin(dLon / 2) ** 2
    return R * 2 * math.atan2(math.sqrt(a), math.sqrt(1 - a))
