from __future__ import annotations
import threading
import queue
import cv2
import numpy as np
from config import CameraConfig


class Camera:
    """后台线程持续采集摄像头帧，外部通过 get_frame() 获取最新帧。"""

    def __init__(self, cfg: CameraConfig) -> None:
        self._cfg = cfg
        self._queue: queue.Queue[np.ndarray] = queue.Queue(maxsize=2)
        self._stop_event = threading.Event()
        self._thread = threading.Thread(target=self._capture_loop, daemon=True)

    def start(self) -> None:
        self._thread.start()

    def stop(self) -> None:
        self._stop_event.set()
        self._thread.join(timeout=3)

    def get_frame(self, timeout: float = 1.0) -> np.ndarray | None:
        try:
            return self._queue.get(timeout=timeout)
        except queue.Empty:
            return None

    def _capture_loop(self) -> None:
        cap = cv2.VideoCapture(self._cfg.source)
        cap.set(cv2.CAP_PROP_FRAME_WIDTH, self._cfg.width)
        cap.set(cv2.CAP_PROP_FRAME_HEIGHT, self._cfg.height)
        cap.set(cv2.CAP_PROP_FPS, self._cfg.fps)

        if not cap.isOpened():
            raise RuntimeError(f"无法打开摄像头: {self._cfg.source}")

        while not self._stop_event.is_set():
            ret, frame = cap.read()
            if not ret:
                continue
            # 丢弃旧帧，保持队列最新
            if self._queue.full():
                try:
                    self._queue.get_nowait()
                except queue.Empty:
                    pass
            self._queue.put_nowait(frame)

        cap.release()
