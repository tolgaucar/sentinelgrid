"""SEZİ modülü simülasyon istemcisi — Doku Analiziyle Kamuflaj Bulma."""

from __future__ import annotations

from src.clients.base import BaseClient


class SeziClient(BaseClient):
    """SEZİ — Doku Analiziyle Kamuflaj Bulma istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9116):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "SEZİ"

    # ── Sağlık & Durum ──

    def health(self) -> dict:
        return self._request("GET", "/health")

    def status(self) -> dict:
        return self._request("GET", "/status")

    def stats(self) -> dict:
        return self._request("GET", "/stats")

    # ── Doku Analizi ──

    def analyze(self, regions: list[dict]) -> dict:
        """Bölge listesini doku analizinden geçir."""
        return self._request("POST", "/analyze", json=regions)

    # ── Kamuflaj Tarama ──

    def scan(self, inputs: list[dict]) -> dict:
        """Bölge listesini kamuflaj taramasından geçir."""
        return self._request("POST", "/scan", json=inputs)

    def process(self, inputs: list[dict]) -> dict:
        """Tam boru hattı: analiz → tespit → değerlendirme."""
        return self._request("POST", "/process", json=inputs)

    # ── Tespit Sorgu ──

    def get_detections(self) -> dict:
        return self._request("GET", "/detections")

    def get_detection(self, detection_id: str) -> dict:
        return self._request("GET", f"/detections/{detection_id}")

    def get_threats(self, level: str = "high") -> dict:
        return self._request("GET", f"/detections/threats?level={level}")

    # ── Arazi Profilleri ──

    def get_terrains(self) -> dict:
        return self._request("GET", "/terrains")
