"""
KATMAN Sensör Füzyon — Çoklu sensör verisini birleştirme ve zaman hizalama.

Farklı cihazlar farklı frekanslarda veri üretir:
  - Radar: 10Hz (her 100ms)
  - FLIR: 5Hz (her 200ms)
  - GPS: 1Hz (her 1000ms)
  - Akustik: 2Hz (her 500ms)
  - Birim telemetri: 2Hz (her 500ms)

Sensör füzyon, bu farklı hızlardaki verileri:
  1. Zaman damgasına göre hizalar
  2. Aynı hedef hakkındaki verileri birleştirir
  3. Güvenilirlik skoru hesaplar
  4. Çelişkili verileri çözer

Prod'da: KATMAN modülü bu işlevi yapar.
Simülasyonda: temel füzyon burada yapılır.
"""

from __future__ import annotations

import logging
import time
from collections import defaultdict
from dataclasses import dataclass, field
from typing import Any

logger = logging.getLogger("otuken.fuzyon")


@dataclass
class SensorOkuma:
    """Tek bir sensör okuması."""
    source_id: str          # Cihaz kimliği
    source_type: str        # radar, flir, optik, akustik, birim
    entity_id: str          # Tespit edilen varlık
    timestamp_ns: int       # Nano-saniye zaman damgası
    data: dict              # Ham veri
    confidence: float = 1.0 # Güvenilirlik (0-1)


@dataclass
class FusedTrack:
    """Birleştirilmiş hedef izleme kaydı."""
    entity_id: str
    lat: float = 0.0
    lon: float = 0.0
    heading: float = 0.0
    speed: float = 0.0
    altitude_m: float = 0.0
    health: float = 100.0
    faction: str = "unknown"
    confidence: float = 0.0
    source_count: int = 0
    last_update_ns: int = 0
    sources: list[str] = field(default_factory=list)
    classification: str = "unknown"
    extra: dict = field(default_factory=dict)

    def to_dict(self) -> dict:
        return {
            "entity_id": self.entity_id,
            "lat": round(self.lat, 6),
            "lon": round(self.lon, 6),
            "heading": round(self.heading, 1),
            "speed_kmh": round(self.speed, 1),
            "altitude_m": round(self.altitude_m, 1),
            "health_pct": round(self.health, 1),
            "faction": self.faction,
            "confidence": round(self.confidence, 3),
            "source_count": self.source_count,
            "sources": self.sources[:5],
            "classification": self.classification,
            "age_ms": (time.time_ns() - self.last_update_ns) / 1_000_000 if self.last_update_ns else 0,
        }


# Sensör tipine göre güvenilirlik ağırlıkları
SENSOR_WEIGHTS = {
    "radar": {"position": 0.7, "speed": 0.9, "heading": 0.8},
    "flir": {"position": 0.6, "classification": 0.8, "health": 0.3},
    "optik_kamera": {"position": 0.5, "classification": 0.9, "heading": 0.6},
    "akustik": {"position": 0.3, "speed": 0.4, "classification": 0.5},
    "gps": {"position": 0.95, "speed": 0.9, "heading": 0.95},
    "birim": {"position": 0.99, "speed": 0.95, "heading": 0.99, "health": 0.99},
}

# Zaman penceresi — bu kadar eski veriler füzyona dahil edilmez
MAX_AGE_NS = 5_000_000_000  # 5 saniye


class SensorFuzyon:
    """
    Çoklu sensör füzyon motoru.

    Her tick'te:
      1. Tüm cihaz verilerini topla
      2. Entity bazında grupla
      3. Ağırlıklı ortalama ile birleştir
      4. Güvenilirlik skoru hesapla
      5. FusedTrack listesi üret
    """

    def __init__(self, time_window_ns: int = MAX_AGE_NS):
        self._time_window = time_window_ns
        self._readings: dict[str, list[SensorOkuma]] = defaultdict(list)  # entity_id → readings
        self._tracks: dict[str, FusedTrack] = {}
        self._total_fused = 0
        self._total_readings = 0
        self._conflicts_resolved = 0

    def ingest(self, reading: SensorOkuma):
        """Yeni sensör okuması ekle."""
        self._readings[reading.entity_id].append(reading)
        self._total_readings += 1

    def ingest_device_results(self, device_results: dict, cihaz_yoneticisi) -> int:
        """CihazYoneticisi.tick_all() sonuçlarını toplu olarak al.

        Returns: eklenen okuma sayısı
        """
        count = 0
        for cihaz_id, payloads in device_results.items():
            device = cihaz_yoneticisi.get_device(cihaz_id)
            if not device:
                continue
            source_type = device.kimlik.tip.value
            for p in payloads:
                entity_id = p.get("entity_id") or p.get("target_id") or p.get("id", "")
                if not entity_id:
                    continue
                reading = SensorOkuma(
                    source_id=cihaz_id,
                    source_type=source_type,
                    entity_id=str(entity_id),
                    timestamp_ns=p.get("timestamp_ns", time.time_ns()),
                    data=p,
                    confidence=p.get("confidence", 0.8),
                )
                self.ingest(reading)
                count += 1
        return count

    def fuse(self) -> dict[str, FusedTrack]:
        """Tüm okumaları birleştir ve güncel track listesi döndür."""
        now = time.time_ns()

        for entity_id, readings in self._readings.items():
            # Eski okumaları temizle
            fresh = [r for r in readings if (now - r.timestamp_ns) < self._time_window]
            self._readings[entity_id] = fresh
            if not fresh:
                continue

            track = self._tracks.get(entity_id) or FusedTrack(entity_id=entity_id)

            # Ağırlıklı füzyon
            total_weight = 0.0
            weighted_lat = 0.0
            weighted_lon = 0.0
            weighted_speed = 0.0
            weighted_heading = 0.0
            sources = set()
            best_classification = "unknown"
            best_class_weight = 0.0
            health_sum = 0.0
            health_count = 0

            for r in fresh:
                w = SENSOR_WEIGHTS.get(r.source_type, {})
                pos_w = w.get("position", 0.5) * r.confidence
                sources.add(r.source_id)

                lat = r.data.get("lat", 0)
                lon = r.data.get("lon", 0)
                if lat and lon:
                    weighted_lat += lat * pos_w
                    weighted_lon += lon * pos_w
                    total_weight += pos_w

                spd = r.data.get("speed_kmh") or r.data.get("speed", 0)
                spd_w = w.get("speed", 0.5) * r.confidence
                weighted_speed += spd * spd_w

                hdg = r.data.get("heading", 0)
                hdg_w = w.get("heading", 0.5) * r.confidence
                weighted_heading += hdg * hdg_w

                # Sınıflandırma — en güvenilir sensörün sınıfı
                cls = r.data.get("type") or r.data.get("classification", "")
                cls_w = w.get("classification", 0.3) * r.confidence
                if cls and cls_w > best_class_weight:
                    best_classification = cls
                    best_class_weight = cls_w

                # Sağlık — doğrudan raporlardan
                h = r.data.get("health") or r.data.get("health_pct")
                if h is not None:
                    health_sum += h * w.get("health", 0.5)
                    health_count += 1

                # Faction
                f = r.data.get("faction", "")
                if f:
                    track.faction = f

            if total_weight > 0:
                track.lat = weighted_lat / total_weight
                track.lon = weighted_lon / total_weight
            if len(sources) > 0:
                n = sum(SENSOR_WEIGHTS.get(r.source_type, {}).get("speed", 0.5) * r.confidence for r in fresh)
                if n > 0:
                    track.speed = weighted_speed / n
                n2 = sum(SENSOR_WEIGHTS.get(r.source_type, {}).get("heading", 0.5) * r.confidence for r in fresh)
                if n2 > 0:
                    track.heading = weighted_heading / n2

            if health_count > 0:
                track.health = health_sum / health_count
            track.classification = best_classification
            track.source_count = len(sources)
            track.sources = list(sources)
            track.last_update_ns = max(r.timestamp_ns for r in fresh)
            # Güvenilirlik: kaynak sayısı ve kalitesine bağlı
            track.confidence = min(1.0, total_weight / max(len(fresh), 1))

            self._tracks[entity_id] = track
            self._total_fused += 1

        # Stale track'leri temizle
        stale = [eid for eid, t in self._tracks.items()
                 if (now - t.last_update_ns) > self._time_window * 3]
        for eid in stale:
            del self._tracks[eid]

        return self._tracks

    def get_track(self, entity_id: str) -> FusedTrack | None:
        return self._tracks.get(entity_id)

    def clear(self):
        self._readings.clear()
        self._tracks.clear()

    def to_dict(self) -> dict:
        return {
            "aktif_track": len(self._tracks),
            "toplam_okuma": self._total_readings,
            "toplam_fuzyon": self._total_fused,
            "celiskiler_cozuldu": self._conflicts_resolved,
            "tracks": {eid: t.to_dict() for eid, t in self._tracks.items()},
        }
