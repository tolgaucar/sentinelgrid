"""
BÜTÜN Modül İstemcisi

BÜTÜN API Gateway'e bağlanarak servis kaydı, keşif, orkestrasyon
ve kaynak yönetimi işlemlerini gerçekleştirir.
"""

from __future__ import annotations

from typing import Any

from src.clients.base import BaseClient


class ButunClient(BaseClient):
    """BÜTÜN (Ana Çatı Sistemi) istemcisi."""

    MODULE_NAME = "BÜTÜN"

    def __init__(self, host: str = "http://127.0.0.1", port: int = 8000, **kwargs):
        super().__init__(f"{host}:{port}", **kwargs)

    # ── Sistem ──

    def get_status(self) -> dict:
        return self._request("GET", "/")

    # ── Servis Yönetimi ──

    def register_service(self, name: str, cluster: str, port: int,
                         host: str = "127.0.0.1", dependencies: list[str] = None,
                         metadata: dict = None) -> dict:
        payload = {
            "name": name,
            "cluster": cluster,
            "host": host,
            "port": port,
            "dependencies": dependencies or [],
            "metadata": metadata or {},
        }
        return self._request("POST", "/api/v1/services/register", json=payload)

    def deregister_service(self, name: str) -> dict:
        return self._request("DELETE", f"/api/v1/services/{name}")

    def heartbeat(self, name: str) -> dict:
        return self._request("POST", f"/api/v1/services/{name}/heartbeat")

    def list_services(self, cluster: str = None) -> dict:
        params = {"cluster": cluster} if cluster else {}
        return self._request("GET", "/api/v1/services", params=params)

    def get_service(self, name: str) -> dict:
        return self._request("GET", f"/api/v1/services/{name}")

    # ── Servis Keşfi ──

    def discover(self, name: str) -> dict:
        return self._request("GET", f"/api/v1/discover/{name}")

    # ── Orkestrasyon ──

    def start_service(self, name: str, service_info: dict) -> dict:
        return self._request("POST", f"/api/v1/orchestrator/start/{name}", json=service_info)

    def stop_service(self, name: str, graceful: bool = True) -> dict:
        return self._request("POST", f"/api/v1/orchestrator/stop/{name}", params={"graceful": graceful})

    def restart_service(self, name: str) -> dict:
        return self._request("POST", f"/api/v1/orchestrator/restart/{name}")

    # ── Kaynak Yönetimi ──

    def get_resources(self) -> dict:
        return self._request("GET", "/api/v1/resources")

    def get_alerts(self) -> dict:
        return self._request("GET", "/api/v1/resources/alerts")

    # ── Bağımlılık ──

    def get_dependencies(self) -> dict:
        return self._request("GET", "/api/v1/dependencies")

    def get_impact(self, node: str) -> dict:
        return self._request("GET", f"/api/v1/dependencies/{node}/impact")

    # ── Circuit Breaker ──

    def get_circuits(self) -> dict:
        return self._request("GET", "/api/v1/circuits")

    def reset_circuit(self, name: str) -> dict:
        return self._request("POST", f"/api/v1/circuits/{name}/reset")
