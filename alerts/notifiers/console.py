from __future__ import annotations
from detectors.base import Detection


class ConsoleNotifier:
    async def notify(self, det: Detection, snapshot_path: str | None = None) -> None:
        snap = f" → {snapshot_path}" if snapshot_path else ""
        print(f"[{det.timestamp.strftime('%H:%M:%S')}] [{det.type.upper()}] {det.label} ({det.confidence:.0%}){snap}")
