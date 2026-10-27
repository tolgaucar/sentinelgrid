"""
BÜTÜN - Service Registry Testleri
"""

import pytest
import asyncio
from src.models.service import ServiceInfo, ServiceStatus, HealthStatus
from src.service_mesh.registry import ServiceRegistry


@pytest.fixture
def registry():
    return ServiceRegistry(heartbeat_interval=1.0, deregister_timeout=5.0)


@pytest.fixture
def sample_service():
    return ServiceInfo(name="BAG", cluster="ULAK", port=8001, dependencies=[])


@pytest.fixture
def sample_service_2():
    return ServiceInfo(name="KUTUK", cluster="ULAK", port=8002, dependencies=["BAG"])


class TestServiceRegistry:
    def test_register_service(self, registry, sample_service):
        result = registry.register(sample_service)
        assert result.name == "BAG"
        assert result.status == ServiceStatus.RUNNING
        assert registry.active_count == 1

    def test_register_updates_existing(self, registry, sample_service):
        registry.register(sample_service)
        sample_service.port = 9001
        result = registry.register(sample_service)
        assert result.port == 9001
        assert registry.active_count == 1

    def test_deregister_service(self, registry, sample_service):
        registry.register(sample_service)
        assert registry.deregister("BAG") is True
        assert registry.active_count == 0

    def test_deregister_nonexistent(self, registry):
        assert registry.deregister("NONEXISTENT") is False

    def test_heartbeat(self, registry, sample_service):
        registry.register(sample_service)
        assert registry.heartbeat("BAG") is True
        assert registry.heartbeat("NONEXISTENT") is False

    def test_get_service(self, registry, sample_service):
        registry.register(sample_service)
        result = registry.get_service("BAG")
        assert result is not None
        assert result.name == "BAG"
        assert registry.get_service("NONEXISTENT") is None

    def test_get_services_by_cluster(self, registry, sample_service, sample_service_2):
        registry.register(sample_service)
        registry.register(sample_service_2)
        ulak_services = registry.get_services_by_cluster("ULAK")
        assert len(ulak_services) == 2

    def test_mark_unhealthy(self, registry, sample_service):
        registry.register(sample_service)
        registry.mark_unhealthy("BAG")
        service = registry.get_service("BAG")
        assert service.health == HealthStatus.UNHEALTHY
        assert service.status == ServiceStatus.DEGRADED

    def test_heartbeat_recovers_degraded(self, registry, sample_service):
        registry.register(sample_service)
        registry.mark_unhealthy("BAG")
        registry.heartbeat("BAG")
        service = registry.get_service("BAG")
        assert service.status == ServiceStatus.RUNNING
        assert service.health == HealthStatus.HEALTHY

    def test_event_handler(self, registry, sample_service):
        events = []
        registry.on_event(lambda e: events.append(e))
        registry.register(sample_service)
        assert len(events) == 1
        assert events[0].event_type == "service.registered"
