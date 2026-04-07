from __future__ import annotations
import time
import cv2
import numpy as np
from detectors.base import Detector, Detection, BBox
from config import MotionConfig


class MotionDetector(Detector):
    """基于 MOG2 背景差分的运动检测，支持入侵持续时间判断。"""

    def __init__(self, cfg: MotionConfig) -> None:
        self._cfg = cfg
        self._bg = cv2.createBackgroundSubtractorMOG2(
            history=cfg.history,
            varThreshold=cfg.var_threshold,
            detectShadows=False,
        )
        self._kernel = cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (5, 5))
        self._motion_start: float | None = None

    def process(self, frame: np.ndarray) -> list[Detection]:
        if not self._cfg.enabled:
            return []

        mask = self._bg.apply(frame)
        mask = cv2.morphologyEx(mask, cv2.MORPH_OPEN, self._kernel)
        mask = cv2.dilate(mask, self._kernel, iterations=2)

        contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
        detections: list[Detection] = []

        significant = [c for c in contours if cv2.contourArea(c) >= self._cfg.min_area]

        if significant:
            if self._motion_start is None:
                self._motion_start = time.time()

            duration = time.time() - self._motion_start
            event_type = "intrusion" if duration >= self._cfg.intrusion_seconds else "motion"

            for c in significant:
                x, y, w, h = cv2.boundingRect(c)
                detections.append(Detection(
                    type=event_type,
                    label="入侵警报" if event_type == "intrusion" else "运动检测",
                    confidence=min(cv2.contourArea(c) / 10000, 1.0),
                    bbox=BBox(x, y, w, h),
                    extra={"duration": round(duration, 1)},
                ))
        else:
            self._motion_start = None

        return detections
