"""Simülasyon platformu testleri."""

from __future__ import annotations

import json
import pytest

# Generators
from src.generators.sensor import SensorGenerator, SensorReading, SENSOR_TYPES
from src.generators.battlefield import (
    BattlefieldGenerator, BattlefieldEntity, Faction, UnitType, UnitStatus
)
from src.generators.comms import CommsGenerator, C2Message, MessageType

# Utils
from src.utils.geo import haversine, random_point_in_radius, move_point, bearing


# ── Sensor Generator Tests ──

class TestSensorGenerator:

    def setup_method(self):
        self.gen = SensorGenerator(seed=42)

    def test_generate_radar(self):
        r = self.gen.generate_radar()
        assert isinstance(r, SensorReading)
        assert r.sensor_type == "radar"
        assert "rcs_dbsm" in r.data
        assert "range_km" in r.data
        assert "track_id" in r.data

    def test_generate_thermal(self):
        r = self.gen.generate_thermal()
        assert r.sensor_type == "thermal"
        assert "temperature_c" in r.data
        assert "confidence" in r.data

    def test_generate_acoustic(self):
        r = self.gen.generate_acoustic()
        assert r.sensor_type == "acoustic"
        assert "frequency_hz" in r.data
        assert "signature_type" in r.data

    def test_generate_optical(self):
        r = self.gen.generate_optical()
        assert r.sensor_type == "optical"
        assert "bbox_x" in r.data
        assert "class_label" in r.data

    def test_generate_osint(self):
        r = self.gen.generate_osint()
        assert r.sensor_type == "osint"
        assert "source" in r.data
        assert "keywords" in r.data

    def test_generate_random(self):
        r = self.gen.generate_random()
        assert r.sensor_type in SENSOR_TYPES

    def test_generate_burst(self):
        readings = self.gen.generate_burst(count=20)
        assert len(readings) == 20
        types = {r.sensor_type for r in readings}
        assert len(types) > 1  # En az 2 farklı tip

    def test_to_bag_payload(self):
        r = self.gen.generate_radar()
        payload = r.to_bag_payload()
        assert "sensor_id" in payload
        assert "sensor_type" in payload
        assert "payload" in payload
        json.loads(payload["payload"])  # Valid JSON

    def test_to_envelope(self):
        r = self.gen.generate_radar()
        env = r.to_envelope()
        assert env["topic"] == "sensor.radar"
        assert "id" in env
        assert env["priority"] in range(1, 11)

    def test_priority_range(self):
        for _ in range(50):
            r = self.gen.generate_random()
            assert 1 <= r.priority <= 10


# ── Battlefield Generator Tests ──

class TestBattlefieldGenerator:

    def setup_method(self):
        self.gen = BattlefieldGenerator(center_lat=39.9, center_lon=32.8, radius_km=10, seed=42)

    def test_create_entity(self):
        e = self.gen.create_entity(Faction.FRIENDLY, UnitType.TANK)
        assert isinstance(e, BattlefieldEntity)
        assert e.faction == Faction.FRIENDLY
        assert e.unit_type == UnitType.TANK
        assert e.health_percent == 100.0

    def test_generate_battlefield(self):
        entities = self.gen.generate_battlefield(friendly=5, enemy=3, drones=2, sensors=4)
        total = len(self.gen.entities)
        assert total >= 14  # 5+3+2+4 + 2 (komuta+radar)

    def test_entity_movement(self):
        e = self.gen.create_entity(Faction.FRIENDLY, UnitType.TANK)
        old_lat, old_lon = e.lat, e.lon
        e.speed_kmh = 60
        e.heading_deg = 90
        e.move(10)
        # Hareket etmiş olmalı
        assert (e.lat != old_lat) or (e.lon != old_lon)

    def test_take_damage(self):
        e = self.gen.create_entity(Faction.ENEMY, UnitType.TANK)
        e.take_damage(80)
        assert e.health_percent == 20
        assert e.status == UnitStatus.DAMAGED
        e.take_damage(30)
        assert e.health_percent == 0
        assert e.status == UnitStatus.DESTROYED

    def test_tick(self):
        self.gen.generate_battlefield(friendly=3, enemy=3, drones=0, sensors=0)
        events = self.gen.tick(duration_sec=1.0)
        assert isinstance(events, list)

    def test_entity_to_dict(self):
        e = self.gen.create_entity(Faction.FRIENDLY, UnitType.UAV)
        d = e.to_dict()
        assert d["faction"] == "friendly"
        assert d["unit_type"] == "uav"
        assert "position" in d
        assert "lat" in d["position"]

    def test_altitude_for_air_units(self):
        e = self.gen.create_entity(Faction.FRIENDLY, UnitType.UAV)
        assert e.altitude_m > 0
        ground = self.gen.create_entity(Faction.FRIENDLY, UnitType.TANK)
        assert ground.altitude_m == 0


# ── Comms Generator Tests ──

class TestCommsGenerator:

    def setup_method(self):
        self.gen = CommsGenerator(seed=42)

    def test_generate_order(self):
        msg = self.gen.generate_order()
        assert msg.msg_type == MessageType.ORDER
        assert msg.requires_ack is True
        assert "action" in msg.content

    def test_generate_report(self):
        msg = self.gen.generate_report()
        assert msg.msg_type == MessageType.REPORT
        assert "personnel_status" in msg.content

    def test_generate_intel(self):
        msg = self.gen.generate_intel()
        assert msg.msg_type == MessageType.INTEL
        assert "threat_level" in msg.content

    def test_generate_alert(self):
        msg = self.gen.generate_alert()
        assert msg.msg_type == MessageType.ALERT
        assert msg.priority == 1  # En yüksek
        assert msg.ttl_sec == 30

    def test_generate_burst(self):
        msgs = self.gen.generate_burst(count=20)
        assert len(msgs) == 20
        types = {m.msg_type for m in msgs}
        assert len(types) >= 2

    def test_to_dict(self):
        msg = self.gen.generate_order()
        d = msg.to_dict()
        assert "id" in d
        assert d["type"] == MessageType.ORDER
        assert "sender" in d

    def test_to_bag_topic(self):
        alert = self.gen.generate_alert()
        assert alert.to_bag_topic() == "alert.critical"
        order = self.gen.generate_order()
        assert order.to_bag_topic() == "command.buyruk"


# ── Geo Utils Tests ──

class TestGeoUtils:

    def test_haversine(self):
        # Ankara → İstanbul ≈ 350 km
        d = haversine(39.925, 32.837, 41.015, 28.979)
        assert 340 < d < 360

    def test_random_point_in_radius(self):
        lat, lon = random_point_in_radius(39.925, 32.837, 10)
        d = haversine(39.925, 32.837, lat, lon)
        assert d <= 11  # Tolerans

    def test_move_point(self):
        lat, lon = move_point(39.925, 32.837, 0, 10)  # Kuzeye 10km
        assert lat > 39.925

    def test_bearing(self):
        b = bearing(39.925, 32.837, 40.0, 32.837)  # Kuzey
        assert -5 < b < 5 or 355 < b < 365
