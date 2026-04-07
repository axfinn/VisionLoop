from __future__ import annotations
import subprocess
import json
from pathlib import Path
from fastapi import APIRouter, HTTPException, Request
from fastapi.responses import StreamingResponse
from storage.recorder import VideoRecorder

router = APIRouter()
_recorder: VideoRecorder | None = None


def set_recorder(r: VideoRecorder) -> None:
    global _recorder
    _recorder = r


def _probe_duration(path: Path) -> float:
    """用 ffprobe 获取视频时长（秒），失败返回 0。"""
    try:
        out = subprocess.check_output(
            ["ffprobe", "-v", "error", "-show_entries", "format=duration",
             "-of", "json", str(path)],
            stderr=subprocess.DEVNULL, timeout=5
        )
        return float(json.loads(out)["format"]["duration"])
    except Exception:
        return 0.0


@router.get("/api/recordings")
async def list_recordings():
    if _recorder is None:
        return []
    return _recorder.list_recordings()


@router.get("/api/recordings/timeline")
async def recordings_timeline():
    """返回按时间排序的录像片段列表，含开始/结束时间戳，供时间轴使用。"""
    if _recorder is None:
        return []
    recs = _recorder.list_recordings()
    recs = sorted(recs, key=lambda r: r["created"])
    result = []
    for rec in recs:
        duration = _probe_duration(Path("recordings") / rec["filename"])
        result.append({**rec, "duration": round(duration, 2)})
    return result


@router.get("/api/recordings/{filename}")
async def serve_recording(filename: str, request: Request):
    """支持 Range 请求的 mp4 文件服务（供 <video> 进度条拖动）。"""
    video_path = Path("recordings") / filename
    if not video_path.exists() or not video_path.is_file():
        raise HTTPException(404, "录像文件不存在")

    file_size = video_path.stat().st_size
    range_header = request.headers.get("range")

    if range_header:
        range_val = range_header.replace("bytes=", "")
        parts = range_val.split("-")
        start = int(parts[0])
        end = int(parts[1]) if parts[1] else file_size - 1
        end = min(end, file_size - 1)
        length = end - start + 1

        def iter_file():
            with open(video_path, "rb") as f:
                f.seek(start)
                remaining = length
                while remaining > 0:
                    chunk = f.read(min(65536, remaining))
                    if not chunk:
                        break
                    remaining -= len(chunk)
                    yield chunk

        return StreamingResponse(
            iter_file(),
            status_code=206,
            media_type="video/mp4",
            headers={
                "Content-Range": f"bytes {start}-{end}/{file_size}",
                "Accept-Ranges": "bytes",
                "Content-Length": str(length),
            },
        )

    def iter_full():
        with open(video_path, "rb") as f:
            while chunk := f.read(65536):
                yield chunk

    return StreamingResponse(
        iter_full(),
        media_type="video/mp4",
        headers={
            "Accept-Ranges": "bytes",
            "Content-Length": str(file_size),
        },
    )
