"""AKIM modülü HTTP istemcisi."""

from __future__ import annotations

from src.clients.base import BaseClient


class AkimClient(BaseClient):
    """AKIM — Veri Trafiği Yoğunluk Yönlendirmesi istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9099):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "AKIM"

    # ── Topoloji ──

    def get_topology(self) -> dict:
        """Topoloji özetini getir."""
        return self._request("GET", "/topology")

    def list_nodes(self) -> list:
        """Tüm düğümleri listele."""
        return self._request("GET", "/nodes")

    def add_node(self, node_id: str, node_type: str = "relay", capacity: float = 1000.0) -> dict:
        """Düğüm ekle."""
        return self._request("POST", "/nodes", json={
            "id": node_id,
            "type": node_type,
            "capacity_mbps": capacity,
        })

    def get_node(self, node_id: str) -> dict:
        """Düğüm bilgisi getir."""
        return self._request("GET", f"/nodes/{node_id}")

    def remove_node(self, node_id: str) -> dict:
        """Düğüm kaldır."""
        return self._request("DELETE", f"/nodes/{node_id}")

    def list_links(self) -> list:
        """Tüm bağlantıları listele."""
        return self._request("GET", "/links")

    def add_link(self, from_id: str, to_id: str, bandwidth: float = 1000.0, latency: float = 2.0) -> dict:
        """Bağlantı ekle."""
        return self._request("POST", "/links", json={
            "from": from_id,
            "to": to_id,
            "bandwidth_mbps": bandwidth,
            "latency_ms": latency,
        })

    def update_link(self, link_id: str, used_bw: float = None, active: bool = None) -> dict:
        """Bağlantı güncelle."""
        body = {}
        if used_bw is not None:
            body["used_bw_mbps"] = used_bw
        if active is not None:
            body["active"] = active
        return self._request("PUT", f"/links/{link_id}", json=body)

    # ── Yönlendirme ──

    def find_route(self, from_id: str, to_id: str, qos: int = 3) -> dict:
        """Tıkanıklık-farkında rota bul."""
        return self._request("POST", "/route", json={
            "from": from_id,
            "to": to_id,
            "qos": qos,
        })

    def find_alternate_routes(self, from_id: str, to_id: str, qos: int = 3, count: int = 3) -> list:
        """Alternatif rotalar bul."""
        return self._request("POST", "/routes/alternate", json={
            "from": from_id,
            "to": to_id,
            "qos": qos,
            "count": count,
        })

    # ── Akış Yönetimi ──

    def list_flows(self, active_only: bool = False) -> list:
        """Akışları listele."""
        params = "?active=true" if active_only else ""
        return self._request("GET", f"/flows{params}")

    def create_flow(self, from_id: str, to_id: str, qos: int = 3,
                    bw: float = 100.0, data_type: str = "sensor") -> dict:
        """Yeni akış oluştur."""
        return self._request("POST", "/flows", json={
            "from": from_id,
            "to": to_id,
            "qos": qos,
            "bw_mbps": bw,
            "data_type": data_type,
        })

    def update_flow(self, flow_id: str, bytes_sent: int = 0, packets: int = 0) -> dict:
        """Akış trafiğini güncelle."""
        return self._request("PUT", f"/flows/{flow_id}", json={
            "bytes": bytes_sent,
            "packets": packets,
        })

    def close_flow(self, flow_id: str) -> dict:
        """Akışı kapat."""
        return self._request("DELETE", f"/flows/{flow_id}")

    def allocate_bandwidth(self, total_bw: float = 1000.0) -> dict:
        """Bant genişliği yeniden tahsis et."""
        return self._request("POST", "/flows/allocate", json={
            "total_bw_mbps": total_bw,
        })

    def expire_flows(self) -> dict:
        """Süresi dolan akışları kapat."""
        return self._request("POST", "/flows/expire")

    # ── Yük Dengeleme ──

    def get_balancer(self) -> dict:
        """Yük dengeleyici durumu."""
        return self._request("GET", "/balancer")

    def select_backend(self) -> dict:
        """En uygun backend seç."""
        return self._request("POST", "/balancer/select")

    def list_backends(self) -> list:
        """Backend listesi."""
        return self._request("GET", "/balancer/backends")

    def add_backend(self, backend_id: str, address: str, weight: int = 10) -> dict:
        """Backend ekle."""
        return self._request("POST", "/balancer/backends", json={
            "id": backend_id,
            "address": address,
            "weight": weight,
        })

    def set_strategy(self, strategy: str) -> dict:
        """Yük dengeleme stratejisi değiştir."""
        return self._request("PUT", "/balancer/strategy", json={
            "strategy": strategy,
        })

    # ── İzleme ──

    def get_snapshots(self) -> list:
        """Link snapshot'ları."""
        return self._request("GET", "/monitor/snapshots")

    def get_alerts(self, n: int = 100) -> list:
        """Tıkanıklık alarmları."""
        return self._request("GET", f"/monitor/alerts?n={n}")

    def get_congestion_report(self) -> dict:
        """Tıkanıklık raporu."""
        return self._request("GET", "/monitor/congestion")

    def get_link_history(self, link_id: str, n: int = 60) -> list:
        """Link geçmişi."""
        return self._request("GET", f"/monitor/history/{link_id}?n={n}")

    def get_stats(self) -> dict:
        """Tüm istatistikler."""
        return self._request("GET", "/stats")
