"""
Temel Savaş Alanı Senaryosu

Cihaz Soyutlama Katmanı ile çalışır:
- Sensörler (radar, FLIR, optik, akustik) → CihazYoneticisi aracılığıyla veri üretir
- Komuta terminali → C2 mesajlarını BAĞ'a yayınlar
- Platform → Varlık durumunu YÜZEY'e iletir
- Modül testleri (BAĞDAŞ, AKIM, YURT vb.) → Altyapı doğrulaması

Prod'a geçişte sadece config/cihazlar.yaml değişir, bu dosyada değişiklik gerekmez.
"""

from __future__ import annotations

import json
import random
import sys

from src.scenarios.base import BaseScenario


class TemelSenaryo(BaseScenario):
    """Temel savaş alanı senaryosu — cihaz tabanlı mimari."""

    name = "Temel Savaş Alanı"
    description = "Standart keşif ve temas senaryosu — cihaz soyutlama katmanı ile sensör, C2 ve platform verileri"

    def setup(self) -> None:
        """Savaş alanı, cihazlar ve servisleri hazırla."""
        bf_cfg = self.config.get("battlefield", {})
        self.battlefield_gen.generate_battlefield(
            friendly=bf_cfg.get("friendly_units", bf_cfg.get("friendly_count", 20)),
            enemy=bf_cfg.get("enemy_units", bf_cfg.get("enemy_count", 15)),
            drones=bf_cfg.get("drone_count", 10),
            sensors=bf_cfg.get("sensor_count", 30),
        )

        # ── Cihazları kayıt et ve başlat (tıpkı gerçek hayatta olduğu gibi) ──
        self.cihaz_yoneticisi.register_all()
        self.cihaz_yoneticisi.connect_all()
        self.cihaz_yoneticisi.start_all(threaded=False)  # Tick bazlı çalışma

        self.module_stats["cihaz"]["toplam_cihaz"] = self.cihaz_yoneticisi.cihaz_sayisi

        # ── BÜTÜN'e modül kaydı ──
        if self.config.get("connect_modules", False):
            try:
                self.butun.register_service("BAG", "ULAK", 9091)
                self.butun.register_service("KUTUK", "ULAK", 9093)
                self.butun.register_service("DIRLIK", "ULAK", 9095)
                self.butun.register_service("BAGDAS", "ULAK", 9097)
                self.butun.register_service("AKIM", "ULAK", 9099)
                self.butun.register_service("YURT", "ULAK", 9101)
                self.butun.register_service("YUZEY", "ULAK", 9103)
                self.butun.register_service("IM", "ULAK", 9105)
                self.butun.register_service("YAYIN", "ULAK", 9107)
                self.butun.register_service("DURAK", "ULAK", 9109)
                self.butun.register_service("TUTUM", "ULAK", 9111)
                self.butun.register_service("TANI", "GORGU", 9113)
                self.butun.register_service("KOZ", "GORGU", 9115)
                self.butun.register_service("SEZI", "GORGU", 9116)
                self.butun.register_service("YANKI", "GORGU", 9118)
                self.butun.register_service("DUYUM", "GORGU", 9120)
                self.butun.register_service("BASKALA", "GORGU", 9122)
                self.butun.register_service("SOZGEC", "GORGU", 9124)
                self.butun.register_service("IZLEK", "GORGU", 9126)
                self.module_stats["butun"]["registrations"] += 19
                self.module_stats["butun"]["requests"] += 19
            except Exception:
                self.module_stats["butun"]["errors"] += 1

    def _build_entity_list(self) -> list[dict]:
        """Savaş alanı varlıklarını cihaz formatına dönüştür."""
        entity_list = []
        for eid, entity in self.battlefield_gen.entities.items():
            faction = entity.faction.value
            if faction == "friendly":
                intel = 4
            elif faction == "neutral":
                intel = 3
            else:
                base_conf = random.uniform(0.3, 0.95)
                if entity.status.value == "destroyed":
                    intel = 3
                elif entity.speed_kmh < 5:
                    intel = min(3, int(base_conf * 4) + 1)
                else:
                    intel = max(1, int(base_conf * 3))

            entity_list.append({
                "id": eid,
                "type": entity.unit_type.value,
                "faction": faction,
                "name": entity.name,
                "callsign": entity.name,
                "status": entity.status.value,
                "lat": entity.lat,
                "lon": entity.lon,
                "altitude": entity.altitude_m,
                "heading": entity.heading_deg,
                "speed": entity.speed_kmh,
                "health": entity.health_percent,
                "fuel": entity.fuel_percent,
                "ammo": getattr(entity, "ammo_percent", 100),
                "intel_level": intel,
            })
        return entity_list

    def on_tick(self, tick: int) -> list[dict]:
        """Her tick'te: cihazlar veri üretir, modüller gerçekten çalışır."""
        events = []

        # ══════════════════════════════════════════════════════
        # BÖLÜM 1: FİZİK MOTORU — Varlıkları hareket ettir
        # ══════════════════════════════════════════════════════
        move_events = self.battlefield_gen.tick(duration_sec=self.tick_interval)
        for ev in move_events:
            ev_type = ev.get("type", "")
            if ev_type == "damage":
                self.battle_events["damage"] += 1
                if ev.get("health", 100) <= 0:
                    self.battle_events["destroyed"] += 1
            elif ev_type == "course_change":
                self.battle_events["course_change"] += 1
            elif ev_type == "low_fuel":
                self.battle_events["low_fuel"] += 1
        events.extend(move_events)

        # ══════════════════════════════════════════════════════
        # BÖLÜM 2: CİHAZ KATMANI — Sensörler ve cihazlar veri üretir
        # (Tıpkı gerçek hayatta: her cihaz kendi verisini üretir ve BAĞ'a yayınlar)
        # ══════════════════════════════════════════════════════
        entity_list = self._build_entity_list()

        # Dost birimleri cihaz olarak senkronize et
        # (Her dost birimin GPS/telsiz/IFF/telemetri cihazı otomatik oluşturulur)
        birim_count = self.cihaz_yoneticisi.sync_friendly_entities(entity_list)
        self.module_stats["cihaz"]["aktif_birim"] = birim_count

        # Cihazlara hedef listesini güncelle (sensörler için)
        self.cihaz_yoneticisi.update_targets(entity_list)

        # Tüm cihazlara tek tick çalıştır
        # Her cihaz: veri üretir → BAĞ'a publish eder → KÜTÜK'e yazar
        device_results = self.cihaz_yoneticisi.tick_all()

        # Sensör füzyon — çoklu sensör verisini birleştir
        fused_tracks = self.cihaz_yoneticisi.fuse_tick(device_results)

        # Cihaz istatistiklerini topla
        for cihaz_id, payloads in device_results.items():
            device = self.cihaz_yoneticisi.get_device(cihaz_id)
            if not device:
                continue

            topic = device.topic
            data_count = len(payloads)

            # Birim cihazları birden fazla topic'e yayınlar (topic_override)
            # Her payload'ın gerçek topic'ini kontrol et
            for p in payloads:
                actual_topic = p.get("topic_override", topic)
                p_size = sys.getsizeof(json.dumps(p))

                if actual_topic.startswith("sensor."):
                    sensor_type = actual_topic.split(".")[-1]
                    self.sensor_stats[sensor_type] = self.sensor_stats.get(sensor_type, 0) + 1
                    self.module_stats["bag"]["sensor_msgs"] += 1
                    self.module_stats["bag"]["publishes"] += 1
                    self.module_stats["bag"]["bytes_sent"] += p_size
                    self.stats["messages_sent"] += 1

                elif actual_topic.startswith("command.") or actual_topic.startswith("alert."):
                    msg_type = p.get("msg_type", "")
                    self.c2_stats[msg_type] = self.c2_stats.get(msg_type, 0) + 1
                    if msg_type == "alert":
                        self.module_stats["bag"]["alert_msgs"] += 1
                        self.battle_events["alerts"] += 1
                        try:
                            self.yuzey.emit_event(
                                event_type="c2_alert",
                                source="SİMÜLASYON",
                                severity="warning",
                                title="C2 ALARM: " + p.get("title", actual_topic),
                                detail=p.get("detail", p.get("message", "")),
                            )
                        except Exception:
                            pass
                    else:
                        self.module_stats["bag"]["c2_msgs"] += 1
                    self.module_stats["bag"]["publishes"] += 1
                    self.stats["messages_sent"] += 1

                elif actual_topic == "entity.update":
                    self.module_stats["yuzey"]["entity_updates"] += 1

                elif actual_topic.startswith("telemetry."):
                    self.module_stats["bag"]["telemetry_msgs"] += 1
                    self.module_stats["bag"]["publishes"] += 1
                    self.stats["messages_sent"] += 1

                elif actual_topic == "iff.beacon":
                    self.module_stats["bag"]["publishes"] += 1
                    self.stats["messages_sent"] += 1

            events.append({
                "type": "device_tick",
                "device_id": cihaz_id,
                "device_type": device.kimlik.tip.value,
                "topic": topic,
                "payload_count": data_count,
            })

        # ── KÜTÜK'e varlık durumlarını yaz (her 3 tick) ──
        if tick % 3 == 0:
            for entry in entity_list:
                try:
                    self.kutuk.put_entity(entry["id"], entry)
                    self.module_stats["kutuk"]["puts"] += 1
                    self.module_stats["kutuk"]["entity_writes"] += 1
                except Exception:
                    self.module_stats["kutuk"]["errors"] += 1

        # ── YÜZEY'e varlık güncellemesi (her 2 tick) ──
        if tick % 2 == 0:
            try:
                self.yuzey.update_entities(entity_list)
                self.module_stats["yuzey"]["entity_updates"] += 1
            except Exception:
                self.module_stats["yuzey"]["errors"] += 1

        # ── YÜZEY'e savaş olaylarını gönder ──
        for ev in move_events:
            ev_type = ev.get("type", "")
            try:
                if ev_type == "damage":
                    eid = ev.get("entity_id", "?")
                    hp = ev.get("health", 0)
                    dmg = ev.get("damage", 0)
                    lat = ev.get("lat", 0)
                    lon = ev.get("lon", 0)
                    if hp <= 0:
                        self.yuzey.emit_event(
                            event_type="unit_destroyed",
                            source="SİMÜLASYON",
                            severity="critical",
                            title=f"BİRİM İMHA: {eid}",
                            detail=f"{eid} imha edildi (hasar: {dmg:.0f})",
                            lat=lat, lon=lon,
                        )
                    elif hp < 30:
                        self.yuzey.emit_event(
                            event_type="heavy_damage",
                            source="SİMÜLASYON",
                            severity="warning",
                            title=f"AĞIR HASAR: {eid}",
                            detail=f"Sağlık %{hp:.0f} (hasar: {dmg:.0f})",
                            lat=lat, lon=lon,
                        )
                elif ev_type == "low_fuel":
                    eid = ev.get("entity_id", "?")
                    fuel = ev.get("fuel", 0)
                    self.yuzey.emit_event(
                        event_type="low_fuel_alert",
                        source="SİMÜLASYON",
                        severity="warning",
                        title=f"YAKIT UYARISI: {eid}",
                        detail=f"Yakıt seviyesi %{fuel:.0f}",
                    )
                elif ev_type == "course_change":
                    eid = ev.get("entity_id", "?")
                    faction = ev.get("faction", "")
                    if faction == "enemy":
                        self.yuzey.emit_event(
                            event_type="enemy_maneuver",
                            source="SİMÜLASYON",
                            severity="info",
                            title=f"DÜŞMAN MANEVRA: {eid}",
                            detail=f"Rota değişikliği tespit edildi",
                            lat=ev.get("lat", 0), lon=ev.get("lon", 0),
                        )
            except Exception:
                pass

        # ── KÖZ termal tarama (her 3 tick) ──
        if tick % 3 == 0 and entity_list:
            koz_entities = [
                {
                    "id": e["id"], "type": e.get("type", "unknown"),
                    "faction": e.get("faction", "unknown"),
                    "status": e.get("status", "active"),
                    "lat": e.get("lat", 0), "lon": e.get("lon", 0),
                    "speed": e.get("speed", 0), "health": e.get("health", 100),
                }
                for e in entity_list
            ]
            try:
                result = self.koz.process(koz_entities)
                scan_info = result.get("scan", {})
                self.module_stats["koz"]["scans"] += 1
                self.module_stats["koz"]["signatures"] += scan_info.get("signatures", 0)
                self.module_stats["koz"]["clusters"] += len(result.get("clusters", []))
                anomalies = result.get("anomalies", [])
                self.module_stats["koz"]["anomalies"] += len(anomalies)

                # KÖZ anomalileri → YÜZEY sensör katmanı + olay
                if anomalies:
                    markers = [
                        {"id": f"koz-{a.get('id', i)}", "lat": a.get("lat", 0), "lon": a.get("lon", 0),
                         "label": f"Termal Anomali ({a.get('temperature', '?')}°C)", "icon": "thermal"}
                        for i, a in enumerate(anomalies) if a.get("lat")
                    ]
                    if markers:
                        try:
                            self.yuzey.push_layer_data("sensors", markers=markers, mode="append")
                        except Exception:
                            pass
                    try:
                        self.yuzey.emit_event(
                            event_type="thermal_anomaly", source="KÖZ", severity="warning",
                            title=f"Termal Anomali: {len(anomalies)} kaynak",
                            detail=f"{scan_info.get('signatures', 0)} termal imza tarandı",
                        )
                    except Exception:
                        pass
            except Exception:
                self.module_stats["koz"]["errors"] += 1

        # ══════════════════════════════════════════════════════
        # BÖLÜM 3: TÜM MODÜL ENTEGRASYONU
        # Simülasyonda da prod'da da aynı akış — modüller gerçek HTTP ile çağrılır
        # Modül ayakta değilse hata loglanır ve devam edilir (graceful)
        # ══════════════════════════════════════════════════════

        # ── BÜTÜN heartbeat (her 10 tick) ──
        if tick % 10 == 0:
            try:
                for svc_name in ["BAG", "KUTUK", "DIRLIK", "BAGDAS", "AKIM", "YURT",
                                 "YUZEY", "IM", "YAYIN", "DURAK", "TUTUM",
                                 "TANI", "KOZ", "SEZI", "YANKI", "DUYUM", "BASKALA", "SOZGEC", "IZLEK"]:
                    self.butun.heartbeat(svc_name)
                self.module_stats["butun"]["heartbeats"] += 19
                self.module_stats["butun"]["requests"] += 19
            except Exception:
                self.module_stats["butun"]["errors"] += 1

        # ── DİRLİK sağlık kontrolü (her 5 tick) ──
        if tick % 5 == 0:
            try:
                self.dirlik.get_current_metrics()
                self.module_stats["dirlik"]["metrics"] += 1
                self.module_stats["dirlik"]["checks"] += 1
            except Exception:
                self.module_stats["dirlik"]["errors"] += 1
            try:
                self.dirlik.get_probes()
                self.module_stats["dirlik"]["probes"] += 1
                self.module_stats["dirlik"]["checks"] += 1
            except Exception:
                self.module_stats["dirlik"]["errors"] += 1
            try:
                summary = self.dirlik.get_alert_summary()
                alert_count = summary.get("total", 0) if isinstance(summary, dict) else 0
                self.module_stats["dirlik"]["alerts"] += alert_count
                self.module_stats["dirlik"]["checks"] += 1
            except Exception:
                self.module_stats["dirlik"]["errors"] += 1

        # ── BAĞDAŞ format dönüşüm (her 4 tick) ──
        if tick % 4 == 0:
            test_data = {
                "sensor_id": f"sim-sensor-{tick}",
                "sensor_type": "radar",
                "timestamp": "2025-01-01T00:00:00Z",
                "lat": 39.925, "lon": 32.866,
                "value": float(tick),
            }
            try:
                self.bagdas.convert(test_data, "json", "msgpack")
                self.module_stats["bagdas"]["converts"] += 1
            except Exception:
                self.module_stats["bagdas"]["errors"] += 1
            try:
                self.bagdas.convert(test_data, "json", "cbor")
                self.module_stats["bagdas"]["converts"] += 1
            except Exception:
                self.module_stats["bagdas"]["errors"] += 1
            try:
                self.bagdas.validate("sensor_data", 1, test_data)
                self.module_stats["bagdas"]["validates"] += 1
            except Exception:
                self.module_stats["bagdas"]["errors"] += 1
            try:
                self.bagdas.execute_pipeline("sensor_convert", test_data)
                self.module_stats["bagdas"]["pipelines"] += 1
            except Exception:
                self.module_stats["bagdas"]["errors"] += 1

        # ── AKIM trafik yönlendirme (her 3 tick) ──
        if tick % 3 == 0:
            node_pairs = [
                ("cmd-center", "edge-alpha", 0, "alert"),
                ("sensor-hub", "cmd-center", 3, "sensor"),
                ("edge-bravo", "relay-south", 2, "command"),
            ]
            pair = node_pairs[tick % len(node_pairs)]
            try:
                self.akim.find_route(pair[0], pair[1], pair[2])
                self.module_stats["akim"]["routes"] += 1
            except Exception:
                self.module_stats["akim"]["errors"] += 1
            try:
                self.akim.select_backend()
                self.module_stats["akim"]["balancer_selects"] += 1
            except Exception:
                self.module_stats["akim"]["errors"] += 1
            try:
                self.akim.create_flow(pair[0], pair[1], pair[2], 50.0, pair[3])
                self.module_stats["akim"]["flows"] += 1
            except Exception:
                self.module_stats["akim"]["errors"] += 1

        # ── AKIM tıkanıklık kontrol (her 10 tick) ──
        if tick % 10 == 0:
            try:
                self.akim.get_congestion_report()
                self.module_stats["akim"]["congestion_checks"] += 1
            except Exception:
                self.module_stats["akim"]["errors"] += 1

        # ── YURT yerel depolama (her 2 tick) ──
        if tick % 2 == 0:
            entity_keys = list(self.battlefield_gen.entities.keys())
            if entity_keys:
                eid = random.choice(entity_keys)
                entity = self.battlefield_gen.entities[eid]
                store_key = f"edge/{eid}"
                try:
                    self.yurt.store_put(store_key, entity.to_dict())
                    self.module_stats["yurt"]["store_puts"] += 1
                except Exception:
                    self.module_stats["yurt"]["errors"] += 1
                try:
                    self.yurt.store_get(store_key)
                    self.module_stats["yurt"]["store_gets"] += 1
                except Exception:
                    self.module_stats["yurt"]["errors"] += 1

        # ── YURT senkronizasyon (her 6 tick) ──
        if tick % 6 == 0:
            try:
                self.yurt.sync_full()
                self.module_stats["yurt"]["syncs"] += 1
            except Exception:
                self.module_stats["yurt"]["errors"] += 1

        # ── YURT proxy isteği (her 4 tick) ──
        if tick % 4 == 0:
            try:
                self.yurt.proxy_request("GET", f"/api/sensor/{tick}", priority=random.randint(0, 5))
                self.module_stats["yurt"]["proxy_reqs"] += 1
            except Exception:
                self.module_stats["yurt"]["errors"] += 1

        # ── YURT bölünme kontrolü (her 8 tick) ──
        if tick % 8 == 0:
            try:
                self.yurt.partition_check()
                self.module_stats["yurt"]["partition_checks"] += 1
            except Exception:
                self.module_stats["yurt"]["errors"] += 1

        # ── YÜZEY taktiksel resim (her 5 tick) ──
        if tick % 5 == 0:
            try:
                self.yuzey.tactical()
                self.module_stats["yuzey"]["tactical_updates"] += 1
            except Exception:
                self.module_stats["yuzey"]["errors"] += 1

        # ── YÜZEY sağlık kontrolü (her 10 tick) ──
        if tick % 10 == 0:
            try:
                self.yuzey.health()
                self.module_stats["yuzey"]["health_checks"] += 1
            except Exception:
                self.module_stats["yuzey"]["errors"] += 1

        # ── İM token yenileme (her 20 tick) ──
        if tick % 20 == 0:
            try:
                self.im.health()
                self.module_stats["im"]["verifications"] += 1
            except Exception:
                self.module_stats["im"]["errors"] += 1

        # ── YAYIN kanal durumu (her 15 tick) ──
        if tick % 15 == 0:
            try:
                self.yayin.list_channels()
                self.module_stats["yayin"]["channels"] += 1
            except Exception:
                self.module_stats["yayin"]["errors"] += 1

        # ── DURAK kuyruk durumu (her 8 tick) ──
        if tick % 8 == 0:
            try:
                self.durak.health()
                self.module_stats["durak"]["queued"] += 1
            except Exception:
                self.module_stats["durak"]["errors"] += 1

        # ── TUTUM kaynak izleme (her 10 tick) ──
        if tick % 10 == 0:
            try:
                self.tutum.health()
                self.module_stats["tutum"]["checks"] += 1
            except Exception:
                self.module_stats["tutum"]["errors"] += 1

        # ── TANI nesne tespit (her 5 tick) ──
        if tick % 5 == 0 and entity_list:
            try:
                source_id = random.choice(["cam-north-01", "uav-kartal-eye", "radar-dome-01", "sat-img-01"])
                result = self.tani.process(
                    frame_id=f"frame-{tick}",
                    width=1280, height=720,
                    source_id=source_id,
                )
                detections = result.get("detections", [])
                classifications = result.get("classifications", [])
                self.module_stats["tani"]["detections"] += len(detections)

                # TANI tespitleri → YÜZEY olay olarak bildir
                enemy_detections = [d for d in classifications if d.get("faction") == "enemy"]
                if enemy_detections:
                    try:
                        self.yuzey.emit_event(
                            event_type="object_detection", source="TANI", severity="info",
                            title=f"Nesne Tespit: {len(detections)} obje, {len(enemy_detections)} düşman",
                            detail=f"Kaynak: {source_id}, Sınıflandırma: {len(classifications)}",
                        )
                    except Exception:
                        pass
            except Exception:
                self.module_stats["tani"]["errors"] += 1

        # ── SEZİ kamuflaj tarama (her 4 tick) ──
        if tick % 4 == 0 and entity_list:
            sezi_entities = [
                {
                    "id": e["id"], "type": e.get("type", "unknown"),
                    "faction": e.get("faction", "unknown"),
                    "lat": e.get("lat", 0), "lon": e.get("lon", 0),
                    "speed": e.get("speed", 0), "health": e.get("health", 100),
                }
                for e in entity_list
            ]
            try:
                result = self.sezi.process(sezi_entities)
                self.module_stats["sezi"]["scans"] += 1
                detections = result.get("detections", [])
                self.module_stats["sezi"]["camo_detected"] += len(detections)
                high_threats = [d for d in detections if d.get("threat_level") in ("high", "critical")]
                self.module_stats["sezi"]["threats"] += len(high_threats)

                # SEZİ kamuflaj tespitleri → YÜZEY tehdit katmanı
                if high_threats:
                    zones = [
                        {"id": f"sezi-{d.get('id', i)}", "lat": d.get("lat", 0), "lon": d.get("lon", 0),
                         "radius": 200, "label": f"Kamuflaj: {d.get('type', '?')}", "color": "orange"}
                        for i, d in enumerate(high_threats) if d.get("lat")
                    ]
                    if zones:
                        try:
                            self.yuzey.push_layer_data("threats", zones=zones, mode="append")
                        except Exception:
                            pass
                    try:
                        self.yuzey.emit_event(
                            event_type="camouflage_detected", source="SEZİ", severity="warning",
                            title=f"Kamuflaj Tespiti: {len(high_threats)} yüksek tehdit",
                            detail=f"Toplam {len(detections)} kamuflajlı varlık tarandı",
                        )
                    except Exception:
                        pass
            except Exception:
                self.module_stats["sezi"]["errors"] += 1

        # ── YANKI akustik konumlandırma (her 3 tick) ──
        if tick % 3 == 0 and entity_list:
            yanki_entities = [
                {
                    "id": e["id"], "type": e.get("type", "unknown"),
                    "lat": e.get("lat", 0), "lon": e.get("lon", 0),
                    "frequency_hz": 200 + (hash(e["id"]) % 2000),
                    "amplitude_db": 50 + (hash(e["id"]) % 100),
                    "direction_deg": hash(e["id"]) % 360,
                    "signature_type": {"tank": "tracked_vehicle", "apc": "wheeled_vehicle",
                                       "helicopter": "helicopter", "jet": "jet_engine",
                                       "drone": "small_drone", "infantry": "footsteps",
                                       }.get(e.get("type", ""), "unknown"),
                    "confidence": 0.6 + (hash(e["id"]) % 40) / 100.0,
                    "range_est_km": e.get("speed", 0) * 0.01 + 0.5,
                }
                for e in entity_list
            ]
            try:
                result = self.yanki.process(yanki_entities)
                detections = result.get("detections", [])
                self.module_stats["yanki"]["detections"] += len(detections)
                self.module_stats["yanki"]["localized"] += result.get("localized", 0)
                gunshots = [d for d in detections if d.get("event_type") == "gunshot"]
                self.module_stats["yanki"]["gunshots"] += len(gunshots)
                threats = [d for d in detections if d.get("threat_level") in ("high", "critical")]
                self.module_stats["yanki"]["threats"] += len(threats)

                # YANKI akustik tespitler → YÜZEY sensör katmanı + olay
                if threats:
                    markers = [
                        {"id": f"yanki-{d.get('id', i)}", "lat": d.get("lat", 0), "lon": d.get("lon", 0),
                         "label": f"Akustik: {d.get('event_type', '?')}", "icon": "acoustic"}
                        for i, d in enumerate(threats) if d.get("lat")
                    ]
                    if markers:
                        try:
                            self.yuzey.push_layer_data("sensors", markers=markers, mode="append")
                        except Exception:
                            pass
                if gunshots:
                    for g in gunshots:
                        try:
                            self.yuzey.emit_event(
                                event_type="gunshot_detected", source="YANKI", severity="critical",
                                title=f"Silah Sesi Tespit: {g.get('event_type', 'ateş')}",
                                detail=f"Yön: {g.get('direction', '?')}°, Mesafe: {g.get('range_km', '?')}km",
                                lat=g.get("lat", 0), lon=g.get("lon", 0),
                            )
                        except Exception:
                            pass
            except Exception:
                self.module_stats["yanki"]["errors"] += 1

        # ── DUYUM açık kaynak istihbarat (her 5 tick) ──
        if tick % 5 == 0 and entity_list:
            duyum_entities = [
                {
                    "id": e["id"], "type": e.get("type", "unknown"),
                    "faction": e.get("faction", "unknown"),
                    "lat": e.get("lat", 0), "lon": e.get("lon", 0),
                    "speed": e.get("speed", 0), "health": e.get("health", 100),
                }
                for e in entity_list
            ]
            try:
                result = self.duyum.process(duyum_entities)
                reports_count = result.get("reports_count", 0)
                high_threats = result.get("high_threats", 0)
                self.module_stats["duyum"]["reports"] += reports_count
                self.module_stats["duyum"]["sources"] = result.get("total_sources", 0)
                self.module_stats["duyum"]["threats"] += high_threats

                # DUYUM tehditler → YÜZEY tehdit katmanı + olay
                if high_threats > 0:
                    try:
                        self.yuzey.emit_event(
                            event_type="osint_threat", source="DUYUM", severity="warning",
                            title=f"OSINT İstihbarat: {high_threats} yüksek tehdit",
                            detail=f"{reports_count} rapor üretildi, {result.get('total_sources', 0)} kaynak tarandı",
                        )
                    except Exception:
                        pass
            except Exception:
                self.module_stats["duyum"]["errors"] += 1

        # ── BAŞKALA uydu değişim tespiti (her 6 tick) ──
        if tick % 6 == 0 and entity_list:
            baskala_entities = [
                {
                    "id": e["id"], "type": e.get("type", "unknown"),
                    "faction": e.get("faction", "unknown"),
                    "lat": e.get("lat", 0), "lon": e.get("lon", 0),
                    "speed": e.get("speed", 0), "health": e.get("health", 100),
                }
                for e in entity_list
            ]
            try:
                result = self.baskala.process(baskala_entities)
                fused = result.get("fused_detections", 0)
                high_threats = result.get("high_threats", 0)
                self.module_stats["baskala"]["changes"] += fused
                self.module_stats["baskala"]["images"] = result.get("total_images", 0)
                self.module_stats["baskala"]["threats"] += high_threats
                self.module_stats["baskala"]["fused"] += result.get("cross_validated", 0)

                # BAŞKALA değişimler → YÜZEY kapsam katmanı + olay
                if high_threats > 0:
                    try:
                        self.yuzey.emit_event(
                            event_type="satellite_change", source="BAŞKALA", severity="warning",
                            title=f"Uydu Değişim: {fused} tespit, {high_threats} yüksek tehdit",
                            detail=f"Toplam {result.get('total_images', 0)} görüntü, "
                                   f"{result.get('cross_validated', 0)} çapraz doğrulanmış",
                        )
                    except Exception:
                        pass
            except Exception:
                self.module_stats["baskala"]["errors"] += 1

        # ── SÖZGEÇ sinyal ayıklama (her 4 tick) ──
        if tick % 4 == 0 and entity_list:
            sozgec_entities = [
                {
                    "id": e["id"], "type": e.get("type", "unknown"),
                    "faction": e.get("faction", "unknown"),
                    "lat": e.get("lat", 0), "lon": e.get("lon", 0),
                }
                for e in entity_list
            ]
            try:
                result = self.sozgec.process(sozgec_entities)
                self.module_stats["sozgec"]["signals"] += result.get("extracted_signals", 0)
                self.module_stats["sozgec"]["bands"] = result.get("active_bands", 0)
                self.module_stats["sozgec"]["friendly"] += result.get("friendly_count", 0)
                hostile = result.get("hostile_count", 0)
                jamming = result.get("jamming_detected", 0)
                self.module_stats["sozgec"]["hostile"] += hostile
                self.module_stats["sozgec"]["jamming"] += jamming

                # SÖZGEÇ karıştırma tespiti → YÜZEY kritik olay
                if jamming > 0:
                    try:
                        self.yuzey.emit_event(
                            event_type="jamming_detected", source="SÖZGEÇ", severity="critical",
                            title=f"SİNYAL KARIŞTIRMA TESPİT: {jamming} bant",
                            detail=f"{hostile} düşman sinyal, {result.get('extracted_signals', 0)} toplam sinyal",
                        )
                    except Exception:
                        pass
                elif hostile > 0:
                    try:
                        self.yuzey.emit_event(
                            event_type="hostile_signal", source="SÖZGEÇ", severity="warning",
                            title=f"Düşman Sinyal: {hostile} tespit",
                            detail=f"{result.get('active_bands', 0)} aktif bant, "
                                   f"{result.get('friendly_count', 0)} dost sinyal",
                        )
                    except Exception:
                        pass
            except Exception:
                self.module_stats["sozgec"]["errors"] += 1

        # ── İZLEK rota tahmini (her 3 tick) ──
        if tick % 3 == 0 and entity_list:
            izlek_entities = [
                {
                    "id": e["id"], "type": e.get("type", "unknown"),
                    "faction": e.get("faction", "unknown"),
                    "lat": e.get("lat", 0), "lon": e.get("lon", 0),
                    "speed": e.get("speed", 0), "heading": e.get("heading", 0),
                }
                for e in entity_list
            ]
            try:
                # İzleri güncelle
                update_result = self.izlek.update_tracks(izlek_entities)
                self.module_stats["izlek"]["tracks"] = update_result.get("updated", 0)

                # Düşman varlıklar için rota tahmini
                enemy_ids = [e["id"] for e in entity_list if e.get("faction") == "enemy"]
                if enemy_ids and tick % 6 == 0:
                    pred_result = self.izlek.predict_batch(enemy_ids[:10], horizon_sec=60)
                    predictions = pred_result.get("predictions", [])
                    self.module_stats["izlek"]["predictions"] += len(predictions)

                    # Manevra yapan hedefleri say
                    maneuvers = [p for p in predictions if p.get("maneuver")]
                    self.module_stats["izlek"]["maneuvers"] += len(maneuvers)

                    # Tahmin edilen rotaları YÜZEY'e polyline olarak gönder
                    route_lines = []
                    for p in predictions:
                        pts = p.get("points", [])
                        if len(pts) >= 2:
                            route_lines.append({
                                "id": f"izlek-route-{p.get('entity_id', '')}",
                                "points": [[pt.get("lat", 0), pt.get("lon", 0)] for pt in pts],
                                "color": "#ff4444" if p.get("maneuver") else "#ffaa00",
                                "width": 2, "style": "dashed",
                            })
                    if route_lines:
                        try:
                            self.yuzey.push_layer_data("routes", lines=route_lines, mode="set")
                        except Exception:
                            pass

                    # Manevra uyarıları
                    if maneuvers:
                        try:
                            self.yuzey.emit_event(
                                event_type="maneuver_detected", source="İZLEK", severity="warning",
                                title=f"Manevra Tespiti: {len(maneuvers)} hedef rota değiştiriyor",
                                detail=f"{len(predictions)} hedef tahmin edildi, güven: "
                                       f"{sum(pt.get('confidence', 0) for p in predictions for pt in p.get('points', [])[-1:]) / max(len(predictions), 1):.0%}",
                            )
                        except Exception:
                            pass
            except Exception:
                self.module_stats["izlek"]["errors"] += 1

        # ── Cihaz istatistiklerini güncelle ──
        cihaz_stats = self.cihaz_yoneticisi.stats()
        self.module_stats["cihaz"]["toplam_veri"] = cihaz_stats["toplam_uretilen_veri"]
        self.module_stats["cihaz"]["toplam_yayın"] = cihaz_stats["toplam_yayinlanan"]
        self.module_stats["cihaz"]["hatalar"] = cihaz_stats["toplam_hata"]

        return events

    def on_finish(self) -> dict:
        """Kapsamlı senaryo raporu."""
        # Cihazları durdur
        self.cihaz_yoneticisi.stop_all()

        result = super().on_finish()

        entities = self.battlefield_gen.entities
        factions = {}
        types = {}
        statuses = {}
        for e in entities.values():
            factions[e.faction.value] = factions.get(e.faction.value, 0) + 1
            types[e.unit_type.value] = types.get(e.unit_type.value, 0) + 1
            statuses[e.status.value] = statuses.get(e.status.value, 0) + 1

        result["battlefield"] = {
            "total_entities": len(entities),
            "destroyed": sum(1 for e in entities.values() if e.status.value == "destroyed"),
            "damaged": sum(1 for e in entities.values() if e.status.value == "damaged"),
            "active": sum(1 for e in entities.values() if e.status.value in ("active", "moving")),
            "by_faction": factions,
            "by_type": types,
            "by_status": statuses,
            "avg_health": round(sum(e.health_percent for e in entities.values()) / max(len(entities), 1), 1),
            "avg_fuel": round(sum(e.fuel_percent for e in entities.values()) / max(len(entities), 1), 1),
        }

        # Cihaz detay raporu
        result["cihaz_detay"] = self.cihaz_yoneticisi.health_all()

        return result
