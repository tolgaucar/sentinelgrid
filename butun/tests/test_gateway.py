"""
BÜTÜN - API Gateway Testleri
"""

import pytest
from fastapi.testclient import TestClient
from src.api_gateway.gateway import ButunGateway


@pytest.fixture
def gateway():
    return ButunGateway()


@pytest.fixture
def client(gateway):
    return TestClient(gateway.app)


class TestGatewayEndpoints:
    def test_root(self, client):
        response = client.get("/")
        assert response.status_code == 200
        data = response.json()
        assert data["system"] == "ÖTÜKEN-MATRİS"
        assert data["module"] == "BÜTÜN"
        assert data["status"] == "operational"

    def test_health(self, client):
        response = client.get("/health")
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "healthy"
        assert "resources" in data

    def test_register_service(self, client):
        service_data = {
            "name": "BAG",
            "cluster": "ULAK",
            "port": 8001,
            "host": "127.0.0.1",
        }
        response = client.post("/api/v1/services/register", json=service_data)
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "registered"
        assert data["service"]["name"] == "BAG"

    def test_list_services_empty(self, client):
        response = client.get("/api/v1/services")
        assert response.status_code == 200
        data = response.json()
        assert data["count"] == 0

    def test_list_services_after_register(self, client):
        client.post("/api/v1/services/register", json={
            "name": "BAG", "cluster": "ULAK", "port": 8001,
        })
        response = client.get("/api/v1/services")
        data = response.json()
        assert data["count"] == 1

    def test_get_service(self, client):
        client.post("/api/v1/services/register", json={
            "name": "KUTUK", "cluster": "ULAK", "port": 8002,
        })
        response = client.get("/api/v1/services/KUTUK")
        assert response.status_code == 200
        assert response.json()["name"] == "KUTUK"

    def test_get_nonexistent_service(self, client):
        response = client.get("/api/v1/services/NONEXISTENT")
        assert response.status_code == 404

    def test_heartbeat(self, client):
        client.post("/api/v1/services/register", json={
            "name": "DIRLIK", "cluster": "ULAK", "port": 8003,
        })
        response = client.post("/api/v1/services/DIRLIK/heartbeat")
        assert response.status_code == 200

    def test_deregister_service(self, client):
        client.post("/api/v1/services/register", json={
            "name": "AKIM", "cluster": "ULAK", "port": 8004,
        })
        response = client.delete("/api/v1/services/AKIM")
        assert response.status_code == 200
        assert response.json()["status"] == "deregistered"

    def test_discover_registered_service(self, client):
        client.post("/api/v1/services/register", json={
            "name": "IM", "cluster": "ULAK", "port": 8005,
        })
        response = client.get("/api/v1/discover/IM")
        assert response.status_code == 200
        assert "address" in response.json()

    def test_discover_unknown_service(self, client):
        response = client.get("/api/v1/discover/UNKNOWN")
        assert response.status_code == 404

    def test_get_resources(self, client):
        response = client.get("/api/v1/resources")
        assert response.status_code == 200
        data = response.json()
        assert "current" in data
        assert "allocation" in data

    def test_get_dependencies(self, client):
        response = client.get("/api/v1/dependencies")
        assert response.status_code == 200
        data = response.json()
        assert "layers" in data
        assert "topological_order" in data

    def test_get_impact(self, client, gateway):
        gateway._init_cluster_graph()
        response = client.get("/api/v1/dependencies/ULAK/impact")
        assert response.status_code == 200
        data = response.json()
        assert data["node"] == "ULAK"
        assert len(data["impact_zone"]) > 0

    def test_circuits(self, client):
        response = client.get("/api/v1/circuits")
        assert response.status_code == 200

    def test_schedule_services(self, client):
        services = [
            {"name": "A", "cluster": "ULAK", "port": 8001, "dependencies": []},
            {"name": "B", "cluster": "ULAK", "port": 8002, "dependencies": ["A"]},
        ]
        response = client.post("/api/v1/orchestrator/schedule", json=services)
        assert response.status_code == 200
        data = response.json()
        assert data["startup_order"].index("A") < data["startup_order"].index("B")

    def test_schedule_cyclic_error(self, client):
        services = [
            {"name": "X", "cluster": "ULAK", "port": 8001, "dependencies": ["Y"]},
            {"name": "Y", "cluster": "ULAK", "port": 8002, "dependencies": ["X"]},
        ]
        response = client.post("/api/v1/orchestrator/schedule", json=services)
        assert response.status_code == 400

    def test_scaler_check(self, client):
        response = client.post("/api/v1/scaler/check")
        assert response.status_code == 200
        assert "decisions" in response.json()

    def test_allocate_resource(self, client):
        response = client.post(
            "/api/v1/resources/allocate",
            params={"service_name": "BAG", "cpu_shares": 100, "memory_limit_mb": 256},
        )
        assert response.status_code == 200
        assert response.json()["status"] == "allocated"

    def test_add_dependency(self, client, gateway):
        gateway._init_cluster_graph()
        response = client.post(
            "/api/v1/dependencies/add",
            params={"node": "TEST_A", "depends_on": "ULAK"},
        )
        assert response.status_code == 200
        assert response.json()["status"] == "added"

    def test_add_dependency_cycle_rejected(self, client, gateway):
        gateway._init_cluster_graph()
        response = client.post(
            "/api/v1/dependencies/add",
            params={"node": "ULAK", "depends_on": "KIN"},
        )
        assert response.status_code == 409

    def test_get_cluster(self, client, gateway):
        from src.models.service import ClusterInfo
        gateway.scheduler.register_cluster(
            ClusterInfo(name="ULAK", description="Altyapı", phase=1, modules=["BÜTÜN", "BAĞ"])
        )
        response = client.get("/api/v1/cluster/ULAK")
        assert response.status_code == 200
        data = response.json()
        assert data["name"] == "ULAK"
        assert data["phase"] == 1

    def test_get_cluster_not_found(self, client):
        response = client.get("/api/v1/cluster/NONEXISTENT")
        assert response.status_code == 404

    def test_list_clusters(self, client, gateway):
        from src.models.service import ClusterInfo
        gateway.scheduler.register_cluster(
            ClusterInfo(name="ULAK", description="Altyapı", phase=1, modules=["BÜTÜN"])
        )
        gateway.scheduler.register_cluster(
            ClusterInfo(name="GÖRGÜ", description="Algılama", phase=2, modules=["TANI"])
        )
        response = client.get("/api/v1/clusters")
        assert response.status_code == 200
        data = response.json()
        assert data["count"] >= 2

    def test_docs_endpoint(self, client):
        response = client.get("/docs")
        assert response.status_code == 200
        assert "swagger" in response.text.lower() or "BÜTÜN" in response.text
