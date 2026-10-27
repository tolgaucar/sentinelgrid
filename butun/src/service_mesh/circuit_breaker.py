"""
BÜTÜN - Circuit Breaker (Devre Kesici)

Mikroservisler arası iletişimde hata yayılmasını engelleyen devre kesici deseni.
Çöken bir servisin diğer servisleri domino etkisiyle çökmesini önler.
"""

from __future__ import annotations

import asyncio
import time
import structlog

from src.models.service import CircuitState

logger = structlog.get_logger(__name__)


class CircuitBreaker:
    """
    Devre kesici (Circuit Breaker) deseni.

    DİRLİK modülü ile koordineli çalışarak, arızalı servislere giden
    istekleri keserek sistemin genel dayanıklılığını korur.

    Durumlar:
    - CLOSED: Normal çalışma, istekler geçer
    - OPEN: Devre açık, istekler anında reddedilir
    - HALF_OPEN: Test modu, sınırlı istek geçirilir
    """

    def __init__(
        self,
        failure_threshold: int = 5,
        recovery_timeout: float = 30.0,
        half_open_max_calls: int = 3,
    ):
        self._breakers: dict[str, _BreakerState] = {}
        self._failure_threshold = failure_threshold
        self._recovery_timeout = recovery_timeout
        self._half_open_max_calls = half_open_max_calls

    def get_state(self, service_name: str) -> CircuitState:
        """Servisin devre durumunu sorgula."""
        breaker = self._get_or_create(service_name)
        self._check_recovery(service_name, breaker)
        return breaker.state

    def is_allowed(self, service_name: str) -> bool:
        """İsteğe izin verilip verilmediğini kontrol et."""
        breaker = self._get_or_create(service_name)
        self._check_recovery(service_name, breaker)

        if breaker.state == CircuitState.CLOSED:
            return True
        elif breaker.state == CircuitState.HALF_OPEN:
            if breaker.half_open_calls < self._half_open_max_calls:
                breaker.half_open_calls += 1
                return True
            return False
        else:  # OPEN
            return False

    def record_success(self, service_name: str) -> None:
        """Başarılı istek kaydet."""
        breaker = self._get_or_create(service_name)
        if breaker.state == CircuitState.HALF_OPEN:
            breaker.state = CircuitState.CLOSED
            breaker.failure_count = 0
            breaker.half_open_calls = 0
            logger.info("devre_kapandı", service=service_name)
        elif breaker.state == CircuitState.CLOSED:
            breaker.failure_count = max(0, breaker.failure_count - 1)

    def record_failure(self, service_name: str) -> None:
        """Başarısız istek kaydet."""
        breaker = self._get_or_create(service_name)
        breaker.failure_count += 1
        breaker.last_failure_time = time.time()

        if breaker.state == CircuitState.HALF_OPEN:
            breaker.state = CircuitState.OPEN
            breaker.opened_at = time.time()
            logger.warning("devre_tekrar_açıldı", service=service_name)
        elif breaker.state == CircuitState.CLOSED and breaker.failure_count >= self._failure_threshold:
            breaker.state = CircuitState.OPEN
            breaker.opened_at = time.time()
            logger.warning("devre_açıldı", service=service_name, failures=breaker.failure_count)

    def reset(self, service_name: str) -> None:
        """Devre durumunu sıfırla."""
        if service_name in self._breakers:
            self._breakers[service_name] = _BreakerState()
            logger.info("devre_sıfırlandı", service=service_name)

    def get_all_states(self) -> dict[str, CircuitState]:
        """Tüm devrelerin durumunu getir."""
        result = {}
        for name, breaker in self._breakers.items():
            self._check_recovery(name, breaker)
            result[name] = breaker.state
        return result

    def _get_or_create(self, service_name: str) -> _BreakerState:
        if service_name not in self._breakers:
            self._breakers[service_name] = _BreakerState()
        return self._breakers[service_name]

    def _check_recovery(self, service_name: str, breaker: _BreakerState) -> None:
        """Kurtarma zaman aşımını kontrol et."""
        if breaker.state == CircuitState.OPEN and breaker.opened_at:
            elapsed = time.time() - breaker.opened_at
            if elapsed >= self._recovery_timeout:
                breaker.state = CircuitState.HALF_OPEN
                breaker.half_open_calls = 0
                logger.info("devre_yarı_açık", service=service_name)


class _BreakerState:
    """Tek bir servis için iç devre durumu."""
    __slots__ = ("state", "failure_count", "last_failure_time", "opened_at", "half_open_calls")

    def __init__(self) -> None:
        self.state = CircuitState.CLOSED
        self.failure_count = 0
        self.last_failure_time = 0.0
        self.opened_at: float | None = None
        self.half_open_calls = 0
