"""SimKomuta — Sanal Komuta Terminali Sürücüsü.

Prod'da: Gerçek C2 terminal yazılımı (ASELSAN PUSAT, Harris vb.) ile değiştirilir.
Komuta emirlerini, raporları ve alarmları BAĞ'a yayınlar.
"""

from __future__ import annotations

import random
import time
import uuid

from src.cihaz.base import BaseCihaz, CihazAyar, CihazKimlik, CihazTip

COMMANDERS = [
    "TAK-KOMUTAN", "TUG-KOMUTAN", "BLK-KOMUTAN",
    "TIM-KOMUTAN", "HAVA-KOMUTAN", "TOP-KOMUTAN",
]

ALERT_TYPES = [
    "HAVA_SALDIRISI", "KİMYASAL_TEHDİT", "SİBER_SALDIRI",
    "TEMAS", "TOP_ATIŞI", "SINIRİHLALİ", "NBC_UYARI",
]

ORDER_ACTIONS = [
    "ileri_hareket", "mevzi_al", "geri_çekil", "keşif_yap",
    "ateş_aç", "hedef_kilitle", "hava_desteği_iste", "takviye_iste",
]


class SimKomuta(BaseCihaz):
    """Simüle komuta terminali — C2 mesajları üretir."""

    def __init__(self, kimlik: CihazKimlik, ayar: CihazAyar):
        kimlik.tip = CihazTip.KOMUTA_TERMINALI
        kimlik.kume = "BUYRUK"
        super().__init__(kimlik, ayar)
        self._msg_counter = 0
        self._entities: list[dict] = []

    @property
    def topic(self) -> str:
        return "command.buyruk"

    def set_entities(self, entities: list[dict]):
        self._entities = entities

    def on_tick(self) -> list[dict]:
        messages = []
        count = random.randint(1, 3)

        for _ in range(count):
            self._msg_counter += 1
            msg_type = random.choices(
                ["order", "report", "intel", "alert", "situation", "logistics"],
                weights=[25, 20, 15, 10, 15, 15],
            )[0]

            if msg_type == "order":
                messages.append(self._generate_order())
            elif msg_type == "report":
                messages.append(self._generate_report())
            elif msg_type == "intel":
                messages.append(self._generate_intel())
            elif msg_type == "alert":
                messages.append(self._generate_alert())
            elif msg_type == "situation":
                messages.append(self._generate_situation())
            else:
                messages.append(self._generate_logistics())

        return messages

    def _generate_order(self) -> dict:
        return {
            "msg_type": "order",
            "msg_id": uuid.uuid4().hex[:12],
            "sender": random.choice(COMMANDERS),
            "receiver": random.choice(COMMANDERS),
            "action": random.choice(ORDER_ACTIONS),
            "priority": random.randint(1, 3),
            "requires_ack": True,
            "ttl_sec": 60,
            "timestamp": time.time(),
            "topic_override": "command.buyruk",
        }

    def _generate_report(self) -> dict:
        return {
            "msg_type": "report",
            "msg_id": uuid.uuid4().hex[:12],
            "sender": random.choice(COMMANDERS),
            "report_type": random.choice(["status", "contact", "damage", "supply"]),
            "personnel_status": random.choice(["green", "yellow", "red"]),
            "vehicle_status": random.choice(["operational", "damaged", "destroyed"]),
            "ammo_level": random.randint(10, 100),
            "morale": random.choice(["high", "medium", "low"]),
            "priority": 4,
            "timestamp": time.time(),
            "topic_override": "command.kovan",
        }

    def _generate_intel(self) -> dict:
        return {
            "msg_type": "intel",
            "msg_id": uuid.uuid4().hex[:12],
            "sender": "ISTIHBARAT",
            "intel_type": random.choice(["HUMINT", "SIGINT", "IMINT", "OSINT"]),
            "threat_level": random.randint(1, 5),
            "enemy_strength": random.choice(["platoon", "company", "battalion"]),
            "reliability": random.choice(["A", "B", "C", "D"]),
            "priority": 3,
            "timestamp": time.time(),
            "topic_override": "sensor.osint",
        }

    def _generate_alert(self) -> dict:
        alert_type = random.choice(ALERT_TYPES)
        return {
            "msg_type": "alert",
            "msg_id": uuid.uuid4().hex[:12],
            "alert_type": alert_type,
            "severity": random.choice(["critical", "warning"]),
            "source": self.cihaz_id,
            "priority": 1,
            "timestamp": time.time(),
            "topic_override": "alert.critical",
        }

    def _generate_situation(self) -> dict:
        active = sum(1 for e in self._entities if e.get("status") in ("active", "moving"))
        destroyed = sum(1 for e in self._entities if e.get("status") == "destroyed")
        return {
            "msg_type": "situation",
            "msg_id": uuid.uuid4().hex[:12],
            "sender": "MERKEZ",
            "total_entities": len(self._entities),
            "active": active,
            "destroyed": destroyed,
            "priority": 6,
            "timestamp": time.time(),
            "topic_override": "command.kovan",
        }

    def _generate_logistics(self) -> dict:
        return {
            "msg_type": "logistics",
            "msg_id": uuid.uuid4().hex[:12],
            "request_type": random.choice(["ammo_resupply", "fuel_resupply", "medevac", "repair"]),
            "urgency": random.choice(["routine", "priority", "immediate"]),
            "priority": 5,
            "timestamp": time.time(),
            "topic_override": "command.kovan",
        }


class SimKomutaFactory:
    @staticmethod
    def create(config: dict) -> SimKomuta:
        konum = config.get("konum", [39.925, 32.837])
        kimlik = CihazKimlik(
            cihaz_id=config.get("id", f"komuta-{random.randint(1000,9999)}"),
            ad=config.get("ad", "Komuta Terminali"),
            surucu=config.get("surucu", "sim"),
            konum=tuple(konum),
        )
        ayar = CihazAyar(
            frekans_hz=config.get("frekans_hz", 0.2),  # Her 5 saniyede
            oncelik=config.get("oncelik", 2),
            ttl_ms=config.get("ttl_ms", 60000),
        )
        return SimKomuta(kimlik, ayar)
