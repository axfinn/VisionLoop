from __future__ import annotations
import asyncio
import time
import threading
import cv2
import numpy as np
from PIL import Image, ImageDraw, ImageFont
from typing import Callable, Awaitable
from detectors.base import Detection
from detectors.motion import MotionDetector
from detectors.face import FaceDetector
from detectors.object_detector import ObjectDetector
from config import AppConfig

# 颜色映射（BGR）
COLORS = {
    "motion": (0, 255, 255),
    "intrusion": (0, 0, 255),
    "face": (0, 255, 0),
    "stranger": (0, 128, 255),
    "object": (255, 128, 0),
}

# 中文字体
_FONT_PATHS = [
    "/System/Library/Fonts/STHeiti Light.ttc",
    "/System/Library/Fonts/PingFang.ttc",
    "/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
    "/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
]
_font_cache: dict[int, ImageFont.FreeTypeFont] = {}


def _get_font(size: int) -> ImageFont.FreeTypeFont | None:
    if size in _font_cache:
        return _font_cache[size]
    for path in _FONT_PATHS:
        try:
            f = ImageFont.truetype(path, size)
            _font_cache[size] = f
            return f
        except Exception:
            continue
    return None


def put_text_cn(img: np.ndarray, text: str, pos: tuple, color_bgr: tuple, size: int = 20) -> np.ndarray:
    """在 numpy 图像上渲染中文文字（PIL 实现）。"""
    font = _get_font(size)
    if font is None:
        cv2.putText(img, text, pos, cv2.FONT_HERSHEY_SIMPLEX, size / 30, color_bgr, 2)
        return img
    pil = Image.fromarray(cv2.cvtColor(img, cv2.COLOR_BGR2RGB))
    draw = ImageDraw.Draw(pil)
    color_rgb = (color_bgr[2], color_bgr[1], color_bgr[0])
    draw.text(pos, text, font=font, fill=color_rgb)
    return cv2.cvtColor(np.array(pil), cv2.COLOR_RGB2BGR)


class FrameProcessor:
    """在独立线程中运行所有检测器，通过回调输出标注帧和检测结果。"""

    def __init__(
        self,
        cfg: AppConfig,
        on_frame: Callable[[bytes], None],
        on_detections: Callable[[list[Detection], np.ndarray], Awaitable[None]],
        loop: asyncio.AbstractEventLoop,
        on_raw_frame: Callable[[np.ndarray], None] | None = None,
    ) -> None:
        self._cfg = cfg
        self._on_frame = on_frame
        self._on_detections = on_detections
        self._loop = loop
        self._on_raw_frame = on_raw_frame

        self._motion = MotionDetector(cfg.detectors.motion)
        self._face = FaceDetector(cfg.detectors.face, cfg.paths.known_faces)
        self._object = ObjectDetector(cfg.detectors.object)

        self._stop_event = threading.Event()
        self._thread: threading.Thread | None = None
        self._fps = 0.0
        self._frame_count = 0
        self._fps_time = time.time()

    @property
    def face_detector(self) -> FaceDetector:
        return self._face

    @property
    def fps(self) -> float:
        return self._fps

    def start(self, get_frame: Callable) -> None:
        self._get_frame = get_frame
        self._thread = threading.Thread(target=self._run, daemon=True)
        self._thread.start()

    def stop(self) -> None:
        self._stop_event.set()
        if self._thread:
            self._thread.join(timeout=5)

    def _run(self) -> None:
        while not self._stop_event.is_set():
            frame = self._get_frame(timeout=1.0)
            if frame is None:
                continue

            detections: list[Detection] = []

            # 运动检测（始终运行）
            motion_dets = self._motion.process(frame)
            detections.extend(motion_dets)

            # 有运动时才运行人脸和物体检测
            if motion_dets:
                face_dets = self._face.process(frame)
                detections.extend(face_dets)
                obj_dets = self._object.process(frame)
                detections.extend(obj_dets)

            # 标注帧
            annotated = self._annotate(frame, detections)

            # 原始帧回调（用于录像）
            if self._on_raw_frame:
                self._on_raw_frame(frame)

            # 编码为 JPEG
            quality = self._cfg.web.stream_quality
            _, buf = cv2.imencode(".jpg", annotated, [cv2.IMWRITE_JPEG_QUALITY, quality])
            self._on_frame(buf.tobytes())

            # 异步发布检测事件（带原始帧用于截图）
            if detections:
                asyncio.run_coroutine_threadsafe(
                    self._on_detections(detections, frame), self._loop
                )

            # FPS 计算
            self._frame_count += 1
            now = time.time()
            if now - self._fps_time >= 1.0:
                self._fps = self._frame_count / (now - self._fps_time)
                self._frame_count = 0
                self._fps_time = now

    def _annotate(self, frame: np.ndarray, detections: list[Detection]) -> np.ndarray:
        out = frame.copy()
        for det in detections:
            if det.bbox is None:
                continue
            color = COLORS.get(det.type, (255, 255, 255))
            x, y, w, h = det.bbox.as_tuple()
            cv2.rectangle(out, (x, y), (x + w, y + h), color, 2)
            label = f"{det.label} {det.confidence:.0%}"
            out = put_text_cn(out, label, (x, max(y - 24, 0)), color, size=20)

        # FPS 显示
        out = put_text_cn(out, f"FPS: {self._fps:.1f}", (10, 8), (0, 255, 0), size=22)
        return out
