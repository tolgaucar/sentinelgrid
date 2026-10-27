#!/usr/bin/env python3
"""
ÖTÜKEN-MATRİS — Tanıtım Sunucusu

Modül tanıtım sayfasını başlatır.

Kullanım:
    python3 serve.py              # http://localhost:8888
    python3 serve.py --port 9999  # Farklı port
"""

import argparse
import http.server
import os
import functools


def main():
    parser = argparse.ArgumentParser(description="ÖTÜKEN-MATRİS Tanıtım Sunucusu")
    parser.add_argument("--port", type=int, default=8888, help="HTTP port (varsayılan: 8888)")
    args = parser.parse_args()

    os.chdir(os.path.dirname(os.path.abspath(__file__)))

    handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=".")
    with http.server.HTTPServer(("0.0.0.0", args.port), handler) as srv:
        print(f"\n╔═══════════════════════════════════════════╗")
        print(f"║   ÖTÜKEN-MATRİS — Tanıtım Sayfası         ║")
        print(f"║   http://localhost:{args.port}                  ║")
        print(f"╚═══════════════════════════════════════════╝\n")
        try:
            srv.serve_forever()
        except KeyboardInterrupt:
            print("\n✅ Sunucu kapatıldı.")


if __name__ == "__main__":
    main()
