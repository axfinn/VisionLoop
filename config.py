from __future__ import annotations
from pathlib import Path
from typing import Union
import yaml
from pydantic import BaseModel
from pydantic_settings import BaseSettings


class MotionConfig(BaseModel):
    enabled: bool = True
    min_area: int = 1500
    history: int = 500
    var_threshold: int = 16
    intrusion_seconds: float = 2.0


class FaceConfig(BaseModel):
    enabled: bool = True
    tolerance: float = 0.55
    scale: float = 0.5


class ObjectConfig(BaseModel):
    enabled: bool = True
    model: str = "yolov8n.pt"
    confidence: float = 0.45
    classes: list[int] = [0, 2, 15, 16]


class DetectorsConfig(BaseModel):
    motion: MotionConfig = MotionConfig()
    face: FaceConfig = FaceConfig()
    object: ObjectConfig = ObjectConfig()


class AlertsConfig(BaseModel):
    cooldown_seconds: int = 30
    save_snapshots: bool = True


class CameraConfig(BaseModel):
    source: Union[int, str] = 0
    width: int = 1280
    height: int = 720
    fps: int = 30


class WebConfig(BaseModel):
    host: str = "0.0.0.0"
    port: int = 8080
    stream_quality: int = 80


class PathsConfig(BaseModel):
    known_faces: str = "known_faces"
    snapshots: str = "snapshots"
    database: str = "data.db"


class RecordingConfig(BaseModel):
    segment_minutes: int = 1
    max_segments: int = 20
    max_locked: int = 20


class AppConfig(BaseModel):
    camera: CameraConfig = CameraConfig()
    detectors: DetectorsConfig = DetectorsConfig()
    alerts: AlertsConfig = AlertsConfig()
    recording: RecordingConfig = RecordingConfig()
    web: WebConfig = WebConfig()
    paths: PathsConfig = PathsConfig()


def load_config(path: str = "config.yaml") -> AppConfig:
    p = Path(path)
    if p.exists():
        with open(p) as f:
            data = yaml.safe_load(f)
        return AppConfig.model_validate(data)
    return AppConfig()
