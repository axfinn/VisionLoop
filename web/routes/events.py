from __future__ import annotations
from fastapi import APIRouter, Query
from storage.database import Database

router = APIRouter()
_db: Database | None = None


def set_db(db: Database) -> None:
    global _db
    _db = db


@router.get("/api/events")
async def get_events(
    limit: int = Query(20, le=200),
    offset: int = Query(0, ge=0),
    type: str | None = None,
    since: str | None = None,
    until: str | None = None,
):
    if _db is None:
        return {"total": 0, "items": []}
    return await _db.get_events(limit=limit, offset=offset, event_type=type, since=since, until=until)
