"""
Sensör Veri Üreteci

Radar, termal, akustik, optik ve OSINT sensörlerinden geliyormuş gibi
gerçekçi sentetik veri üretir. Her sensör tipi kendine özgü veri deseni üretir.
"""

from __future__ import annotations

import json
import random
import time
import uuid
from dataclasses import dataclass, field
from typing import Any


SENSOR_TYPES = ["radar", "thermal", "acoustic", "optical", "osint"]

# Sensör tipleri ve özellikleri
SENSOR_PROFILES = {
    "radar": {
        "range_km": 150.0,
        "update_hz": 10,
        "fields": ["rcs_dbsm", "range_km", "azimuth_deg", "elevation_deg", "velocity_mps", "track_id"],
    },
    "thermal": {
        "range_km": 20.0,
        "update_hz": 30,
        "fields": ["temperature_c", "delta_t", "blob_area_px", "confidence", "classification"],
    },
    "acoustic": {
        "range_km": 10.0,
        "update_hz": 50,
        "fields": ["frequency_hz", "amplitude_db", "direction_deg", "duration_ms", "signature_type"],
    },
    "optical": {
        "range_km": 30.0,
        "update_hz": 25,
        "fields": ["bbox_x", "bbox_y", "bbox_w", "bbox_h", "class_label", "confidence", "frame_id"],
    },
    "osint": {
        "range_km": 0,
        "update_hz": 0.1,
        "fields": ["source", "content_type", "relevance_score", "language", "keywords"],
    },
}

VEHICLE_TYPES = ["T-72_TANK", "BMP-3_IFV", "BTR-80_APC", "2S19_SPG", "S-400_SAM",
                 "LEOPARD2_TANK", "M113_APC", "TB2_UAV", "KIZILELMA_UCAV", "CIVILIAN_TRUCK"]

ACOUSTIC_SIGNATURES = ["tracked_vehicle", "wheeled_vehicle", "helicopter", "fixed_wing",
                       "small_arms", "artillery", "explosion", "drone_motor"]


@dataclass
class SensorReading:
    """Tek bir sensör okuması."""
    sensor_id: str
    sensor_type: str
    timestamp_ns: int
    priority: int
    data: dict[str, Any]
    source_module: str = "GORGU"
    metadata: dict[str, str] = field(default_factory=dict)

    def to_bag_payload(self) -> dict:
        """BAĞ sistemine gönderilecek formata çevir."""
        return {
            "sensor_id": self.sensor_id,
            "sensor_type": self.sensor_type,
            "source_module": self.source_module,
            "payload": json.dumps(self.data),
            "timestamp_ns": self.timestamp_ns,
            "priority": self.priority,
            "metadata": self.metadata,
        }

    def to_envelope(self, destination: str = "") -> dict:
        """BAĞ Envelope formatına çevir."""
        return {
            "id": str(uuid.uuid4()),
            "topic": f"sensor.{self.sensor_type}",
            "source": self.sensor_id,
            "destination": destination,
            "payload": json.dumps(self.data),
            "timestamp_ns": self.timestamp_ns,
            "priority": self.priority,
            "headers": self.metadata,
        }


class SensorGenerator:
    """Sensör veri üreteci."""

    def __init__(self, center_lat: float = 39.925, center_lon: float = 32.837, seed: int = None):
        self._center_lat = center_lat
        self._center_lon = center_lon
        if seed is not None:
            random.seed(seed)
        self._sensor_counter = 0

    def create_sensor_id(self, sensor_type: str) -> str:
        self._sensor_counter += 1
        return f"{sensor_type.upper()}-{self._sensor_counter:04d}"

    def generate_radar(self, sensor_id: str = None, target_type: str = None) -> SensorReading:
        """Radar sensör verisi üret."""
        if not sensor_id:
            sensor_id = self.create_sensor_id("radar")
        if not target_type:
            target_type = random.choice(VEHICLE_TYPES)

        return SensorReading(
            sensor_id=sensor_id,
            sensor_type="radar",
            timestamp_ns=time.time_ns(),
            priority=random.choice([1, 2, 3]),  # Radar genelde yüksek öncelik
            data={
                "rcs_dbsm": round(random.uniform(-10, 30), 2),
                "range_km": round(random.uniform(0.5, 150), 2),
                "azimuth_deg": round(random.uniform(0, 360), 2),
                "elevation_deg": round(random.uniform(-5, 45), 2),
                "velocity_mps": round(random.uniform(0, 80), 1),
                "track_id": f"TRK-{random.randint(1000, 9999)}",
                "target_type_est": target_type,
            },
            metadata={"lat": str(self._center_lat), "lon": str(self._center_lon)},
        )

    def generate_thermal(self, sensor_id: str = None) -> SensorReading:
        """Termal/kızılötesi sensör verisi üret."""
        if not sensor_id:
            sensor_id = self.create_sensor_id("thermal")

        is_vehicle = random.random() > 0.3
        temp = random.uniform(60, 120) if is_vehicle else random.uniform(20, 40)

        return SensorReading(
            sensor_id=sensor_id,
            sensor_type="thermal",
            timestamp_ns=time.time_ns(),
            priority=4 if is_vehicle else 7,
            data={
                "temperature_c": round(temp, 1),
                "delta_t": round(temp - 25.0, 1),
                "blob_area_px": random.randint(10, 5000),
                "confidence": round(random.uniform(0.6, 0.99), 3),
                "classification": random.choice(["vehicle_hot", "person", "animal", "decoy", "unknown"]),
            },
        )

    def generate_acoustic(self, sensor_id: str = None) -> SensorReading:
        """Akustik sensör verisi üret."""
        if not sensor_id:
            sensor_id = self.create_sensor_id("acoustic")

        sig = random.choice(ACOUSTIC_SIGNATURES)
        is_threat = sig in ("artillery", "explosion", "small_arms")

        return SensorReading(
            sensor_id=sensor_id,
            sensor_type="acoustic",
            timestamp_ns=time.time_ns(),
            priority=1 if is_threat else 6,
            data={
                "frequency_hz": round(random.uniform(20, 20000), 1),
                "amplitude_db": round(random.uniform(30, 140), 1),
                "direction_deg": round(random.uniform(0, 360), 1),
                "duration_ms": random.randint(50, 5000),
                "signature_type": sig,
            },
        )

    def generate_optical(self, sensor_id: str = None) -> SensorReading:
        """Optik/kamera sensör verisi (YOLO benzeri bbox)."""
        if not sensor_id:
            sensor_id = self.create_sensor_id("optical")

        label = random.choice(["tank", "apc", "truck", "person", "drone", "building", "civilian_vehicle"])

        return SensorReading(
            sensor_id=sensor_id,
            sensor_type="optical",
            timestamp_ns=time.time_ns(),
            priority=3 if label in ("tank", "apc", "drone") else 6,
            data={
                "bbox_x": random.randint(0, 1920),
                "bbox_y": random.randint(0, 1080),
                "bbox_w": random.randint(20, 400),
                "bbox_h": random.randint(20, 300),
                "class_label": label,
                "confidence": round(random.uniform(0.5, 0.99), 3),
                "frame_id": random.randint(1, 1_000_000),
            },
        )

    def generate_osint(self, sensor_id: str = None) -> SensorReading:
        """Açık kaynak istihbarat (OSINT) verisi üret."""
        if not sensor_id:
            sensor_id = self.create_sensor_id("osint")

        return SensorReading(
            sensor_id=sensor_id,
            sensor_type="osint",
            timestamp_ns=time.time_ns(),
            priority=8,  # OSINT genelde düşük aciliyet
            data={
                "source": random.choice(["twitter", "telegram", "news_rss", "sigint_intercept", "humint_report"]),
                "content_type": random.choice(["text", "image", "coordinates", "document"]),
                "relevance_score": round(random.uniform(0.1, 1.0), 2),
                "language": random.choice(["tr", "en", "ru", "ar"]),
                "keywords": random.sample(
                    ["troop_movement", "convoy", "airstrike", "ceasefire", "reinforcement",
                     "supply_route", "enemy_position", "civilian_area"],
                    k=random.randint(1, 3),
                ),
            },
        )

    def generate_random(self) -> SensorReading:
        """Rastgele tipte sensör verisi üret."""
        generators = {
            "radar": self.generate_radar,
            "thermal": self.generate_thermal,
            "acoustic": self.generate_acoustic,
            "optical": self.generate_optical,
            "osint": self.generate_osint,
        }
        sensor_type = random.choice(SENSOR_TYPES)
        return generators[sensor_type]()

    def generate_burst(self, count: int = 10, sensor_type: str = None) -> list[SensorReading]:
        """Toplu sensör verisi patlaması üret."""
        readings = []
        for _ in range(count):
            if sensor_type:
                generators = {
                    "radar": self.generate_radar,
                    "thermal": self.generate_thermal,
                    "acoustic": self.generate_acoustic,
                    "optical": self.generate_optical,
                    "osint": self.generate_osint,
                }
                readings.append(generators[sensor_type]())
            else:
                readings.append(self.generate_random())
        return readings
