"""
BÜTÜN - Auto Scaler (Otomatik Ölçekleyici)

Yatay (horizontal) ve dikey (vertical) ölçekleme kararlarını,
kaynak kullanım metriklerine göre otomatik olarak alır.
"""

from __future__ import annotations

import asyncio
import time
import structlog

from src.models.service import ServiceInfo, ScalePolicy, ResourceUsage
from src.service_mesh.registry import ServiceRegistry

logger = structlog.get_logger(__name__)


class ScaleDecision:
    """Ölçekleme kararı."""
    __slots__ = ("service_name", "direction", "current_replicas", "target_replicas", "reason", "timestamp")

    def __init__(self, service_name: str, direction: str, current: int, target: int, reason: str):
        self.service_name = service_name
        self.direction = direction  # "up" | "down" | "none"
        self.current_replicas = current
        self.target_replicas = target
        self.reason = reason
        self.timestamp = time.time()


class AutoScaler:
    """
    Otomatik ölçekleyici.

    CPU ve bellek kullanım yüzdelerine göre replika sayısını artırır/azaltır.
    Kubernetes HPA (Horizontal Pod Autoscaler) mantığını taklit eder.
    """

    def __init__(self, registry: ServiceRegistry, check_interval: float = 10.0):
        self._registry = registry
        self._check_interval = check_interval
        self._last_scale_time: dict[str, float] = {}
        self._scale_history: list[ScaleDecision] = []
        self._monitor_task: asyncio.Task | None = None

    def evaluate(self, service: ServiceInfo) -> ScaleDecision:
        """Servis için ölçekleme kararı al."""
        policy = service.scale_policy
        usage = service.resource_usage
        now = time.time()

        # Cooldown kontrolü
        last_scale = self._last_scale_time.get(service.name, 0)
        if now - last_scale < policy.cooldown_sec:
            return ScaleDecision(service.name, "none", service.replicas, service.replicas, "cooldown_aktif")

        # Yukarı ölçekleme
        if usage.cpu_percent > policy.scale_up_threshold:
            if service.replicas < policy.max_replicas:
                target = min(service.replicas + 1, policy.max_replicas)
                decision = ScaleDecision(
                    service.name, "up", service.replicas, target,
                    f"CPU {usage.cpu_percent:.1f}% > eşik {policy.scale_up_threshold:.1f}%"
                )
                logger.info("ölçekleme_yukarı", service=service.name, target=target, reason=decision.reason)
                return decision

        # Aşağı ölçekleme
        if usage.cpu_percent < policy.scale_down_threshold:
            if service.replicas > policy.min_replicas:
                target = max(service.replicas - 1, policy.min_replicas)
                decision = ScaleDecision(
                    service.name, "down", service.replicas, target,
                    f"CPU {usage.cpu_percent:.1f}% < eşik {policy.scale_down_threshold:.1f}%"
                )
                logger.info("ölçekleme_aşağı", service=service.name, target=target, reason=decision.reason)
                return decision

        return ScaleDecision(service.name, "none", service.replicas, service.replicas, "metrikler_normal")

    def apply_decision(self, decision: ScaleDecision) -> bool:
        """Ölçekleme kararını uygula."""
        if decision.direction == "none":
            return False

        service = self._registry.get_service(decision.service_name)
        if not service:
            return False

        service.replicas = decision.target_replicas
        self._last_scale_time[decision.service_name] = time.time()
        self._scale_history.append(decision)

        logger.info(
            "ölçekleme_uygulandı",
            service=decision.service_name,
            direction=decision.direction,
            replicas=decision.target_replicas,
        )
        return True

    def get_history(self, service_name: str | None = None, limit: int = 50) -> list[ScaleDecision]:
        """Ölçekleme geçmişini getir."""
        history = self._scale_history
        if service_name:
            history = [d for d in history if d.service_name == service_name]
        return history[-limit:]

    async def start_monitor(self) -> None:
        """Otomatik ölçekleme izleme döngüsünü başlat."""
        self._monitor_task = asyncio.create_task(self._monitor_loop())
        logger.info("otomatik_ölçekleyici_başlatıldı", interval=self._check_interval)

    async def stop_monitor(self) -> None:
        """İzleme döngüsünü durdur."""
        if self._monitor_task:
            self._monitor_task.cancel()
            try:
                await self._monitor_task
            except asyncio.CancelledError:
                pass

    async def _monitor_loop(self) -> None:
        """Periyodik ölçekleme kontrolü."""
        while True:
            await asyncio.sleep(self._check_interval)
            for service in self._registry.services.values():
                if service.is_alive:
                    decision = self.evaluate(service)
                    if decision.direction != "none":
                        self.apply_decision(decision)
