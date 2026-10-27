"""
Coğrafi ve koordinat yardımcı fonksiyonları.

Savaş alanı varlıklarının konum hesaplamaları, mesafe ölçümleri ve
rastgele konum üretimi.
"""

from __future__ import annotations

import math
import random

EARTH_RADIUS_KM = 6371.0


def haversine(lat1: float, lon1: float, lat2: float, lon2: float) -> float:
    """İki koordinat arasındaki mesafe (km) - Haversine formülü."""
    rlat1, rlon1 = math.radians(lat1), math.radians(lon1)
    rlat2, rlon2 = math.radians(lat2), math.radians(lon2)
    dlat = rlat2 - rlat1
    dlon = rlon2 - rlon1
    a = math.sin(dlat / 2) ** 2 + math.cos(rlat1) * math.cos(rlat2) * math.sin(dlon / 2) ** 2
    return 2 * EARTH_RADIUS_KM * math.asin(math.sqrt(a))


def random_point_in_radius(center_lat: float, center_lon: float, radius_km: float) -> tuple[float, float]:
    """Merkez etrafında rastgele koordinat üret."""
    r = radius_km / EARTH_RADIUS_KM
    u, v = random.random(), random.random()
    w = r * math.sqrt(u)
    t = 2 * math.pi * v
    dlat = w * math.cos(t)
    dlon = w * math.sin(t) / math.cos(math.radians(center_lat))
    return center_lat + math.degrees(dlat), center_lon + math.degrees(dlon)


def bearing(lat1: float, lon1: float, lat2: float, lon2: float) -> float:
    """İki nokta arasındaki yön açısı (derece, 0=kuzey)."""
    rlat1, rlon1 = math.radians(lat1), math.radians(lon1)
    rlat2, rlon2 = math.radians(lat2), math.radians(lon2)
    dlon = rlon2 - rlon1
    x = math.sin(dlon) * math.cos(rlat2)
    y = math.cos(rlat1) * math.sin(rlat2) - math.sin(rlat1) * math.cos(rlat2) * math.cos(dlon)
    return (math.degrees(math.atan2(x, y)) + 360) % 360


def move_point(lat: float, lon: float, bearing_deg: float, distance_km: float) -> tuple[float, float]:
    """Bir noktayı belirli yön ve mesafede ilerlet."""
    d = distance_km / EARTH_RADIUS_KM
    brng = math.radians(bearing_deg)
    rlat = math.radians(lat)
    rlon = math.radians(lon)
    new_lat = math.asin(math.sin(rlat) * math.cos(d) + math.cos(rlat) * math.sin(d) * math.cos(brng))
    new_lon = rlon + math.atan2(
        math.sin(brng) * math.sin(d) * math.cos(rlat),
        math.cos(d) - math.sin(rlat) * math.sin(new_lat),
    )
    return math.degrees(new_lat), math.degrees(new_lon)
