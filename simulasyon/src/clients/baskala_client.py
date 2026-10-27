"""BAŞKALA modülü simülasyon istemcisi — Uydu Değişim Tespiti."""

from __future__ import annotations

from src.clients.base import BaseClient


class BaskalaClient(BaseClient):
    """BAŞKALA — Uydu Fotoğraflarındaki Değişimleri Bulma istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9122):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "BAŞKALA"

    # ── Sağlık & Durum ──

    def health(self) -> dict:
        return self._request("GET", "/health")

    def status(self) -> dict:
        return self._request("GET", "/status")

    def stats(self) -> dict:
        return self._request("GET", "/stats")

    # ── Görüntü Yönetimi ──

    def get_images(self) -> dict:
        return self._request("GET", "/images")

    def ingest_image(self, image: dict) -> dict:
        return self._request("POST", "/images", json=image)

    def get_image(self, image_id: str) -> dict:
        return self._request("GET", f"/images/{image_id}")

    def delete_image(self, image_id: str) -> dict:
        return self._request("DELETE", f"/images/{image_id}")

    # ── Karşılaştırma ve Analiz ──

    def compare(self, image_a_id: str, image_b_id: str) -> dict:
        return self._request("POST", "/compare", json={
            "image_a_id": image_a_id,
            "image_b_id": image_b_id,
        })

    def analyze(self, lat: float, lon: float, radius_km: float) -> dict:
        return self._request("POST", "/analyze", json={
            "lat": lat, "lon": lon, "radius_km": radius_km,
        })

    # ── Simülasyon Entegrasyonu ──

    def process(self, entities: list[dict]) -> dict:
        """Toplu işleme: sentetik SAR+Optik üret → karşılaştır → füzyon."""
        return self._request("POST", "/process", json={"entities": entities})

    # ── Değişimler ──

    def get_changes(self) -> dict:
        return self._request("GET", "/changes")

    def get_change(self, change_id: str) -> dict:
        return self._request("GET", f"/changes/{change_id}")

    def get_threats(self) -> dict:
        return self._request("GET", "/threats")

    def get_timeline(self) -> dict:
        return self._request("GET", "/timeline")

    # ── Bakım ──

    def prune(self) -> dict:
        return self._request("GET", "/prune")
