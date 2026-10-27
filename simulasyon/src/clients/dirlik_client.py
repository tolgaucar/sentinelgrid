"""DİRLİK modülü HTTP istemcisi."""

from __future__ import annotations

from src.clients.base import BaseClient


class DirlikClient(BaseClient):
    """DİRLİK — Sistem Sağlık Taraması istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9095):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "DİRLİK"

    # ── Metrikler ──

    def get_current_metrics(self) -> dict:
        """Anlık sistem metriklerini getir."""
        return self._request("GET", "/metrics/current")

    def get_metrics_history(self, n: int = 60) -> dict:
        """Metrik geçmişini getir."""
        return self._request("GET", f"/metrics/history?n={n}")

    def get_service_metrics(self) -> dict:
        """Servis metriklerini getir."""
        return self._request("GET", "/metrics/services")

    # ── Probe'lar ──

    def get_probes(self) -> dict:
        """Tüm probe durumlarını getir."""
        return self._request("GET", "/probes")

    def add_probe(self, service_id: str, endpoint: str, probe_type: str = "http") -> dict:
        """Yeni probe hedefi ekle."""
        return self._request("POST", "/probes", json={
            "service_id": service_id,
            "type": probe_type,
            "endpoint": endpoint,
        })

    def check_probe(self, service_id: str) -> dict:
        """Tek seferlik probe yap."""
        return self._request("GET", f"/probes/{service_id}/check")

    def get_probe_state(self, service_id: str) -> dict:
        """Probe durumunu getir."""
        return self._request("GET", f"/probes/{service_id}")

    def remove_probe(self, service_id: str) -> dict:
        """Probe hedefini kaldır."""
        return self._request("DELETE", f"/probes/{service_id}")

    # ── Alarmlar ──

    def get_alerts(self, n: int = 100) -> dict:
        """Alarm geçmişini getir."""
        return self._request("GET", f"/alerts?n={n}")

    def get_alert_summary(self) -> dict:
        """Alarm özetini getir."""
        return self._request("GET", "/alerts/summary")

    def get_alert_records(self, n: int = 100) -> dict:
        """Gönderilmiş alarm kayıtlarını getir."""
        return self._request("GET", f"/alerts/records?n={n}")
