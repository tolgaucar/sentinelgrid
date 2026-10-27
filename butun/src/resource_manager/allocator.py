"""
BÜTÜN - Resource Allocator (Kaynak Dağıtıcı)

Servislere CPU ve bellek kotalarını adil şekilde dağıtan kaynak
tahsis motoru. Öncelik bazlı ve ağırlıklı dağıtım destekler.
"""

from __future__ import annotations

import structlog
from dataclasses import dataclass, field

logger = structlog.get_logger(__name__)


@dataclass
class ResourceQuota:
    """Bir servis için ayrılan kaynak kotası."""
    service_name: str
    cpu_shares: int = 100       # Göreli CPU payı (0-1000)
    memory_limit_mb: int = 512  # Maksimum bellek (MB)
    priority: int = 5           # Öncelik (1=en yüksek, 10=en düşük)
    is_critical: bool = False   # Kritik servis mi (kapatılamaz)


class ResourceAllocator:
    """
    Kaynak tahsis motoru.

    Kubernetes'in kaynak yönetim mantığını (requests/limits) taklit ederek
    her servise adil CPU ve bellek payı dağıtır.
    """

    def __init__(self, total_cpu_shares: int = 10000, total_memory_mb: int = 8192):
        self._total_cpu = total_cpu_shares
        self._total_memory = total_memory_mb
        self._quotas: dict[str, ResourceQuota] = {}
        self._allocated_cpu = 0
        self._allocated_memory = 0

    @property
    def available_cpu(self) -> int:
        return self._total_cpu - self._allocated_cpu

    @property
    def available_memory(self) -> int:
        return self._total_memory - self._allocated_memory

    def allocate(self, quota: ResourceQuota) -> bool:
        """Servise kaynak tahsis et."""
        if quota.cpu_shares > self.available_cpu:
            logger.warning("cpu_yetersiz", service=quota.service_name, requested=quota.cpu_shares, available=self.available_cpu)
            return False

        if quota.memory_limit_mb > self.available_memory:
            logger.warning("bellek_yetersiz", service=quota.service_name, requested=quota.memory_limit_mb, available=self.available_memory)
            return False

        self._quotas[quota.service_name] = quota
        self._allocated_cpu += quota.cpu_shares
        self._allocated_memory += quota.memory_limit_mb

        logger.info(
            "kaynak_tahsis_edildi",
            service=quota.service_name,
            cpu_shares=quota.cpu_shares,
            memory_mb=quota.memory_limit_mb,
            priority=quota.priority,
        )
        return True

    def release(self, service_name: str) -> bool:
        """Servisten kaynak geri al."""
        if service_name not in self._quotas:
            return False

        quota = self._quotas.pop(service_name)
        self._allocated_cpu -= quota.cpu_shares
        self._allocated_memory -= quota.memory_limit_mb

        logger.info("kaynak_serbest_bırakıldı", service=service_name)
        return True

    def get_quota(self, service_name: str) -> ResourceQuota | None:
        return self._quotas.get(service_name)

    def get_all_quotas(self) -> dict[str, ResourceQuota]:
        return dict(self._quotas)

    def rebalance(self) -> dict[str, ResourceQuota]:
        """
        Tüm servislerin kaynaklarını öncelik bazında yeniden dengele.

        Yüksek öncelikli (düşük sayı) servislere daha fazla pay verir.
        """
        if not self._quotas:
            return {}

        # Önceliğe göre sırala
        sorted_quotas = sorted(self._quotas.values(), key=lambda q: q.priority)

        total_priority_inv = sum(1.0 / q.priority for q in sorted_quotas)

        for quota in sorted_quotas:
            weight = (1.0 / quota.priority) / total_priority_inv
            quota.cpu_shares = int(self._total_cpu * weight * 0.8)  # %80 dağıtılabilir
            quota.memory_limit_mb = int(self._total_memory * weight * 0.8)

        # Toplam tahsisi güncelle
        self._allocated_cpu = sum(q.cpu_shares for q in self._quotas.values())
        self._allocated_memory = sum(q.memory_limit_mb for q in self._quotas.values())

        logger.info("kaynaklar_yeniden_dengelendi", service_count=len(self._quotas))
        return dict(self._quotas)

    def get_utilization_summary(self) -> dict:
        """Kaynak kullanım özeti."""
        return {
            "total_cpu_shares": self._total_cpu,
            "allocated_cpu_shares": self._allocated_cpu,
            "available_cpu_shares": self.available_cpu,
            "cpu_utilization_percent": (self._allocated_cpu / self._total_cpu * 100) if self._total_cpu > 0 else 0,
            "total_memory_mb": self._total_memory,
            "allocated_memory_mb": self._allocated_memory,
            "available_memory_mb": self.available_memory,
            "memory_utilization_percent": (self._allocated_memory / self._total_memory * 100) if self._total_memory > 0 else 0,
            "service_count": len(self._quotas),
        }
