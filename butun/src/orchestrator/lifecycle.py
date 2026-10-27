"""
BÜTÜN - Lifecycle Manager (Yaşam Döngüsü Yöneticisi)

Servislerin başlatılması, durdurulması, yeniden başlatılması ve
güncellenmesi gibi yaşam döngüsü olaylarını yönetir.
"""

from __future__ import annotations

import asyncio
import time
import structlog

from src.models.service import ServiceInfo, ServiceStatus, HealthStatus
from src.service_mesh.registry import ServiceRegistry

logger = structlog.get_logger(__name__)


class LifecycleManager:
    """
    Servis yaşam döngüsü yöneticisi.

    BÜTÜN'ün "Kral" rolü gereği, tüm servislerin başlatılma sırası,
    bağımlılık çözümlemesi ve sağlık durumu takibi bu modülle yapılır.
    """

    def __init__(self, registry: ServiceRegistry):
        self._registry = registry
        self._start_order: list[str] = []
        self._shutdown_hooks: dict[str, callable] = {}

    async def start_service(self, service: ServiceInfo) -> bool:
        """Servisi başlat ve kaydet."""
        logger.info("servis_başlatılıyor", name=service.name, cluster=service.cluster)

        # Bağımlılık kontrolü
        for dep in service.dependencies:
            dep_service = self._registry.get_service(dep)
            if not dep_service or not dep_service.is_alive:
                logger.error("bağımlılık_eksik", service=service.name, dependency=dep)
                service.status = ServiceStatus.FAILED
                return False

        service.status = ServiceStatus.STARTING
        await asyncio.sleep(0.1)  # Simüle edilmiş başlatma gecikmesi

        self._registry.register(service)
        service.status = ServiceStatus.RUNNING
        service.health = HealthStatus.HEALTHY
        self._start_order.append(service.name)

        logger.info("servis_başlatıldı", name=service.name)
        return True

    async def stop_service(self, name: str, graceful: bool = True) -> bool:
        """Servisi durdur."""
        service = self._registry.get_service(name)
        if not service:
            logger.warning("durdurulacak_servis_yok", name=name)
            return False

        logger.info("servis_durduruluyor", name=name, graceful=graceful)

        # Graceful shutdown - bağımlı servisleri uyar
        if graceful:
            dependents = self._find_dependents(name)
            for dep_name in dependents:
                logger.warning("bağımlı_servis_etkilenecek", service=name, dependent=dep_name)

            # Shutdown hook çağır
            if name in self._shutdown_hooks:
                try:
                    hook = self._shutdown_hooks[name]
                    if asyncio.iscoroutinefunction(hook):
                        coro = hook()
                    else:
                        coro = asyncio.to_thread(hook)
                    await asyncio.wait_for(coro, timeout=10.0)
                except asyncio.TimeoutError:
                    logger.warning("shutdown_hook_zaman_aşımı", name=name)

        service.status = ServiceStatus.STOPPED
        self._registry.deregister(name)

        if name in self._start_order:
            self._start_order.remove(name)

        logger.info("servis_durduruldu", name=name)
        return True

    async def restart_service(self, name: str) -> bool:
        """Servisi yeniden başlat."""
        service = self._registry.get_service(name)
        if not service:
            return False

        service_copy = service.model_copy()
        await self.stop_service(name, graceful=True)
        await asyncio.sleep(0.2)
        return await self.start_service(service_copy)

    def register_shutdown_hook(self, service_name: str, hook: callable) -> None:
        """Servis kapanma öncesi çalıştırılacak fonksiyonu kaydet."""
        self._shutdown_hooks[service_name] = hook

    def get_start_order(self) -> list[str]:
        """Servislerin başlatılma sırasını döndür."""
        return list(self._start_order)

    async def graceful_shutdown_all(self) -> None:
        """Tüm servisleri ters sırada kapat."""
        logger.info("tüm_servisler_kapatılıyor", count=len(self._start_order))
        for name in reversed(self._start_order[:]):
            await self.stop_service(name, graceful=True)
        logger.info("tüm_servisler_kapatıldı")

    def _find_dependents(self, service_name: str) -> list[str]:
        """Bir servise bağımlı olan diğer servisleri bul."""
        dependents = []
        for name, service in self._registry.services.items():
            if service_name in service.dependencies:
                dependents.append(name)
        return dependents
