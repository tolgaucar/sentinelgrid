"""KÖZ modülü simülasyon istemcisi — Termal İz ve Motor Sıcaklığı Analizi."""

from __future__ import annotations

from src.clients.base import BaseClient


class KozClient(BaseClient):
    """KÖZ — Termal İz ve Motor Sıcaklığı Analizi istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9115):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "KÖZ"

    # ── Sağlık & Durum ──

    def health(self) -> dict:
        return self._request("GET", "/health")

    def status(self) -> dict:
        return self._request("GET", "/status")

    def stats(self) -> dict:
        return self._request("GET", "/stats")

    # ── Termal Tarama ──

    def scan(self, entities: list[dict]) -> dict:
        """Varlık listesini termal taramadan geçir."""
        return self._request("POST", "/scan", json=entities)

    def process(self, entities: list[dict]) -> dict:
        """Tam boru hattı: tarama → kümeleme → soğuma → anomali."""
        return self._request("POST", "/process", json=entities)

    # ── İmza Sorgu ──

    def get_signatures(self) -> dict:
        return self._request("GET", "/signatures")

    def get_signature(self, entity_id: str) -> dict:
        return self._request("GET", f"/signatures/{entity_id}")

    def get_hot_signatures(self, min_delta: float = 20.0) -> dict:
        return self._request("GET", f"/signatures/hot?min_delta={min_delta}")

    # ── Analiz ──

    def get_clusters(self) -> dict:
        return self._request("GET", "/analyze/clusters")

    def get_cooling_predictions(self) -> dict:
        return self._request("GET", "/analyze/cooling")

    def detect_anomalies(self) -> dict:
        return self._request("POST", "/analyze/anomalies", json={})

    def get_anomalies(self, n: int = 50) -> dict:
        return self._request("GET", f"/analyze/anomalies?n={n}")

    # ── Yapılandırma ──

    def get_ambient(self) -> dict:
        return self._request("GET", "/ambient")

    def set_ambient(self, temp_c: float) -> dict:
        return self._put("/ambient", {"temp_c": temp_c})
