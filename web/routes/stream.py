from __future__ import annotations
import asyncio
import json
from fastapi import APIRouter, WebSocket, WebSocketDisconnect
from web.broadcaster import StreamBroadcaster

router = APIRouter()
_broadcaster: StreamBroadcaster | None = None


def set_broadcaster(b: StreamBroadcaster) -> None:
    global _broadcaster
    _broadcaster = b


@router.websocket("/ws/stream")
async def stream(ws: WebSocket) -> None:
    if _broadcaster is None:
        await ws.close()
        return
    await _broadcaster.connect(ws)
    try:
        while True:
            await ws.receive_text()  # 保持连接，接收心跳
    except WebSocketDisconnect:
        pass
    finally:
        await _broadcaster.disconnect(ws)
