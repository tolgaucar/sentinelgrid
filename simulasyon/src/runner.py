"""
Senaryo Çalıştırıcısı

Senaryoları yükler, çalıştırır ve sonuçlarını raporlar.
Dashboard ile entegre çalışır.
"""

from __future__ import annotations

import importlib
import time
import yaml
from pathlib import Path
from typing import Any

from src.scenarios.base import BaseScenario
from src.scenarios.temel_senaryo import TemelSenaryo

# Kayıtlı senaryolar
SCENARIO_REGISTRY = {
    "temel": TemelSenaryo,
}


def load_config(config_path: str = None) -> dict:
    """YAML konfigürasyon dosyasını yükle."""
    if config_path is None:
        config_path = str(Path(__file__).parent.parent / "config" / "default.yaml")
    with open(config_path, "r", encoding="utf-8") as f:
        return yaml.safe_load(f)


class SimulationRunner:
    """Simülasyon çalıştırıcısı."""

    def __init__(self, config: dict = None, config_path: str = None):
        if config is None:
            config = load_config(config_path)
        self.config = config
        self.scenario: BaseScenario = None
        self.results: dict = {}

    def select_scenario(self, name: str = "temel") -> BaseScenario:
        """Senaryo seç ve hazırla."""
        cls = SCENARIO_REGISTRY.get(name)
        if cls is None:
            raise ValueError(f"Bilinmeyen senaryo: {name}. Mevcut: {list(SCENARIO_REGISTRY.keys())}")
        sim_cfg = self.config.get("simulation", {})
        # Modül bilgilerini simülasyon config'ine aktar
        if "modules" not in sim_cfg and "modules" in self.config:
            sim_cfg["modules"] = self.config["modules"]
        self.scenario = cls(config=sim_cfg)
        return self.scenario

    def run(self, scenario_name: str = "temel", callback=None) -> dict:
        """Senaryo çalıştır."""
        scenario = self.select_scenario(scenario_name)

        # Eğitici giriş bilgisi
        self._print_educational_intro(scenario)

        try:
            self.results = scenario.run(callback=callback)
        except KeyboardInterrupt:
            scenario.stop()
            self.results = scenario.on_finish()
            print("\n⚠️  Simülasyon kullanıcı tarafından durduruldu")
        finally:
            scenario.close()

        self._print_summary()
        self._print_educational_outro()
        return self.results

    def _print_educational_intro(self, scenario: BaseScenario) -> None:
        """Simülasyon başlamadan önce eğitici giriş bilgisi yazdır."""
        print("""
╔══════════════════════════════════════════════════════════════════╗
║          🏴 ÖTÜKEN-MATRİS SİMÜLASYON PLATFORMU                ║
║          Yazılım Tanımlı C4ISR Savunma Sistemi                  ║
╚══════════════════════════════════════════════════════════════════╝

📚 ÖTÜKEN-MATRİS NEDİR?
   ÖTÜKEN-MATRİS, 60 modülden oluşan yazılım tanımlı bir C4ISR
   (Komuta, Kontrol, Haberleşme, Bilgisayar, İstihbarat, Gözetleme,
   Keşif) savunma platformudur. Gerçek savaş alanı koşullarını
   simüle ederek tüm modüllerin birlikte çalışmasını test eder.

🎯 BU SİMÜLASYON NE YAPAR?
   Sanal bir savaş alanı oluşturur: dost birlikler, düşman kuvvetler,
   İHA'lar ve sensörler yerleştirilir. Her "tick" (adım) zamanında:
   - Radar, termal, akustik sensörler veri üretir
   - Birlikler hareket eder, çatışmalar olur
   - Komuta mesajları (emir, rapor, istihbarat) gönderilir
   - Tüm bu veriler 20 modülden geçirilir

🧩 8 TEMEL MODÜL VE GÖREVLERİ:

   🏛️  BÜTÜN (Orkestratör)
       Ana yönetim sistemi. Tüm modülleri başlatır, durdurur, izler.
       Servis kayıt defteri tutar, heartbeat ile canlılık kontrol eder.
       Bir modül çökerse yeniden başlatır.

   📡 BAĞ (Mesaj Kuyruğu)
       Modüller arası veri taşıyıcı. Topic-based publish/subscribe
       mimarisi kullanır. Sensör verisi, komuta mesajı, alarm —
       her şey BAĞ üzerinden akar.

   🗄️  KÜTÜK (Anahtar-Değer Deposu)
       Anlık durum veritabanı. RAFT konsensüs ile tutarlı.
       Varlık konumları, sensör okumaları, sistem durumları burada
       saklanır. Dağıtık ortamda bile tutarlılık garanti eder.

   🏥 DİRLİK (Sağlık İzleme)
       Tüm sistemin nabzını tutar. CPU, RAM, disk kullanımını ölçer.
       Servis probe'ları ile her modülün canlılığını kontrol eder.
       Anomali tespit ederse alarm üretir.

   🔄 BAĞDAŞ (Veri Dönüştürücü)
       Farklı veri formatları arasında köprü kurar.
       JSON ↔ MsgPack ↔ CBOR dönüşümü yapar.
       Şema doğrulama ve veri pipeline'ları çalıştırır.

   🌊 AKIM (Ağ Yönlendirici)
       Veri trafiğini yönetir. QoS (Hizmet Kalitesi) bazlı
       önceliklendirme yapar. Kritik alarm verisi → düşük gecikme,
       telemetri → normal kuyruk. Tıkanıklık yönetimi sağlar.

   🏕️  YURT (Uç Bilişim)
       Sahada çalışan edge düğüm. Merkeze bağlanamadığında bile
       yerel depolama ile çalışmaya devam eder. Bağlantı gelince
       senkronize olur. Bölünme (partition) toleransı sağlar.

   🗺️  YÜZEY (Taktiksel Görüntüleme)
       Savaş alanı haritası. Tüm varlıkları, sensör verilerini,
       alarm bölgelerini katmanlı olarak görüntüler.
       Komutanın "büyük resmi" görmesini sağlar.
""")
        print(f"🎯 Senaryo: {scenario.name}")
        print(f"   {scenario.description}")
        print(f"   Toplam adım: {scenario.max_ticks} | Adım aralığı: {scenario.tick_interval}s")
        print(f"   Tahmini süre: ~{scenario.max_ticks * scenario.tick_interval:.0f}s")
        print("-" * 60)
        print("   Simülasyon başlıyor... Her adımda neler olduğunu izleyin.\n")

    def _print_summary(self) -> None:
        """Kapsamlı sonuç raporu yazdır."""
        r = self.results
        stats = r.get("stats", {})
        duration = r.get("duration_sec", (stats.get("end_time", 0) - stats.get("start_time", 0)))
        tp = r.get("throughput", {})
        mods = r.get("modules", {})
        sensors = r.get("sensors", {})
        c2 = r.get("c2_messages", {})
        battle = r.get("battle_events", {})
        bf = r.get("battlefield", {})

        print("\n" + "═" * 60)
        print("📊 ÖTÜKEN-MATRİS SİMÜLASYON RAPORU")
        print("═" * 60)

        # Genel
        print(f"\n⚡ GENEL PERFORMANS")
        print(f"  Senaryo        : {r.get('scenario', '?')}")
        print(f"  Süre           : {duration:.2f}s ({r.get('ticks', 0)}/{r.get('max_ticks', 0)} tick)")
        print(f"  Toplam Mesaj   : {stats.get('messages_sent', 0)}")
        print(f"  Toplam Hata    : {stats.get('errors', 0)}")
        print(f"  Toplam Olay    : {r.get('event_count', 0)}")
        print(f"  Mesaj/saniye   : {tp.get('messages_per_sec', 0)}")
        print(f"  Pik mesaj/tick : {tp.get('peak_per_tick', 0)}")

        # BÜTÜN
        b = mods.get("butun", {})
        bt = (b.get("requests", 0) + b.get("registrations", 0) + b.get("heartbeats", 0))
        print(f"\n🏛️  BÜTÜN — Ana Çatı Sistemi ({bt} işlem)")
        print(f"  Kayıt: {b.get('registrations', 0)} | Heartbeat: {b.get('heartbeats', 0)} | Keşif: {b.get('discoveries', 0)} | Hata: {b.get('errors', 0)}")

        # BAĞ
        bg = mods.get("bag", {})
        bag_kb = bg.get("bytes_sent", 0) / 1024
        print(f"\n📡 BAĞ — Veri Transfer Katmanı ({bg.get('publishes', 0) or (bg.get('sensor_msgs',0)+bg.get('c2_msgs',0))} publish)")
        print(f"  Sensör: {bg.get('sensor_msgs', 0)} | C2: {bg.get('c2_msgs', 0)} | Alarm: {bg.get('alert_msgs', 0)} | Veri: {bag_kb:.1f}KB | Hata: {bg.get('errors', 0)}")

        # KÜTÜK
        k = mods.get("kutuk", {})
        kt = k.get("puts", 0) + k.get("gets", 0) + k.get("deletes", 0)
        print(f"\n🗄️  KÜTÜK — Anlık Durum Veritabanı ({kt} işlem)")
        print(f"  PUT: {k.get('puts', 0)} | GET: {k.get('gets', 0)} | Varlık: {k.get('entity_writes', 0)} | Sensör: {k.get('sensor_writes', 0)} | Hata: {k.get('errors', 0)}")

        # DİRLİK
        dl = mods.get("dirlik", {})
        dlt = dl.get("checks", 0)
        print(f"\n🏥 DİRLİK — Sistem Sağlık Taraması ({dlt} kontrol)")
        print(f"  Metrik: {dl.get('metrics', 0)} | Probe: {dl.get('probes', 0)} | Alarm: {dl.get('alerts', 0)} | Hata: {dl.get('errors', 0)}")

        # BAĞDAŞ
        bd = mods.get("bagdas", {})
        bdt = (bd.get("converts", 0) + bd.get("validates", 0) + bd.get("pipelines", 0) + bd.get("bridges", 0))
        print(f"\n🔄 BAĞDAŞ — Polyglot Köprü ({bdt} işlem)")
        print(f"  Dönüşüm: {bd.get('converts', 0)} | Doğrulama: {bd.get('validates', 0)} | Pipeline: {bd.get('pipelines', 0)} | Köprü: {bd.get('bridges', 0)} | Hata: {bd.get('errors', 0)}")

        # AKIM
        ak = mods.get("akim", {})
        akt = (ak.get("routes", 0) + ak.get("flows", 0) + ak.get("balancer_selects", 0) + ak.get("congestion_checks", 0))
        print(f"\n🌊 AKIM — Veri Trafiği Yönlendirme ({akt} işlem)")
        print(f"  Rota: {ak.get('routes', 0)} | Akış: {ak.get('flows', 0)} | Dengeleme: {ak.get('balancer_selects', 0)} | Tıkanıklık: {ak.get('congestion_checks', 0)} | Hata: {ak.get('errors', 0)}")

        # YURT
        yt = mods.get("yurt", {})
        ytt = (yt.get("store_puts", 0) + yt.get("store_gets", 0) + yt.get("syncs", 0) + yt.get("proxy_reqs", 0) + yt.get("partition_checks", 0))
        print(f"\n🏕️  YURT — Uç Bilişim ve Yerel Depolama ({ytt} işlem)")
        print(f"  Yazma: {yt.get('store_puts', 0)} | Okuma: {yt.get('store_gets', 0)} | Senk: {yt.get('syncs', 0)} | Proxy: {yt.get('proxy_reqs', 0)} | Bölünme: {yt.get('partition_checks', 0)} | Hata: {yt.get('errors', 0)}")

        # YÜZEY
        yz = mods.get("yuzey", {})
        yzt = (yz.get("tactical_updates", 0) + yz.get("entity_updates", 0) + yz.get("events_emitted", 0) + yz.get("layer_updates", 0) + yz.get("health_checks", 0))
        print(f"\n🗺️  YÜZEY — Taktiksel İzleme ({yzt} işlem)")
        print(f"  Taktik: {yz.get('tactical_updates', 0)} | Varlık Gün.: {yz.get('entity_updates', 0)} | Olay: {yz.get('events_emitted', 0)} | Katman: {yz.get('layer_updates', 0)} | Sağlık: {yz.get('health_checks', 0)} | Hata: {yz.get('errors', 0)}")

        # Sensörler
        sensor_total = sum(sensors.values())
        print(f"\n📡 SENSÖR DAĞILIMI ({sensor_total} toplam)")
        for s, c in sorted(sensors.items(), key=lambda x: -x[1]):
            bar = "█" * int(c / max(sensor_total, 1) * 30)
            print(f"  {s:10s} {bar} {c}")

        # C2
        c2_total = sum(c2.values())
        names = {"order": "Emir", "report": "Rapor", "intel": "İstihbarat", "alert": "Alarm"}
        if c2_total:
            print(f"\n📨 KOMUTA MESAJLARI ({c2_total} toplam)")
            for t, c in sorted(c2.items(), key=lambda x: -x[1]):
                if c > 0:
                    print(f"  {names.get(t, t):12s} : {c}")

        # Savaş olayları
        battle_total = sum(battle.values())
        if battle_total:
            print(f"\n💥 SAVAŞ OLAYLARI ({battle_total} toplam)")
            bn = {"damage": "Hasar", "destroyed": "İmha", "course_change": "Rota Değişimi", "low_fuel": "Düşük Yakıt", "alerts": "Alarm"}
            for t, c in battle.items():
                if c > 0:
                    print(f"  {bn.get(t, t):16s} : {c}")

        # Savaş alanı
        if bf:
            print(f"\n🗺️  SAVAŞ ALANI")
            print(f"  Toplam: {bf.get('total_entities', 0)} | Aktif: {bf.get('active', 0)} | Hasarlı: {bf.get('damaged', 0)} | İmha: {bf.get('destroyed', 0)}")
            print(f"  Ort. Sağlık: %{bf.get('avg_health', 0)} | Ort. Yakıt: %{bf.get('avg_fuel', 0)}")
            if bf.get("by_faction"):
                fn = {"friendly": "Dost", "enemy": "Düşman", "neutral": "Tarafsız"}
                parts = [f"{fn.get(f,f)}: {c}" for f, c in bf["by_faction"].items()]
                print(f"  Saflar: {' | '.join(parts)}")

        print("\n" + "═" * 60)

    def _print_educational_outro(self) -> None:
        """Simülasyon sonrası eğitici açıklama yazdır."""
        r = self.results
        mods = r.get("modules", {})
        stats = r.get("stats", {})
        total_msgs = stats.get("messages_sent", 0)
        total_errors = stats.get("errors", 0)

        print("""
╔══════════════════════════════════════════════════════════════════╗
║              📚 SİMÜLASYON ANALİZ VE ÖĞRENİM RAPORU           ║
╚══════════════════════════════════════════════════════════════════╝

🔍 SİMÜLASYONDA NE OLDU?
   Bu simülasyonda sanal bir savaş alanı canlandırıldı.
   Dost ve düşman birlikler yerleştirildi, sensörler
   aktifleştirildi ve gerçek zamanlı veri akışı başladı.
   Her "tick" (adım) bir zaman dilimini temsil eder.
""")

        # Modül bazında eğitici açıklama
        print("📖 HER MODÜL NE YAPTI?\n")

        b = mods.get("butun", {})
        regs = b.get("registrations", 0)
        hbs = b.get("heartbeats", 0)
        print(f"   🏛️  BÜTÜN — {regs} servis kaydetti, {hbs} heartbeat gönderdi.")
        print(f"       → Tüm modüllerin \"canlı\" olduğunu doğruladı.")
        print(f"       → Gerçek ortamda: Çöken servisleri otomatik yeniden başlatır.\n")

        bg = mods.get("bag", {})
        s_msgs = bg.get("sensor_msgs", 0)
        c2_msgs = bg.get("c2_msgs", 0)
        print(f"   📡 BAĞ — {s_msgs} sensör verisi + {c2_msgs} komuta mesajı taşıdı.")
        print(f"       → Veriler topic'lere (konulara) ayrılarak iletildi.")
        print(f"       → Gerçek ortamda: Milisaniye gecikmeyle güvenli mesajlaşma.\n")

        k = mods.get("kutuk", {})
        puts = k.get("puts", 0)
        e_writes = k.get("entity_writes", 0)
        print(f"   🗄️  KÜTÜK — {puts} yazma işlemi yaptı, {e_writes} varlık durumu kaydetti.")
        print(f"       → Her birlik ve sensörün anlık durumunu sakladı.")
        print(f"       → Gerçek ortamda: RAFT konsensüs ile 3+ düğümde tutarlı.\n")

        dl = mods.get("dirlik", {})
        checks = dl.get("checks", 0)
        alerts = dl.get("alerts", 0)
        print(f"   🏥 DİRLİK — {checks} sağlık kontrolü yaptı, {alerts} alarm tespit etti.")
        print(f"       → CPU, RAM, disk ve servis durumlarını sürekli izledi.")
        print(f"       → Gerçek ortamda: Anormal durumlarda otomatik alarm ve müdahale.\n")

        bd = mods.get("bagdas", {})
        converts = bd.get("converts", 0)
        validates = bd.get("validates", 0)
        print(f"   🔄 BAĞDAŞ — {converts} format dönüşümü + {validates} şema doğrulama yaptı.")
        print(f"       → JSON → MsgPack → CBOR: Farklı sistemler arasında köprü kurdu.")
        print(f"       → Gerçek ortamda: NATO STANAG formatlarıyla uyumluluk sağlar.\n")

        ak = mods.get("akim", {})
        routes = ak.get("routes", 0)
        flows = ak.get("flows", 0)
        cong = ak.get("congestion_checks", 0)
        print(f"   🌊 AKIM — {routes} rota hesapladı, {flows} akış oluşturdu, {cong} tıkanıklık kontrolü.")
        print(f"       → Kritik veriler (alarm) düşük gecikme, telemetri normal kuyrukta.")
        print(f"       → Gerçek ortamda: QoS ile bant genişliği optimizasyonu sağlar.\n")

        yt = mods.get("yurt", {})
        sp = yt.get("store_puts", 0)
        sg = yt.get("store_gets", 0)
        syncs = yt.get("syncs", 0)
        pc = yt.get("partition_checks", 0)
        print(f"   🏕️  YURT — {sp} yazma, {sg} okuma, {syncs} senkronizasyon, {pc} bölünme kontrolü.")
        print(f"       → Sahada internet kesilse bile veriler yerel olarak saklandı.")
        print(f"       → Gerçek ortamda: Bağlantı gelince merkez ile otomatik senkronize.\n")

        yz = mods.get("yuzey", {})
        eu = yz.get("entity_updates", 0)
        tu = yz.get("tactical_updates", 0)
        ev = yz.get("events_emitted", 0)
        print(f"   🗺️  YÜZEY — {eu} varlık güncellemesi, {tu} taktiksel resim, {ev} olay yayını.")
        print(f"       → Komutan için harita üzerinde canlı durum resmi oluşturuldu.")
        print(f"       → Gerçek ortamda: Katmanlı harita ile dost/düşman görüntüleme.\n")

        # Genel değerlendirme
        error_rate = (total_errors / total_msgs * 100) if total_msgs > 0 else 0
        print("═" * 60)
        print(f"\n📊 GENEL DEĞERLENDİRME")
        print(f"   Toplam mesaj: {total_msgs} | Toplam hata: {total_errors} | Hata oranı: %{error_rate:.1f}")
        if error_rate < 1:
            print("   ✅ Sistem mükemmel çalıştı — hata oranı %1'in altında.")
        elif error_rate < 5:
            print("   ⚠️  Kabul edilebilir performans — bazı hatalar oluştu.")
        else:
            print("   ❌ Yüksek hata oranı — modül bağlantıları kontrol edilmeli.")

        bf = r.get("battlefield", {})
        if bf:
            destroyed = bf.get("destroyed", 0)
            total_ent = bf.get("total_entities", 1)
            print(f"\n   Savaş alanı: {total_ent} varlık, {destroyed} imha ({destroyed/max(total_ent,1)*100:.0f}%)")
            print(f"   Ortalama sağlık: %{bf.get('avg_health', 0)} | Ortalama yakıt: %{bf.get('avg_fuel', 0)}")

        print(f"""
💡 SONUÇ
   Bu simülasyon, ÖTÜKEN-MATRİS'in 8 temel modülünün birlikte
   nasıl çalıştığını gösterdi. Gerçek bir C4ISR sisteminde
   bu modüller fiziksel sensörler, gerçek haberleşme altyapısı
   ve operatör konsollarıyla entegre çalışır.

   ℹ️  Detaylı bilgi: plan.md dosyasını inceleyin.
   🔧 Simülasyon sadece development ortamında çalışır.
       (OTUKEN_ENV=development)
{"═" * 60}
""")
