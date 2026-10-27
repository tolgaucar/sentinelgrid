"""
BÜTÜN - Service Scheduler (Servis Zamanlayıcı)

Servislerin hangi sırada ve hangi koşullarda başlatılacağını belirleyen
zamanlama ve önceliklendirme mantığı.
"""

from __future__ import annotations

import asyncio
import structlog
from collections import defaultdict

from src.models.service import ServiceInfo, ClusterInfo
from src.service_mesh.registry import ServiceRegistry

logger = structlog.get_logger(__name__)


class ServiceScheduler:
    """
    Servis zamanlayıcı.

    Bağımlılık grafiğine ve faz sırasına göre servislerin optimal
    başlatma sırasını hesaplar (topolojik sıralama).
    """

    def __init__(self, registry: ServiceRegistry):
        self._registry = registry
        self._clusters: dict[str, ClusterInfo] = {}
        self._schedule_queue: list[ServiceInfo] = []

    def register_cluster(self, cluster: ClusterInfo) -> None:
        """Küme kaydı ekle."""
        self._clusters[cluster.name] = cluster
        logger.info("küme_kaydedildi", name=cluster.name, phase=cluster.phase)

    def get_cluster(self, name: str) -> ClusterInfo | None:
        return self._clusters.get(name)

    def get_clusters_by_phase(self) -> list[ClusterInfo]:
        """Kümeleri faz sırasına göre getir."""
        return sorted(self._clusters.values(), key=lambda c: c.phase)

    def compute_startup_order(self, services: list[ServiceInfo]) -> list[ServiceInfo]:
        """
        Bağımlılıklara göre topolojik başlatma sırası hesapla.

        Kahn algoritması ile topolojik sıralama yapar. Döngüsel bağımlılık
        tespit edilirse hata döner.
        """
        name_to_service = {s.name: s for s in services}
        in_degree: dict[str, int] = defaultdict(int)
        graph: dict[str, list[str]] = defaultdict(list)

        for s in services:
            if s.name not in in_degree:
                in_degree[s.name] = 0
            for dep in s.dependencies:
                graph[dep].append(s.name)
                in_degree[s.name] += 1

        # Bağımlılığı olmayan servisleri kuyruğa ekle
        queue = [name for name, deg in in_degree.items() if deg == 0]
        ordered: list[ServiceInfo] = []

        while queue:
            # Faz sırasına göre önceliklendir
            queue.sort(key=lambda n: self._get_phase_priority(name_to_service.get(n)))
            current = queue.pop(0)

            if current in name_to_service:
                ordered.append(name_to_service[current])

            for neighbor in graph[current]:
                in_degree[neighbor] -= 1
                if in_degree[neighbor] == 0:
                    queue.append(neighbor)

        if len(ordered) != len(services):
            cycle_nodes = [s.name for s in services if s.name not in {o.name for o in ordered}]
            logger.error("döngüsel_bağımlılık_tespit_edildi", nodes=cycle_nodes)
            raise ValueError(f"Döngüsel bağımlılık tespit edildi: {cycle_nodes}")

        return ordered

    def _get_phase_priority(self, service: ServiceInfo | None) -> int:
        """Servisin faz önceliğini hesapla."""
        if not service:
            return 999

        for cluster in self._clusters.values():
            if service.cluster == cluster.name:
                return cluster.phase
        return 999

    async def schedule_batch(self, services: list[ServiceInfo]) -> list[str]:
        """
        Toplu servis zamanlaması yap.

        Servisleri bağımlılık sırasına göre dizilime alır ve başlatılabilir
        olanların listesini döndürür.
        """
        ordered = self.compute_startup_order(services)
        self._schedule_queue = ordered

        ready = []
        for service in ordered:
            deps_met = all(
                self._registry.get_service(dep) and self._registry.get_service(dep).is_alive
                for dep in service.dependencies
            )
            if deps_met:
                ready.append(service.name)

        logger.info("zamanlama_tamamlandı", total=len(ordered), ready=len(ready))
        return ready
