"""
BÜTÜN - Resource Monitor (Kaynak İzleyici)

Sistem genelindeki CPU, bellek, disk ve ağ kaynak kullanımını izler.
DİRLİK modülüne telemetri verisi sağlar.
"""

from __future__ import annotations

import asyncio
import time
import structlog

try:
    import psutil
    HAS_PSUTIL = True
except ImportError:
    HAS_PSUTIL = False

from src.models.service import ResourceUsage

logger = structlog.get_logger(__name__)


class ResourceMonitor:
    """
    Sistem kaynak izleyici.

    İşletim sistemi seviyesinde kaynak kullanımını ölçerek servislerin
    adil dağıtımını denetler.
    """

    def __init__(self, sample_interval: float = 5.0):
        self._sample_interval = sample_interval
        self._history: list[ResourceUsage] = []
        self._max_history = 1000
        self._monitor_task: asyncio.Task | None = None
        self._alerts: list[dict] = []

    def snapshot(self) -> ResourceUsage:
        """Anlık kaynak kullanım görüntüsü al."""
        if not HAS_PSUTIL:
            return ResourceUsage(timestamp=time.time())

        cpu = psutil.cpu_percent(interval=0.1)
        mem = psutil.virtual_memory()
        disk = psutil.disk_usage("/")
        net = psutil.net_io_counters()

        usage = ResourceUsage(
            cpu_percent=cpu,
            memory_mb=mem.used / (1024 * 1024),
            memory_percent=mem.percent,
            disk_mb=disk.used / (1024 * 1024),
            disk_percent=(disk.used / disk.total) * 100 if disk.total > 0 else 0,
            network_in_bytes=net.bytes_recv,
            network_out_bytes=net.bytes_sent,
            timestamp=time.time(),
        )

        self._history.append(usage)
        if len(self._history) > self._max_history:
            self._history = self._history[-self._max_history:]

        return usage

    def get_average(self, window: int = 10) -> ResourceUsage:
        """Son N örneğin ortalamasını al."""
        if not self._history:
            return ResourceUsage()

        samples = self._history[-window:]
        n = len(samples)

        return ResourceUsage(
            cpu_percent=sum(s.cpu_percent for s in samples) / n,
            memory_mb=sum(s.memory_mb for s in samples) / n,
            memory_percent=sum(s.memory_percent for s in samples) / n,
            disk_mb=sum(s.disk_mb for s in samples) / n,
            disk_percent=sum(s.disk_percent for s in samples) / n,
            timestamp=time.time(),
        )

    def check_thresholds(
        self,
        cpu_limit: float = 90.0,
        memory_limit: float = 90.0,
        disk_limit: float = 90.0,
    ) -> list[dict]:
        """Kaynak eşiklerini kontrol et ve alarm üret."""
        usage = self.snapshot()
        alerts = []

        if usage.cpu_percent > cpu_limit:
            alert = {"type": "cpu_critical", "value": usage.cpu_percent, "limit": cpu_limit, "timestamp": time.time()}
            alerts.append(alert)
            logger.warning("cpu_kritik", percent=usage.cpu_percent)

        if usage.memory_percent > memory_limit:
            alert = {"type": "memory_critical", "value": usage.memory_percent, "limit": memory_limit, "timestamp": time.time()}
            alerts.append(alert)
            logger.warning("bellek_kritik", percent=usage.memory_percent)

        if usage.disk_percent > disk_limit:
            alert = {"type": "disk_critical", "value": usage.disk_percent, "limit": disk_limit, "timestamp": time.time()}
            alerts.append(alert)
            logger.warning("disk_kritik", percent=usage.disk_percent)

        self._alerts.extend(alerts)
        return alerts

    def get_alerts(self, limit: int = 50) -> list[dict]:
        return self._alerts[-limit:]

    def clear_alerts(self) -> None:
        self._alerts.clear()

    async def start_monitor(self) -> None:
        self._monitor_task = asyncio.create_task(self._monitor_loop())
        logger.info("kaynak_izleyici_başlatıldı", interval=self._sample_interval)

    async def stop_monitor(self) -> None:
        if self._monitor_task:
            self._monitor_task.cancel()
            try:
                await self._monitor_task
            except asyncio.CancelledError:
                pass

    async def _monitor_loop(self) -> None:
        while True:
            try:
                await asyncio.sleep(self._sample_interval)
                self.check_thresholds()
            except asyncio.CancelledError:
                raise
            except Exception as e:
                logger.error("kaynak_izleme_hatası", error=str(e))
