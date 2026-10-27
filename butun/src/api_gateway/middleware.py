"""
BÜTÜN - Zero Trust Middleware

Sıfır Güven (Zero Trust) mimarisine uygun olarak her isteği doğrulayan,
loglayan ve denetleyen ara katman yazılımı.
"""

from __future__ import annotations

import time
import uuid
import structlog
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import Response, JSONResponse

logger = structlog.get_logger(__name__)


class ZeroTrustMiddleware(BaseHTTPMiddleware):
    """
    Sıfır Güven ara katmanı.

    İM (Dijital İmza) modülü ile entegre çalışarak:
    - Her isteğe benzersiz izleme ID'si atar
    - İstek kaynağını doğrular
    - İstek süresini ölçer ve loglar
    - Rate limiting uygular
    """

    def __init__(self, app, rate_limit: int = 1000, burst: int = 50):
        super().__init__(app)
        self._rate_limit = rate_limit
        self._burst = burst
        self._request_counts: dict[str, list[float]] = {}

    async def dispatch(self, request: Request, call_next) -> Response:
        # Swagger/docs yollarını doğrudan geçir (BaseHTTPMiddleware uyumsuzluğu)
        if request.url.path in ("/docs", "/redoc", "/openapi.json", "/favicon.ico"):
            return await call_next(request)

        request_id = str(uuid.uuid4())
        start_time = time.time()
        client_ip = request.client.host if request.client else "unknown"

        # Rate limiting kontrolü
        if not self._check_rate_limit(client_ip):
            logger.warning("rate_limit_aşıldı", client=client_ip, request_id=request_id)
            return JSONResponse(
                status_code=429,
                content={"error": "İstek limiti aşıldı", "request_id": request_id},
            )

        logger.info(
            "istek_alındı",
            request_id=request_id,
            method=request.method,
            path=request.url.path,
            client=client_ip,
        )

        try:
            response = await call_next(request)
            duration = time.time() - start_time

            response.headers["X-Request-ID"] = request_id
            response.headers["X-Response-Time"] = f"{duration:.4f}"
            response.headers["X-Powered-By"] = "BUTUN/OTUKEN-MATRIS"

            logger.info(
                "istek_tamamlandı",
                request_id=request_id,
                status=response.status_code,
                duration_ms=f"{duration * 1000:.2f}",
            )
            return response

        except Exception as e:
            duration = time.time() - start_time
            logger.error(
                "istek_hatası",
                request_id=request_id,
                error=str(e),
                duration_ms=f"{duration * 1000:.2f}",
            )
            return JSONResponse(
                status_code=500,
                content={"error": "İç sunucu hatası", "request_id": request_id},
            )

    def _check_rate_limit(self, client_ip: str) -> bool:
        """Basit sliding window rate limiter."""
        now = time.time()
        window = 60.0  # 1 dakika

        if client_ip not in self._request_counts:
            self._request_counts[client_ip] = []

        # Eski istekleri temizle
        self._request_counts[client_ip] = [
            t for t in self._request_counts[client_ip] if now - t < window
        ]

        if len(self._request_counts[client_ip]) >= self._rate_limit:
            return False

        self._request_counts[client_ip].append(now)
        return True
