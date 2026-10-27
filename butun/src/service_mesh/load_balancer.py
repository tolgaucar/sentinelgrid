"""
BÜTÜN - Load Balancer (Yük Dengeleyici)

Gelen istekleri servislere dağıtan yük dengeleme stratejileri.
Round Robin, Least Connections ve Weighted stratejilerini destekler.
"""

from __future__ import annotations

import structlog
from collections import defaultdict

from src.models.service import ServiceInfo, LoadBalanceStrategy
from src.service_mesh.registry import ServiceRegistry

logger = structlog.get_logger(__name__)


class LoadBalancer:
    """
    Yük dengeleyici.

    AKIM modülü ile entegre çalışarak veri trafiğini uygun uç noktalara yönlendirir.
    """

    def __init__(self, registry: ServiceRegistry, strategy: LoadBalanceStrategy = LoadBalanceStrategy.ROUND_ROBIN):
        self._registry = registry
        self._strategy = strategy
        self._rr_counters: dict[str, int] = defaultdict(int)
        self._connections: dict[str, int] = defaultdict(int)

    @property
    def strategy(self) -> LoadBalanceStrategy:
        return self._strategy

    @strategy.setter
    def strategy(self, value: LoadBalanceStrategy) -> None:
        self._strategy = value
        logger.info("strateji_değiştirildi", strategy=value.value)

    def select(self, service_name: str) -> ServiceInfo | None:
        """Stratejiye göre hedef servis seç."""
        candidates = self._get_healthy_instances(service_name)
        if not candidates:
            logger.warning("aday_servis_yok", name=service_name)
            return None

        if self._strategy == LoadBalanceStrategy.ROUND_ROBIN:
            return self._round_robin(service_name, candidates)
        elif self._strategy == LoadBalanceStrategy.LEAST_CONNECTIONS:
            return self._least_connections(candidates)
        elif self._strategy == LoadBalanceStrategy.WEIGHTED:
            return self._weighted(candidates)

        return candidates[0]

    def report_connection_start(self, service_name: str) -> None:
        """Bağlantı başladığını raporla (least_connections için)."""
        self._connections[service_name] += 1

    def report_connection_end(self, service_name: str) -> None:
        """Bağlantı sonlandığını raporla."""
        if self._connections[service_name] > 0:
            self._connections[service_name] -= 1

    def get_connection_count(self, service_name: str) -> int:
        return self._connections[service_name]

    def _get_healthy_instances(self, service_name: str) -> list[ServiceInfo]:
        """Aktif servis örneklerini getir."""
        service = self._registry.get_service(service_name)
        if service and service.is_alive:
            return [service]
        return []

    def _round_robin(self, service_name: str, candidates: list[ServiceInfo]) -> ServiceInfo:
        """Round Robin seçimi."""
        idx = self._rr_counters[service_name] % len(candidates)
        self._rr_counters[service_name] += 1
        return candidates[idx]

    def _least_connections(self, candidates: list[ServiceInfo]) -> ServiceInfo:
        """En az bağlantıya sahip servisi seç."""
        return min(candidates, key=lambda s: self._connections[s.name])

    def _weighted(self, candidates: list[ServiceInfo]) -> ServiceInfo:
        """Ağırlıklı seçim."""
        total_weight = sum(c.weight for c in candidates)
        if total_weight == 0:
            return candidates[0]

        import random
        r = random.randint(1, total_weight)
        cumulative = 0
        for candidate in candidates:
            cumulative += candidate.weight
            if r <= cumulative:
                return candidate
        return candidates[-1]
