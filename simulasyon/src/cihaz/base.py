"""
BaseCihaz — Cihaz Soyutlama Katmanı (Device Abstraction Layer).

Tüm cihazlar (gerçek veya simüle) bu arayüzü implemente eder.
Prod'a geçişte kod değişikliği gerekmez — sadece sürücü (driver) değişir.

Veri akışı:
  Cihaz.start() → Cihaz kendi döngüsünde veri üretir
                 → BÜTÜN'e kayıt olur
                 → BAĞ topic'lerine yayın yapar
                 → Modüller BAĞ'dan subscribe ederek otomatik alır
"""

from __future__ import annotations

import logging
import threading
import time
import uuid
from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, Callable

logger = logging.getLogger("otuken.cihaz")


class CihazDurum(Enum):
    """Cihaz yaşam döngüsü durumları."""
    OLUSTURULDU = "olusturuldu"      # Created, not yet registered
    KAYITLI = "kayitli"              # Registered with BÜTÜN
    BAGLI = "bagli"                  # Connected to BAĞ
    CALISIYOR = "calisiyor"          # Running, producing data
    DURAKLATILDI = "duraklatildi"    # Paused
    DURDURULDU = "durduruldu"        # Stopped
    HATA = "hata"                    # Error state


class CihazTip(Enum):
    """Desteklenen cihaz tipleri."""
    RADAR = "radar"
    FLIR = "flir"
    OPTIK_KAMERA = "optik_kamera"
    AKUSTIK = "akustik"
    GPS = "gps"
    KOMUTA_TERMINALI = "komuta_terminali"
    PLATFORM = "platform"            # Araç/birlik platformu
    BIRIM = "birim"                  # Dost birim (GPS+telsiz+IFF+telemetri)


@dataclass
class CihazKimlik:
    """Cihaz kimlik bilgileri — BÜTÜN'e kayıtta kullanılır."""
    cihaz_id: str = field(default_factory=lambda: f"cihaz-{uuid.uuid4().hex[:8]}")
    tip: CihazTip = CihazTip.RADAR
    ad: str = ""
    surucu: str = "sim"              # "sim" veya gerçek sürücü adı
    kume: str = "GORGU"              # BÜTÜN küme adı (ULAK, GORGU, BUYRUK)
    konum: tuple[float, float] = (0.0, 0.0)  # (lat, lon)
    meta: dict[str, Any] = field(default_factory=dict)


@dataclass
class CihazAyar:
    """Cihaz yapılandırması — YAML'dan yüklenir."""
    frekans_hz: float = 1.0          # Veri üretim sıklığı
    menzil_km: float = 0.0           # Algılama menzili
    cozunurluk: int = 0              # Sensör çözünürlüğü
    fov_derece: float = 0.0          # Görüş alanı
    oncelik: int = 5                 # Varsayılan mesaj önceliği (1-8)
    ttl_ms: int = 5000               # Mesaj yaşam süresi
    ozel: dict[str, Any] = field(default_factory=dict)  # Sürücüye özel ayarlar


class BaseCihaz(ABC):
    """
    Soyut cihaz arayüzü.

    Her cihaz (gerçek veya simüle) bu sınıfı extend eder.
    Yaşam döngüsü:
        __init__ → register() → connect() → start() → [produce data] → stop()
    """

    def __init__(self, kimlik: CihazKimlik, ayar: CihazAyar):
        self.kimlik = kimlik
        self.ayar = ayar
        self.durum = CihazDurum.OLUSTURULDU

        # Bağlantı referansları — CihazYoneticisi tarafından enjekte edilir
        self._butun_client = None
        self._bag_client = None
        self._kutuk_client = None

        # Protokol adaptör — varsayılan HTTP, prod'da YAML'dan okunur
        self._transport = None  # Lazy init: set via inject_transport()

        # İM kimlik doğrulama — prod'da JWT/sertifika
        self._im_client = None
        self._auth_token: str | None = None

        # YAYIN şifreleme — prod'da AES-256-GCM
        self._yayin_client = None
        self._encryption_enabled = False

        # Zaman senkronizasyonu
        self._time_offset_ns: int = 0  # NTP/PTP kaynaklı saat farkı

        # İç durum
        self._thread: threading.Thread | None = None
        self._running = False
        self._tick_count = 0
        self._error_count = 0
        self._last_error = ""
        self._start_time = 0.0

        # Callback'ler — dışarıdan bağlanan modüllere veri iletimi
        self._on_data_callbacks: list[Callable] = []

        # İstatistikler
        self.stats = {
            "uretilen_veri": 0,
            "yayinlanan_mesaj": 0,
            "hatalar": 0,
            "son_veri_zamani": 0.0,
        }

    @property
    def cihaz_id(self) -> str:
        return self.kimlik.cihaz_id

    @property
    def tip(self) -> CihazTip:
        return self.kimlik.tip

    @property
    def calisiyor(self) -> bool:
        return self._running and self.durum == CihazDurum.CALISIYOR

    # ── Bağımlılık Enjeksiyonu ──

    def inject_clients(self, butun=None, bag=None, kutuk=None,
                        im=None, yayin=None, transport=None):
        """Modül istemcilerini enjekte et — CihazYoneticisi tarafından çağrılır."""
        self._butun_client = butun
        self._bag_client = bag
        self._kutuk_client = kutuk
        if im is not None:
            self._im_client = im
        if yayin is not None:
            self._yayin_client = yayin
        if transport is not None:
            self._transport = transport

    def on_data(self, callback: Callable):
        """Veri üretildiğinde çağrılacak callback ekle."""
        self._on_data_callbacks.append(callback)

    # ── Yaşam Döngüsü ──

    def register(self) -> bool:
        """BÜTÜN'e cihaz kaydı yap. Prod'da da aynı çağrı."""
        if not self._butun_client:
            logger.warning(f"[{self.cihaz_id}] BÜTÜN client yok, kayıt atlanıyor")
            self.durum = CihazDurum.KAYITLI
            return True
        try:
            self._butun_client.register_service(
                name=self.cihaz_id,
                cluster=self.kimlik.kume,
                port=0,  # Cihaz port dinlemez, sadece yayın yapar
            )
            self.durum = CihazDurum.KAYITLI
            logger.info(f"[{self.cihaz_id}] BÜTÜN'e kayıt oldu → küme: {self.kimlik.kume}")
            return True
        except Exception as e:
            self._error(f"Kayıt hatası: {e}")
            return False

    def connect(self) -> bool:
        """BAĞ mesaj kuyruğuna bağlan."""
        # BAĞ topic oluşturma opsiyonel — publish zaten topic yoksa oluşturur
        self.durum = CihazDurum.BAGLI
        logger.info(f"[{self.cihaz_id}] BAĞ'a bağlandı → topic: {self.topic}")
        return True

    def start(self) -> bool:
        """Cihazı başlat — kendi thread'inde veri üretmeye başlar."""
        if self._running:
            return True
        if self.durum not in (CihazDurum.KAYITLI, CihazDurum.BAGLI, CihazDurum.DURAKLATILDI):
            # Kayıt ve bağlantı yapılmamışsa otomatik yap
            if self.durum == CihazDurum.OLUSTURULDU:
                self.register()
                self.connect()

        self._running = True
        self._start_time = time.time()
        self.durum = CihazDurum.CALISIYOR
        self._thread = threading.Thread(
            target=self._run_loop,
            name=f"cihaz-{self.cihaz_id}",
            daemon=True,
        )
        self._thread.start()
        logger.info(f"[{self.cihaz_id}] Başlatıldı — frekans: {self.ayar.frekans_hz}Hz")
        return True

    def stop(self):
        """Cihazı durdur."""
        self._running = False
        self.durum = CihazDurum.DURDURULDU
        if self._thread and self._thread.is_alive():
            self._thread.join(timeout=2.0)
        logger.info(f"[{self.cihaz_id}] Durduruldu — üretilen: {self.stats['uretilen_veri']}")

    def pause(self):
        """Cihazı duraksat."""
        self._running = False
        self.durum = CihazDurum.DURAKLATILDI

    # ── Veri Yayını ──

    def publish(self, topic: str, data: dict, priority: int | None = None):
        """BAĞ'a veri yayınla. Tüm cihazlar bu metodu kullanır.

        Prod katmanları (opsiyonel, varsa otomatik aktif):
          1. Zaman damgası: NTP/PTP offset uygulanır
          2. İM auth: JWT token eklenir
          3. YAYIN şifreleme: payload encrypt edilir
          4. Transport: uygun protokol ile gönderilir
        """
        prio = priority or self.ayar.oncelik
        envelope = {
            "id": uuid.uuid4().hex,
            "source": self.cihaz_id,
            "source_type": self.kimlik.tip.value,
            "driver": self.kimlik.surucu,
            "timestamp_ns": time.time_ns() + self._time_offset_ns,
            "data": data,
            "priority": prio,
        }

        # Prod katman 1: İM kimlik doğrulama header
        if self._auth_token:
            envelope["auth_token"] = self._auth_token

        # Prod katman 2: YAYIN şifreli yayın
        # YAYIN modülü sunucu tarafında şifreleme yapar — biz sadece publish çağırırız
        if self._encryption_enabled and self._yayin_client:
            try:
                import json as _json
                self._yayin_client.publish(
                    channel_id=topic,
                    sender_id=self.cihaz_id,
                    content_type="application/json",
                    payload=_json.dumps(envelope),
                    priority=envelope.get("priority", 1),
                )
            except Exception as e:
                pass  # YAYIN kapalıysa sessizce geç — BAĞ yolundan devam eder

        # Prod katman 3: Transport adaptör
        if self._transport and self._transport.is_connected:
            try:
                self._transport.send(topic, envelope)
            except Exception as e:
                self._error(f"Transport gönderim hatası: {e}")

        # BAĞ client (her zaman — transport üstündeki fallback/birincil yol)
        if self._bag_client:
            try:
                self._bag_client.publish(topic, envelope, priority=prio)
                self.stats["yayinlanan_mesaj"] += 1
            except Exception as e:
                self._error(f"BAĞ yayın hatası: {e}")

        # Callback'leri çağır (doğrudan modül entegrasyonu için)
        for cb in self._on_data_callbacks:
            try:
                cb(topic, envelope)
            except Exception:
                pass

        self.stats["uretilen_veri"] += 1
        self.stats["son_veri_zamani"] = time.time()

    def store(self, namespace: str, key: str, value: dict, ttl_ms: int | None = None):
        """KÜTÜK'e veri depola."""
        if self._kutuk_client:
            try:
                self._kutuk_client.put(namespace, key, value, ttl_ms=ttl_ms or self.ayar.ttl_ms)
            except Exception as e:
                self._error(f"KÜTÜK depolama hatası: {e}")

    # ── Prod Katmanları — Opsiyonel, varsa otomatik aktif ──

    def authenticate(self) -> bool:
        """İM modülüyle cihaz kimlik doğrulaması yap.

        Gerçek akış (prod'da da simülasyonda da aynı):
          1. İM'e enroll → cihaz kimliği oluştur
          2. JWT token al → her publish'te header olarak ekle
        İM client yoksa atlanır (graceful degradation).
        """
        if not self._im_client:
            return True  # İM yok — atla

        try:
            # 1. Cihaz kaydı — İM'e enroll (cihaz kimliği + sertifika + token bir arada)
            enroll_result = self._im_client.enroll(
                unit_id=self.cihaz_id,
                unit_type=self.kimlik.tip.value,
                roles=["device.publish", "device.subscribe"],
                name=self.kimlik.ad,
            )
            # 409 Conflict = zaten kayıtlı — token_issue ile devam et
            if enroll_result.get("status_code") == 409:
                logger.info(f"[{self.cihaz_id}] İM: zaten kayıtlı, token yenileniyor")
                token_result = self._im_client.token_issue(
                    unit_id=self.cihaz_id,
                    roles=["device"],
                    perms=["publish", "subscribe", "store"],
                )
                self._auth_token = token_result.get("access_token") or token_result.get("token")
                if self._auth_token:
                    logger.info(f"[{self.cihaz_id}] İM token yenilendi")
                    return True
                return False
            if enroll_result.get("error"):
                logger.warning(f"[{self.cihaz_id}] İM enroll hatası: {enroll_result['error']}")
                return False

            # Enroll başarılıysa access_token zaten dönüyor
            self._auth_token = enroll_result.get("access_token")
            if self._auth_token:
                logger.info(f"[{self.cihaz_id}] İM auth başarılı — token alındı")
                return True

            # Token yoksa ayrıca token_issue çağır
            token_result = self._im_client.token_issue(
                unit_id=self.cihaz_id,
                roles=["device"],
                perms=["publish", "subscribe", "store"],
            )
            self._auth_token = token_result.get("access_token") or token_result.get("token")
            if self._auth_token:
                logger.info(f"[{self.cihaz_id}] İM token_issue başarılı")
            return True
        except Exception as e:
            self._error(f"İM auth hatası: {e}")
            return False

    def enable_encryption(self) -> bool:
        """YAYIN şifreleme katmanını aktif et.

        Gerçek akış: YAYIN'dan mevcut kripto anahtarlarını al.
        YAYIN client yoksa atlanır.
        """
        if not self._yayin_client:
            return False

        try:
            keys = self._yayin_client.crypto_keys()
            if keys.get("error"):
                logger.warning(f"[{self.cihaz_id}] YAYIN şifreleme anahtarı alınamadı")
                return False
            self._encryption_enabled = True
            logger.info(f"[{self.cihaz_id}] YAYIN şifreleme aktif")
            return True
        except Exception as e:
            self._error(f"YAYIN şifreleme hatası: {e}")
        return False

    def sync_time(self, offset_ns: int = 0):
        """NTP/PTP zaman senkronizasyonu.

        Prod'da: NTP sunucusundan veya PTP master'dan alınan offset.
        Simülasyonda: 0 (yerel saat kullanılır).
        """
        self._time_offset_ns = offset_ns

    # ── Soyut Metodlar — Sürücüler İmplemente Eder ──

    @property
    @abstractmethod
    def topic(self) -> str:
        """Bu cihazın yayın yaptığı BAĞ topic'i. Örn: 'sensor.radar'"""

    @abstractmethod
    def on_tick(self) -> list[dict]:
        """
        Bir tick'te üretilecek verileri döndür.
        Her dict bir mesaj payload'ıdır.
        Boş liste dönerse yayın yapılmaz.
        """

    def on_start(self):
        """Cihaz başladığında çağrılır (opsiyonel override)."""
        pass

    def on_stop(self):
        """Cihaz durduğunda çağrılır (opsiyonel override)."""
        pass

    # ── İç Döngü ──

    def _run_loop(self):
        """Cihaz ana döngüsü — thread içinde çalışır."""
        self.on_start()
        interval = 1.0 / self.ayar.frekans_hz if self.ayar.frekans_hz > 0 else 1.0

        while self._running:
            try:
                payloads = self.on_tick()
                for payload in payloads:
                    self.publish(self.topic, payload)
                self._tick_count += 1
            except Exception as e:
                self._error(f"Tick hatası: {e}")

            time.sleep(interval)

        self.on_stop()

    # ── Tek Tick (dış döngü için — thread kullanmadan) ──

    def tick_once(self) -> list[dict]:
        """
        Tek bir tick çalıştır ve veri üret.
        CihazYoneticisi senaryonun tick döngüsünden çağırabilir.
        Thread yerine bu kullanılabilir.
        """
        if self.durum not in (CihazDurum.CALISIYOR, CihazDurum.BAGLI, CihazDurum.KAYITLI):
            return []
        try:
            payloads = self.on_tick()
            for payload in payloads:
                self.publish(self.topic, payload)
            self._tick_count += 1
            return payloads
        except Exception as e:
            self._error(f"Tick hatası: {e}")
            return []

    # ── Yardımcılar ──

    def _error(self, msg: str):
        self._error_count += 1
        self._last_error = msg
        self.stats["hatalar"] += 1
        logger.error(f"[{self.cihaz_id}] {msg}")

    def health(self) -> dict:
        """Cihaz sağlık raporu — DİRLİK tarafından sorgulanır."""
        uptime = time.time() - self._start_time if self._start_time else 0
        return {
            "cihaz_id": self.cihaz_id,
            "tip": self.kimlik.tip.value,
            "surucu": self.kimlik.surucu,
            "durum": self.durum.value,
            "calisiyor": self.calisiyor,
            "konum": list(self.kimlik.konum),
            "tick_count": self._tick_count,
            "uptime_s": round(uptime, 1),
            "stats": dict(self.stats),
            "hata_sayisi": self._error_count,
            "son_hata": self._last_error,
        }

    def __repr__(self):
        return f"<{self.__class__.__name__} id={self.cihaz_id} tip={self.kimlik.tip.value} durum={self.durum.value}>"
