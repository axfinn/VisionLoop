from __future__ import annotations
import yaml
from pathlib import Path
from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

router = APIRouter()
_config_path: str = "config.yaml"
_reload_callback = None  # 可选：配置变更后回调


def set_config_path(p: str) -> None:
    global _config_path
    _config_path = p


def set_reload_callback(fn) -> None:
    global _reload_callback
    _reload_callback = fn


@router.get("/api/config")
async def get_config():
    p = Path(_config_path)
    if not p.exists():
        return {}
    with open(p) as f:
        return yaml.safe_load(f) or {}


@router.post("/api/config")
async def save_config(body: dict):
    p = Path(_config_path)
    # 读取现有配置做深度合并
    existing = {}
    if p.exists():
        with open(p) as f:
            existing = yaml.safe_load(f) or {}
    _deep_merge(existing, body)
    with open(p, "w") as f:
        yaml.dump(existing, f, allow_unicode=True, default_flow_style=False)
    if _reload_callback:
        _reload_callback()
    return {"message": "配置已保存，重启后完全生效"}


def _deep_merge(base: dict, override: dict) -> None:
    for k, v in override.items():
        if k in base and isinstance(base[k], dict) and isinstance(v, dict):
            _deep_merge(base[k], v)
        else:
            base[k] = v
