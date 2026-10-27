"""
YANKI Simülasyon İstemcisi — Akustik Konumlandırma Modülü

Simülasyondan YANKI modülüne HTTP ile bağlanır.
Akustik olay tespiti, TOA üçgenlemesi ve tehdit izleme yapar.
"""

from __future__ import annotations

from src.clients.base import BaseClient

YANKI_PORT = 9118


class YankiClient(BaseClient):
    """YANKI — Akustik Konumlandırma istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = YANKI_PORT):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "YANKI"

    def health(self) -> dict:
        return self._request("GET", "/health")

    def status(self) -> dict:
        return self._request("GET", "/status")

    def stats(self) -> dict:
        return self._request("GET", "/stats")

    def detect(self, readings: list[dict]) -> dict:
        """Sensör okumalarından akustik olay tespiti."""
        return self._request("POST", "/detect", json={"readings": readings})

    def locate(self, toas: list[dict]) -> dict:
        """TOA üçgenlemesi ile kaynak konumlandırma."""
        return self._request("POST", "/locate", json={"toas": toas})

    def process(self, entities: list[dict]) -> dict:
        """Toplu veri işleme (simülasyon entegrasyonu)."""
        return self._request("POST", "/process", json={"entities": entities})

    def get_detections(self, event_type: str = None) -> dict:
        """Tüm aktif tespitler."""
        params = f"?type={event_type}" if event_type else ""
        return self._request("GET", f"/detections{params}")

    def get_threats(self) -> dict:
        """Yüksek ve kritik tehdit tespitleri."""
        return self._request("GET", "/threats")

    def get_gunshots(self) -> dict:
        """Silah atışı tespitleri."""
        return self._request("GET", "/gunshots")

    def prune(self) -> dict:
        """Süresi dolmuş tespitleri temizle."""
        return self._request("GET", "/prune")
