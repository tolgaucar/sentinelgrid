"""
KÜTÜK Modül İstemcisi

KÜTÜK Anlık Durum Veritabanı'na HTTP üzerinden bağlanarak
varlık durumlarını okuma/yazma/tarama yapar.
"""

from __future__ import annotations

import json
import time
from typing import Any

from src.clients.base import BaseClient


class KutukClient(BaseClient):
    """KÜTÜK (Anlık Durum Veritabanı) istemcisi."""

    MODULE_NAME = "KÜTÜK"

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9093, **kwargs):
        super().__init__(f"{host}:{port}", **kwargs)

    # ── Durum ──

    def get_status(self) -> dict:
        return self._request("GET", "/status")

    def get_raft_state(self) -> dict:
        return self._request("GET", "/raft")

    def get_namespaces(self) -> dict:
        return self._request("GET", "/namespaces")

    # ── CRUD ──

    def put(self, namespace: str, key: str, value: Any, ttl_ms: int = 0,
            metadata: dict = None) -> dict:
        """Anahtar-değer yaz."""
        payload = {
            "namespace": namespace,
            "key": key,
            "value": value,
            "ttl_ms": ttl_ms,
        }
        if metadata:
            payload["metadata"] = metadata
        return self._request("POST", "/put", json=payload)

    def get(self, namespace: str, key: str) -> dict:
        """Anahtar oku."""
        return self._request("GET", "/get", params={"ns": namespace, "key": key})

    def delete(self, namespace: str, key: str) -> dict:
        """Anahtar sil."""
        return self._request("DELETE", "/delete", params={"ns": namespace, "key": key})

    def scan(self, namespace: str, prefix: str = "", limit: int = 100) -> dict:
        """Prefix ile tarama yap."""
        return self._request("GET", "/scan", params={
            "ns": namespace, "prefix": prefix, "limit": limit
        })

    def keys(self, namespace: str = "") -> dict:
        """Anahtarları listele."""
        params = {"ns": namespace} if namespace else {}
        return self._request("GET", "/keys", params=params)

    # ── Varlık Kısayolları ──

    def put_entity(self, entity_id: str, data: dict, ttl_ms: int = 0) -> dict:
        """Savaş alanı varlığını kaydet."""
        return self.put("entity", entity_id, data, ttl_ms)

    def get_entity(self, entity_id: str) -> dict:
        """Savaş alanı varlığını oku."""
        return self.get("entity", entity_id)

    def put_sensor(self, sensor_id: str, data: dict, ttl_ms: int = 5000) -> dict:
        """Sensör verisini kaydet (varsayılan 5s TTL)."""
        return self.put("sensor", sensor_id, data, ttl_ms)

    def put_track(self, track_id: str, data: dict, ttl_ms: int = 30000) -> dict:
        """İz kaydı yaz (varsayılan 30s TTL)."""
        return self.put("track", track_id, data, ttl_ms)

    def put_alert(self, alert_id: str, data: dict, ttl_ms: int = 60000) -> dict:
        """Alarm kaydı yaz."""
        return self.put("alert", alert_id, data, ttl_ms)
