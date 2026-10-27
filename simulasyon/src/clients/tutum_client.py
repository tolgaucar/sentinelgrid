"""TUTUM modülü simülasyon istemcisi — Batarya ve Kaynak Tüketimi İzleyicisi."""

from __future__ import annotations

import random

from src.clients.base import BaseClient

# Platform tipleri
PLATFORM_TYPES = ["iha", "otonom", "radyo", "sensor", "röle"]

# Arazi tipleri ve enerji çarpanları
TERRAIN_TYPES = {
    "düz": 1.0,
    "engebeli": 1.35,
    "dağlık": 1.70,
    "kentsel": 1.15,
}


class TutumClient(BaseClient):
    """TUTUM — Batarya ve Kaynak Tüketimi İzleyicisi istemcisi."""

    def __init__(self, host: str = "http://127.0.0.1", port: int = 9111):
        super().__init__(f"{host}:{port}")
        self.MODULE_NAME = "TUTUM"

    # ── Sağlık & Durum ──

    def health(self) -> dict:
        return self._request("GET", "/health")

    def status(self) -> dict:
        return self._request("GET", "/status")

    # ── Platform Yönetimi ──

    def list_platforms(self) -> dict:
        return self._request("GET", "/platforms")

    def register_platform(self, id: str, name: str, ptype: str, capacity_wh: float) -> dict:
        return self._request("POST", "/platforms", json={
            "id": id, "name": name, "type": ptype, "capacity_wh": capacity_wh,
        })

    def get_platform(self, platform_id: str) -> dict:
        return self._request("GET", f"/platforms/{platform_id}")

    def remove_platform(self, platform_id: str) -> dict:
        return self._request("DELETE", f"/platforms/{platform_id}")

    # ── Batarya ──

    def battery_sample(self, platform_id: str, voltage: float, current: float,
                       temp_c: float, terrain: str = "", wind_ms: float = 0,
                       load_factor: float = 1.0) -> dict:
        body = {
            "platform_id": platform_id,
            "voltage_v": voltage,
            "current_a": current,
            "temp_c": temp_c,
        }
        if terrain:
            body["terrain"] = terrain
            body["wind_speed_ms"] = wind_ms
            body["load_factor"] = load_factor
        return self._request("POST", "/battery/sample", json=body)

    def battery_stats(self) -> dict:
        return self._request("GET", "/battery/stats")

    def battery_critical(self) -> dict:
        return self._request("GET", "/battery/critical")

    # ── Kaynak İzleme ──

    def resource_metric(self, platform_id: str, cpu: float = None,
                        memory: float = None, disk: float = None,
                        temp_c: float = None) -> dict:
        body = {"platform_id": platform_id}
        if cpu is not None:
            body["cpu_percent"] = cpu
        if memory is not None:
            body["memory_percent"] = memory
        if disk is not None:
            body["disk_percent"] = disk
        if temp_c is not None:
            body["temp_c"] = temp_c
        return self._request("POST", "/resource/metric", json=body)

    def resource_stats(self) -> dict:
        return self._request("GET", "/resource/stats")

    # ── Tahmin ──

    def predict(self, platform_id: str) -> dict:
        return self._request("POST", "/predict", json={"platform_id": platform_id})

    def check_mission(self, platform_id: str, mission_minutes: float) -> dict:
        return self._request("POST", "/predict/mission", json={
            "platform_id": platform_id,
            "mission_minutes": mission_minutes,
        })

    # ── Uyarılar ──

    def alerts(self, severity: str = None, platform_id: str = None) -> dict:
        params = []
        if severity:
            params.append(f"severity={severity}")
        if platform_id:
            params.append(f"platform_id={platform_id}")
        qs = "?" + "&".join(params) if params else ""
        return self._request("GET", f"/alerts{qs}")

    def alert_ack(self, alert_id: str) -> dict:
        return self._request("POST", "/alerts/ack", json={"alert_id": alert_id})

    def alert_resolve(self, alert_id: str) -> dict:
        return self._request("POST", "/alerts/resolve", json={"alert_id": alert_id})

    def alert_stats(self) -> dict:
        return self._request("GET", "/alerts/stats")

    # ── RTB & Görev Devri ──

    def rtb_recommendations(self) -> dict:
        return self._request("GET", "/rtb")

    def handoff_recommendations(self) -> dict:
        return self._request("GET", "/handoff")

    # ── Simülasyon ──

    def simulate_battery_drain(self, platform_id: str = "iha-kartal-01",
                                rounds: int = 15) -> dict:
        """Batarya boşalma simülasyonu — azalan voltaj ve akım ile.

        İHA'nın uçuş sırasında yaşadığı gerçekçi boşalma eğrisini taklit eder.
        Her turda voltaj düşer, akım (tüketim) değişir, arazi/rüzgar koşulları
        rastgele güncellenir.
        """
        results = {"samples": 0, "alerts_triggered": 0, "errors": 0}

        base_voltage = 11.1 + random.uniform(0, 1.5)  # 11.1-12.6V Li-Po 3S
        base_current = -3.0 - random.uniform(0, 2.0)   # -3A ile -5A arası

        for i in range(rounds):
            # Her turda voltaj düşer
            voltage = base_voltage - (i * 0.4) + random.uniform(-0.1, 0.1)
            voltage = max(voltage, 9.0)  # Min voltaj

            # Akım (tüketim) rüzgar/manevra ile değişir
            current = base_current - random.uniform(-0.5, 0.5)

            # Sıcaklık: yüksekliğe göre
            temp = 25 + random.uniform(-5, 15)

            # Arazi ve rüzgar rastgele
            terrain = random.choice(list(TERRAIN_TYPES.keys()))
            wind = random.uniform(0, 15)
            load = random.uniform(0.8, 1.5)

            resp = self.battery_sample(
                platform_id, voltage, current, temp,
                terrain=terrain, wind_ms=wind, load_factor=load,
            )
            if resp and "error" not in resp:
                results["samples"] += 1
                if resp.get("status") in ("critical", "warning"):
                    results["alerts_triggered"] += 1
            else:
                results["errors"] += 1

        return results
