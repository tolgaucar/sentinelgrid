"""İM (Dijital Kimlik & İmza) modülü HTTP istemcisi."""

from __future__ import annotations

from src.clients.base import BaseClient


class ImClient(BaseClient):
    """İM — Dijital Kimlik ve İmza Yönetimi istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9105):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "İM"

    # ── Sağlık & İstatistik ──

    def health(self) -> dict:
        return self._request("GET", "/health")

    def status(self) -> dict:
        return self._request("GET", "/status")

    # ── PKI ──

    def pki_ca_info(self) -> dict:
        return self._request("GET", "/pki/ca")

    def pki_issue(self, unit_id: str, unit_type: str) -> dict:
        return self._request("POST", "/pki/issue", json={"unit_id": unit_id, "unit_type": unit_type})

    def pki_verify(self, serial: str) -> dict:
        return self._request("POST", "/pki/verify", json={"serial": serial})

    def pki_revoke(self, serial: str) -> dict:
        return self._request("POST", "/pki/revoke", json={"serial": serial})

    def pki_list(self) -> dict:
        return self._request("GET", "/pki/certs")

    # ── Token ──

    def token_issue(self, unit_id: str, roles: list, perms: list) -> dict:
        return self._request("POST", "/token/issue", json={"unit_id": unit_id, "roles": roles, "perms": perms})

    def token_verify(self, token: str) -> dict:
        return self._request("POST", "/token/verify", json={"token": token})

    def token_refresh(self, refresh_token: str) -> dict:
        return self._request("POST", "/token/refresh", json={"refresh_token": refresh_token})

    def token_revoke(self, token: str) -> dict:
        return self._request("POST", "/token/revoke", json={"token": token})

    # ── Kimlik ──

    def enroll(self, unit_id: str, unit_type: str, roles: list, name: str = "") -> dict:
        return self._request("POST", "/enroll", json={
            "id": unit_id, "type": unit_type, "roles": roles, "name": name,
        })

    def identity_get(self, unit_id: str) -> dict:
        return self._request("GET", f"/identity/{unit_id}")

    def identity_suspend(self, unit_id: str) -> dict:
        return self._request("POST", f"/identity/{unit_id}/suspend")

    def identity_reactivate(self, unit_id: str) -> dict:
        return self._request("POST", f"/identity/{unit_id}/reactivate")
