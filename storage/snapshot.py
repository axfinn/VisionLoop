from __future__ import annotations
import cv2
import numpy as np
from datetime import datetime
from pathlib import Path
from detectors.base import Detection


class SnapshotSaver:
    def __init__(self, snapshots_dir: str = "snapshots") -> None:
        self._dir = Path(snapshots_dir)
        self._dir.mkdir(parents=True, exist_ok=True)

    def save(self, frame: np.ndarray, det: Detection) -> str:
        ts = det.timestamp.strftime("%Y%m%d_%H%M%S_%f")
        filename = f"{det.type}_{det.label}_{ts}.jpg"
        path = self._dir / filename
        cv2.imwrite(str(path), frame)
        return str(path)
