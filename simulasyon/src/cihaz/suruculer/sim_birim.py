"""
SimBirimModulu — Dost Birim Onboard Cihaz Sürücüsü.

Her dost birimin (tank, piyade, İHA vb.) üzerindeki donanımları simüle eder:
- GPS izleyici → konum raporlar (telemetry.position)
- Telsiz → C2 mesajları alır/gönderir (command.unit)
- IFF transponderi → dost/düşman tanıma sinyali (iff.beacon)
- Telemetri → sağlık, yakıt, mühimmat (telemetry.status)

Prod'da: Bu sürücü gerçek birim donanımıyla değiştirilir.
  surucu: "sim" → surucu: "aselsan_birim_v2"

Kullanım:
    # cihazlar.yaml'da otomatik oluşturulmaz — BattlefieldGenerator'dan dinamik eklenir
    birim = SimBirimFactory.create({"id": "birim-tank-01", "entity": entity_dict, ...})
"""

from __future__ import annotations

import random
import time
from typing import Any

from src.cihaz.base import BaseCihaz, CihazKimlik, CihazAyar, CihazTip


# Birim tiplerine göre IFF kodları
IFF_KODLARI = {
    "tank": "MBT", "ifv": "IFV", "apc": "APC", "artillery": "ART",
    "air_defense": "ADS", "infantry": "INF", "recon": "RCN",
    "logistics": "LOG", "uav": "UAV", "ucav": "UCV",
    "helicopter": "HEL", "fighter": "FTR", "patrol_boat": "PTB",
    "command_post": "CMD", "radar_station": "RDR", "sensor_node": "SNS",
}

# Birim tipine göre telsiz kapasitesi
TELSIZ_MESAJ_OLASLIK = {
    "command_post": 0.8,  # Komuta merkezleri sık mesaj üretir
    "recon": 0.6,         # Keşif birimleri rapor verir
    "uav": 0.4, "ucav": 0.4,
    "tank": 0.2, "ifv": 0.2, "apc": 0.2,
    "infantry": 0.15,
    "artillery": 0.3,
    "air_defense": 0.3,
}


class SimBirimModulu(BaseCihaz):
    """
    Dost birimin onboard cihaz simülasyonu.

    Tek bir cihaz, birimin tüm alt sistemlerini temsil eder:
    GPS + Telsiz + IFF + Telemetri.

    topic: birden fazla topic'e yayınlar (topic_override ile).
    """

    def __init__(self, kimlik: CihazKimlik, ayar: CihazAyar):
        super().__init__(kimlik, ayar)
        self._entity: dict = {}
        self._iff_mode: int = 1       # 1=Mode 1 (Mil), 2=Mode 2, 3=Mode 3/A
        self._son_c2_tick: int = 0
        self._telsiz_frekans: float = 225.0 + random.uniform(0, 174)  # MHz UHF
        self._msg_counter: int = 0

    @property
    def topic(self) -> str:
        return "telemetry.position"

    def set_entity(self, entity: dict):
        """Birimin güncel durumunu güncelle (BattlefieldGenerator'dan)."""
        self._entity = entity
        # Konum güncelle
        self.kimlik.konum = (entity.get("lat", 0), entity.get("lon", 0))

    def on_tick(self) -> list[dict]:
        """Her tick'te GPS + IFF + telemetri + (opsiyonel) C2 mesajı üret."""
        if not self._entity:
            return []

        now_ns = time.time_ns()
        e = self._entity
        payloads = []

        # ── 1. GPS Konum Raporu ──
        gps_data = {
            "msg_type": "gps_position",
            "entity_id": e.get("id", self.cihaz_id),
            "callsign": e.get("callsign", e.get("name", "")),
            "lat": e.get("lat", 0) + random.gauss(0, 0.00001),  # ~1m GPS hatası
            "lon": e.get("lon", 0) + random.gauss(0, 0.00001),
            "altitude_m": e.get("altitude_m", 0),
            "heading": e.get("heading", 0),
            "speed_kmh": e.get("speed", 0),
            "hdop": round(random.uniform(0.8, 2.5), 1),  # GPS kalitesi
            "satellites": random.randint(6, 14),
            "fix_type": "3D",
            "timestamp_ns": now_ns,
            "source": self.cihaz_id,
        }
        payloads.append(gps_data)

        # ── 2. IFF Beacon ──
        birim_tipi = e.get("type", "unknown")
        iff_data = {
            "msg_type": "iff_beacon",
            "entity_id": e.get("id", self.cihaz_id),
            "mode": self._iff_mode,
            "iff_code": IFF_KODLARI.get(birim_tipi, "UNK"),
            "faction": "friendly",
            "unit_type": birim_tipi,
            "squawk": f"{random.randint(1000,7777):04d}",
            "encrypted": True,
            "timestamp_ns": now_ns,
            "source": self.cihaz_id,
            "topic_override": "iff.beacon",
        }
        payloads.append(iff_data)

        # ── 3. Telemetri (sağlık/yakıt/mühimmat) ──
        telem_data = {
            "msg_type": "unit_telemetry",
            "entity_id": e.get("id", self.cihaz_id),
            "callsign": e.get("callsign", e.get("name", "")),
            "status": e.get("status", "active"),
            "health_pct": e.get("health", 100),
            "fuel_pct": e.get("fuel", 100),
            "ammo_pct": e.get("ammo", 100),
            "crew_status": "nominal" if e.get("health", 100) > 30 else "degraded",
            "comms_status": "operational",
            "sensor_status": "operational" if e.get("health", 100) > 10 else "offline",
            "timestamp_ns": now_ns,
            "source": self.cihaz_id,
            "topic_override": "telemetry.status",
        }
        payloads.append(telem_data)

        # ── 4. Telsiz C2 Mesajı (olasılıkla) ──
        msg_prob = TELSIZ_MESAJ_OLASLIK.get(birim_tipi, 0.1)
        if random.random() < msg_prob:
            self._msg_counter += 1
            msg_types = ["status_report", "contact_report", "request_support",
                         "acknowledge", "position_update", "sitrep"]
            msg_type = random.choice(msg_types)

            c2_data = {
                "msg_type": msg_type,
                "entity_id": e.get("id", self.cihaz_id),
                "callsign": e.get("callsign", e.get("name", "")),
                "sender": self.cihaz_id,
                "receiver": "komuta-merkez-01",
                "priority": random.randint(0, 4),
                "frequency_mhz": self._telsiz_frekans,
                "content": self._generate_message(msg_type, e),
                "sequence": self._msg_counter,
                "encrypted": True,
                "timestamp_ns": now_ns,
                "source": self.cihaz_id,
                "topic_override": "command.unit",
            }
            payloads.append(c2_data)

        return payloads

    def _generate_message(self, msg_type: str, e: dict) -> dict:
        """Mesaj tipine göre içerik üret."""
        lat, lon = e.get("lat", 0), e.get("lon", 0)
        if msg_type == "status_report":
            return {
                "text": f"Durum normal. Sağlık %{e.get('health', 100):.0f}, Yakıt %{e.get('fuel', 100):.0f}",
                "grid": f"{lat:.4f}N {lon:.4f}E",
            }
        elif msg_type == "contact_report":
            return {
                "text": "Temas raporu: bölgede hareketlilik gözlemlendi",
                "bearing": random.randint(0, 359),
                "range_est_km": round(random.uniform(1, 15), 1),
            }
        elif msg_type == "request_support":
            return {
                "text": "Destek talebi",
                "support_type": random.choice(["fire", "medevac", "resupply", "air"]),
                "urgency": random.choice(["routine", "priority", "immediate"]),
            }
        elif msg_type == "sitrep":
            return {
                "text": f"SITREP: Konum {lat:.4f}N {lon:.4f}E, {e.get('status', 'active')}",
                "morale": random.choice(["high", "good", "fair"]),
            }
        return {"text": f"{msg_type} mesajı", "ref": self._msg_counter}

    def on_start(self):
        pass

    def on_stop(self):
        pass


class SimBirimFactory:
    """SimBirimModulu cihaz factory'si."""

    @staticmethod
    def create(config: dict) -> SimBirimModulu:
        entity = config.get("entity", {})
        entity_id = entity.get("id", config.get("id", "birim-unknown"))
        birim_tipi = entity.get("type", "infantry")
        lat = entity.get("lat", 0)
        lon = entity.get("lon", 0)

        kimlik = CihazKimlik(
            cihaz_id=f"birim-{entity_id}",
            tip=CihazTip.BIRIM,
            ad=entity.get("name", entity.get("callsign", entity_id)),
            surucu=config.get("surucu", "sim"),
            kume="ULAK",
            konum=(lat, lon),
            meta={"entity_id": entity_id, "unit_type": birim_tipi},
        )
        ayar = CihazAyar(
            frekans_hz=config.get("frekans_hz", 2.0),
            menzil_km=0,
            oncelik=config.get("oncelik", 2),
            ozel={"birim_tipi": birim_tipi},
        )
        device = SimBirimModulu(kimlik, ayar)
        device.set_entity(entity)
        return device
