from __future__ import annotations
import os
from pathlib import Path
import cv2
import numpy as np
import face_recognition
from detectors.base import Detector, Detection, BBox
from config import FaceConfig


class FaceDetector(Detector):
    """人脸检测 + 识别，未知人脸触发陌生人警报。"""

    def __init__(self, cfg: FaceConfig, known_faces_dir: str = "known_faces") -> None:
        self._cfg = cfg
        self._known_encodings: list[np.ndarray] = []
        self._known_names: list[str] = []
        self._load_known_faces(known_faces_dir)

    def _load_known_faces(self, directory: str) -> None:
        self._known_encodings.clear()
        self._known_names.clear()
        p = Path(directory)
        if not p.exists():
            return
        for img_path in p.glob("*.[jp][pn]g"):
            img = face_recognition.load_image_file(str(img_path))
            encs = face_recognition.face_encodings(img)
            if encs:
                self._known_encodings.append(encs[0])
                self._known_names.append(img_path.stem)

    def reload(self, known_faces_dir: str = "known_faces") -> None:
        self._load_known_faces(known_faces_dir)

    @property
    def known_count(self) -> int:
        return len(self._known_names)

    def process(self, frame: np.ndarray) -> list[Detection]:
        if not self._cfg.enabled:
            return []

        scale = self._cfg.scale
        small = cv2.resize(frame, (0, 0), fx=scale, fy=scale)
        rgb = cv2.cvtColor(small, cv2.COLOR_BGR2RGB)

        locations = face_recognition.face_locations(rgb, model="hog")
        if not locations:
            return []

        encodings = face_recognition.face_encodings(rgb, locations)
        detections: list[Detection] = []

        for enc, loc in zip(encodings, locations):
            top, right, bottom, left = loc
            # 还原到原始分辨率
            top = int(top / scale)
            right = int(right / scale)
            bottom = int(bottom / scale)
            left = int(left / scale)

            name = "陌生人"
            confidence = 0.0
            is_stranger = True

            if self._known_encodings:
                distances = face_recognition.face_distance(self._known_encodings, enc)
                best_idx = int(np.argmin(distances))
                best_dist = float(distances[best_idx])
                if best_dist <= self._cfg.tolerance:
                    name = self._known_names[best_idx]
                    confidence = round(1.0 - best_dist, 3)
                    is_stranger = False
                else:
                    confidence = round(best_dist, 3)

            detections.append(Detection(
                type="stranger" if is_stranger else "face",
                label=name,
                confidence=confidence,
                bbox=BBox(left, top, right - left, bottom - top),
                extra={"is_stranger": is_stranger},
            ))

        return detections
