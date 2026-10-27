"""
Protokol Adaptör Katmanı — Cihaz ↔ BAĞ arasındaki iletişim soyutlaması.

Gerçek ortamda cihazlar farklı protokollerle konuşur:
  - TCP/IP (modern sensörler, IP tabanlı radar)
  - UDP (hızlı telemetri akışı, video)
  - Seri Port (RS-232/RS-485, eski donanım)
  - MIL-STD-1553 (askeri araç veri yolu)
  - Link-16 (taktik veri bağlantısı)
  - HTTP/REST (simülasyon, yazılım tabanlı)

Bu katman BaseCihaz.publish() çağrısını gerçek protokole çevirir.
Prod'a geçişte: sadece transport_type değişir, üst katman etkilenmez.
"""

from __future__ import annotations

import json
import logging
import time
from abc import ABC, abstractmethod
from enum import Enum
from typing import Any

logger = logging.getLogger("protokol")


class TransportType(Enum):
    """Desteklenen iletişim protokolleri."""
    HTTP = "http"          # Varsayılan — simülasyon ve test
    TCP = "tcp"            # IP tabanlı sensörler
    UDP = "udp"            # Hızlı telemetri akışı
    SERIAL = "serial"      # RS-232/RS-485
    MILSTD1553 = "mil1553" # Askeri veri yolu
    LINK16 = "link16"      # Taktik veri bağlantısı
    MQTT = "mqtt"          # IoT sensör ağı
    GRPC = "grpc"          # gRPC (BAĞ native)


class TransportConfig:
    """Protokol yapılandırması."""
    __slots__ = ("transport_type", "host", "port", "baudrate", "timeout_ms",
                 "retry_count", "buffer_size", "encryption", "compression", "extra")

    def __init__(self, transport_type: TransportType = TransportType.HTTP, **kwargs):
        self.transport_type = transport_type
        self.host = kwargs.get("host", "127.0.0.1")
        self.port = kwargs.get("port", 9091)
        self.baudrate = kwargs.get("baudrate", 115200)  # Seri port için
        self.timeout_ms = kwargs.get("timeout_ms", 5000)
        self.retry_count = kwargs.get("retry_count", 3)
        self.buffer_size = kwargs.get("buffer_size", 65536)
        self.encryption = kwargs.get("encryption", False)
        self.compression = kwargs.get("compression", False)
        self.extra = kwargs.get("extra", {})


class TransportStats:
    """İletişim istatistikleri."""
    __slots__ = ("bytes_sent", "bytes_received", "messages_sent", "messages_received",
                 "errors", "retries", "latency_sum_ms", "latency_count")

    def __init__(self):
        self.bytes_sent = 0
        self.bytes_received = 0
        self.messages_sent = 0
        self.messages_received = 0
        self.errors = 0
        self.retries = 0
        self.latency_sum_ms = 0.0
        self.latency_count = 0

    def record_send(self, byte_count: int, latency_ms: float = 0):
        self.bytes_sent += byte_count
        self.messages_sent += 1
        if latency_ms > 0:
            self.latency_sum_ms += latency_ms
            self.latency_count += 1

    def record_error(self):
        self.errors += 1

    @property
    def avg_latency_ms(self) -> float:
        return self.latency_sum_ms / max(self.latency_count, 1)

    def to_dict(self) -> dict:
        return {
            "bytes_sent": self.bytes_sent,
            "bytes_received": self.bytes_received,
            "messages_sent": self.messages_sent,
            "errors": self.errors,
            "retries": self.retries,
            "avg_latency_ms": round(self.avg_latency_ms, 2),
        }


class BaseTransport(ABC):
    """
    Soyut iletişim katmanı.
    Her protokol implementasyonu bunu extend eder.
    """

    def __init__(self, config: TransportConfig):
        self.config = config
        self.stats = TransportStats()
        self._connected = False

    @abstractmethod
    def connect(self) -> bool:
        """Bağlantı kur."""
        ...

    @abstractmethod
    def disconnect(self):
        """Bağlantı kapat."""
        ...

    @abstractmethod
    def send(self, topic: str, payload: dict) -> bool:
        """Veri gönder. Başarı durumu döner."""
        ...

    @abstractmethod
    def receive(self, timeout_ms: int = 1000) -> dict | None:
        """Veri al (opsiyonel, dinleyici cihazlar için)."""
        ...

    @property
    def is_connected(self) -> bool:
        return self._connected


# ── Protokol Implementasyonları ──


class HttpTransport(BaseTransport):
    """HTTP/REST — simülasyon ve test ortamı (mevcut BAĞ client wrapper)."""

    def connect(self) -> bool:
        self._connected = True
        logger.debug(f"HTTP transport bağlandı: {self.config.host}:{self.config.port}")
        return True

    def disconnect(self):
        self._connected = False

    def send(self, topic: str, payload: dict) -> bool:
        """Mevcut BAĞ client üzerinden gönderim (BaseCihaz.publish zaten yapıyor)."""
        start = time.monotonic()
        data = json.dumps(payload).encode()
        # Gerçek HTTP gönderimi BaseCihaz.publish() tarafından yapılır
        # Bu transport sadece istatistik ve encode/decode katmanı sağlar
        latency = (time.monotonic() - start) * 1000
        self.stats.record_send(len(data), latency)
        return True

    def receive(self, timeout_ms: int = 1000) -> dict | None:
        return None


class TcpTransport(BaseTransport):
    """TCP/IP — güvenilir bağlantı tabanlı (prod sensörler için)."""

    def __init__(self, config: TransportConfig):
        super().__init__(config)
        self._socket = None

    def connect(self) -> bool:
        """Prod'da gerçek TCP soket açar. Şimdilik simülasyon wrapper."""
        # Prod implementasyonu:
        # import socket
        # self._socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        # self._socket.connect((self.config.host, self.config.port))
        self._connected = True
        logger.info(f"TCP transport hazır: {self.config.host}:{self.config.port}")
        return True

    def disconnect(self):
        if self._socket:
            try:
                self._socket.close()
            except Exception:
                pass
        self._connected = False

    def send(self, topic: str, payload: dict) -> bool:
        if not self._connected:
            self.stats.record_error()
            return False
        start = time.monotonic()
        # Frame: [4-byte len][topic_len][topic][payload_json]
        data = json.dumps(payload).encode()
        topic_bytes = topic.encode()
        frame = len(topic_bytes).to_bytes(2, "big") + topic_bytes + data
        # Prod: self._socket.sendall(len(frame).to_bytes(4, "big") + frame)
        latency = (time.monotonic() - start) * 1000
        self.stats.record_send(len(frame), latency)
        return True

    def receive(self, timeout_ms: int = 1000) -> dict | None:
        # Prod: read frame from socket, parse, return
        return None


class UdpTransport(BaseTransport):
    """UDP — düşük gecikmeli telemetri akışı."""

    def connect(self) -> bool:
        # UDP bağlantısız — soket oluştur yeter
        self._connected = True
        logger.info(f"UDP transport hazır: {self.config.host}:{self.config.port}")
        return True

    def disconnect(self):
        self._connected = False

    def send(self, topic: str, payload: dict) -> bool:
        if not self._connected:
            self.stats.record_error()
            return False
        start = time.monotonic()
        data = json.dumps(payload).encode()
        # Prod: self._socket.sendto(data, (self.config.host, self.config.port))
        latency = (time.monotonic() - start) * 1000
        self.stats.record_send(len(data), latency)
        return True

    def receive(self, timeout_ms: int = 1000) -> dict | None:
        return None


class SerialTransport(BaseTransport):
    """RS-232/RS-485 — eski askeri donanım bağlantısı."""

    def connect(self) -> bool:
        # Prod: import serial; self._port = serial.Serial(self.config.extra["device"], self.config.baudrate)
        self._connected = True
        logger.info(f"Serial transport hazır: baud={self.config.baudrate}")
        return True

    def disconnect(self):
        self._connected = False

    def send(self, topic: str, payload: dict) -> bool:
        if not self._connected:
            self.stats.record_error()
            return False
        start = time.monotonic()
        # NMEA-benzeri frame: $TOPIC,json_payload*checksum\r\n
        data = json.dumps(payload, separators=(",", ":")).encode()
        frame = f"${topic},".encode() + data + b"\r\n"
        # Prod: self._port.write(frame)
        latency = (time.monotonic() - start) * 1000
        self.stats.record_send(len(frame), latency)
        return True

    def receive(self, timeout_ms: int = 1000) -> dict | None:
        return None


class GrpcTransport(BaseTransport):
    """gRPC — BAĞ modülüyle native iletişim."""

    def connect(self) -> bool:
        # Prod: grpc.insecure_channel(f"{host}:{port}") veya grpc.secure_channel(...)
        self._connected = True
        logger.info(f"gRPC transport hazır: {self.config.host}:{self.config.port}")
        return True

    def disconnect(self):
        self._connected = False

    def send(self, topic: str, payload: dict) -> bool:
        if not self._connected:
            self.stats.record_error()
            return False
        start = time.monotonic()
        data = json.dumps(payload).encode()
        # Prod: stub.Publish(PublishRequest(topic=topic, payload=data))
        latency = (time.monotonic() - start) * 1000
        self.stats.record_send(len(data), latency)
        return True

    def receive(self, timeout_ms: int = 1000) -> dict | None:
        return None


# ── Transport Factory ──

TRANSPORT_REGISTRY: dict[TransportType, type[BaseTransport]] = {
    TransportType.HTTP: HttpTransport,
    TransportType.TCP: TcpTransport,
    TransportType.UDP: UdpTransport,
    TransportType.SERIAL: SerialTransport,
    TransportType.GRPC: GrpcTransport,
}


def create_transport(config: TransportConfig) -> BaseTransport:
    """TransportConfig'e göre uygun transport oluştur."""
    cls = TRANSPORT_REGISTRY.get(config.transport_type)
    if not cls:
        logger.warning(f"Bilinmeyen transport: {config.transport_type}, HTTP'ye fallback")
        cls = HttpTransport
    return cls(config)


def transport_from_yaml(cihaz_config: dict) -> BaseTransport:
    """YAML cihaz tanımından transport oluştur.

    Örnek YAML:
      transport:
        type: tcp
        host: 192.168.1.50
        port: 5000
        timeout_ms: 3000
    """
    t_config = cihaz_config.get("transport", {})
    t_type_str = t_config.get("type", "http")
    try:
        t_type = TransportType(t_type_str)
    except ValueError:
        t_type = TransportType.HTTP
    config = TransportConfig(transport_type=t_type, **{k: v for k, v in t_config.items() if k != "type"})
    return create_transport(config)
