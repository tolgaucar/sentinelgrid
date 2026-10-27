"""SimFLIR — Sanal Termal Kamera (FLIR) Sürücüsü.

Prod'da: FLIR A700, ASELSAN CATS vb. gerçek termal kamera sürücüsüyle değiştirilir.
"""

from __future__ import annotations

import math
import random
import time

from src.cihaz.base import BaseCihaz, CihazAyar, CihazKimlik, CihazTip

# Araç termal profilleri (idle_c, active_c, peak_c)
THERMAL_PROFILES = {
    "tank": (45, 120, 350), "apc": (40, 90, 200), "truck": (35, 70, 150),
    "humvee": (33, 60, 120), "fighter_jet": (80, 400, 900),
    "helicopter": (60, 200, 500), "uav": (28, 40, 60), "ucav": (30, 50, 80),
    "infantry": (36, 37, 38), "command_post": (30, 45, 60),
    "radar_station": (35, 55, 80), "sam_launcher": (40, 100, 250),
    "artillery": (38, 80, 300), "ship": (50, 100, 200),
}


class SimFLIR(BaseCihaz):
    """Simüle FLIR termal kamera — kızılötesi görüntü verisi üretir."""

    def __init__(self, kimlik: CihazKimlik, ayar: CihazAyar):
        kimlik.tip = CihazTip.FLIR
        kimlik.kume = "GORGU"
        super().__init__(kimlik, ayar)
        self._targets: list[dict] = []
        self._ambient_c = ayar.ozel.get("ambient_c", 22.0)

    @property
    def topic(self) -> str:
        return "sensor.thermal"

    def set_targets(self, entities: list[dict]):
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

            t_type = target.get("type", "unknown")
            profile = THERMAL_PROFILES.get(t_type, (30, 50, 100))
            idle_c, active_c, peak_c = profile

            status = target.get("status", "active")
            speed = target.get("speed", 0)
            health = target.get("health", 100)

            # Sıcaklık hesapla
            if status == "destroyed":
                temp_c = peak_c * 0.6 + random.gauss(0, 10)  # Patlama sonrası soğuma
            elif status == "damaged":
                temp_c = active_c * 1.3 + random.gauss(0, 5)  # Hasar = daha sıcak
            elif speed > 0:
                speed_factor = min(speed / 100.0, 1.0)
                temp_c = idle_c + (active_c - idle_c) * speed_factor + random.gauss(0, 3)
            else:
                temp_c = idle_c + random.gauss(0, 2)

            delta_t = temp_c - self._ambient_c
            if delta_t < 1.0:
                continue  # Ortam sıcaklığından ayırt edilemez

            # Mesafe bazlı güven azalması
            confidence = max(0.1, 1.0 - (dist_m / menzil_m) * 0.7) * min(delta_t / 20, 1.0)

            blob_area = max(1, int(10 * (1 - dist_m / menzil_m) * (delta_t / 30)))

            readings.append({
                "sensor_id": self.cihaz_id,
                "sensor_type": "thermal",
                "timestamp_ns": time.time_ns(),
                "target_id": target.get("id", ""),
                "temperature_c": round(temp_c, 1),
                "delta_t": round(delta_t, 1),
                "ambient_c": self._ambient_c,
                "blob_area_px": blob_area,
                "confidence": round(confidence, 2),
                "classification": _classify_thermal(temp_c),
                "lat": round(t_lat + random.gauss(0, 0.001), 6),
                "lon": round(t_lon + random.gauss(0, 0.001), 6),
                "range_km": round(dist_m / 1000, 2),
            })

        return readings


class SimFLIRFactory:
    @staticmethod
    def create(config: dict) -> SimFLIR:
        konum = config.get("konum", [39.925, 32.837])
        kimlik = CihazKimlik(
            cihaz_id=config.get("id", f"flir-{random.randint(1000,9999)}"),
            ad=config.get("ad", "Termal Kamera"),
            surucu=config.get("surucu", "sim"),
            konum=tuple(konum),
        )
        ayar = CihazAyar(
            frekans_hz=config.get("frekans_hz", 30.0),
            menzil_km=config.get("menzil_km", 20.0),
            cozunurluk=config.get("cozunurluk", 640),
            fov_derece=config.get("fov_derece", 24.0),
            oncelik=config.get("oncelik", 4),
            ttl_ms=config.get("ttl_ms", 5000),
            ozel=config.get("ozel", {"ambient_c": config.get("ambient_c", 22.0)}),
        )
        return SimFLIR(kimlik, ayar)


def _classify_thermal(temp_c: float) -> str:
    if temp_c > 300:
        return "jet_engine"
    if temp_c > 150:
        return "heavy_vehicle"
    if temp_c > 80:
        return "medium_vehicle"
    if temp_c > 50:
        return "light_vehicle"
    if temp_c > 35:
        return "personnel"
    return "background"


def _haversine_m(lat1, lon1, lat2, lon2) -> float:
    R = 6_371_000
    dLat = math.radians(lat2 - lat1)
    dLon = math.radians(lon2 - lon1)
    a = math.sin(dLat / 2) ** 2 + math.cos(math.radians(lat1)) * math.cos(math.radians(lat2)) * math.sin(dLon / 2) ** 2
    return R * 2 * math.atan2(math.sqrt(a), math.sqrt(1 - a))
