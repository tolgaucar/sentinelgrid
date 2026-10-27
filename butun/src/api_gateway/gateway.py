"""
BÜTÜN - API Gateway (API Ağ Geçidi)

Tüm dış istekleri karşılayan, yetkilendiren ve ilgili servise yönlendiren
merkezi API geçidi. Sıfır Güven (Zero Trust) prensibine uygun çalışır.
"""

from __future__ import annotations

import time
from contextlib import asynccontextmanager
import structlog
from fastapi import FastAPI, Request, Response, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse, HTMLResponse

from src.models.service import (
    ServiceInfo,
    ServiceStatus,
    GatewayRoute,
    ClusterInfo,
)
from src.service_mesh.registry import ServiceRegistry
from src.service_mesh.discovery import ServiceDiscovery
from src.service_mesh.load_balancer import LoadBalancer
from src.service_mesh.circuit_breaker import CircuitBreaker
from src.orchestrator.lifecycle import LifecycleManager
from src.orchestrator.scheduler import ServiceScheduler
from src.orchestrator.scaler import AutoScaler
from src.resource_manager.monitor import ResourceMonitor
from src.resource_manager.allocator import ResourceAllocator, ResourceQuota
from src.dependency.graph import DependencyGraph

logger = structlog.get_logger(__name__)


class ButunGateway:
    """
    BÜTÜN Ana API Gateway.

    ÖTÜKEN-MATRİS'in merkezi sinir sistemi olarak tüm küme ve modüllere
    erişimi denetler, Service Mesh bileşenlerini koordine eder.
    """

    def __init__(self):
        # Service Mesh bileşenleri
        self.registry = ServiceRegistry()
        self.discovery = ServiceDiscovery(self.registry)
        self.load_balancer = LoadBalancer(self.registry)
        self.circuit_breaker = CircuitBreaker()

        # Orkestrasyon bileşenleri
        self.lifecycle = LifecycleManager(self.registry)
        self.scheduler = ServiceScheduler(self.registry)
        self.scaler = AutoScaler(self.registry)

        # Kaynak yönetimi
        self.resource_monitor = ResourceMonitor()
        self.resource_allocator = ResourceAllocator()

        # Bağımlılık grafiği
        self.dependency_graph = DependencyGraph()

        # Gateway rotaları
        self._routes: dict[str, GatewayRoute] = {}

        # FastAPI uygulaması
        self.app = self._create_app()

    def _create_app(self) -> FastAPI:
        """FastAPI uygulamasını oluştur ve endpoint'leri kaydet."""

        @asynccontextmanager
        async def lifespan(application: FastAPI):
            logger.info("bütün_başlatılıyor", system="ÖTÜKEN-MATRİS")
            await self.registry.start_monitor()
            await self.scaler.start_monitor()
            await self.resource_monitor.start_monitor()
            self._init_cluster_graph()
            logger.info("bütün_hazır", system="ÖTÜKEN-MATRİS")
            yield
            logger.info("bütün_kapatılıyor")
            await self.lifecycle.graceful_shutdown_all()
            await self.registry.stop_monitor()
            await self.scaler.stop_monitor()
            await self.resource_monitor.stop_monitor()
            logger.info("bütün_kapatıldı")

        app = FastAPI(
            title="BÜTÜN - ÖTÜKEN-MATRİS Ana Çatı Sistemi",
            description="Tüm modülleri birleştiren mikroservis orkestrasyon platformu",
            version="0.1.0",
            docs_url=None,
            redoc_url=None,
            lifespan=lifespan,
        )

        app.add_middleware(
            CORSMiddleware,
            allow_origins=["*"],
            allow_credentials=True,
            allow_methods=["*"],
            allow_headers=["*"],
        )

        self._register_routes(app)

        return app

    def _init_cluster_graph(self) -> None:
        """ÖTÜKEN-MATRİS küme bağımlılık grafiğini kur."""
        clusters = ["ULAK", "GÖRGÜ", "BUYRUK", "KOVAN", "KIN"]
        for c in clusters:
            self.dependency_graph.add_node(c)

        # Faz sırasına göre bağımlılıklar
        self.dependency_graph.add_dependency("GÖRGÜ", "ULAK")
        self.dependency_graph.add_dependency("BUYRUK", "ULAK")
        self.dependency_graph.add_dependency("BUYRUK", "GÖRGÜ")
        self.dependency_graph.add_dependency("KOVAN", "ULAK")
        self.dependency_graph.add_dependency("KOVAN", "BUYRUK")
        self.dependency_graph.add_dependency("KIN", "ULAK")
        self.dependency_graph.add_dependency("KIN", "KOVAN")

        logger.info("küme_bağımlılık_grafiği_kuruldu", clusters=clusters)

    def _register_routes(self, app: FastAPI) -> None:
        """API endpoint'lerini kaydet."""

        # === Swagger UI (CDN bağımsız, gömülü) ===
        @app.get("/docs", include_in_schema=False)
        async def custom_swagger_ui():
            return HTMLResponse("""<!DOCTYPE html>
<html lang="tr">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
<title>BÜTÜN — API Belgeleri</title>
<style>
body{margin:0;background:#1a1a2e;font-family:system-ui,sans-serif}
.topbar{display:none!important}
.swagger-ui .info .title{color:#e94560!important}
.swagger-ui{max-width:1200px;margin:0 auto}
#header{background:linear-gradient(135deg,#0f3460,#1a1a2e);color:#e94560;
padding:16px 24px;font-size:14px;border-bottom:1px solid #16213e}
#header b{color:#0ff}
#loading{color:#888;text-align:center;padding:60px;font-size:16px}
</style>
</head>
<body>
<div id="header">🏛️ <b>BÜTÜN</b> — ÖTÜKEN-MATRİS Ana Çatı Sistemi / API Belgeleri</div>
<div id="loading">⏳ Swagger UI yükleniyor…</div>
<div id="swagger-ui"></div>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.18.2/swagger-ui.css"/>
<script src="https://unpkg.com/swagger-ui-dist@5.18.2/swagger-ui-bundle.js"></script>
<script>
document.addEventListener('DOMContentLoaded', function(){
  var el = document.getElementById('loading');
  if(typeof SwaggerUIBundle==='undefined'){
    el.innerHTML='❌ Swagger UI yüklenemedi (CDN erişim sorunu).<br><br>' +
      '<a href="/openapi.json" style="color:#0ff">OpenAPI JSON</a> | ' +
      '<a href="/health" style="color:#0ff">Sağlık Kontrolü</a>';
    return;
  }
  el.style.display='none';
  SwaggerUIBundle({
    url:'/openapi.json',dom_id:'#swagger-ui',deepLinking:true,
    tryItOutEnabled:true,filter:true,
    layout:'BaseLayout',
    presets:[SwaggerUIBundle.presets.apis,SwaggerUIBundle.SwaggerUIStandalonePreset]
  });
});
</script>
</body>
</html>""")

        # === Sistem Durumu ===
        @app.get("/", tags=["Sistem"])
        async def root():
            return {
                "system": "ÖTÜKEN-MATRİS",
                "module": "BÜTÜN",
                "version": "0.1.0",
                "motto": "Görünmeyeni gör, duyulmayanı duy, emredileni yap",
                "status": "operational",
                "active_services": self.registry.active_count,
                "timestamp": time.time(),
            }

        @app.get("/health", tags=["Sistem"])
        async def health():
            return {
                "status": "healthy",
                "services": self.discovery.health_check_summary(),
                "resources": self.resource_monitor.snapshot().model_dump(),
            }

        # === Servis Yönetimi ===
        @app.post("/api/v1/services/register", tags=["Servis Yönetimi"])
        async def register_service(service: ServiceInfo):
            registered = self.registry.register(service)
            return {"status": "registered", "service": registered.model_dump()}

        @app.delete("/api/v1/services/{name}", tags=["Servis Yönetimi"])
        async def deregister_service(name: str):
            if self.registry.deregister(name):
                return {"status": "deregistered", "name": name}
            raise HTTPException(404, f"Servis bulunamadı: {name}")

        @app.post("/api/v1/services/{name}/heartbeat", tags=["Servis Yönetimi"])
        async def heartbeat(name: str):
            if self.registry.heartbeat(name):
                return {"status": "ok", "name": name}
            raise HTTPException(404, f"Servis bulunamadı: {name}")

        @app.get("/api/v1/services", tags=["Servis Yönetimi"])
        async def list_services(cluster: str | None = None):
            if cluster:
                services = self.registry.get_services_by_cluster(cluster)
            else:
                services = list(self.registry.services.values())
            return {"services": [s.model_dump() for s in services], "count": len(services)}

        @app.get("/api/v1/services/{name}", tags=["Servis Yönetimi"])
        async def get_service(name: str):
            service = self.registry.get_service(name)
            if not service:
                raise HTTPException(404, f"Servis bulunamadı: {name}")
            return service.model_dump()

        # === Servis Keşfi ===
        @app.get("/api/v1/discover/{name}", tags=["Servis Keşfi"])
        async def discover_service(name: str):
            address = self.discovery.resolve(name)
            if not address:
                raise HTTPException(404, f"Servis çözümlenemedi: {name}")
            return {"name": name, "address": address}

        # === Orkestrasyon ===
        @app.post("/api/v1/orchestrator/start/{name}", tags=["Orkestrasyon"])
        async def start_service(name: str, service: ServiceInfo):
            success = await self.lifecycle.start_service(service)
            if success:
                return {"status": "started", "name": name}
            raise HTTPException(500, f"Servis başlatılamadı: {name}")

        @app.post("/api/v1/orchestrator/stop/{name}", tags=["Orkestrasyon"])
        async def stop_service(name: str, graceful: bool = True):
            success = await self.lifecycle.stop_service(name, graceful)
            if success:
                return {"status": "stopped", "name": name}
            raise HTTPException(404, f"Servis bulunamadı: {name}")

        @app.post("/api/v1/orchestrator/restart/{name}", tags=["Orkestrasyon"])
        async def restart_service(name: str):
            success = await self.lifecycle.restart_service(name)
            if success:
                return {"status": "restarted", "name": name}
            raise HTTPException(500, f"Servis yeniden başlatılamadı: {name}")

        # === Kaynak Yönetimi ===
        @app.get("/api/v1/resources", tags=["Kaynak Yönetimi"])
        async def get_resources():
            return {
                "current": self.resource_monitor.snapshot().model_dump(),
                "average": self.resource_monitor.get_average().model_dump(),
                "allocation": self.resource_allocator.get_utilization_summary(),
            }

        @app.get("/api/v1/resources/alerts", tags=["Kaynak Yönetimi"])
        async def get_alerts():
            return {"alerts": self.resource_monitor.get_alerts()}

        # === Bağımlılık Grafiği ===
        @app.get("/api/v1/dependencies", tags=["Bağımlılık"])
        async def get_dependencies():
            try:
                layers = self.dependency_graph.get_layers()
                return {
                    "layers": layers,
                    "topological_order": self.dependency_graph.topological_sort(),
                    "graph": self.dependency_graph.visualize_text(),
                }
            except ValueError as e:
                raise HTTPException(500, str(e))

        @app.get("/api/v1/dependencies/{node}/impact", tags=["Bağımlılık"])
        async def get_impact(node: str):
            impact = self.dependency_graph.get_impact_zone(node)
            return {"node": node, "impact_zone": sorted(impact), "impact_count": len(impact)}

        # === Circuit Breaker ===
        @app.get("/api/v1/circuits", tags=["Circuit Breaker"])
        async def get_circuits():
            return {"circuits": {k: v.value for k, v in self.circuit_breaker.get_all_states().items()}}

        @app.post("/api/v1/circuits/{name}/reset", tags=["Circuit Breaker"])
        async def reset_circuit(name: str):
            self.circuit_breaker.reset(name)
            return {"status": "reset", "name": name}

        # === Orkestrasyon — Zamanlama ===
        @app.post("/api/v1/orchestrator/schedule", tags=["Orkestrasyon"])
        async def schedule_services(services: list[ServiceInfo]):
            """Servislerin bağımlılık sırasına göre başlatma planını hesapla."""
            try:
                ordered = self.scheduler.compute_startup_order(services)
                return {
                    "status": "ok",
                    "startup_order": [s.name for s in ordered],
                    "count": len(ordered),
                }
            except ValueError as e:
                raise HTTPException(400, str(e))

        # === Ölçekleme ===
        @app.post("/api/v1/scaler/check", tags=["Ölçekleme"])
        async def scaler_check():
            """Tüm aktif servislerin ölçekleme ihtiyacını kontrol et."""
            decisions = []
            for service in self.registry.services.values():
                if service.is_alive:
                    decision = self.scaler.evaluate(service)
                    decisions.append({
                        "service": decision.service_name,
                        "direction": decision.direction,
                        "current_replicas": decision.current_replicas,
                        "target_replicas": decision.target_replicas,
                        "reason": decision.reason,
                    })
            return {"decisions": decisions, "count": len(decisions)}

        # === Kaynak Tahsisi ===
        @app.post("/api/v1/resources/allocate", tags=["Kaynak Yönetimi"])
        async def allocate_resource(
            service_name: str,
            cpu_shares: int = 100,
            memory_limit_mb: int = 512,
            priority: int = 5,
        ):
            """Servise CPU ve bellek kotası tahsis et."""
            quota = ResourceQuota(
                service_name=service_name,
                cpu_shares=cpu_shares,
                memory_limit_mb=memory_limit_mb,
                priority=priority,
            )
            success = self.resource_allocator.allocate(quota)
            if success:
                return {"status": "allocated", "service": service_name, "cpu_shares": cpu_shares, "memory_mb": memory_limit_mb}
            raise HTTPException(409, f"Kaynak tahsisi başarısız: yetersiz kapasite")

        # === Bağımlılık Ekleme ===
        @app.post("/api/v1/dependencies/add", tags=["Bağımlılık"])
        async def add_dependency(node: str, depends_on: str):
            """Bağımlılık grafiğine yeni bağımlılık ekle."""
            success = self.dependency_graph.add_dependency(node, depends_on)
            if success:
                return {"status": "added", "node": node, "depends_on": depends_on}
            raise HTTPException(409, f"Bağımlılık eklenemedi: döngüsel bağımlılık oluşur")

        # === Küme Bilgisi ===
        @app.get("/api/v1/cluster/{name}", tags=["Küme Yönetimi"])
        async def get_cluster(name: str):
            """Belirli bir kümenin bilgilerini getir."""
            cluster = self.scheduler.get_cluster(name)
            if not cluster:
                raise HTTPException(404, f"Küme bulunamadı: {name}")
            active = [s for s in self.registry.get_services_by_cluster(name) if s.is_alive]
            return {
                "name": cluster.name,
                "description": cluster.description,
                "phase": cluster.phase,
                "modules": cluster.modules,
                "active_services": len(active),
                "total_modules": len(cluster.modules),
            }

        @app.get("/api/v1/clusters", tags=["Küme Yönetimi"])
        async def list_clusters():
            """Tüm kümeleri faz sırasına göre listele."""
            clusters = self.scheduler.get_clusters_by_phase()
            return {
                "clusters": [
                    {
                        "name": c.name,
                        "description": c.description,
                        "phase": c.phase,
                        "module_count": len(c.modules),
                    }
                    for c in clusters
                ],
                "count": len(clusters),
            }

    def add_route(self, route: GatewayRoute) -> None:
        """Gateway'e yeni rota ekle."""
        self._routes[route.path] = route
        logger.info("rota_eklendi", path=route.path, target=route.target_service)

    def get_routes(self) -> dict[str, GatewayRoute]:
        return dict(self._routes)
