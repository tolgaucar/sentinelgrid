"""
BÜTÜN - Orchestrator ve Dependency Graph Testleri
"""

import pytest
from src.models.service import ServiceInfo, ClusterInfo
from src.service_mesh.registry import ServiceRegistry
from src.orchestrator.scheduler import ServiceScheduler
from src.orchestrator.scaler import AutoScaler, ScaleDecision
from src.dependency.graph import DependencyGraph
from src.resource_manager.allocator import ResourceAllocator, ResourceQuota
from src.service_mesh.circuit_breaker import CircuitBreaker
from src.models.service import CircuitState, ResourceUsage


class TestDependencyGraph:
    @pytest.fixture
    def graph(self):
        g = DependencyGraph()
        g.add_node("ULAK")
        g.add_node("GORGU")
        g.add_node("BUYRUK")
        g.add_node("KOVAN")
        g.add_node("KIN")
        g.add_dependency("GORGU", "ULAK")
        g.add_dependency("BUYRUK", "ULAK")
        g.add_dependency("BUYRUK", "GORGU")
        g.add_dependency("KOVAN", "BUYRUK")
        g.add_dependency("KIN", "KOVAN")
        return g

    def test_topological_sort(self, graph):
        order = graph.topological_sort()
        assert order.index("ULAK") < order.index("GORGU")
        assert order.index("GORGU") < order.index("BUYRUK")
        assert order.index("BUYRUK") < order.index("KOVAN")
        assert order.index("KOVAN") < order.index("KIN")

    def test_get_dependencies(self, graph):
        deps = graph.get_dependencies("BUYRUK")
        assert "ULAK" in deps
        assert "GORGU" in deps

    def test_get_dependents(self, graph):
        dependents = graph.get_dependents("ULAK")
        assert "GORGU" in dependents
        assert "BUYRUK" in dependents

    def test_impact_zone(self, graph):
        impact = graph.get_impact_zone("ULAK")
        assert "GORGU" in impact
        assert "BUYRUK" in impact
        assert "KOVAN" in impact
        assert "KIN" in impact

    def test_cycle_detection(self, graph):
        assert graph.has_cycle() is False
        result = graph.add_dependency("ULAK", "KIN")
        assert result is False  # Cycle would be created

    def test_layers(self, graph):
        layers = graph.get_layers()
        assert len(layers) >= 2
        assert "ULAK" in layers[0]

    def test_all_transitive_dependencies(self, graph):
        all_deps = graph.get_all_dependencies("KIN")
        assert "KOVAN" in all_deps
        assert "BUYRUK" in all_deps
        assert "ULAK" in all_deps


class TestServiceScheduler:
    def test_compute_startup_order(self):
        registry = ServiceRegistry()
        scheduler = ServiceScheduler(registry)

        s1 = ServiceInfo(name="ULAK_BASE", cluster="ULAK", port=8001, dependencies=[])
        s2 = ServiceInfo(name="GORGU_BASE", cluster="GORGU", port=8002, dependencies=["ULAK_BASE"])
        s3 = ServiceInfo(name="BUYRUK_BASE", cluster="BUYRUK", port=8003, dependencies=["GORGU_BASE"])

        order = scheduler.compute_startup_order([s1, s2, s3])
        names = [s.name for s in order]
        assert names.index("ULAK_BASE") < names.index("GORGU_BASE")
        assert names.index("GORGU_BASE") < names.index("BUYRUK_BASE")

    def test_cyclic_dependency_error(self):
        registry = ServiceRegistry()
        scheduler = ServiceScheduler(registry)

        s1 = ServiceInfo(name="A", cluster="ULAK", port=8001, dependencies=["B"])
        s2 = ServiceInfo(name="B", cluster="ULAK", port=8002, dependencies=["A"])

        with pytest.raises(ValueError, match="Döngüsel"):
            scheduler.compute_startup_order([s1, s2])


class TestAutoScaler:
    def test_scale_up_decision(self):
        registry = ServiceRegistry()
        scaler = AutoScaler(registry)

        service = ServiceInfo(name="BAG", cluster="ULAK", port=8001)
        service.resource_usage = ResourceUsage(cpu_percent=90.0)
        service.replicas = 1

        decision = scaler.evaluate(service)
        assert decision.direction == "up"
        assert decision.target_replicas == 2

    def test_scale_down_decision(self):
        registry = ServiceRegistry()
        scaler = AutoScaler(registry)

        service = ServiceInfo(name="BAG", cluster="ULAK", port=8001)
        service.resource_usage = ResourceUsage(cpu_percent=5.0)
        service.replicas = 3

        decision = scaler.evaluate(service)
        assert decision.direction == "down"
        assert decision.target_replicas == 2

    def test_no_scale_needed(self):
        registry = ServiceRegistry()
        scaler = AutoScaler(registry)

        service = ServiceInfo(name="BAG", cluster="ULAK", port=8001)
        service.resource_usage = ResourceUsage(cpu_percent=50.0)

        decision = scaler.evaluate(service)
        assert decision.direction == "none"


class TestResourceAllocator:
    def test_allocate_resources(self):
        allocator = ResourceAllocator(total_cpu_shares=1000, total_memory_mb=4096)
        quota = ResourceQuota(service_name="BAG", cpu_shares=100, memory_limit_mb=512)
        assert allocator.allocate(quota) is True
        assert allocator.available_cpu == 900
        assert allocator.available_memory == 3584

    def test_allocate_exceeds_limits(self):
        allocator = ResourceAllocator(total_cpu_shares=100, total_memory_mb=256)
        quota = ResourceQuota(service_name="BAG", cpu_shares=200, memory_limit_mb=128)
        assert allocator.allocate(quota) is False

    def test_release_resources(self):
        allocator = ResourceAllocator(total_cpu_shares=1000, total_memory_mb=4096)
        quota = ResourceQuota(service_name="BAG", cpu_shares=100, memory_limit_mb=512)
        allocator.allocate(quota)
        assert allocator.release("BAG") is True
        assert allocator.available_cpu == 1000

    def test_utilization_summary(self):
        allocator = ResourceAllocator(total_cpu_shares=1000, total_memory_mb=4096)
        summary = allocator.get_utilization_summary()
        assert summary["service_count"] == 0
        assert summary["available_cpu_shares"] == 1000


class TestCircuitBreaker:
    def test_initial_state_closed(self):
        cb = CircuitBreaker()
        assert cb.get_state("BAG") == CircuitState.CLOSED
        assert cb.is_allowed("BAG") is True

    def test_opens_after_threshold(self):
        cb = CircuitBreaker(failure_threshold=3)
        for _ in range(3):
            cb.record_failure("BAG")
        assert cb.get_state("BAG") == CircuitState.OPEN
        assert cb.is_allowed("BAG") is False

    def test_success_resets_in_half_open(self):
        cb = CircuitBreaker(failure_threshold=2, recovery_timeout=0.0)
        cb.record_failure("BAG")
        cb.record_failure("BAG")
        assert cb.get_state("BAG") == CircuitState.HALF_OPEN  # recovery_timeout=0
        cb.record_success("BAG")
        assert cb.get_state("BAG") == CircuitState.CLOSED

    def test_reset(self):
        cb = CircuitBreaker(failure_threshold=2)
        cb.record_failure("BAG")
        cb.record_failure("BAG")
        cb.reset("BAG")
        assert cb.get_state("BAG") == CircuitState.CLOSED
