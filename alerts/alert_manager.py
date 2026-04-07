from __future__ import annotations
import time
from detectors.base import Detection
from config import AlertsConfig


class AlertManager:
    """订阅检测事件，应用冷却时间，触发通知。"""

    def __init__(self, cfg: AlertsConfig, notifiers: list) -> None:
        self._cfg = cfg
        self._notifiers = notifiers
        self._last_alert: dict[str, float] = {}

    async def handle(self, detections: list[Detection], frame=None, snapshot_saver=None, db=None) -> None:
        for det in detections:
            key = f"{det.type}:{det.label}"
            now = time.time()
            last = self._last_alert.get(key, 0)
            if now - last < self._cfg.cooldown_seconds:
                continue
            self._last_alert[key] = now

            snapshot_path = None
            if self._cfg.save_snapshots and frame is not None and snapshot_saver:
                snapshot_path = snapshot_saver.save(frame, det)

            if db:
                await db.insert_event(det, snapshot_path)

            for notifier in self._notifiers:
                await notifier.notify(det, snapshot_path)
