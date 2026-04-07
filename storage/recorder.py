from __future__ import annotations
import cv2
import numpy as np
import threading
import time
import os
from pathlib import Path
from datetime import datetime


class VideoRecorder:
    """
    循环录制：每 segment_minutes 分钟一个文件，最多保留 max_segments 个文件。
    事件锁定：lock_segment() 标记某文件不被覆盖（最多保留 max_locked 个锁定文件）。
    """

    def __init__(
        self,
        recordings_dir: str = "recordings",
        segment_minutes: int = 1,
        max_segments: int = 20,
        max_locked: int = 20,
    ) -> None:
        self._dir = Path(recordings_dir)
        self._dir.mkdir(parents=True, exist_ok=True)
        self._segment_seconds = segment_minutes * 60
        self._max_segments = max_segments
        self._max_locked = max_locked

        self._writer: cv2.VideoWriter | None = None
        self._segment_start: float = 0
        self._current_file: str = ""
        self._fps = 15.0
        self._lock = threading.Lock()
        self._locked_files: set[str] = set()

    def write(self, frame: np.ndarray) -> None:
        with self._lock:
            now = time.time()
            if self._writer is None or (now - self._segment_start) >= self._segment_seconds:
                self._rotate(frame)
            if self._writer:
                self._writer.write(frame)

    def current_file(self) -> str:
        return self._current_file

    def lock_around_event(self) -> list[str]:
        """锁定当前文件及前一个文件（覆盖事件前后约1分钟），返回被锁定的文件列表。"""
        with self._lock:
            locked = []
            # 当前文件
            if self._current_file and Path(self._current_file).exists():
                self._locked_files.add(self._current_file)
                locked.append(self._current_file)
            # 前一个文件
            prev = self._prev_file()
            if prev and Path(prev).exists():
                self._locked_files.add(prev)
                locked.append(prev)
            # 超出 max_locked 时解锁最旧的
            if len(self._locked_files) > self._max_locked:
                oldest = sorted(self._locked_files)[0]
                self._locked_files.discard(oldest)
            return locked

    def _prev_file(self) -> str | None:
        files = sorted(self._dir.glob("rec_*.mp4"))
        if len(files) < 2:
            return None
        # 找当前文件的前一个
        names = [str(f) for f in files]
        if self._current_file in names:
            idx = names.index(self._current_file)
            if idx > 0:
                return names[idx - 1]
        return str(files[-2]) if files else None

    def _rotate(self, frame: np.ndarray) -> None:
        if self._writer:
            self._writer.release()
        h, w = frame.shape[:2]
        ts = datetime.now().strftime("%Y%m%d_%H%M%S")
        self._current_file = str(self._dir / f"rec_{ts}.mp4")
        fourcc = cv2.VideoWriter_fourcc(*"avc1")
        self._writer = cv2.VideoWriter(self._current_file, fourcc, self._fps, (w, h))
        self._segment_start = time.time()
        self._cleanup()

    def _cleanup(self) -> None:
        """删除超出 max_segments 的最旧未锁定文件。"""
        files = sorted(self._dir.glob("rec_*.mp4"))
        unlocked = [f for f in files if str(f) not in self._locked_files]
        while len(unlocked) > self._max_segments:
            unlocked[0].unlink(missing_ok=True)
            unlocked.pop(0)

    def stop(self) -> None:
        with self._lock:
            if self._writer:
                self._writer.release()
                self._writer = None

    def list_recordings(self) -> list[dict]:
        files = sorted(self._dir.glob("rec_*.mp4"), reverse=True)
        result = []
        for f in files:
            stat = f.stat()
            result.append({
                "filename": f.name,
                "path": str(f),
                "size_mb": round(stat.st_size / 1024 / 1024, 2),
                "created": datetime.fromtimestamp(stat.st_ctime).isoformat(),
                "locked": str(f) in self._locked_files,
            })
        return result
