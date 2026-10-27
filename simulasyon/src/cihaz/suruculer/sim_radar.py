"""SimRadar — Sanal Radar Sensörü Sürücüsü.

Prod'da: Thales, ASELSAN ÇAFRAD vb. gerçek radar sürücüsüyle değiştirilir.
Aynı BaseCihaz arayüzü, aynı BAĞ topic'leri, aynı veri formatı.
"""

from __future__ import annotations

import math
import random
import time

from src.cihaz.base import BaseCihaz, CihazAyar, CihazKimlik, CihazTip

# Gerçekçi RCS değerleri (dBsm)
RCS_PROFILES = {
    "tank": 20.0, "apc": 15.0, "truck": 12.0, "humvee": 8.0,
    "fighter_jet": 1.0, "helicopter": 5.0, "uav": -10.0, "ucav": -5.0,
    "infantry": -15.0, "command_post": 18.0, "radar_station": 25.0,
    "sam_launcher": 16.0, "artillery": 14.0, "ship": 30.0,
}


class SimRadar(BaseCihaz):
    """Simüle radar sensörü — pulse-Doppler radar verisi üretir."""

    def __init__(self, kimlik: CihazKimlik, ayar: CihazAyar):
        kimlik.tip = CihazTip.RADAR
        kimlik.kume = "GORGU"
        super().__init__(kimlik, ayar)
        self._track_counter = 0
        self._targets: list[dict] = []

    @property
    def topic(self) -> str:
        return "sensor.radar"

    def set_targets(self, entities: list[dict]):
        """Savaş alanı varlıklarını radar hedefi olarak ayarla."""
        self._targets = entities

    def on_tick(self) -> list[dict]:
        readings = []
        lat, lon = self.kimlik.konum
        menzil_m = self.ayar.menzil_km * 1000

        for target in self._targets:
            t_lat = target.get("lat", 0)
            t_lon = target.get("lon", 0)
            dist_m = _haversine_m(lat, lon, t_lat, t_lon)

            if dist_m > menzil_m:
                continue
            if target.get("status") == "destroyed":
                continue

            # RCS tabanlı tespit olasılığı
            t_type = target.get("type", "unknown")
            rcs = RCS_PROFILES.get(t_type, 5.0)
            detection_prob = min(1.0, max(0.1, (rcs + 20) / 40 * (1 - dist_m / menzil_m)))
            if random.random() > detection_prob:
                continue

            self._track_counter += 1
            azimuth = math.degrees(math.atan2(t_lon - lon, t_lat - lat)) % 360
            speed = target.get("speed", 0)
            velocity_mps = speed * 1000 / 3600  # km/h → m/s

            readings.append({
                "sensor_id": self.cihaz_id,
                "sensor_type": "radar",
                "timestamp_ns": time.time_ns(),
                "target_id": target.get("id", ""),
                "rcs_dbsm": round(rcs + random.gauss(0, 1.5), 1),
                "range_km": round(dist_m / 1000, 2),
                "azimuth_deg": round(azimuth + random.gauss(0, 0.5), 1) % 360,
                "elevation_deg": round(random.uniform(-2, 15), 1),
                "velocity_mps": round(velocity_mps + random.gauss(0, 2), 1),
                "track_id": f"TRK-{self._track_counter:05d}",
                "target_type_est": t_type,
                "confidence": round(detection_prob, 2),
                "lat": round(t_lat + random.gauss(0, 0.0005), 6),
                "lon": round(t_lon + random.gauss(0, 0.0005), 6),
            })

        return readings


class SimRadarFactory:
    """YAML'dan SimRadar oluşturucu."""
    @staticmethod
    def create(config: dict) -> SimRadar:
        konum = config.get("konum", [39.925, 32.837])
        kimlik = CihazKimlik(
            cihaz_id=config.get("id", f"radar-{random.randint(1000,9999)}"),
            ad=config.get("ad", "Radar Sensörü"),
            surucu=config.get("surucu", "sim"),
            konum=tuple(konum),
        )
        ayar = CihazAyar(
            frekans_hz=config.get("frekans_hz", 10.0),
            menzil_km=config.get("menzil_km", 150.0),
            cozunurluk=config.get("cozunurluk", 0),
            fov_derece=config.get("fov_derece", 360.0),
            oncelik=config.get("oncelik", 2),
            ttl_ms=config.get("ttl_ms", 5000),
            ozel=config.get("ozel", {}),
        )
        return SimRadar(kimlik, ayar)


def _haversine_m(lat1, lon1, lat2, lon2) -> float:
    R = 6_371_000
    dLat = math.radians(lat2 - lat1)
    dLon = math.radians(lon2 - lon1)
    a = math.sin(dLat / 2) ** 2 + math.cos(math.radians(lat1)) * math.cos(math.radians(lat2)) * math.sin(dLon / 2) ** 2
    return R * 2 * math.atan2(math.sqrt(a), math.sqrt(1 - a))
