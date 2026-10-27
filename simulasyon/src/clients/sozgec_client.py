"""SÖZGEÇ modülü simülasyon istemcisi — Karmaşık Sinyal Ayıklama."""

from __future__ import annotations

from src.clients.base import BaseClient


class SozgecClient(BaseClient):
    """SÖZGEÇ — Karmaşık Sinyal Ayıklama istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9124):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "SÖZGEÇ"

    # ── Sağlık & Durum ──

    def health(self) -> dict:
        return self._request("GET", "/health")

    def status(self) -> dict:
        return self._request("GET", "/status")

    def stats(self) -> dict:
        return self._request("GET", "/stats")

    # ── Spektral Analiz ──

    def analyze(self, signals: list[dict]) -> dict:
        return self._request("POST", "/analyze", json={"signals": signals})

    def get_bands(self) -> dict:
        return self._request("GET", "/bands")

    def get_band(self, band_id: str) -> dict:
        return self._request("GET", f"/bands/{band_id}")

    # ── Filtrelenmiş Sinyaller ──

    def get_signals(self) -> dict:
        return self._request("GET", "/signals")

    def get_threats(self) -> dict:
        return self._request("GET", "/threats")

    def get_friendly(self) -> dict:
        return self._request("GET", "/friendly")

    # ── Dost Frekans Bantları ──

    def get_friendly_bands(self) -> dict:
        return self._request("GET", "/friendly-bands")

    def add_friendly_band(self, band: dict) -> dict:
        return self._request("POST", "/friendly-bands", json=band)

    def remove_friendly_band(self, band_id: str) -> dict:
        return self._request("DELETE", "/friendly-bands", json={"id": band_id})

    # ── Simülasyon Entegrasyonu ──

    def process(self, entities: list[dict]) -> dict:
        """Toplu işleme: varlıklardan RF sinyal → FFT → filtre → rapor."""
        return self._request("POST", "/process", json={"entities": entities})

    # ── Bakım ──

    def prune(self) -> dict:
        return self._request("GET", "/prune")
