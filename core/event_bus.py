from __future__ import annotations
import asyncio
from collections import defaultdict
from typing import Callable, Any


class EventBus:
    """轻量级进程内发布/订阅，支持同步和异步订阅者。"""

    def __init__(self) -> None:
        self._sync_handlers: dict[str, list[Callable]] = defaultdict(list)
        self._async_handlers: dict[str, list[Callable]] = defaultdict(list)

    def subscribe(self, event_type: str, handler: Callable) -> None:
        if asyncio.iscoroutinefunction(handler):
            self._async_handlers[event_type].append(handler)
        else:
            self._sync_handlers[event_type].append(handler)

    def unsubscribe(self, event_type: str, handler: Callable) -> None:
        self._sync_handlers[event_type].discard(handler) if hasattr(
            self._sync_handlers[event_type], "discard"
        ) else None
        try:
            self._sync_handlers[event_type].remove(handler)
        except ValueError:
            pass
        try:
            self._async_handlers[event_type].remove(handler)
        except ValueError:
            pass

    async def publish(self, event_type: str, data: Any) -> None:
        for handler in self._sync_handlers.get(event_type, []):
            handler(data)
        for handler in self._async_handlers.get(event_type, []):
            await handler(data)

    def publish_sync(self, event_type: str, data: Any) -> None:
        for handler in self._sync_handlers.get(event_type, []):
            handler(data)


# 全局单例
bus = EventBus()
