"""
Terminal Canlı Dashboard

Rich kütüphanesi ile anlık simülasyon durumunu gösterir.
Modül durumları, mesaj oranları, varlık konumları, olaylar.
"""

from __future__ import annotations

import time
from typing import Any

try:
    from rich.console import Console
    from rich.table import Table
    from rich.layout import Layout
    from rich.panel import Panel
    from rich.live import Live
    from rich.text import Text
    HAS_RICH = True
except ImportError:
    HAS_RICH = False


class SimDashboard:
    """Terminal tabanlı canlı simülasyon dashboard'u."""

    def __init__(self):
        if not HAS_RICH:
            print("⚠️  rich kütüphanesi yok, basit çıktı kullanılacak")
        self.console = Console() if HAS_RICH else None
        self.tick = 0
        self.events: list[dict] = []
        self.module_status: dict[str, dict] = {}
        self.message_count = 0
        self.error_count = 0
        self.start_time = time.time()

    def update(self, tick: int, total_ticks: int, new_events: list[dict]) -> None:
        """Dashboard verilerini güncelle."""
        self.tick = tick
        self.events.extend(new_events)
        self.message_count += sum(1 for e in new_events if "published" in e.get("type", "") or "generated" in e.get("type", ""))
        self.error_count += sum(1 for e in new_events if e.get("type") == "error")

        if HAS_RICH:
            self._rich_update(tick, total_ticks, new_events)
        else:
            self._simple_update(tick, total_ticks, new_events)

    def _simple_update(self, tick: int, total_ticks: int, new_events: list[dict]) -> None:
        """Rich olmadan basit metin çıktısı."""
        elapsed = time.time() - self.start_time
        bar_len = 30
        progress = int(bar_len * (tick + 1) / total_ticks) if total_ticks > 0 else 0
        bar = "█" * progress + "░" * (bar_len - progress)

        print(f"\r  [{bar}] Tick {tick+1}/{total_ticks} | "
              f"Mesaj: {self.message_count} | Hata: {self.error_count} | "
              f"Süre: {elapsed:.0f}s", end="", flush=True)

        # Önemli olayları göster
        for e in new_events:
            if e.get("type") == "alert":
                print(f"\n  🚨 ALARM: {e.get('alert_type')} - {e.get('detail')}")
            elif e.get("type") == "damage":
                print(f"\n  💥 HASAR: Varlık {e.get('entity')} - Sağlık: {e.get('health', 0):.0f}%")

    def _rich_update(self, tick: int, total_ticks: int, new_events: list[dict]) -> None:
        """Rich kütüphanesi ile zengin çıktı."""
        elapsed = time.time() - self.start_time
        rate = self.message_count / elapsed if elapsed > 0 else 0

        # Progress
        progress = (tick + 1) / total_ticks * 100 if total_ticks > 0 else 0
        self.console.print(
            f"  [bold cyan]Tick[/] {tick+1}/{total_ticks} "
            f"([green]{progress:.0f}%[/]) | "
            f"[bold]Mesaj:[/] {self.message_count} "
            f"([yellow]{rate:.1f}/s[/]) | "
            f"[bold red]Hata:[/] {self.error_count} | "
            f"[dim]{elapsed:.0f}s[/]",
            end="\r",
        )

        # Önemli olaylar
        for e in new_events:
            if e.get("type") == "alert":
                self.console.print(f"\n  🚨 [bold red]ALARM:[/] {e.get('alert_type')} - {e.get('detail')}")
            elif e.get("type") == "damage":
                self.console.print(f"\n  💥 [yellow]HASAR:[/] {e.get('entity')} - Sağlık: {e.get('health', 0):.0f}%")

    def final_report(self, results: dict) -> None:
        """Son raporu göster."""
        if HAS_RICH:
            self._rich_report(results)
        else:
            # Basit çıktı runner.py'de zaten yapılıyor
            print()

    def _rich_report(self, results: dict) -> None:
        """Rich ile detaylı son rapor."""
        self.console.print()
        table = Table(title="📊 Simülasyon Sonuçları", show_header=True, header_style="bold cyan")
        table.add_column("Metrik", style="bold")
        table.add_column("Değer", justify="right")

        stats = results.get("stats", {})
        duration = stats.get("end_time", 0) - stats.get("start_time", 0)

        table.add_row("Senaryo", results.get("scenario", "?"))
        table.add_row("Tick Sayısı", str(results.get("ticks", 0)))
        table.add_row("Süre", f"{duration:.1f}s")
        table.add_row("Mesaj Gönderilen", str(stats.get("messages_sent", 0)))
        table.add_row("Hata", str(stats.get("errors", 0)))
        table.add_row("Olay Sayısı", str(results.get("event_count", 0)))

        bf = results.get("battlefield", {})
        if bf:
            table.add_row("─" * 20, "─" * 10)
            table.add_row("Toplam Varlık", str(bf.get("total_entities", 0)))
            table.add_row("Aktif", str(bf.get("active", 0)))
            table.add_row("İmha", str(bf.get("destroyed", 0)))

        self.console.print(table)
