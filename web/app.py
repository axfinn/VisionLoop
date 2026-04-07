from __future__ import annotations
from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles
from fastapi.responses import FileResponse
from pathlib import Path

from web.routes import stream, events, faces, playback, config_route
from web.broadcaster import StreamBroadcaster


def create_app(
    broadcaster: StreamBroadcaster,
    db,
    face_detector,
    recorder,
    known_faces_dir: str,
    fps_getter,
    snapshots_dir: str = "snapshots",
) -> FastAPI:
    app = FastAPI(title="BBLuVideo 智能监控")

    stream.set_broadcaster(broadcaster)
    events.set_db(db)
    faces.set_face_detector(face_detector, known_faces_dir)
    faces.set_snapshots_dir(snapshots_dir)
    playback.set_recorder(recorder)
    config_route.set_config_path("config.yaml")

    app.include_router(stream.router)
    app.include_router(events.router)
    app.include_router(faces.router)
    app.include_router(playback.router)
    app.include_router(config_route.router)

    @app.get("/api/status")
    async def status():
        return {
            "fps": round(fps_getter(), 1),
            "known_faces": face_detector.known_count,
        }

    static_dir = Path(__file__).parent / "static"
    app.mount("/static", StaticFiles(directory=str(static_dir)), name="static")

    @app.get("/")
    async def index():
        return FileResponse(str(static_dir / "index.html"))

    return app
