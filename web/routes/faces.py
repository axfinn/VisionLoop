from __future__ import annotations
import shutil
from pathlib import Path
from fastapi import APIRouter, UploadFile, File, Form, HTTPException
from fastapi.responses import JSONResponse

router = APIRouter()
_known_faces_dir: str = "known_faces"
_face_detector = None
_snapshots_dir: str = "snapshots"


def set_snapshots_dir(d: str) -> None:
    global _snapshots_dir
    _snapshots_dir = d


def set_face_detector(fd, known_dir: str) -> None:
    global _face_detector, _known_faces_dir
    _face_detector = fd
    _known_faces_dir = known_dir


@router.get("/api/faces")
async def list_faces():
    p = Path(_known_faces_dir)
    names = [f.stem for f in p.glob("*.[jp][pn]g")]
    return {"faces": names, "count": len(names)}


@router.post("/api/faces")
async def enroll_face(name: str = Form(...), file: UploadFile = File(...)):
    if not name.strip():
        raise HTTPException(400, "名字不能为空")
    ext = Path(file.filename).suffix.lower()
    if ext not in (".jpg", ".jpeg", ".png"):
        raise HTTPException(400, "仅支持 jpg/png 格式")
    dest = Path(_known_faces_dir) / f"{name.strip()}{ext}"
    with open(dest, "wb") as f:
        shutil.copyfileobj(file.file, f)
    if _face_detector:
        _face_detector.reload(_known_faces_dir)
    return {"message": f"已注册: {name}", "path": str(dest)}


@router.post("/api/faces/from-snapshot")
async def enroll_from_snapshot(name: str = Form(...), snapshot_filename: str = Form(...)):
    """从事件截图直接注册人脸。"""
    if not name.strip():
        raise HTTPException(400, "名字不能为空")
    src = Path(_snapshots_dir) / snapshot_filename
    if not src.exists():
        raise HTTPException(404, "截图文件不存在")
    ext = src.suffix.lower()
    dest = Path(_known_faces_dir) / f"{name.strip()}{ext}"
    shutil.copy2(src, dest)
    if _face_detector:
        _face_detector.reload(_known_faces_dir)
    return {"message": f"已注册: {name}", "path": str(dest)}


@router.delete("/api/faces/{name}")
async def delete_face(name: str):
    p = Path(_known_faces_dir)
    deleted = []
    for f in p.glob(f"{name}.*"):
        f.unlink()
        deleted.append(f.name)
    if not deleted:
        raise HTTPException(404, "未找到该人脸")
    if _face_detector:
        _face_detector.reload(_known_faces_dir)
    return {"deleted": deleted}
