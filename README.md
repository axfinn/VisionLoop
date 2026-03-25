# VisionLoop - 智能监控循环录制系统

版本 1.0.0 | 2026-03-25

## 特性

- **实时监看**: WebRTC低延迟推送，延迟<1秒
- **循环录制**: MP4分段录制，自动GC清理
- **事件检测**: YOLOv8姿态/哭声/异响/陌生人检测
- **历史回放**: Range请求拖拽，事件时间轴标注
- **开箱即用**: 双击运行，浏览器访问

## 系统要求

- Windows 10/11 (x64)
- Go 1.21+
- Python 3.9+ (用于检测进程)
- FFmpeg (可选，用于硬编码)

## 快速开始

### 1. 构建

```bash
# 安装Go依赖
go mod download

# 构建主程序
go build -ldflags="-s -w" -o VisionLoop.exe ./cmd/server

# 构建前端 (需要Node.js)
cd web && npm install && npm run build && cd ..

# 构建检测进程 (需要Python)
cd detection && pip install -r requirements.txt && cd ..
```

### 2. 运行

```bash
# 启动主程序
./VisionLoop.exe

# 启动检测进程 (独立终端)
cd detection && python main.py
```

### 3. 访问

打开浏览器访问 http://localhost:8080

## 项目结构

```
VisionLoop/
├── cmd/server/           # 主入口
├── internal/
│   ├── capture/          # gocv摄像头采集
│   ├── encoder/          # FFmpeg编码器
│   ├── mp4/              # MP4分段写入
│   ├── storage/          # 存储GC
│   ├── webrtc/           # Pion WebRTC
│   ├── ipc/              # Unix Socket IPC
│   └── api/              # Gin HTTP服务
├── web/                  # Vue3前端
│   ├── src/views/        # 页面组件
│   └── dist/             # 构建输出
├── detection/            # Python检测进程
│   ├── main.py           # 入口
│   ├── yolo.py           # YOLOv8检测
│   └── audio.py          # 音频检测
└── clips/                # 录像存储目录
```

## API

| 接口 | 说明 |
|------|------|
| GET / | Vue3 SPA |
| GET /api/clips | 录像列表 |
| GET /api/clips/:name | Range拖拽 |
| GET /api/events | 事件列表 |
| GET /api/storage | 存储信息 |
| GET /api/settings | 获取设置 |
| POST /api/settings | 更新设置 |
| WS /api/ws/signal | WebRTC信令 |

## 配置

配置文件: `config.yaml`

```yaml
server:
  port: 8080

storage:
  max_gb: 50
  clip_dir: ./clips

capture:
  device: 0
  width: 640
  height: 480
  fps: 25

encoding:
  record_bitrate: 4000000  # 4Mbps
  monitor_bitrate: 500000   # 500kbps
  segment_min: 5

detection:
  enabled: true
  socket: /tmp/visionloop_det.sock
  detect_fall: true
  detect_cry: true
  detect_noise: true
  detect_intruder: true
  sensitivity: 0.7
```

## 性能

| 指标 | 目标 |
|------|------|
| 监看延迟 | < 1秒 |
| 编码CPU | < 5% (硬编码) |
| 存储GC | 自动清理 |
| 启动时间 | < 5秒 |

## 技术栈

- **Go**: gocv, Pion WebRTC, Gin
- **FFmpeg**: H.264/H.265硬编码
- **Python**: YOLOv8, onnxruntime, librosa
- **Vue3**: Vite, Pinia, video.js
- **ML**: YOLOv8-pose, YOLOv8-face
