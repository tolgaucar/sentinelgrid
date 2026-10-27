"""
BÜTÜN - Ana Giriş Noktası

ÖTÜKEN-MATRİS Ana Çatı Sistemi'ni başlatan ana modül.
Tüm bileşenleri başlatır, yapılandırmayı yükler ve API Gateway'i ayağa kaldırır.
"""

from __future__ import annotations

import os
import sys
import yaml
import structlog
import uvicorn

from src.api_gateway.gateway import ButunGateway
from src.api_gateway.middleware import ZeroTrustMiddleware
from src.models.service import ClusterInfo

structlog.configure(
    processors=[
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.add_log_level,
        structlog.dev.ConsoleRenderer(),
    ],
    wrapper_class=structlog.make_filtering_bound_logger(0),
)

logger = structlog.get_logger(__name__)

BANNER = """
======================================================
  BUTUN - OTUKEN-MATRIS Ana Cati Sistemi v0.1.0
  "Gorunmeyeni gor, duyulmayani duy, emredileni yap"
======================================================
"""


def load_config(config_path: str | None = None) -> dict:
    """Yapilandirma dosyasini yukle. OTUKEN_ENV=prod ise prod.yaml da yüklenir."""
    base_dir = os.path.join(os.path.dirname(__file__), "..", "config")

    if config_path is None:
        config_path = os.path.join(base_dir, "default.yaml")

    config_path = os.path.abspath(config_path)

    if not os.path.exists(config_path):
        logger.warning("yapilandirma_bulunamadi", path=config_path)
        return {}

    with open(config_path, "r", encoding="utf-8") as f:
        config = yaml.safe_load(f) or {}

    # Üretim ortamı: prod.yaml ile üzerine yaz
    env = os.environ.get("OTUKEN_ENV", "dev")
    if env == "prod":
        prod_path = os.path.join(base_dir, "prod.yaml")
        if os.path.exists(prod_path):
            with open(prod_path, "r", encoding="utf-8") as f:
                prod_cfg = yaml.safe_load(f) or {}
            config = _deep_merge(config, prod_cfg)
            logger.info("uretim_yapilandirmasi_yuklendi", path=prod_path)

    # Ortam değişkeni override'ları
    if os.environ.get("BUTUN_PORT"):
        config.setdefault("gateway", {})["port"] = int(os.environ["BUTUN_PORT"])
    if os.environ.get("BUTUN_CORS_ORIGINS"):
        config.setdefault("gateway", {})["cors_origins"] = os.environ["BUTUN_CORS_ORIGINS"].split(",")

    logger.info("yapilandirma_yuklendi", path=config_path, ortam=env)
    return config


def _deep_merge(base: dict, override: dict) -> dict:
    """İç içe sözlükleri birleştirir (override kazanır)."""
    result = base.copy()
    for k, v in override.items():
        if k in result and isinstance(result[k], dict) and isinstance(v, dict):
            result[k] = _deep_merge(result[k], v)
        else:
            result[k] = v
    return result


def setup_clusters(gateway: ButunGateway, config: dict) -> None:
    """Kume yapilandirmasini yukle."""
    clusters_config = config.get("clusters", [])
    for cluster_data in clusters_config:
        cluster = ClusterInfo(
            name=cluster_data["name"],
            description=cluster_data.get("description", ""),
            phase=cluster_data["phase"],
            modules=cluster_data.get("modules", []),
        )
        gateway.scheduler.register_cluster(cluster)
        logger.info("kume_kaydedildi", name=cluster.name, phase=cluster.phase, modules=len(cluster.modules))


def create_app(config_path: str | None = None) -> ButunGateway:
    """BUTUN uygulamasini olustur ve yapilandir."""
    config = load_config(config_path)
    gateway = ButunGateway()

    # Zero Trust middleware ekle
    gw_config = config.get("gateway", {})
    rate_config = gw_config.get("rate_limit", {})
    gateway.app.add_middleware(
        ZeroTrustMiddleware,
        rate_limit=rate_config.get("requests_per_minute", 1000),
        burst=rate_config.get("burst", 50),
    )

    # Kumeleri kaydet
    setup_clusters(gateway, config)

    return gateway


def main():
    """Ana giris noktasi."""
    print(BANNER)

    config = load_config()
    host = config.get("gateway", {}).get("host", "0.0.0.0")
    port = config.get("gateway", {}).get("port", 8000)

    gateway = create_app()

    logger.info("butun_baslatiliyor", host=host, port=port)

    uvicorn.run(
        gateway.app,
        host=host,
        port=port,
        log_level="info",
    )


# Uvicorn CLI entrypoint: python3 -m uvicorn src.main:app
app = create_app().app


if __name__ == "__main__":
    main()
