"""
BÜTÜN - Service Registry (Servis Kayıt Defteri)

Tüm mikroservislerin merkezi kaydını tutar. Servis kaydı, kaldırma,
heartbeat takibi ve durum yönetimi bu modülde gerçekleşir.
"""

from __future__ import annotations

import asyncio
import time
import structlog

from src.models.service import (
    ServiceInfo,
    ServiceStatus,
    HealthStatus,
    SystemEvent,
)

logger = structlog.get_logger(__name__)


class ServiceRegistry:
    """
    Merkezi servis kayıt defteri.

    ÖTÜKEN-MATRİS'teki tüm modüllerin (BÜTÜN, BAĞ, KÜTÜK vb.) kayıtlarını
    tutar ve yaşam döngülerini izler.
    """

    def __init__(self, heartbeat_interval: float = 5.0, deregister_timeout: float = 30.0):
        self._services: dict[str, ServiceInfo] = {}
        self._heartbeat_interval = heartbeat_interval
        self._deregister_timeout = deregister_timeout
        self._event_handlers: list[callable] = []
        self._monitor_task: asyncio.Task | None = None

    @property
    def services(self) -> dict[str, ServiceInfo]:
        return dict(self._services)

    @property
    def active_count(self) -> int:
        return sum(1 for s in self._services.values() if s.is_alive)

    def register(self, service: ServiceInfo) -> ServiceInfo:
        """Yeni servis kaydet veya mevcut kaydı güncelle."""
        if service.name in self._services:
            existing = self._services[service.name]
            existing.host = service.host
            existing.port = service.port
            existing.version = service.version
            existing.metadata = service.metadata
            existing.dependencies = service.dependencies
            existing.status = ServiceStatus.RUNNING
            existing.last_heartbeat = time.time()
            logger.info("servis_güncellendi", name=service.name, address=service.address)
            self._emit_event("service.updated", service.name, {"address": service.address})
            return existing

        service.status = ServiceStatus.RUNNING
        service.last_heartbeat = time.time()
        self._services[service.name] = service
        logger.info("servis_kaydedildi", name=service.name, address=service.address, cluster=service.cluster)
        self._emit_event("service.registered", service.name, {"address": service.address, "cluster": service.cluster})
        return service

    def deregister(self, name: str) -> bool:
        """Servisi kayıttan çıkar."""
        if name not in self._services:
            logger.warning("servis_bulunamadı", name=name)
            return False

        service = self._services.pop(name)
        service.status = ServiceStatus.STOPPED
        logger.info("servis_kaldırıldı", name=name)
        self._emit_event("service.deregistered", name, {})
        return True

    def heartbeat(self, name: str) -> bool:
        """Servis kalp atışı güncelle."""
        if name not in self._services:
            return False

        service = self._services[name]
        service.last_heartbeat = time.time()
        if service.status == ServiceStatus.DEGRADED:
            service.status = ServiceStatus.RUNNING
            service.health = HealthStatus.HEALTHY
        return True

    def get_service(self, name: str) -> ServiceInfo | None:
        """İsme göre servis bilgisi getir."""
        return self._services.get(name)

    def get_services_by_cluster(self, cluster: str) -> list[ServiceInfo]:
        """Kümeye göre servisleri listele."""
        return [s for s in self._services.values() if s.cluster == cluster]

    def get_services_by_status(self, status: ServiceStatus) -> list[ServiceInfo]:
        """Duruma göre servisleri listele."""
        return [s for s in self._services.values() if s.status == status]

    def mark_unhealthy(self, name: str) -> None:
        """Servisi sağlıksız olarak işaretle."""
        if name in self._services:
            service = self._services[name]
            service.health = HealthStatus.UNHEALTHY
            service.status = ServiceStatus.DEGRADED
            logger.warning("servis_sağlıksız", name=name)
            self._emit_event("service.unhealthy", name, {})

    def on_event(self, handler: callable) -> None:
        """Olay dinleyici ekle."""
        self._event_handlers.append(handler)

    def _emit_event(self, event_type: str, source: str, data: dict) -> None:
        event = SystemEvent(event_type=event_type, source=source, data=data)
        for handler in self._event_handlers:
            try:
                handler(event)
            except Exception as e:
                logger.error("olay_işleme_hatası", event_type=event_type, error=str(e))

    async def start_monitor(self) -> None:
        """Heartbeat izleme döngüsünü başlat."""
        self._monitor_task = asyncio.create_task(self._monitor_loop())
        logger.info("heartbeat_izleyici_başlatıldı", interval=self._heartbeat_interval)

    async def stop_monitor(self) -> None:
        """Heartbeat izleme döngüsünü durdur."""
        if self._monitor_task:
            self._monitor_task.cancel()
            try:
                await self._monitor_task
            except asyncio.CancelledError:
                pass
            logger.info("heartbeat_izleyici_durduruldu")

    async def _monitor_loop(self) -> None:
        """Periyodik heartbeat kontrolü."""
        while True:
            await asyncio.sleep(self._heartbeat_interval)
            now = time.time()
            stale_services = []

            for name, service in self._services.items():
                if not service.is_alive:
                    continue
                elapsed = now - service.last_heartbeat
                if elapsed > self._deregister_timeout:
                    stale_services.append(name)
                elif elapsed > self._heartbeat_interval * 3:
                    self.mark_unhealthy(name)

            for name in stale_services:
                logger.warning("servis_zaman_aşımı", name=name)
                self._services[name].status = ServiceStatus.FAILED
                self._emit_event("service.timeout", name, {})
