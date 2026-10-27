#!/usr/bin/env python3
"""
ÖTÜKEN-MATRİS — Tam Sistem Başlatıcı

Tüm modülleri arka planda başlatır, sağlık kontrolü yapar,
ardından simülasyon web arayüzünü çalıştırır.

Kullanım:
    python3 baslatici.py              # Tüm modüller + simülasyon web UI
    python3 baslatici.py --no-web     # Sadece modülleri başlat
    python3 baslatici.py --kill       # Tüm modülleri durdur
"""

from __future__ import annotations

import argparse
import http.client
import json
import os
import signal
import subprocess
import sys
import time
from pathlib import Path

BASE_DIR = Path(__file__).parent.parent
GO_BIN = "/opt/homebrew/bin/go"
PYTHON = sys.executable or "python3"
IS_PROD = os.environ.get("OTUKEN_ENV", "dev") == "prod"
BIN_DIR = BASE_DIR / "bin"
PID_FILE = BASE_DIR / ".pids.json"


def _go_cmd(mod_id: str) -> list:
    """Prod'da derlenmiş binary, dev'de go run kullanır."""
    if IS_PROD and (BIN_DIR / mod_id).exists():
        return [str(BIN_DIR / mod_id)]
    return [GO_BIN, "run", f"./cmd/{mod_id}/main.go"]

# Modül tanımları: (id, klasör, başlatma komutu, sağlık portu, sağlık yolu)
MODULES = [
    {
        "id": "butun",
        "name": "BÜTÜN",
        "icon": "🏛️",
        "dir": str(BASE_DIR / "butun"),
        "cmd": [PYTHON, "-m", "uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8000", "--log-level", "warning"],
        "health_port": 8000,
        "health_path": "/health",
        "type": "python",
    },
    {
        "id": "bag",
        "name": "BAĞ",
        "icon": "📡",
        "dir": str(BASE_DIR / "bag"),
        "cmd": _go_cmd("bag"),
        "health_port": 9091,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "kutuk",
        "name": "KÜTÜK",
        "icon": "🗄️",
        "dir": str(BASE_DIR / "kutuk"),
        "cmd": _go_cmd("kutuk"),
        "health_port": 9093,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "dirlik",
        "name": "DİRLİK",
        "icon": "🏥",
        "dir": str(BASE_DIR / "dirlik"),
        "cmd": _go_cmd("dirlik"),
        "health_port": 9095,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "bagdas",
        "name": "BAĞDAŞ",
        "icon": "🔄",
        "dir": str(BASE_DIR / "bagdas"),
        "cmd": _go_cmd("bagdas"),
        "health_port": 9097,
        "health_path": "/healthz",
        "type": "go",
    },
    {
        "id": "akim",
        "name": "AKIM",
        "icon": "🌊",
        "dir": str(BASE_DIR / "akim"),
        "cmd": _go_cmd("akim"),
        "health_port": 9099,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "yurt",
        "name": "YURT",
        "icon": "🏕️",
        "dir": str(BASE_DIR / "yurt"),
        "cmd": _go_cmd("yurt"),
        "health_port": 9101,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "yuzey",
        "name": "YÜZEY",
        "icon": "🗺️",
        "dir": str(BASE_DIR / "yuzey"),
        "cmd": _go_cmd("yuzey"),
        "health_port": 9103,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "im",
        "name": "İM",
        "icon": "🔐",
        "dir": str(BASE_DIR / "im"),
        "cmd": _go_cmd("im"),
        "health_port": 9105,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "yayin",
        "name": "YAYIN",
        "icon": "📡",
        "dir": str(BASE_DIR / "yayin"),
        "cmd": _go_cmd("yayin"),
        "health_port": 9107,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "durak",
        "name": "DURAK",
        "icon": "⏸️",
        "dir": str(BASE_DIR / "durak"),
        "cmd": _go_cmd("durak"),
        "health_port": 9109,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "tutum",
        "name": "TUTUM",
        "icon": "🔋",
        "dir": str(BASE_DIR / "tutum"),
        "cmd": _go_cmd("tutum"),
        "health_port": 9111,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "tani",
        "name": "TANI",
        "icon": "🎯",
        "dir": str(BASE_DIR / "tani"),
        "cmd": _go_cmd("tani"),
        "health_port": 9113,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "koz",
        "name": "KÖZ",
        "icon": "🔥",
        "dir": str(BASE_DIR / "koz"),
        "cmd": _go_cmd("koz"),
        "health_port": 9115,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "sezi",
        "name": "SEZİ",
        "icon": "🎨",
        "dir": str(BASE_DIR / "sezi"),
        "cmd": _go_cmd("sezi"),
        "health_port": 9116,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "yanki",
        "name": "YANKI",
        "icon": "🔊",
        "dir": str(BASE_DIR / "yanki"),
        "cmd": _go_cmd("yanki"),
        "health_port": 9118,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "duyum",
        "name": "DUYUM",
        "icon": "🔍",
        "dir": str(BASE_DIR / "duyum"),
        "cmd": _go_cmd("duyum"),
        "health_port": 9120,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "baskala",
        "name": "BAŞKALA",
        "icon": "🛰️",
        "dir": str(BASE_DIR / "baskala"),
        "cmd": _go_cmd("baskala"),
        "health_port": 9122,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "sozgec",
        "name": "SÖZGEÇ",
        "icon": "📶",
        "dir": str(BASE_DIR / "sozgec"),
        "cmd": _go_cmd("sozgec"),
        "health_port": 9124,
        "health_path": "/health",
        "type": "go",
    },
    {
        "id": "izlek",
        "name": "İZLEK",
        "icon": "🧭",
        "dir": str(BASE_DIR / "izlek"),
        "cmd": _go_cmd("izlek"),
        "health_port": 9126,
        "health_path": "/health",
        "type": "go",
    },
]

BANNER = """
╔═══════════════════════════════════════════════════════════════╗
║          ÖTÜKEN-MATRİS — TAM SİSTEM BAŞLATICI                ║
║         20 Modül + Simülasyon Platformu                       ║
║         Gerçek Zamanlı C4ISR Ortamı                           ║
╚═══════════════════════════════════════════════════════════════╝
"""


def check_health(port: int, path: str, timeout: float = 1.0) -> bool:
    """HTTP sağlık kontrolü."""
    try:
        conn = http.client.HTTPConnection("127.0.0.1", port, timeout=timeout)
        conn.request("GET", path)
        resp = conn.getresponse()
        conn.close()
        return resp.status == 200
    except Exception:
        return False


def start_module(mod: dict, log_dir: Path) -> subprocess.Popen:
    """Modülü arka planda başlat."""
    log_file = log_dir / f"{mod['id']}.log"
    env = os.environ.copy()
    env["PATH"] = f"/opt/homebrew/bin:{env.get('PATH', '')}"
    env.setdefault("OTUKEN_ENV", "dev")

    with open(log_file, "w") as f:
        proc = subprocess.Popen(
            mod["cmd"],
            cwd=mod["dir"],
            stdout=f,
            stderr=subprocess.STDOUT,
            env=env,
            preexec_fn=os.setsid,
        )
    return proc


def wait_for_health(mod: dict, max_wait: int = 30) -> bool:
    """Modülün sağlıklı olmasını bekle."""
    for _ in range(max_wait * 2):
        if check_health(mod["health_port"], mod["health_path"]):
            return True
        time.sleep(0.5)
    return False


def save_pids(pids: dict[str, int]):
    """PID'leri dosyaya kaydet."""
    with open(PID_FILE, "w") as f:
        json.dump(pids, f)


def load_pids() -> dict[str, int]:
    """PID'leri dosyadan oku."""
    if not PID_FILE.exists():
        return {}
    with open(PID_FILE) as f:
        return json.load(f)


def _find_pids_by_port() -> dict[str, int]:
    """PID dosyası yoksa portlardan çalışan modülleri bul (lsof)."""
    found = {}
    for mod in MODULES:
        port = mod["health_port"]
        try:
            out = subprocess.check_output(
                ["lsof", "-iTCP:%d" % port, "-sTCP:LISTEN", "-t"],
                stderr=subprocess.DEVNULL, text=True,
            ).strip()
            if out:
                for line in out.splitlines():
                    pid = int(line.strip())
                    found[mod["id"]] = pid
                    break
        except (subprocess.CalledProcessError, ValueError):
            pass
    return found


def kill_all():
    """Tüm modülleri durdur."""
    pids = load_pids()

    # PID dosyası boşsa portlardan bul
    if not pids:
        pids = _find_pids_by_port()

    if not pids:
        print("❌ Çalışan modül bulunamadı.")
        return

    print("\n🛑 Modüller durduruluyor...\n")
    for name, pid in pids.items():
        try:
            os.killpg(pid, signal.SIGTERM)
            print(f"  ✅ {name} (PID {pid}) durduruldu")
        except PermissionError:
            try:
                os.kill(pid, signal.SIGTERM)
                print(f"  ✅ {name} (PID {pid}) durduruldu (tekil)")
            except ProcessLookupError:
                print(f"  ⚪ {name} (PID {pid}) zaten kapalı")
            except PermissionError:
                print(f"  ⚪ {name} (PID {pid}) artık bu processe ait değil")
            except Exception as e:
                print(f"  ❌ {name} (PID {pid}) hata: {e}")
        except ProcessLookupError:
            print(f"  ⚪ {name} (PID {pid}) zaten kapalı")
        except Exception as e:
            print(f"  ❌ {name} (PID {pid}) hata: {e}")

    # Kısa süre bekle, SIGKILL ile zorla kapat
    time.sleep(1)
    for name, pid in pids.items():
        try:
            os.kill(pid, 0)
            os.kill(pid, signal.SIGKILL)
            print(f"  🔪 {name} (PID {pid}) zorla kapatıldı")
        except (ProcessLookupError, PermissionError):
            pass

    PID_FILE.unlink(missing_ok=True)
    print("\n✅ Tüm modüller durduruldu.\n")


def start_all(no_web: bool = False):
    """Tüm modülleri başlat."""
    print(BANNER)

    log_dir = BASE_DIR / "simulasyon" / "logs"
    log_dir.mkdir(exist_ok=True)

    # Önce eski süreçleri temizle
    old_pids = load_pids()
    if old_pids:
        print("⚠️  Önceki süreçler temizleniyor...\n")
        kill_all()
        time.sleep(1)

    pids = {}
    procs = {}

    # ── Modülleri başlat ──
    print("🚀 Modüller başlatılıyor...\n")
    for mod in MODULES:
        sys.stdout.write(f"  {mod['icon']}  {mod['name']:10s} başlatılıyor... ")
        sys.stdout.flush()

        try:
            proc = start_module(mod, log_dir)
            procs[mod["id"]] = proc
            pids[mod["id"]] = proc.pid
            print(f"PID={proc.pid}")
        except Exception as e:
            print(f"❌ HATA: {e}")

    save_pids(pids)

    # ── Sağlık kontrolü ──
    print("\n⏳ Modüller başlaması bekleniyor...\n")
    alive = 0
    for mod in MODULES:
        sys.stdout.write(f"  {mod['icon']}  {mod['name']:10s} :{mod['health_port']}  ")
        sys.stdout.flush()

        if mod["id"] not in procs:
            print("⚪ atlandı")
            continue

        ok = wait_for_health(mod, max_wait=20)
        if ok:
            print("✅ hazır")
            alive += 1
        else:
            print("❌ zaman aşımı")

    # ── Özet ──
    total = len(MODULES)
    print(f"\n{'═' * 50}")
    print(f"  📊 {alive}/{total} modül aktif")

    if alive > 0:
        print(f"\n  🔗 Modül Adresleri:")
        for mod in MODULES:
            if mod["id"] in procs:
                status = "✅" if check_health(mod["health_port"], mod["health_path"], 1) else "❌"
                print(f"     {status} {mod['name']:10s} → http://127.0.0.1:{mod['health_port']}")

    print(f"\n  📁 Loglar: {log_dir}/")
    print(f"  🛑 Durdurmak için: python3 {Path(__file__).name} --kill")

    # ── Simülasyon Web UI ──
    if not no_web:
        print(f"\n{'═' * 50}")
        print(f"  🎮 Simülasyon Web Arayüzü başlatılıyor...")
        print(f"  🌐 http://localhost:8080")
        print(f"  ⚡ connect_modules=true → Gerçek modüllere bağlanır")
        print(f"{'═' * 50}\n")

        try:
            env = os.environ.copy()
            env["PATH"] = f"/opt/homebrew/bin:{env.get('PATH', '')}"
            subprocess.run(
                [PYTHON, "-m", "uvicorn", "src.web:app",
                 "--host", "0.0.0.0", "--port", "8080",
                 "--log-level", "info"],
                cwd=str(BASE_DIR / "simulasyon"),
                env=env,
            )
        except KeyboardInterrupt:
            print("\n\n⚠️  Web arayüzü kapatıldı.")
            ask = input("Modülleri de durdurmak ister misiniz? [E/h]: ").strip().lower()
            if ask != "h":
                kill_all()
    else:
        print(f"\n  ℹ️  --no-web: Web arayüzü başlatılmadı.")
        print(f"  Manuel başlatmak için:")
        print(f"     cd simulasyon && python3 -m uvicorn src.web:app --port 8080")
        print(f"\n  Ctrl+C ile bu süreci kapatabilirsiniz.")
        try:
            signal.pause()
        except KeyboardInterrupt:
            print("\n")
            ask = input("Modülleri durdurmak ister misiniz? [E/h]: ").strip().lower()
            if ask != "h":
                kill_all()


def main():
    parser = argparse.ArgumentParser(description="ÖTÜKEN-MATRİS Tam Sistem Başlatıcı")
    parser.add_argument("--kill", action="store_true", help="Tüm modülleri durdur")
    parser.add_argument("--no-web", action="store_true", help="Web arayüzü olmadan başlat")
    parser.add_argument("--status", action="store_true", help="Modül durumlarını kontrol et")
    parser.add_argument("--prod", action="store_true", help="Üretim ortamında başlat (derlenmiş binary)")
    args = parser.parse_args()

    if args.prod:
        os.environ["OTUKEN_ENV"] = "prod"

    if args.kill:
        kill_all()
    elif args.status:
        print("\n📊 Modül Durumları:\n")
        for mod in MODULES:
            ok = check_health(mod["health_port"], mod["health_path"])
            status = "✅ aktif" if ok else "❌ kapalı"
            print(f"  {mod['icon']}  {mod['name']:10s} :{mod['health_port']:5d}  {status}")
        print()
    else:
        start_all(no_web=args.no_web)


if __name__ == "__main__":
    main()
