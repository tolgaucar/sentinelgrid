"""
BÜTÜN - Servis ve Küme Veri Modelleri

ÖTÜKEN-MATRİS sistemi içindeki tüm servislerin, kümelerin ve kaynakların
veri yapılarını tanımlar.
"""

from __future__ import annotations

import time
import uuid
from enum import Enum
from typing import Any

from pydantic import BaseModel, Field


class ServiceStatus(str, Enum):
    """Bir servisin mevcut durumu."""
    REGISTERED = "registered"
    STARTING = "starting"
    RUNNING = "running"
    DEGRADED = "degraded"
    STOPPED = "stopped"
    FAILED = "failed"


class HealthStatus(str, Enum):
    """Sağlık kontrolü sonucu."""
    HEALTHY = "healthy"
    UNHEALTHY = "unhealthy"
    UNKNOWN = "unknown"


class LoadBalanceStrategy(str, Enum):
    """Yük dengeleme stratejisi."""
    ROUND_ROBIN = "round_robin"
    LEAST_CONNECTIONS = "least_connections"
    WEIGHTED = "weighted"


class CircuitState(str, Enum):
    """Circuit breaker durumları."""
    CLOSED = "closed"        # Normal çalışma
    OPEN = "open"            # Devre açık, istekler reddedilir
    HALF_OPEN = "half_open"  # Test aşaması


class ResourceUsage(BaseModel):
    """Sistem kaynak kullanım bilgisi."""
    cpu_percent: float = Field(0.0, ge=0.0, le=100.0, description="CPU kullanım yüzdesi")
    memory_mb: float = Field(0.0, ge=0.0, description="Bellek kullanımı (MB)")
    memory_percent: float = Field(0.0, ge=0.0, le=100.0, description="Bellek kullanım yüzdesi")
    disk_mb: float = Field(0.0, ge=0.0, description="Disk kullanımı (MB)")
    disk_percent: float = Field(0.0, ge=0.0, le=100.0, description="Disk kullanım yüzdesi")
    network_in_bytes: int = Field(0, ge=0, description="Gelen ağ trafiği (byte)")
    network_out_bytes: int = Field(0, ge=0, description="Giden ağ trafiği (byte)")
    timestamp: float = Field(default_factory=time.time)


class ScalePolicy(BaseModel):
    """Otomatik ölçekleme politikası."""
    min_replicas: int = Field(1, ge=1, description="Minimum replika sayısı")
    max_replicas: int = Field(10, ge=1, description="Maksimum replika sayısı")
    target_cpu_percent: float = Field(70.0, description="Hedef CPU kullanımı")
    scale_up_threshold: float = Field(80.0, description="Yukarı ölçekleme eşiği")
    scale_down_threshold: float = Field(20.0, description="Aşağı ölçekleme eşiği")
    cooldown_sec: int = Field(60, description="Ölçekleme bekleme süresi (sn)")


class ServiceInfo(BaseModel):
    """Kayıtlı bir servisin tüm bilgileri."""
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    name: str = Field(..., description="Servis adı (modül kodu)")
    cluster: str = Field(..., description="Ait olduğu küme (ULAK, GÖRGÜ, vb.)")
    host: str = Field("127.0.0.1", description="Servis adresi")
    port: int = Field(..., ge=1, le=65535, description="Servis portu")
    status: ServiceStatus = Field(ServiceStatus.REGISTERED)
    health: HealthStatus = Field(HealthStatus.UNKNOWN)
    version: str = Field("0.1.0", description="Servis sürümü")
    metadata: dict[str, Any] = Field(default_factory=dict, description="Ek metadata")
    resource_usage: ResourceUsage = Field(default_factory=ResourceUsage)
    scale_policy: ScalePolicy = Field(default_factory=ScalePolicy)
    replicas: int = Field(1, ge=0, description="Aktif replika sayısı")
    weight: int = Field(1, ge=1, description="Yük dengeleme ağırlığı")
    registered_at: float = Field(default_factory=time.time)
    last_heartbeat: float = Field(default_factory=time.time)
    dependencies: list[str] = Field(default_factory=list, description="Bağımlı olduğu servisler")

    @property
    def address(self) -> str:
        return f"{self.host}:{self.port}"

    @property
    def is_alive(self) -> bool:
        return self.status in (ServiceStatus.RUNNING, ServiceStatus.DEGRADED)


class ClusterInfo(BaseModel):
    """Bir kümenin bilgileri."""
    name: str = Field(..., description="Küme adı")
    description: str = Field("", description="Küme açıklaması")
    phase: int = Field(..., ge=1, le=5, description="Geliştirme fazı (1-5)")
    modules: list[str] = Field(default_factory=list, description="Kümeye ait modüller")
    active_services: int = Field(0, ge=0, description="Aktif servis sayısı")
    total_services: int = Field(0, ge=0, description="Toplam servis sayısı")


class GatewayRoute(BaseModel):
    """API Gateway yönlendirme kuralı."""
    path: str = Field(..., description="URL yolu (örn: /api/v1/gorgu)")
    target_service: str = Field(..., description="Hedef servis adı")
    methods: list[str] = Field(default_factory=lambda: ["GET", "POST"], description="İzin verilen HTTP metodları")
    require_auth: bool = Field(True, description="Kimlik doğrulama gerekli mi")
    rate_limit: int | None = Field(None, description="Dakika başına istek limiti")
    timeout_sec: float = Field(30.0, description="İstek zaman aşımı (sn)")
    strip_prefix: bool = Field(True, description="Yol ön ekini kaldır")


class SystemEvent(BaseModel):
    """Sistem genelinde yayımlanan olay."""
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    event_type: str = Field(..., description="Olay tipi")
    source: str = Field(..., description="Olay kaynağı")
    data: dict[str, Any] = Field(default_factory=dict)
    timestamp: float = Field(default_factory=time.time)
