"""
Zaman Senkronizasyonu — NTP/PTP tabanlı saat hizalama.

Tüm cihazların zaman damgaları aynı referansa göre olmalıdır.
Simülasyonda yerel saat yeterlidir. Prod'da:
  - NTP: ±1ms hassasiyet (uzak sunucu)
  - PTP (IEEE 1588): ±1µs hassasiyet (yerel ağ)

Kullanım:
    sync = ZamanSync(mode="ntp", server="ntp.tsk.mil.tr")
    offset = sync.measure_offset()
    cihaz.sync_time(offset)
"""

from __future__ import annotations

import logging
import time
from enum import Enum
from typing import Any

logger = logging.getLogger("otuken.zaman")


class SyncMode(Enum):
    LOCAL = "local"    # Simülasyon — yerel saat
    NTP = "ntp"        # Network Time Protocol
    PTP = "ptp"        # Precision Time Protocol (IEEE 1588)
    GPS_TIME = "gps"   # GPS zaman sinyali


class ZamanSync:
    """Zaman senkronizasyon yöneticisi."""

    def __init__(self, mode: SyncMode = SyncMode.LOCAL, **kwargs):
        self.mode = mode
        self.server = kwargs.get("server", "pool.ntp.org")
        self.port = kwargs.get("port", 123)
        self._offset_ns: int = 0
        self._last_sync: float = 0
        self._sync_count: int = 0
        self._drift_rate_ns_per_s: float = 0  # Saat kayma hızı
        self._measurements: list[int] = []

    def measure_offset(self) -> int:
        """Zaman farkını ölç (nanosaniye).

        Returns: offset_ns — yerel saate eklenecek düzeltme
        """
        if self.mode == SyncMode.LOCAL:
            self._offset_ns = 0
            return 0

        if self.mode == SyncMode.NTP:
            return self._ntp_measure()

        if self.mode == SyncMode.PTP:
            return self._ptp_measure()

        if self.mode == SyncMode.GPS_TIME:
            return self._gps_measure()

        return 0

    def _ntp_measure(self) -> int:
        """NTP offset ölçümü.

        Prod'da gerçek NTP paketi gönderilir.
        Simülasyonda: 0 döner (yerel saat doğru kabul edilir).
        """
        # Prod implementasyonu:
        # import struct, socket
        # client = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        # client.settimeout(2)
        # msg = b'\x1b' + 47 * b'\0'
        # t1 = time.time_ns()
        # client.sendto(msg, (self.server, self.port))
        # data, _ = client.recvfrom(1024)
        # t4 = time.time_ns()
        # t2, t3 = parse_ntp_timestamps(data)
        # offset = ((t2 - t1) + (t3 - t4)) // 2
        # self._offset_ns = offset

        self._offset_ns = 0
        self._last_sync = time.time()
        self._sync_count += 1
        self._measurements.append(self._offset_ns)
        if len(self._measurements) > 100:
            self._measurements = self._measurements[-100:]
        return self._offset_ns

    def _ptp_measure(self) -> int:
        """PTP (IEEE 1588) offset ölçümü — sub-microsecond hassasiyet."""
        # Prod: PTP daemon'dan okuma (linuxptp / ptp4l)
        # cat /var/run/ptp4l.status veya pmc command
        self._offset_ns = 0
        self._last_sync = time.time()
        self._sync_count += 1
        return self._offset_ns

    def _gps_measure(self) -> int:
        """GPS zaman sinyali — atomik saat referansı."""
        # Prod: GPS alıcısından PPS (Pulse Per Second) sinyali
        self._offset_ns = 0
        self._last_sync = time.time()
        self._sync_count += 1
        return self._offset_ns

    def get_corrected_time_ns(self) -> int:
        """Düzeltilmiş zaman damgası (nanosaniye)."""
        raw = time.time_ns()
        # Drift kompanzasyonu
        if self._drift_rate_ns_per_s != 0 and self._last_sync > 0:
            elapsed = time.time() - self._last_sync
            drift_correction = int(self._drift_rate_ns_per_s * elapsed)
        else:
            drift_correction = 0
        return raw + self._offset_ns + drift_correction

    def estimate_drift(self) -> float:
        """Saat kayma hızını tahmin et (ns/s)."""
        if len(self._measurements) < 2:
            return 0.0
        # Basit lineer regresyon
        n = len(self._measurements)
        recent = self._measurements[-min(10, n):]
        if len(recent) < 2:
            return 0.0
        drift = (recent[-1] - recent[0]) / max(len(recent) - 1, 1)
        self._drift_rate_ns_per_s = drift
        return drift

    def to_dict(self) -> dict:
        return {
            "mode": self.mode.value,
            "server": self.server,
            "offset_ns": self._offset_ns,
            "drift_ns_per_s": round(self._drift_rate_ns_per_s, 2),
            "sync_count": self._sync_count,
            "last_sync": self._last_sync,
        }
