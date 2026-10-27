"""SimAkustik — Sanal Akustik Sensör Sürücüsü.

Prod'da: ASELSAN ASKARAD, RAFAEL vb. gerçek akustik sensörle değiştirilir.
"""

from __future__ import annotations

import math
import random
import time

from src.cihaz.base import BaseCihaz, CihazAyar, CihazKimlik, CihazTip

ACOUSTIC_SIGNATURES = {
    "tank": {"freq_range": (50, 200), "amp_range": (80, 110), "sig": "tracked_vehicle"},
    "apc": {"freq_range": (60, 250), "amp_range": (70, 100), "sig": "tracked_vehicle"},
    "truck": {"freq_range": (80, 300), "amp_range": (60, 90), "sig": "wheeled_vehicle"},
    "humvee": {"freq_range": (100, 400), "amp_range": (55, 85), "sig": "wheeled_vehicle"},
    "helicopter": {"freq_range": (20, 500), "amp_range": (75, 105), "sig": "helicopter"},
    "fighter_jet": {"freq_range": (100, 2000), "amp_range": (90, 130), "sig": "jet_engine"},
    "uav": {"freq_range": (200, 800), "amp_range": (30, 60), "sig": "small_drone"},
    "ucav": {"freq_range": (150, 600), "amp_range": (40, 70), "sig": "medium_drone"},
    "artillery": {"freq_range": (10, 100), "amp_range": (100, 140), "sig": "explosion"},
    "infantry": {"freq_range": (200, 1000), "amp_range": (20, 45), "sig": "footsteps"},
}


class SimAkustik(BaseCihaz):
    """Simüle akustik sensör — ses dalgası analizi verisi üretir."""

    def __init__(self, kimlik: CihazKimlik, ayar: CihazAyar):
        kimlik.tip = CihazTip.AKUSTIK
        kimlik.kume = "GORGU"
        super().__init__(kimlik, ayar)
        self._targets: list[dict] = []

    @property
    def topic(self) -> str:
        return "sensor.acoustic"

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
            if target.get("status") == "destroyed":
                continue

            t_type = target.get("type", "unknown")
            sig = ACOUSTIC_SIGNATURES.get(t_type)
            if not sig:
                continue

            speed = target.get("speed", 0)
            if speed == 0 and t_type not in ("artillery", "helicopter", "fighter_jet"):
                continue  # Duran kara aracı ses çıkarmaz

            freq_lo, freq_hi = sig["freq_range"]
            amp_lo, amp_hi = sig["amp_range"]
            frequency = random.uniform(freq_lo, freq_hi)
            amplitude = random.uniform(amp_lo, amp_hi)

            # Mesafe ile ses azalması (inverse square law)
            amplitude -= 20 * math.log10(max(dist_m, 1) / 100)
            if amplitude < 15:
                continue  # Duyma eşiğinin altında

            direction = math.degrees(math.atan2(t_lon - lon, t_lat - lat)) % 360

            readings.append({
                "sensor_id": self.cihaz_id,
                "sensor_type": "acoustic",
                "timestamp_ns": time.time_ns(),
                "target_id": target.get("id", ""),
                "frequency_hz": round(frequency, 1),
                "amplitude_db": round(amplitude, 1),
                "direction_deg": round(direction + random.gauss(0, 3), 1) % 360,
                "duration_ms": random.randint(100, 2000),
                "signature_type": sig["sig"],
                "confidence": round(max(0.1, 1.0 - dist_m / menzil_m), 2),
                "range_est_km": round(dist_m / 1000, 2),
            })

        return readings


class SimAkustikFactory:
    @staticmethod
    def create(config: dict) -> SimAkustik:
        konum = config.get("konum", [39.925, 32.837])
        kimlik = CihazKimlik(
            cihaz_id=config.get("id", f"akustik-{random.randint(1000,9999)}"),
            ad=config.get("ad", "Akustik Sensör"),
            surucu=config.get("surucu", "sim"),
            konum=tuple(konum),
        )
        ayar = CihazAyar(
            frekans_hz=config.get("frekans_hz", 50.0),
            menzil_km=config.get("menzil_km", 10.0),
            oncelik=config.get("oncelik", 5),
            ttl_ms=config.get("ttl_ms", 3000),
        )
        return SimAkustik(kimlik, ayar)


def _haversine_m(lat1, lon1, lat2, lon2) -> float:
    R = 6_371_000
    dLat = math.radians(lat2 - lat1)
    dLon = math.radians(lon2 - lon1)
    a = math.sin(dLat / 2) ** 2 + math.cos(math.radians(lat1)) * math.cos(math.radians(lat2)) * math.sin(dLon / 2) ** 2
    return R * 2 * math.atan2(math.sqrt(a), math.sqrt(1 - a))
