from __future__ import annotations
import asyncio
import uvicorn
from config import load_config
from core.camera import Camera
from core.frame_processor import FrameProcessor
from storage.database import Database
from storage.snapshot import SnapshotSaver
from storage.recorder import VideoRecorder
from alerts.alert_manager import AlertManager
from alerts.notifiers.console import ConsoleNotifier
from alerts.notifiers.desktop import DesktopNotifier
from web.broadcaster import StreamBroadcaster
from web.app import create_app
from web.routes import config_route


async def main() -> None:
    cfg = load_config()

    # 存储
    db = Database(cfg.paths.database)
    await db.init()
    snapshot_saver = SnapshotSaver(cfg.paths.snapshots)
    recorder = VideoRecorder(
        "recordings",
        segment_minutes=cfg.recording.segment_minutes,
        max_segments=cfg.recording.max_segments,
        max_locked=cfg.recording.max_locked,
    )
    recorder.enabled = cfg.recording.enabled

    # 告警
    notifiers = [ConsoleNotifier(), DesktopNotifier()]
    alert_mgr = AlertManager(cfg.alerts, notifiers)

    # 广播器
    broadcaster = StreamBroadcaster()

    def on_frame(jpeg_bytes: bytes) -> None:
        broadcaster.set_frame(jpeg_bytes)
        asyncio.run_coroutine_threadsafe(
            broadcaster.broadcast(jpeg_bytes), loop
        )

    async def on_detections(detections, frame) -> None:
        await alert_mgr.handle(detections, frame, snapshot_saver, db)
        if any(det.type in ("intrusion", "stranger") for det in detections):
            recorder.lock_around_event()
        for det in detections:
            await broadcaster.broadcast_event({
                "type": det.type,
                "label": det.label,
                "confidence": det.confidence,
                "timestamp": det.timestamp.isoformat(),
            })

    loop = asyncio.get_event_loop()

    # 摄像头 + 处理器
    camera = Camera(cfg.camera)
    processor = FrameProcessor(
        cfg, on_frame, on_detections, loop,
        on_raw_frame=recorder.write,
    )

    camera.start()
    processor.start(camera.get_frame)

    # FastAPI
    app = create_app(
        broadcaster=broadcaster,
        db=db,
        face_detector=processor.face_detector,
        recorder=recorder,
        known_faces_dir=cfg.paths.known_faces,
        fps_getter=lambda: processor.fps,
        snapshots_dir=cfg.paths.snapshots,
    )

    def on_config_reload() -> None:
        new_cfg = load_config()
        processor.apply_config(new_cfg)
        alert_mgr._cfg = new_cfg.alerts
        recorder.enabled = new_cfg.recording.enabled
        recorder.apply_config(
            new_cfg.recording.segment_minutes,
            new_cfg.recording.max_segments,
            new_cfg.recording.max_locked,
        )

    config_route.set_reload_callback(on_config_reload)

    # 挂载 snapshots 静态目录
    from fastapi.staticfiles import StaticFiles
    import pathlib
    pathlib.Path(cfg.paths.snapshots).mkdir(exist_ok=True)
    app.mount("/snapshots", StaticFiles(directory=cfg.paths.snapshots), name="snapshots")

    print(f"\n🎥 VisionLoop 智能监控已启动")
    print(f"   浏览器访问: http://localhost:{cfg.web.port}\n")

    config = uvicorn.Config(app, host=cfg.web.host, port=cfg.web.port, log_level="warning")
    server = uvicorn.Server(config)

    try:
        await server.serve()
    finally:
        processor.stop()
        camera.stop()
        recorder.stop()
        await db.close()


if __name__ == "__main__":
    asyncio.run(main())
