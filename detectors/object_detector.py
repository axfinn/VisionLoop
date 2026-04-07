from __future__ import annotations
import numpy as np
import cv2
from detectors.base import Detector, Detection, BBox
from config import ObjectConfig

# COCO 类别名称（中文）
COCO_NAMES_ZH = {
    0: "人", 1: "自行车", 2: "汽车", 3: "摩托车", 4: "飞机",
    5: "公共汽车", 6: "火车", 7: "卡车", 8: "船", 9: "交通灯",
    15: "猫", 16: "狗", 17: "马", 18: "羊", 19: "牛",
}


class ObjectDetector(Detector):
    """YOLOv8n 物体检测，仅检测配置中指定的类别。"""

    def __init__(self, cfg: ObjectConfig) -> None:
        self._cfg = cfg
        self._model = None
        if cfg.enabled:
            self._load_model()

    def _load_model(self) -> None:
        from ultralytics import YOLO
        self._model = YOLO(self._cfg.model)
        # 预热
        dummy = np.zeros((640, 640, 3), dtype=np.uint8)
        self._model(dummy, verbose=False)

    def process(self, frame: np.ndarray) -> list[Detection]:
        if not self._cfg.enabled or self._model is None:
            return []

        h, w = frame.shape[:2]
        # 缩放到640px宽，保持比例
        scale = 640 / w
        resized = cv2.resize(frame, (640, int(h * scale)))

        results = self._model(
            resized,
            conf=self._cfg.confidence,
            classes=self._cfg.classes,
            verbose=False,
        )

        detections: list[Detection] = []
        for r in results:
            for box in r.boxes:
                cls_id = int(box.cls[0])
                conf = float(box.conf[0])
                x1, y1, x2, y2 = box.xyxy[0].tolist()
                # 还原到原始分辨率
                inv = 1.0 / scale
                x1, y1, x2, y2 = int(x1*inv), int(y1*inv), int(x2*inv), int(y2*inv)
                label = COCO_NAMES_ZH.get(cls_id, str(cls_id))
                detections.append(Detection(
                    type="object",
                    label=label,
                    confidence=round(conf, 3),
                    bbox=BBox(x1, y1, x2 - x1, y2 - y1),
                    extra={"class_id": cls_id},
                ))

        return detections
