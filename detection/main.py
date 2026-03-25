#!/usr/bin/env python3
"""
VisionLoop Detection Process
YOLOv8姿态检测 + 音频频谱分析
"""

import os
import sys
import json
import time
import socket
import struct
import threading
import argparse
from datetime import datetime

# 添加当前目录到路径
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from yolo import YOLODetector
from audio import AudioDetector

SOCKET_PATH = "/tmp/visionloop_det.sock"
API_BASE = "http://localhost:8080"


class DetectionProcess:
    """检测主进程"""

    def __init__(self, socket_path=SOCKET_PATH, api_base=API_BASE):
        self.socket_path = socket_path
        self.api_base = api_base
        self.running = True
        self.frame_interval = 0.5  # 500ms抽一帧

        # 初始化检测器
        self.yolo = YOLODetector()
        self.audio = AudioDetector()

        # 检测开关
        self.config = {
            "detect_fall": True,
            "detect_cry": True,
            "detect_noise": True,
            "detect_intruder": True,
            "sensitivity": 0.7,
        }

        # 事件回调
        self.on_event = None

    def start(self):
        """启动检测进程"""
        print(f"[Detection] Starting on {self.socket_path}")

        # 清理旧socket
        if os.path.exists(self.socket_path):
            os.unlink(self.socket_path)

        server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        server.settimeout(1.0)
        server.bind(self.socket_path)
        server.listen(1)

        print("[Detection] Listening for frames...")

        while self.running:
            try:
                conn, _ = server.accept()
                self._handle_connection(conn)
            except socket.timeout:
                continue
            except Exception as e:
                print(f"[Detection] Accept error: {e}")
                continue

        server.close()
        if os.path.exists(self.socket_path):
            os.unlink(self.socket_path)
        print("[Detection] Stopped")

    def _handle_connection(self, conn):
        """处理连接"""
        print("[Detection] Client connected")

        try:
            while self.running:
                # 读取帧头 (24 bytes)
                header = self._recv_exact(conn, 24)
                if not header:
                    break

                width, height, channels, frame_type, timestamp = struct.unpack(
                    "<iiiiq", header
                )

                # 读取图像数据
                expected_size = width * height * channels
                data = self._recv_exact(conn, expected_size)
                if not data or len(data) != expected_size:
                    break

                # 处理帧
                self._process_frame(data, width, height, channels, timestamp)

        except Exception as e:
            print(f"[Detection] Connection error: {e}")
        finally:
            conn.close()
            print("[Detection] Client disconnected")

    def _recv_exact(self, conn, size):
        """接收指定大小的数据"""
        chunks = []
        remaining = size
        while remaining > 0:
            try:
                chunk = conn.recv(remaining)
                if not chunk:
                    return None
                chunks.append(chunk)
                remaining -= len(chunk)
            except socket.timeout:
                return None
        return b"".join(chunks)

    def _process_frame(self, data, width, height, channels, timestamp):
        """处理帧"""
        ts_ns = timestamp
        ts = datetime.fromtimestamp(ts_ns / 1e9)

        # YOLO检测
        if self.config["detect_fall"] or self.config["detect_intruder"]:
            results = self.yolo.detect(data, width, height)
            for r in results:
                if r["type"] == "fall" and self.config["detect_fall"]:
                    self._emit_event("fall", r["confidence"], ts_ns, ts, r)
                elif r["type"] == "person" and self.config["detect_intruder"]:
                    self._emit_event("intruder", r["confidence"], ts_ns, ts, r)

        # 音频检测
        if self.config["detect_cry"] or self.config["detect_noise"]:
            audio_results = self.audio.detect(data, width, height)
            for r in audio_results:
                if r["type"] == "cry" and self.config["detect_cry"]:
                    self._emit_event("cry", r["confidence"], ts_ns, ts, r)
                elif r["type"] == "noise" and self.config["detect_noise"]:
                    self._emit_event("noise", r["confidence"], ts_ns, ts, r)

    def _emit_event(self, event_type, confidence, timestamp_ns, timestamp, data):
        """发送事件"""
        event = {
            "type": event_type,
            "timestamp": timestamp_ns,
            "confidence": confidence,
            "clip_name": "",  # Go侧填充
            "clip_offset": 0,
            "screenshot": data.get("screenshot", ""),
            "created_at": int(time.time() * 1000),
        }

        print(f"[Detection] Event: {event_type} @ {timestamp}, conf={confidence:.2f}")

        # 发送到Go API
        self._send_event_to_api(event)

        if self.on_event:
            self.on_event(event)

    def _send_event_to_api(self, event):
        """发送事件到Go API"""
        try:
            import urllib.request

            req = urllib.request.Request(
                f"{self.api_base}/api/events",
                data=json.dumps(event).encode(),
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            with urllib.request.urlopen(req, timeout=5) as resp:
                if resp.status == 200:
                    print(f"[Detection] Event sent to API")
        except Exception as e:
            print(f"[Detection] Failed to send event to API: {e}")

    def update_config(self, config):
        """更新配置"""
        self.config.update(config)
        print(f"[Detection] Config updated: {self.config}")

    def stop(self):
        """停止"""
        self.running = False


def main():
    parser = argparse.ArgumentParser(description="VisionLoop Detection Process")
    parser.add_argument(
        "--socket", default=SOCKET_PATH, help="Unix socket path"
    )
    parser.add_argument(
        "--api", default=API_BASE, help="Go API base URL"
    )
    args = parser.parse_args()

    proc = DetectionProcess(socket_path=args.socket, api_base=args.api)

    # 信号处理
    def signal_handler(sig, frame):
        print("\n[Detection] Shutting down...")
        proc.stop()
        sys.exit(0)

    import signal

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    proc.start()


if __name__ == "__main__":
    main()
