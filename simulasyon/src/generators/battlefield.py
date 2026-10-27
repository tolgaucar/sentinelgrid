"""
Savaş Alanı Varlık Üreteci

Dost birlikler, düşman unsurlar, dronlar ve sabit tesisler gibi
savaş alanı varlıklarını oluşturur ve hareket ettirir.
"""

from __future__ import annotations

import random
import time
import uuid
from dataclasses import dataclass, field
from enum import Enum
from typing import Optional

from src.utils.geo import random_point_in_radius, move_point, haversine


class Faction(str, Enum):
    FRIENDLY = "friendly"
    ENEMY = "enemy"
    NEUTRAL = "neutral"
    CIVILIAN = "civilian"


class UnitType(str, Enum):
    # Kara
    TANK = "tank"
    IFV = "ifv"          # Infantry Fighting Vehicle
    APC = "apc"          # Armored Personnel Carrier
    ARTILLERY = "artillery"
    AIR_DEFENSE = "air_defense"
    INFANTRY = "infantry"
    RECON = "recon"
    LOGISTICS = "logistics"
    # Hava
    UAV = "uav"
    UCAV = "ucav"
    HELICOPTER = "helicopter"
    FIGHTER = "fighter"
    # Deniz
    PATROL_BOAT = "patrol_boat"
    # Sabit
    COMMAND_POST = "command_post"
    RADAR_STATION = "radar_station"
    SENSOR_NODE = "sensor_node"


class UnitStatus(str, Enum):
    ACTIVE = "active"
    MOVING = "moving"
    ENGAGING = "engaging"
    DAMAGED = "damaged"
    DESTROYED = "destroyed"
    RETREATING = "retreating"
    IDLE = "idle"


@dataclass
class BattlefieldEntity:
    """Savaş alanı varlığı."""
    id: str
    name: str
    faction: Faction
    unit_type: UnitType
    lat: float
    lon: float
    altitude_m: float = 0.0
    heading_deg: float = 0.0
    speed_kmh: float = 0.0
    status: UnitStatus = UnitStatus.ACTIVE
    health_percent: float = 100.0
    fuel_percent: float = 100.0
    ammo_percent: float = 100.0
    battery_percent: float = 100.0
    sensors: list[str] = field(default_factory=list)
    metadata: dict[str, str] = field(default_factory=dict)
    created_at: float = field(default_factory=time.time)

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "name": self.name,
            "faction": self.faction.value,
            "unit_type": self.unit_type.value,
            "position": {"lat": self.lat, "lon": self.lon, "alt": self.altitude_m},
            "heading_deg": self.heading_deg,
            "speed_kmh": self.speed_kmh,
            "status": self.status.value,
            "health": self.health_percent,
            "fuel": self.fuel_percent,
            "ammo": self.ammo_percent,
            "battery": self.battery_percent,
        }

    def move(self, duration_sec: float) -> None:
        """Varlığı mevcut yön ve hızda ilerlet."""
        if self.speed_kmh <= 0:
            return
        dist_km = (self.speed_kmh / 3600) * duration_sec
        self.lat, self.lon = move_point(self.lat, self.lon, self.heading_deg, dist_km)
        # Yakıt tüketimi
        self.fuel_percent = max(0, self.fuel_percent - dist_km * 0.1)

    def take_damage(self, amount: float) -> None:
        """Hasar al."""
        self.health_percent = max(0, self.health_percent - amount)
        if self.health_percent <= 0:
            self.status = UnitStatus.DESTROYED
        elif self.health_percent < 30:
            self.status = UnitStatus.DAMAGED


# Birim isimleri
FRIENDLY_NAMES = {
    UnitType.TANK: ["Bozkurt", "Altay", "Yavuz", "Fatih", "Kılıç"],
    UnitType.UAV: ["Bayraktar", "Aksungur", "Anka", "Gözcü", "Kartal"],
    UnitType.UCAV: ["Kızılelma", "Hürjet", "Akıncı", "Teber", "Şimşek"],
    UnitType.INFANTRY: ["Nöbetçi", "Serhat", "Yıldırım", "Fırtına", "Pençe"],
    UnitType.ARTILLERY: ["Bora", "Gökbey", "Topçu", "Fırtına", "Kasırga"],
    UnitType.AIR_DEFENSE: ["Siper", "Hisar", "Korkut", "Göktürk", "Kalkan"],
}

ENEMY_NAMES = {
    UnitType.TANK: ["Bear", "Storm", "Hammer", "Wolf", "Viper"],
    UnitType.UAV: ["Shadow", "Eagle", "Hawk", "Ghost", "Raven"],
    UnitType.INFANTRY: ["Red-1", "Red-2", "Red-3", "Red-4", "Red-5"],
}

MAX_SPEEDS = {
    UnitType.TANK: 65, UnitType.IFV: 75, UnitType.APC: 80,
    UnitType.ARTILLERY: 40, UnitType.AIR_DEFENSE: 50, UnitType.INFANTRY: 5,
    UnitType.RECON: 90, UnitType.LOGISTICS: 60,
    UnitType.UAV: 150, UnitType.UCAV: 500, UnitType.HELICOPTER: 280, UnitType.FIGHTER: 2000,
    UnitType.PATROL_BOAT: 60,
    UnitType.COMMAND_POST: 0, UnitType.RADAR_STATION: 0, UnitType.SENSOR_NODE: 0,
}


class BattlefieldGenerator:
    """Savaş alanı varlık üreteci."""

    def __init__(self, center_lat: float = 39.925, center_lon: float = 32.837,
                 radius_km: float = 50.0, seed: int = None):
        self._center_lat = center_lat
        self._center_lon = center_lon
        self._radius_km = radius_km
        self._entities: dict[str, BattlefieldEntity] = {}
        self._counter = 0
        if seed is not None:
            random.seed(seed)

    @property
    def entities(self) -> dict[str, BattlefieldEntity]:
        return dict(self._entities)

    def _next_id(self) -> str:
        self._counter += 1
        return f"ENT-{self._counter:04d}"

    def create_entity(self, faction: Faction, unit_type: UnitType,
                      name: str = None, lat: float = None, lon: float = None) -> BattlefieldEntity:
        """Tek varlık oluştur."""
        if lat is None or lon is None:
            lat, lon = random_point_in_radius(self._center_lat, self._center_lon, self._radius_km)

        if name is None:
            names = FRIENDLY_NAMES if faction == Faction.FRIENDLY else ENEMY_NAMES
            name_list = names.get(unit_type, ["Birim"])
            name = f"{random.choice(name_list)}-{random.randint(1, 99)}"

        altitude = 0.0
        if unit_type in (UnitType.UAV, UnitType.UCAV):
            altitude = random.uniform(500, 5000)
        elif unit_type == UnitType.HELICOPTER:
            altitude = random.uniform(100, 3000)
        elif unit_type == UnitType.FIGHTER:
            altitude = random.uniform(5000, 15000)

        entity = BattlefieldEntity(
            id=self._next_id(),
            name=name,
            faction=faction,
            unit_type=unit_type,
            lat=lat,
            lon=lon,
            altitude_m=altitude,
            heading_deg=random.uniform(0, 360),
            speed_kmh=random.uniform(0, MAX_SPEEDS.get(unit_type, 50) * 0.6),
            status=UnitStatus.ACTIVE,
        )
        self._entities[entity.id] = entity
        return entity

    def generate_battlefield(self, friendly: int = 20, enemy: int = 15,
                             drones: int = 10, sensors: int = 30) -> list[BattlefieldEntity]:
        """Tam savaş alanı oluştur."""
        entities = []

        # Dost kuvvetler
        friendly_types = [UnitType.TANK, UnitType.IFV, UnitType.APC, UnitType.INFANTRY,
                          UnitType.ARTILLERY, UnitType.AIR_DEFENSE, UnitType.RECON]
        for _ in range(friendly):
            e = self.create_entity(Faction.FRIENDLY, random.choice(friendly_types))
            entities.append(e)

        # Düşman kuvvetler
        enemy_types = [UnitType.TANK, UnitType.IFV, UnitType.APC, UnitType.INFANTRY,
                       UnitType.ARTILLERY, UnitType.AIR_DEFENSE]
        for _ in range(enemy):
            e = self.create_entity(Faction.ENEMY, random.choice(enemy_types))
            entities.append(e)

        # Dronlar (dost)
        drone_types = [UnitType.UAV, UnitType.UCAV]
        for _ in range(drones):
            e = self.create_entity(Faction.FRIENDLY, random.choice(drone_types))
            entities.append(e)

        # Sensör düğümleri
        for _ in range(sensors):
            e = self.create_entity(Faction.FRIENDLY, UnitType.SENSOR_NODE)
            e.speed_kmh = 0
            entities.append(e)

        # Komuta merkezleri
        cmd = self.create_entity(Faction.FRIENDLY, UnitType.COMMAND_POST, name="ANA-KOMUTA")
        cmd.speed_kmh = 0
        entities.append(cmd)
        radar_hq = self.create_entity(Faction.FRIENDLY, UnitType.RADAR_STATION, name="RADAR-MERKEZ")
        radar_hq.speed_kmh = 0
        entities.append(radar_hq)

        return entities

    def tick(self, duration_sec: float = 0.5) -> list[dict]:
        """Tüm varlıkları bir adım ilerlet. Değişiklikleri döndür."""
        events = []
        for entity in self._entities.values():
            if entity.status in (UnitStatus.DESTROYED,):
                continue

            entity.move(duration_sec)

            # Rastgele olaylar
            roll = random.random()
            if roll < 0.01 and entity.faction == Faction.ENEMY:
                entity.take_damage(random.uniform(5, 30))
                events.append({"type": "damage", "entity": entity.id, "health": entity.health_percent})
            elif roll < 0.02:
                entity.heading_deg = (entity.heading_deg + random.uniform(-30, 30)) % 360
                events.append({"type": "course_change", "entity": entity.id, "heading": entity.heading_deg})

            # Yakıt uyarısı
            if entity.fuel_percent < 20 and entity.status == UnitStatus.ACTIVE:
                events.append({"type": "low_fuel", "entity": entity.id, "fuel": entity.fuel_percent})

        return events
