"""
Komuta ve Kontrol (C2) Mesaj Üreteci

Askeri komuta zinciri boyunca akan emir, rapor ve haberleşme
mesajları üretir. BUYRUK ve KOVAN kümeleri için gerçekçi trafik.
"""

from __future__ import annotations

import random
import time
import uuid
from dataclasses import dataclass, field
from typing import Any


class MessageType:
    ORDER = "order"              # Emir
    REPORT = "report"            # Rapor
    REQUEST = "request"          # Talep
    ACK = "ack"                  # Onay
    INTEL = "intel"              # İstihbarat
    LOGISTICS = "logistics"      # Lojistik
    ALERT = "alert"              # Alarm
    SITUATION = "situation"      # Durum


PRIORITY_MAP = {
    MessageType.ALERT: 1,
    MessageType.ORDER: 2,
    MessageType.INTEL: 3,
    MessageType.REPORT: 4,
    MessageType.REQUEST: 5,
    MessageType.ACK: 6,
    MessageType.LOGISTICS: 7,
    MessageType.SITUATION: 8,
}


@dataclass
class C2Message:
    """Komuta-Kontrol mesajı."""
    id: str
    msg_type: str
    sender: str
    receiver: str
    content: dict[str, Any]
    priority: int
    timestamp: float = field(default_factory=time.time)
    ttl_sec: int = 60
    requires_ack: bool = False

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "type": self.msg_type,
            "sender": self.sender,
            "receiver": self.receiver,
            "content": self.content,
            "priority": self.priority,
            "timestamp": self.timestamp,
            "ttl_sec": self.ttl_sec,
            "requires_ack": self.requires_ack,
        }

    def to_bag_topic(self) -> str:
        """BAĞ topic adını belirle."""
        if self.msg_type == MessageType.ALERT:
            return "alert.critical"
        elif self.msg_type == MessageType.ORDER:
            return "command.buyruk"
        elif self.msg_type == MessageType.INTEL:
            return "sensor.osint"
        else:
            return "command.kovan"


COMMANDERS = ["TAK-KOMUTAN", "TUG-KOMUTAN", "BLK-KOMUTAN", "TIM-KOMUTAN", "HAVA-KOMUTAN"]
UNITS = ["1-TANK-BLK", "2-MEK-TUG", "3-KEŞIF-TIM", "4-TOP-BT", "5-HAVA-SAV",
         "6-İHA-FİL", "7-LOJ-BLK", "8-MUH-BLK", "KUVVET-KOMUTANLIĞI"]


class CommsGenerator:
    """C2 mesaj üreteci."""

    def __init__(self, seed: int = None):
        if seed is not None:
            random.seed(seed)

    def generate_order(self) -> C2Message:
        """Emir mesajı üret."""
        actions = ["İLERLE", "MEVZİ-AL", "ATEŞ-AÇ", "GERİ-ÇEKİL", "KEŞİF-YAP",
                   "MEVZİ-TAHKİM", "PUSU-KUR", "KONVOY-KORUMA", "ALAN-KONTROL"]
        return C2Message(
            id=str(uuid.uuid4()),
            msg_type=MessageType.ORDER,
            sender=random.choice(COMMANDERS),
            receiver=random.choice(UNITS),
            content={
                "action": random.choice(actions),
                "objective": f"OBJ-{random.randint(1,20)}",
                "urgency": random.choice(["ACİL", "NORMAL", "PLANLANAN"]),
                "rules_of_engagement": random.choice(["SERBEST", "KISITLI", "SAVUNMA"]),
            },
            priority=PRIORITY_MAP[MessageType.ORDER],
            requires_ack=True,
        )

    def generate_report(self, reporter: str = None) -> C2Message:
        """Durum raporu üret."""
        return C2Message(
            id=str(uuid.uuid4()),
            msg_type=MessageType.REPORT,
            sender=reporter or random.choice(UNITS),
            receiver=random.choice(COMMANDERS),
            content={
                "type": random.choice(["TEMAS", "DURUM", "HASAR", "KEŞİF", "LOJİSTİK"]),
                "personnel_status": {"active": random.randint(15, 50), "wounded": random.randint(0, 5)},
                "vehicle_status": {"operational": random.randint(3, 12), "damaged": random.randint(0, 3)},
                "ammo_level": random.choice(["YETERLİ", "DÜŞÜK", "KRİTİK"]),
                "morale": random.choice(["YÜKSEK", "NORMAL", "DÜŞÜK"]),
            },
            priority=PRIORITY_MAP[MessageType.REPORT],
        )

    def generate_intel(self) -> C2Message:
        """İstihbarat mesajı üret."""
        return C2Message(
            id=str(uuid.uuid4()),
            msg_type=MessageType.INTEL,
            sender=random.choice(["İSTH-MERKEZ", "3-KEŞIF-TIM", "6-İHA-FİL"]),
            receiver="KUVVET-KOMUTANLIĞI",
            content={
                "intel_type": random.choice(["ELINT", "SIGINT", "IMINT", "HUMINT", "OSINT"]),
                "threat_level": random.choice(["LOW", "MEDIUM", "HIGH", "CRITICAL"]),
                "enemy_strength": random.choice(["TAKIM", "MANGA", "BÖLÜK", "TABUR"]),
                "reliability": random.choice(["A1", "B2", "C3", "D4", "F6"]),
                "description": "Düşman hareketi tespit edildi",
            },
            priority=PRIORITY_MAP[MessageType.INTEL],
            requires_ack=True,
        )

    def generate_alert(self) -> C2Message:
        """Alarm mesajı üret."""
        alerts = [
            {"type": "HAVA_SALDIRISI", "detail": "Yaklaşan düşman hava aracı"},
            {"type": "KİMYASAL", "detail": "Kimyasal madde şüphesi"},
            {"type": "SİBER_SALDIRI", "detail": "Ağ anomalisi tespit"},
            {"type": "TEMAS", "detail": "Düşman teması yakın mesafe"},
            {"type": "TOP_ATIŞI", "detail": "Gelen topçu ateşi"},
        ]
        alert = random.choice(alerts)
        return C2Message(
            id=str(uuid.uuid4()),
            msg_type=MessageType.ALERT,
            sender="SİSTEM",
            receiver="TÜM-BİRLİKLER",
            content=alert,
            priority=PRIORITY_MAP[MessageType.ALERT],
            ttl_sec=30,
            requires_ack=True,
        )

    def generate_random(self) -> C2Message:
        """Rastgele mesaj tipi üret."""
        generators = [self.generate_order, self.generate_report,
                      self.generate_intel, self.generate_alert]
        weights = [0.3, 0.35, 0.2, 0.15]
        gen = random.choices(generators, weights=weights, k=1)[0]
        return gen()

    def generate_burst(self, count: int = 10) -> list[C2Message]:
        """Toplu mesaj üret."""
        return [self.generate_random() for _ in range(count)]
