from __future__ import annotations
import asyncio
import json
from typing import Callable
from fastapi import WebSocket, WebSocketDisconnect


class StreamBroadcaster:
    """管理所有 WebSocket 连接，广播 JPEG 帧。"""

    def __init__(self) -> None:
        self._clients: set[WebSocket] = set()
        self._latest_frame: bytes | None = None
        self._lock = asyncio.Lock()

    def set_frame(self, frame_bytes: bytes) -> None:
        self._latest_frame = frame_bytes

    async def connect(self, ws: WebSocket) -> None:
        await ws.accept()
        async with self._lock:
            self._clients.add(ws)
        # 立即发送最新帧
        if self._latest_frame:
            try:
                await ws.send_bytes(self._latest_frame)
            except Exception:
                pass

    async def disconnect(self, ws: WebSocket) -> None:
        async with self._lock:
            self._clients.discard(ws)

    async def broadcast(self, frame_bytes: bytes) -> None:
        self._latest_frame = frame_bytes
        dead = set()
        for ws in list(self._clients):
            try:
                await ws.send_bytes(frame_bytes)
            except Exception:
                dead.add(ws)
        async with self._lock:
            self._clients -= dead

    async def broadcast_event(self, event: dict) -> None:
        msg = json.dumps(event, ensure_ascii=False)
        dead = set()
        for ws in list(self._clients):
            try:
                await ws.send_text(msg)
            except Exception:
                dead.add(ws)
        async with self._lock:
            self._clients -= dead
