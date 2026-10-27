"""
BÜTÜN - Service Discovery (Servis Keşfi)

Servislerin birbirini bulmasını sağlayan keşif mekanizması.
DNS-benzeri çözümleme ve servis adresi sorgulama yetenekleri sunar.
"""

from __future__ import annotations

import structlog

from src.models.service import ServiceInfo, ServiceStatus
from src.service_mesh.registry import ServiceRegistry

logger = structlog.get_logger(__name__)


class ServiceDiscovery:
    """
    Servis keşif katmanı.

    Modüller arası iletişimde hedef servisin adresini çözer.
    """

    def __init__(self, registry: ServiceRegistry):
        self._registry = registry

    def resolve(self, service_name: str) -> str | None:
        """Servis adından adres çözümle (DNS-benzeri)."""
        service = self._registry.get_service(service_name)
        if service and service.is_alive:
            return service.address
        logger.warning("servis_çözümlenemedi", name=service_name)
        return None

    def resolve_all(self, service_name: str) -> list[str]:
        """
        Bir servisin tüm aktif replikalarının adreslerini döndür.
        Şu an tek instans desteklenir; gelecekte replika desteği eklenecek.
        """
        service = self._registry.get_service(service_name)
        if service and service.is_alive:
            return [service.address]
        return []

    def find_by_cluster(self, cluster: str) -> list[ServiceInfo]:
        """Kümedeki tüm aktif servisleri bul."""
        return [
            s for s in self._registry.get_services_by_cluster(cluster)
            if s.is_alive
        ]

    def find_by_metadata(self, key: str, value: str) -> list[ServiceInfo]:
        """Metadata değerine göre servis ara."""
        results = []
        for service in self._registry.services.values():
            if service.is_alive and service.metadata.get(key) == value:
                results.append(service)
        return results

    def get_dependency_chain(self, service_name: str) -> list[str]:
        """Bir servisin tüm bağımlılık zincirini çözümle (BFS)."""
        visited = set()
        queue = [service_name]
        chain = []

        while queue:
            current = queue.pop(0)
            if current in visited:
                continue
            visited.add(current)

            service = self._registry.get_service(current)
            if service:
                for dep in service.dependencies:
                    if dep not in visited:
                        queue.append(dep)
                        chain.append(dep)

        return chain

    def health_check_summary(self) -> dict[str, int]:
        """Tüm servislerin durum özeti."""
        summary: dict[str, int] = {}
        for service in self._registry.services.values():
            status = service.status.value
            summary[status] = summary.get(status, 0) + 1
        return summary
