from __future__ import annotations
from detectors.base import Detection

TYPE_LABELS = {
    "motion": "运动检测",
    "intrusion": "⚠️ 入侵警报",
    "face": "人脸识别",
    "stranger": "🚨 陌生人警报",
    "object": "物体检测",
}


class DesktopNotifier:
    async def notify(self, det: Detection, snapshot_path: str | None = None) -> None:
        # 只对高优先级事件发桌面通知
        if det.type not in ("intrusion", "stranger"):
            return
        try:
            from plyer import notification
            title = TYPE_LABELS.get(det.type, det.type)
            msg = f"检测到: {det.label}  置信度: {det.confidence:.0%}"
            notification.notify(title=title, message=msg, timeout=5)
        except Exception:
            pass
