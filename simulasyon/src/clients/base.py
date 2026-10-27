"""
Modül İstemci Temel Sınıfı

Tüm ÖTÜKEN-MATRİS modül istemcileri bu sınıftan türer.
Yeni modül eklendiğinde BaseClient'ten türetilip clients/ altına eklenir.
"""

from __future__ import annotations

import time
from typing import Any, Optional

import httpx


class BaseClient:
    """Modül istemci temel sınıfı."""

    MODULE_NAME: str = "UNKNOWN"

    def __init__(self, base_url: str, timeout: float = 10.0):
        self._base_url = base_url.rstrip("/")
        self._timeout = timeout
        self._client = httpx.Client(base_url=self._base_url, timeout=timeout)
        self._request_count = 0
        self._error_count = 0
        self._last_latency_ms = 0.0

    @property
    def base_url(self) -> str:
        return self._base_url

    @property
    def stats(self) -> dict:
        return {
            "module": self.MODULE_NAME,
            "base_url": self._base_url,
            "requests": self._request_count,
            "errors": self._error_count,
            "last_latency_ms": self._last_latency_ms,
        }

    def _request(self, method: str, path: str, **kwargs) -> dict | list | None:
        """HTTP istek gönder, hata yönetimi ile."""
        self._request_count += 1
        start = time.time()
        try:
            resp = self._client.request(method, path, **kwargs)
            self._last_latency_ms = (time.time() - start) * 1000
            resp.raise_for_status()
            return resp.json()
        except httpx.HTTPStatusError as e:
            self._error_count += 1
            return {"error": str(e), "status_code": e.response.status_code}
        except httpx.ConnectError:
            self._error_count += 1
            return {"error": f"{self.MODULE_NAME} bağlantı hatası: {self._base_url}"}
        except Exception as e:
            self._error_count += 1
            return {"error": str(e)}

    def health_check(self) -> dict:
        """Sağlık kontrolü."""
        return self._request("GET", "/health") or {"error": "yanıt yok"}

    def is_alive(self) -> bool:
        """Modül erişilebilir mi?"""
        try:
            result = self.health_check()
            return "error" not in result
        except Exception:
            return False

    def close(self) -> None:
        self._client.close()
