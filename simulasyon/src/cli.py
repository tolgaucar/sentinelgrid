#!/usr/bin/env python3
"""
ÖTÜKEN-MATRİS Simülasyon Platformu CLI

Kullanım:
    python -m src.cli run [--scenario temel] [--ticks 100] [--interval 0.5] [--connect]
    python -m src.cli status
    python -m src.cli list-scenarios
    python -m src.cli generate --type sensor --count 10
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from src.runner import SimulationRunner, load_config, SCENARIO_REGISTRY
from src.dashboard.console import SimDashboard
from src.generators.sensor import SensorGenerator
from src.generators.battlefield import BattlefieldGenerator
from src.generators.comms import CommsGenerator
from src.clients.butun_client import ButunClient
from src.clients.bag_client import BagClient
from src.clients.kutuk_client import KutukClient


def cmd_run(args) -> None:
    """Simülasyon çalıştır."""
    import os
    env = os.environ.get("OTUKEN_ENV", "development").lower()
    if env not in ("development", "dev", "test", "local"):
        print("❌ Simülasyon sadece geliştirme ortamında çalışır.")
        print(f"   Mevcut ortam: OTUKEN_ENV={env}")
        print("   Geliştirme ortamı için: export OTUKEN_ENV=development")
        sys.exit(1)

    config = load_config(args.config) if args.config else load_config()

    # CLI argümanları ile config'i override et
    sim_cfg = config.setdefault("simulation", {})
    if args.ticks:
        sim_cfg["max_ticks"] = args.ticks
    if args.interval:
        sim_cfg["tick_interval"] = args.interval
    if args.connect:
        sim_cfg["connect_modules"] = True

    dashboard = SimDashboard()
    runner = SimulationRunner(config=config)

    print("\n🏴 ÖTÜKEN-MATRİS Simülasyon Platformu")
    print("=" * 60)

    results = runner.run(scenario_name=args.scenario, callback=dashboard.update)
    dashboard.final_report(results)


def cmd_status(args) -> None:
    """Modül durumlarını kontrol et."""
    print("\n📡 Modül Durumları")
    print("-" * 40)

    butun = ButunClient()
    bag = BagClient()
    kutuk = KutukClient()

    for name, client in [("BÜTÜN", butun), ("BAĞ", bag), ("KÜTÜK", kutuk)]:
        alive = client.is_alive()
        emoji = "✅" if alive else "❌"
        print(f"  {emoji} {name:8s} ({client.base_url}) - {'ÇALIŞIYOR' if alive else 'ERİŞİLEMİYOR'}")

    butun.close()
    bag.close()
    kutuk.close()


def cmd_list(args) -> None:
    """Mevcut senaryoları listele."""
    print("\n📋 Mevcut Senaryolar")
    print("-" * 40)
    for name, cls in SCENARIO_REGISTRY.items():
        print(f"  • {name:15s} - {cls.description}")


def cmd_generate(args) -> None:
    """Test verisi üret."""
    gen_type = args.type
    count = args.count

    print(f"\n🔧 {count} adet {gen_type} verisi üretiliyor...\n")

    if gen_type == "sensor":
        gen = SensorGenerator(seed=42)
        for reading in gen.generate_burst(count):
            print(json.dumps(reading.to_bag_payload(), ensure_ascii=False, indent=2))
    elif gen_type == "battlefield":
        gen = BattlefieldGenerator(seed=42)
        entities = gen.generate_battlefield(friendly=count//3, enemy=count//3, drones=count//6, sensors=count//6)
        for e in entities:
            print(json.dumps(e.to_dict(), ensure_ascii=False, indent=2))
    elif gen_type == "comms":
        gen = CommsGenerator(seed=42)
        for msg in gen.generate_burst(count):
            print(json.dumps(msg.to_dict(), ensure_ascii=False, indent=2))
    else:
        print(f"❌ Bilinmeyen veri tipi: {gen_type}")
        print("   Geçerli tipler: sensor, battlefield, comms")


def cmd_web(args) -> None:
    """Web arayüzünü başlat."""
    from src.web import start_server
    start_server(host=args.host, port=args.port)


def main():
    parser = argparse.ArgumentParser(
        prog="otuken-sim",
        description="ÖTÜKEN-MATRİS Simülasyon Platformu",
    )
    subparsers = parser.add_subparsers(dest="command", help="Komut")

    # run
    p_run = subparsers.add_parser("run", help="Simülasyon çalıştır")
    p_run.add_argument("--scenario", "-s", default="temel", help="Senaryo adı")
    p_run.add_argument("--ticks", "-t", type=int, help="Maksimum tick sayısı")
    p_run.add_argument("--interval", "-i", type=float, help="Tick aralığı (saniye)")
    p_run.add_argument("--config", "-c", help="Konfigürasyon dosyası yolu")
    p_run.add_argument("--connect", action="store_true", help="Modüllere bağlan (BÜTÜN/BAĞ çalışıyor olmalı)")
    p_run.set_defaults(func=cmd_run)

    # status
    p_status = subparsers.add_parser("status", help="Modül durumlarını kontrol et")
    p_status.set_defaults(func=cmd_status)

    # list
    p_list = subparsers.add_parser("list-scenarios", help="Senaryoları listele")
    p_list.set_defaults(func=cmd_list)

    # generate
    p_gen = subparsers.add_parser("generate", help="Test verisi üret")
    p_gen.add_argument("--type", "-t", default="sensor", help="Veri tipi: sensor, battlefield, comms")
    p_gen.add_argument("--count", "-n", type=int, default=5, help="Üretilecek veri sayısı")
    p_gen.set_defaults(func=cmd_generate)

    # web
    p_web = subparsers.add_parser("web", help="Web arayüzünü başlat")
    p_web.add_argument("--host", default="0.0.0.0", help="Dinlenecek adres")
    p_web.add_argument("--port", "-p", type=int, default=8080, help="Port numarası")
    p_web.set_defaults(func=cmd_web)

    args = parser.parse_args()
    if not args.command:
        parser.print_help()
        sys.exit(1)

    args.func(args)


if __name__ == "__main__":
    main()
