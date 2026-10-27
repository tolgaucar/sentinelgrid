"""
BAĞ Modül İstemcisi

BAĞ Veri Transfer Katmanı'nın HTTP status portuna bağlanarak
mesaj yayınlama, topic yönetimi ve durum sorgulama yapar.
"""

from __future__ import annotations

import json
import time
import uuid
from typing import Any

from src.clients.base import BaseClient


class BagClient(BaseClient):
    """BAĞ (Veri Transfer Katmanı) istemcisi."""

    MODULE_NAME = "BAĞ"

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9091, **kwargs):
        super().__init__(f"{host}:{port}", **kwargs)

    # ── Durum ──

    def get_status(self) -> dict:
        return self._request("GET", "/status")

    def get_topics(self) -> dict:
        return self._request("GET", "/topics")

    # ── Mesaj Yayınlama ──

    def publish(self, topic: str, data: dict, priority: int = 5) -> dict:
        """Topic'e mesaj yayınla."""
        payload = {
            "id": str(uuid.uuid4()),
            "data": data,
            "priority": priority,
            "timestamp_ns": time.time_ns(),
        }
        return self._request("POST", f"/publish?topic={topic}", json=payload)

    def publish_sensor_data(self, sensor_reading: dict) -> dict:
        """Sensör verisini ilgili topic'e yayınla."""
        sensor_type = sensor_reading.get("sensor_type", "unknown")
        topic = f"sensor.{sensor_type}"
        return self.publish(topic, sensor_reading)

    def publish_alert(self, level: str, message: str, source: str, data: dict = None) -> dict:
        """Alarm mesajı yayınla."""
        topic = f"alert.{level}"
        payload = {
            "level": level,
            "message": message,
            "source": source,
            "data": data or {},
            "timestamp": time.time(),
        }
        return self.publish(topic, payload, priority=1 if level == "critical" else 3)

    def publish_telemetry(self, module: str, metrics: dict) -> dict:
        """Telemetri verisi yayınla."""
        topic = f"telemetry.{module.lower()}"
        return self.publish(topic, metrics, priority=7)

    def publish_command(self, target: str, command: str, params: dict = None) -> dict:
        """Komut mesajı yayınla."""
        topic = f"command.{target.lower()}"
        payload = {
            "command": command,
            "params": params or {},
            "issued_at": time.time(),
        }
        return self.publish(topic, payload, priority=2)

    # ── Toplu Yayın ──

    def publish_batch(self, messages: list[dict]) -> list[dict]:
        """Toplu mesaj yayınla."""
        results = []
        for msg in messages:
            topic = msg.get("topic", "default")
            data = msg.get("data", {})
            priority = msg.get("priority", 5)
            result = self.publish(topic, data, priority)
            results.append(result)
        return results
