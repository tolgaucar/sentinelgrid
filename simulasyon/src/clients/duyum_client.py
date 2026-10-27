"""DUYUM modülü simülasyon istemcisi — Açık Kaynak İstihbarat (OSINT)."""

from __future__ import annotations

from src.clients.base import BaseClient


class DuyumClient(BaseClient):
    """DUYUM — Açık Kaynak İstihbarat istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9120):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "DUYUM"

    # ── Sağlık & Durum ──

    def health(self) -> dict:
        return self._request("GET", "/health")

    def status(self) -> dict:
        return self._request("GET", "/status")

    def stats(self) -> dict:
        return self._request("GET", "/stats")

    # ── Kaynak Yönetimi (CRUD) ──

    def get_sources(self) -> dict:
        return self._request("GET", "/sources")

    def add_source(self, source: dict) -> dict:
        return self._request("POST", "/sources", json=source)

    def get_source(self, source_id: str) -> dict:
        return self._request("GET", f"/sources/{source_id}")

    def update_source(self, source_id: str, update: dict) -> dict:
        return self._request("PUT", f"/sources/{source_id}", json=update)

    def delete_source(self, source_id: str) -> dict:
        return self._request("DELETE", f"/sources/{source_id}")

    # ── Veri Toplama ──

    def collect(self, source_id: str) -> dict:
        return self._request("POST", "/collect", json={"source_id": source_id})

    def collect_all(self) -> dict:
        return self._request("GET", "/collect/all")

    # ── Analiz ──

    def analyze(self, items: list[dict]) -> dict:
        return self._request("POST", "/analyze", json={"items": items})

    # ── Simülasyon Entegrasyonu ──

    def process(self, entities: list[dict]) -> dict:
        """Toplu veri işleme: varlıklardan OSINT üret → analiz et → rapor."""
        return self._request("POST", "/process", json={"entities": entities})

    # ── Raporlar ──

    def get_reports(self) -> dict:
        return self._request("GET", "/reports")

    def get_report(self, report_id: str) -> dict:
        return self._request("GET", f"/reports/{report_id}")

    def get_threats(self) -> dict:
        return self._request("GET", "/threats")

    def get_trends(self) -> dict:
        return self._request("GET", "/trends")

    # ── Bakım ──

    def prune(self) -> dict:
        return self._request("GET", "/prune")
