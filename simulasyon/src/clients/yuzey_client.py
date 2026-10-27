"""YÜZEY modülü HTTP istemcisi."""

from __future__ import annotations

from src.clients.base import BaseClient


class YuzeyClient(BaseClient):
    """YÜZEY — 3D Taktiksel Arayüz istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9103):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "YÜZEY"

    # ── Sağlık & İstatistik ──

    def health(self) -> dict:
        """Sistem sağlık durumu."""
        return self._request("GET", "/health")

    def stats(self) -> dict:
        """Genel istatistikler (hub, events, layers)."""
        return self._request("GET", "/stats")

    # ── Taktiksel Resim ──

    def tactical(self) -> dict:
        """Tam taktiksel resim."""
        return self._request("GET", "/tactical")

    def tactical_picture(self) -> dict:
        """Katmanlar ve olaylar dahil genişletilmiş taktiksel resim."""
        return self._request("GET", "/tactical/picture")

    # ── Modüller ──

    def modules(self) -> dict:
        """Tüm modül durumları."""
        return self._request("GET", "/modules")

    def module_detail(self, module_id: str) -> dict:
        """Belirli modülün durumu."""
        return self._request("GET", f"/modules/{module_id}")

    def module_stats(self, module_id: str) -> dict:
        """Modülün /stats endpoint'ini çek."""
        return self._request("GET", f"/modules/{module_id}/stats")

    # ── Varlıklar ──

    def entities(self, faction: str = "") -> list:
        """Varlık listesi, opsiyonel taraf filtresi."""
        q = f"?faction={faction}" if faction else ""
        return self._request("GET", f"/entities{q}")

    def entity_detail(self, entity_id: str) -> dict:
        """Belirli varlık bilgisi."""
        return self._request("GET", f"/entities/{entity_id}")

    def update_entities(self, entities: list) -> dict:
        """Toplu varlık güncellemesi."""
        return self._request("POST", "/entities/update", json=entities)

    def clear_entities(self) -> dict:
        """Tüm varlıkları temizle."""
        return self._request("DELETE", "/entities")

    def entity_factions(self) -> dict:
        """Taraf bazlı varlık sayıları."""
        return self._request("GET", "/entities/factions")

    # ── Katmanlar ──

    def layers(self) -> list:
        """Tüm katmanlar."""
        return self._request("GET", "/layers")

    def layer_detail(self, layer_id: str) -> dict:
        """Belirli katman."""
        return self._request("GET", f"/layers/{layer_id}")

    def update_layer(self, layer_id: str, visible: bool = None, opacity: float = None) -> dict:
        """Katman güncelleme."""
        body = {}
        if visible is not None:
            body["visible"] = visible
        if opacity is not None:
            body["opacity"] = opacity
        return self._request("PUT", f"/layers/{layer_id}", json=body)

    def push_layer_data(self, layer: str, markers: list = None, zones: list = None,
                        lines: list = None, mode: str = "set") -> dict:
        """Katmana veri gönder (marker, zone, line)."""
        body = {"layer": layer, "mode": mode}
        if markers is not None:
            body["markers"] = markers
        if zones is not None:
            body["zones"] = zones
        if lines is not None:
            body["lines"] = lines
        return self._request("POST", "/layers/update", json=body)

    # ── Olaylar ──

    def events(self, n: int = 50, source: str = "", severity: str = "", event_type: str = "") -> list:
        """Olay listesi, filtreli."""
        params = [f"n={n}"]
        if source:
            params.append(f"source={source}")
        if severity:
            params.append(f"severity={severity}")
        if event_type:
            params.append(f"type={event_type}")
        return self._request("GET", f"/events?{'&'.join(params)}")

    def emit_event(self, event_type: str, source: str, severity: str, title: str,
                   detail: str = "", lat: float = 0, lon: float = 0) -> dict:
        """Olay yayınla."""
        return self._request("POST", "/events/emit", json={
            "type": event_type, "source": source, "severity": severity,
            "title": title, "detail": detail, "lat": lat, "lon": lon,
        })

    def event_stats(self) -> dict:
        """Olay istatistikleri."""
        return self._request("GET", "/events/stats")

    # ── Hub ──

    def hub_stats(self) -> dict:
        """Hub istatistikleri."""
        return self._request("GET", "/hub/stats")

    def hub_summary(self) -> dict:
        """Hub özet."""
        return self._request("GET", "/hub/summary")

    def hub_poll(self, module_id: str) -> dict:
        """Belirli modülü hub üzerinden yokla."""
        return self._request("GET", f"/hub/poll/{module_id}")
