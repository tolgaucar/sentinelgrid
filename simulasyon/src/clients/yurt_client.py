"""YURT modülü HTTP istemcisi."""

from __future__ import annotations

from src.clients.base import BaseClient


class YurtClient(BaseClient):
    """YURT — Uç Bilişim ve Yerel Depolama istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9101):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "YURT"

    # ── Yerel Depolama ──

    def store_put(self, key: str, value: dict) -> dict:
        """Yerel depoya veri yaz."""
        return self._request("POST", "/store/put", json={"key": key, "value": value})

    def store_get(self, key: str) -> dict:
        """Yerel depodan veri oku."""
        return self._request("GET", f"/store/get?key={key}")

    def store_delete(self, key: str) -> dict:
        """Yerel depodan veri sil."""
        return self._request("POST", "/store/delete", json={"key": key})

    def store_keys(self, prefix: str = "") -> list:
        """Anahtar listesi."""
        return self._request("GET", f"/store/keys?prefix={prefix}")

    def store_dirty(self, limit: int = 100) -> list:
        """Senkronize edilmemiş kayıtlar."""
        return self._request("GET", f"/store/dirty?limit={limit}")

    def store_snapshot(self) -> dict:
        """Tam depo snapshot'ı."""
        return self._request("GET", "/store/snapshot")

    def store_expire(self) -> dict:
        """Süresi dolan kayıtları temizle."""
        return self._request("POST", "/store/expire")

    def store_stats(self) -> dict:
        """Depo istatistikleri."""
        return self._request("GET", "/store/stats")

    # ── Senkronizasyon ──

    def sync_push(self) -> dict:
        """Yerel → Bulut push."""
        return self._request("POST", "/sync/push")

    def sync_pull(self) -> dict:
        """Bulut → Yerel pull."""
        return self._request("POST", "/sync/pull")

    def sync_full(self) -> dict:
        """Tam çift yönlü senkronizasyon."""
        return self._request("POST", "/sync/full")

    def sync_state(self) -> dict:
        """Senkronizasyon durumu."""
        return self._request("GET", "/sync/state")

    def sync_conflicts(self, n: int = 50) -> list:
        """Çatışma geçmişi."""
        return self._request("GET", f"/sync/conflicts?n={n}")

    def sync_changelog(self, since: str = "") -> list:
        """Değişiklik günlüğü."""
        return self._request("GET", f"/sync/changelog?since={since}")

    def sync_stats(self) -> dict:
        """Senkronizasyon istatistikleri."""
        return self._request("GET", "/sync/stats")

    # ── Proxy ──

    def proxy_request(self, method: str, path: str, body: dict = None, priority: int = 3) -> dict:
        """Proxy üzerinden istek gönder."""
        return self._request("POST", "/proxy/request", json={
            "method": method, "path": path, "body": body, "priority": priority,
        })

    def proxy_replay(self) -> dict:
        """Tamponlanmış istekleri tekrar gönder."""
        return self._request("POST", "/proxy/replay")

    def proxy_buffer(self) -> list:
        """Tampon içeriği."""
        return self._request("GET", "/proxy/buffer")

    def proxy_connection(self) -> dict:
        """Bağlantı durumu."""
        return self._request("GET", "/proxy/connection")

    def proxy_set_connection(self, state: str) -> dict:
        """Bağlantı durumunu değiştir (ONLINE/OFFLINE/DEGRADED)."""
        return self._request("PUT", "/proxy/connection", json={"state": state})

    def proxy_health(self) -> dict:
        """Proxy sağlık kontrolü."""
        return self._request("GET", "/proxy/health")

    def proxy_clear(self) -> dict:
        """Tamponu temizle."""
        return self._request("POST", "/proxy/clear")

    def proxy_stats(self) -> dict:
        """Proxy istatistikleri."""
        return self._request("GET", "/proxy/stats")

    def proxy_summary(self) -> dict:
        """Proxy durum özeti."""
        return self._request("GET", "/proxy/summary")

    # ── Partition ──

    def partition_state(self) -> dict:
        """Bölünme durumu."""
        return self._request("GET", "/partition/state")

    def partition_check(self) -> dict:
        """Bölünme kontrolü yap."""
        return self._request("POST", "/partition/check")

    def partition_peers(self) -> list:
        """Düğüm listesi."""
        return self._request("GET", "/partition/peers")

    def partition_add_peer(self, peer_id: str, address: str, role: str) -> dict:
        """Düğüm ekle."""
        return self._request("POST", "/partition/peers", json={
            "id": peer_id, "address": address, "role": role,
        })

    def partition_remove_peer(self, peer_id: str) -> dict:
        """Düğüm kaldır."""
        return self._request("DELETE", f"/partition/peers/{peer_id}")

    def partition_heartbeat(self, peer_id: str, latency_ms: float) -> dict:
        """Heartbeat gönder."""
        return self._request("POST", "/partition/heartbeat", json={
            "peer_id": peer_id, "latency_ms": latency_ms,
        })

    def partition_events(self, n: int = 50) -> list:
        """Bölünme olayları."""
        return self._request("GET", f"/partition/events?n={n}")

    def partition_stats(self) -> dict:
        """Bölünme istatistikleri."""
        return self._request("GET", "/partition/stats")

    def partition_summary(self) -> dict:
        """Bölünme durum özeti."""
        return self._request("GET", "/partition/summary")

    def get_stats(self) -> dict:
        """Tüm istatistikler."""
        return self._request("GET", "/stats")
