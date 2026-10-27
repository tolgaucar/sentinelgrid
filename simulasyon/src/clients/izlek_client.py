"""İZLEK modülü HTTP istemcisi — Rota Tahmini."""

from __future__ import annotations

from src.clients.base import BaseClient


class IzlekClient(BaseClient):
    """İZLEK — Rota Tahmini istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9126):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "İZLEK"

    # ── Sağlık & İstatistik ──

    def health(self) -> dict:
        return self._request("GET", "/health")

    def stats(self) -> dict:
        return self._request("GET", "/stats")

    # ── İz Yönetimi ──

    def update_tracks(self, entities: list) -> dict:
        """Varlık konumlarını toplu güncelle."""
        return self._request("POST", "/tracks/update", json=entities)

    def list_tracks(self) -> dict:
        """Aktif izleri listele."""
        return self._request("GET", "/tracks")

    def get_track(self, track_id: str) -> dict:
        """Belirli izi getir."""
        return self._request("GET", f"/tracks/{track_id}")

    def clear_tracks(self) -> dict:
        """Tüm izleri temizle."""
        return self._request("DELETE", "/tracks")

    # ── Tahmin ──

    def predict(self, entity_id: str, horizon_sec: float = 60) -> dict:
        """Tek varlık için rota tahmini."""
        return self._request("POST", "/predict", json={
            "entity_id": entity_id,
            "horizon_sec": horizon_sec,
        })

    def predict_batch(self, entity_ids: list, horizon_sec: float = 60) -> dict:
        """Toplu rota tahmini."""
        return self._request("POST", "/predict/batch", json={
            "entity_ids": entity_ids,
            "horizon_sec": horizon_sec,
        })

    # ── Önleme Atışı ──

    def intercept(self, entity_id: str, projectile_speed: float,
                  launcher_lat: float, launcher_lon: float) -> dict:
        """Önleme atışı koordinatı hesapla."""
        return self._request("POST", "/intercept", json={
            "entity_id": entity_id,
            "projectile_speed": projectile_speed,
            "launcher_lat": launcher_lat,
            "launcher_lon": launcher_lon,
        })
