"""TANI modülü simülasyon istemcisi — Nesne Tespit ve Sınıflandırma."""

from __future__ import annotations

import random
import time

from src.clients.base import BaseClient

# Simülasyon senaryoları için nesne sınıfları
MILITARY_CLASSES = [
    "tank", "apc", "truck", "humvee",
    "helicopter", "fighter_jet", "uav",
    "soldier", "sniper", "civilian",
    "radar", "sam_launcher", "artillery", "command_post",
]

# Kaynak ID'leri (sensör/kamera)
SOURCES = ["cam-north-01", "cam-south-02", "uav-kartal-eye", "radar-dome-01", "sat-img-01"]


class TaniClient(BaseClient):
    """TANI — Nesne Tespit ve Sınıflandırma istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9113):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "TANI"

    # ── Sağlık & Durum ──

    def health(self) -> dict:
        return self._request("GET", "/health")

    def status(self) -> dict:
        return self._request("GET", "/status")

    # ── Tespit ──

    def detect(self, frame_id: str = "", width: int = 640, height: int = 640,
               source_id: str = "") -> dict:
        """Bir kare (frame) üzerinde nesne tespiti yap."""
        return self._request("POST", "/detect", json={
            "frame_id": frame_id,
            "width": width,
            "height": height,
            "source_id": source_id,
        })

    def detect_last(self) -> dict:
        """Son tespit sonucunu getir."""
        return self._request("GET", "/detect/last")

    # ── Sınıflandırma ──

    def classify(self, detections: list[dict]) -> dict:
        """Tespitleri sınıflandır (IFF, tehdit seviyesi)."""
        return self._request("POST", "/classify", json={"detections": detections})

    def classify_stats(self) -> dict:
        return self._request("GET", "/classify/stats")

    # ── İzleme ──

    def tracks(self) -> dict:
        """Aktif izleri getir."""
        return self._request("GET", "/tracks")

    def hostile_tracks(self) -> dict:
        """Düşman izleri getir."""
        return self._request("GET", "/tracks/hostile")

    def track_by_id(self, track_id: str) -> dict:
        return self._request("GET", f"/tracks/{track_id}")

    def track_stats(self) -> dict:
        return self._request("GET", "/tracks/stats")

    # ── Tam Pipeline ──

    def process(self, frame_id: str = "", width: int = 640, height: int = 640,
                source_id: str = "") -> dict:
        """Tam pipeline: tespit → sınıflandırma → izleme."""
        return self._request("POST", "/process", json={
            "frame_id": frame_id,
            "width": width,
            "height": height,
            "source_id": source_id,
        })

    # ── IFF ──

    def iff_register(self, transponder_id: str) -> dict:
        """Dost transponder kaydı."""
        return self._request("POST", "/iff/register", json={"id": transponder_id})

    def iff_unregister(self, transponder_id: str) -> dict:
        """Dost transponder kaydını sil."""
        return self._request("POST", "/iff/unregister", json={"id": transponder_id})

    # ── İstatistik ──

    def stats(self) -> dict:
        """Tüm alt sistem istatistikleri."""
        return self._request("GET", "/stats")

    # ── Simülasyon ──

    def simulate_detection_scenario(self, frames: int = 10) -> dict:
        """Askeri alan tarama simülasyonu.

        Birden fazla sensörden gelen kare verilerini işler. Her karede rastgele
        nesneler tespit edilir, sınıflandırılır ve izlenir. Sonuçlar:
        - Toplam tespit sayısı
        - Düşman hedef sayısı
        - Aktif iz sayısı
        - Ortalama çıkarım süresi
        """
        results = {
            "frames_processed": 0,
            "total_detections": 0,
            "hostile_detected": 0,
            "active_tracks": 0,
            "avg_inference_ms": 0.0,
            "errors": 0,
        }

        inference_times = []

        for i in range(frames):
            source = random.choice(SOURCES)
            frame_id = f"sim-{source}-{int(time.time())}-{i:04d}"

            resp = self.process(
                frame_id=frame_id,
                width=random.choice([640, 1280]),
                height=random.choice([640, 720]),
                source_id=source,
            )

            if resp and "error" not in resp:
                results["frames_processed"] += 1
                results["total_detections"] += resp.get("detect_count", 0)
                results["hostile_detected"] += sum(
                    1 for c in resp.get("classified", [])
                    if c.get("allegiance") == "hostile"
                )
                if resp.get("inference_ms"):
                    inference_times.append(resp["inference_ms"])
            else:
                results["errors"] += 1

        if inference_times:
            results["avg_inference_ms"] = round(
                sum(inference_times) / len(inference_times), 2
            )

        # Son durum
        trk = self.tracks()
        if trk and "count" in trk:
            results["active_tracks"] = trk["count"]

        hostile = self.hostile_tracks()
        if hostile and "count" in hostile:
            results["hostile_active"] = hostile["count"]

        return results
