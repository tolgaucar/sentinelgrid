"""YAYIN (Şifreli Veri Yayımı) modülü HTTP istemcisi."""

from __future__ import annotations

from src.clients.base import BaseClient


class YayinClient(BaseClient):
    """YAYIN — Şifreli ve Sıkıştırılmış Veri Yayımı istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9107):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "YAYIN"

    # ── Sağlık & İstatistik ──

    def health(self) -> dict:
        return self._request("GET", "/health")

    def status(self) -> dict:
        return self._request("GET", "/status")

    # ── Kanallar ──

    def list_channels(self) -> list:
        return self._request("GET", "/channels")

    def create_channel(self, name: str, description: str = "", priority: int = 1,
                       encrypted: bool = True, compressed: bool = True, fec: bool = True) -> dict:
        return self._request("POST", "/channels", json={
            "name": name, "description": description, "priority": priority,
            "encrypted": encrypted, "compressed": compressed, "fec": fec,
        })

    def get_channel(self, channel_id: str) -> dict:
        return self._request("GET", f"/channels/{channel_id}")

    def delete_channel(self, channel_id: str) -> dict:
        return self._request("DELETE", f"/channels/{channel_id}")

    # ── Yayın ──

    def publish(self, channel_id: str, sender_id: str, content_type: str,
                payload: str, priority: int = 1) -> dict:
        return self._request("POST", "/publish", json={
            "channel_id": channel_id, "sender_id": sender_id,
            "content_type": content_type, "payload": payload, "priority": priority,
        })

    def decode(self, channel_id: str, message_id: str) -> dict:
        return self._request("POST", "/decode", json={
            "channel_id": channel_id, "message_id": message_id,
        })

    # ── Abonelik ──

    def subscribe(self, channel_id: str, unit_id: str) -> dict:
        return self._request("POST", "/subscribe", json={
            "channel_id": channel_id, "unit_id": unit_id,
        })

    def unsubscribe(self, channel_id: str, unit_id: str) -> dict:
        return self._request("POST", "/unsubscribe", json={
            "channel_id": channel_id, "unit_id": unit_id,
        })

    # ── Geçmiş ──

    def history(self, channel_id: str, n: int = 50) -> list:
        return self._request("GET", f"/history/{channel_id}?n={n}")

    # ── Şifreleme ──

    def crypto_keys(self) -> dict:
        return self._request("GET", "/crypto/keys")

    def crypto_rotate(self) -> dict:
        return self._request("POST", "/crypto/rotate")
