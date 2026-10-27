"""DURAK modülü simülasyon istemcisi — Veri Paketi Önceliklendirme (QoS)."""

from __future__ import annotations

import random
import time

from src.clients.base import BaseClient

# Öncelik seviyeleri
PRIORITIES = {
    "FLASH": 0,     # NBC alarm, geri çekilme emri
    "CRITICAL": 1,  # Hedef vurma onayı, düşman teması
    "HIGH": 2,      # Komut, koordinat
    "NORMAL": 3,    # Telemetri, sensör
    "LOW": 4,       # Güncelleme, log
    "BULK": 5,      # Video, görüntü
}

# Varsayılan kuyruklar
DEFAULT_QUEUES = ["komut", "sensor", "telemetri", "video", "bulk"]


class DurakClient(BaseClient):
    """DURAK — Veri Paketi Önceliklendirme (QoS) istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9109):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "DURAK"

    # ── Sağlık & Durum ──

    def health(self) -> dict:
        return self._request("GET", "/health")

    def status(self) -> dict:
        return self._request("GET", "/status")

    def queues(self) -> dict:
        return self._request("GET", "/queues")

    # ── Kuyruk İşlemleri ──

    def enqueue(self, queue_id: str, priority: int, payload: str,
                source: str = "sim", destination: str = "hedef",
                content_type: str = "text/plain", ttl_seconds: int = 300) -> dict:
        """Kuyruğa paket ekle."""
        return self._request("POST", "/enqueue", json={
            "queue_id": queue_id,
            "priority": priority,
            "source": source,
            "destination": destination,
            "content_type": content_type,
            "payload": payload,
            "ttl_seconds": ttl_seconds,
        })

    def dequeue(self, queue_id: str) -> dict | None:
        """Kuyruktan paket çıkar."""
        return self._request("POST", "/dequeue", json={"queue_id": queue_id})

    # ── Zamanlayıcı ──

    def scheduler_stats(self) -> dict:
        return self._request("GET", "/scheduler/stats")

    def set_bandwidth(self, bps: int) -> dict:
        return self._request("POST", "/scheduler/bandwidth", json={"bps": bps})

    def flush(self, queue_id: str = "") -> dict:
        body = {"queue_id": queue_id} if queue_id else {}
        return self._request("POST", "/flush", json=body)

    # ── Simülasyon ──

    def simulate_traffic(self, count: int = 20) -> dict:
        """Askeri trafik simülasyonu — karışık öncelikte paketler üret."""
        scenarios = [
            ("komut", PRIORITIES["FLASH"], "NBC alarm: kimyasal tespit, tahliye emri"),
            ("komut", PRIORITIES["CRITICAL"], "Hedef vurma onayı: T-72 tank, mesafe 2.4km"),
            ("komut", PRIORITIES["HIGH"], "Takım hareket emri: kuzey yönü"),
            ("sensor", PRIORITIES["NORMAL"], "Radar tarama: sektör 240-280°, hedef yok"),
            ("telemetri", PRIORITIES["NORMAL"], "GPS: 39.925°N 32.837°E, hız 45km/h"),
            ("video", PRIORITIES["LOW"], "İHA kamera akışı: 1280x720, 15fps"),
            ("bulk", PRIORITIES["BULK"], "Harita güncellemesi: tile z14 x8792 y5941"),
        ]

        results = {"enqueued": 0, "errors": 0}
        for i in range(count):
            queue_id, priority, payload = random.choice(scenarios)
            payload_with_ts = f"[{time.strftime('%H:%M:%S')}] {payload} (seq={i})"
            resp = self.enqueue(queue_id, priority, payload_with_ts)
            if resp and "error" in resp:
                results["errors"] += 1
            else:
                results["enqueued"] += 1

        return results
