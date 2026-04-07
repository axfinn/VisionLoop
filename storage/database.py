from __future__ import annotations
import aiosqlite
from datetime import datetime
from pathlib import Path
from detectors.base import Detection

SCHEMA = """
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    label TEXT NOT NULL,
    confidence REAL,
    bbox_x INTEGER, bbox_y INTEGER, bbox_w INTEGER, bbox_h INTEGER,
    snapshot_path TEXT,
    timestamp TEXT NOT NULL
);
"""


class Database:
    def __init__(self, db_path: str = "data.db") -> None:
        self._path = db_path
        self._db: aiosqlite.Connection | None = None

    async def init(self) -> None:
        self._db = await aiosqlite.connect(self._path)
        await self._db.execute(SCHEMA)
        await self._db.commit()

    async def insert_event(self, det: Detection, snapshot_path: str | None = None) -> int:
        bbox = det.bbox
        row = (
            det.type, det.label, det.confidence,
            bbox.x if bbox else None, bbox.y if bbox else None,
            bbox.w if bbox else None, bbox.h if bbox else None,
            snapshot_path,
            det.timestamp.isoformat(),
        )
        async with self._db.execute(
            "INSERT INTO events (type,label,confidence,bbox_x,bbox_y,bbox_w,bbox_h,snapshot_path,timestamp) "
            "VALUES (?,?,?,?,?,?,?,?,?)", row
        ) as cur:
            rowid = cur.lastrowid
        await self._db.commit()
        return rowid

    async def get_events(
        self,
        limit: int = 20,
        offset: int = 0,
        event_type: str | None = None,
        since: str | None = None,
        until: str | None = None,
    ) -> dict:
        conditions = []
        params: list = []
        if event_type:
            conditions.append("type = ?")
            params.append(event_type)
        if since:
            conditions.append("timestamp >= ?")
            params.append(since)
        if until:
            conditions.append("timestamp <= ?")
            params.append(until)
        where = ("WHERE " + " AND ".join(conditions)) if conditions else ""

        async with self._db.execute(f"SELECT COUNT(*) FROM events {where}", params) as cur:
            total = (await cur.fetchone())[0]

        sql = f"SELECT * FROM events {where} ORDER BY id DESC LIMIT ? OFFSET ?"
        async with self._db.execute(sql, params + [limit, offset]) as cur:
            cols = [d[0] for d in cur.description]
            rows = await cur.fetchall()
        return {"total": total, "items": [dict(zip(cols, r)) for r in rows]}

    async def close(self) -> None:
        if self._db:
            await self._db.close()
