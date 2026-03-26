# VisionLoop - 智能监控循环录制系统

**版本 1.0.0** | 2026-03-26

本地智能监控解决方案，支持实时监看、历史回放和事件检测。

---

## 特性

- **实时监看**: WebRTC 低延迟推送，延迟 < 1 秒
- **循环录制**: MP4 分段录制，自动 GC 清理
- **事件检测**: 摔倒(YOLOv8姿态) / 哭声(音频频谱) / 异响 / 陌生人闯入
- **历史回放**: HTTP Range 请求拖拽，事件时间轴标注
- **开箱即用**: 双击运行，浏览器访问
- **多平台**: Windows / Linux / Docker

---

## 快速开始

### Docker 部署 (推荐)

```bash
# 克隆项目
git clone https://github.com/axfinn/VisionLoop.git
cd VisionLoop

# 创建数据目录
mkdir -p clips events screenshots logs

# 启动服务 (检测服务可选)
docker compose up -d
# 或启用检测服务
docker compose --profile detection up -d

# 访问
open http://localhost:8080
```

### Linux 直接部署

```bash
# 安装依赖
sudo apt-get install -y build-essential cmake git ffmpeg python3-dev python3-pip

# 构建
go mod download
go build -ldflags="-s -w" -o VisionLoop ./cmd/server

# 运行
mkdir -p clips events screenshots
./VisionLoop
```

### Windows 部署

1. 安装 [Go 1.21+](https://go.dev/dl/)
2. 安装 [FFmpeg](https://ffmpeg.org/download.html)
3. 克隆项目并运行:
   ```batch
   go build -ldflags="-s -w" -o VisionLoop.exe .\cmd\server
   mkdir clips events screenshots
   VisionLoop.exe
   ```

### 访问

打开浏览器访问 **http://localhost:8080**

---

## 项目结构

```
VisionLoop/
├── cmd/server/              # Go 主入口
├── internal/                 # 内部包
│   ├── api/                  # Gin HTTP 服务
│   ├── capture/              # 摄像头采集 (gocv)
│   ├── encoder/              # H.264 编码器
│   ├── mp4/                  # MP4 分段封装
│   ├── storage/              # 存储 GC
│   ├── webrtc/              # Pion WebRTC
│   └── ipc/                  # Unix Socket IPC
├── web/                      # Vue3 前端
│   ├── src/views/            # 页面组件
│   │   ├── LiveView.vue      # 实时监看
│   │   ├── PlaybackView.vue  # 历史回放
│   │   ├── EventCenter.vue   # 事件中心
│   │   └── SettingsView.vue  # 设置页
│   └── dist/                 # 构建输出
├── detection/                # Python 检测进程
│   ├── main.py              # 入口
│   ├── yolo.py              # YOLOv8 检测
│   └── audio.py             # 音频检测
├── config.yaml              # 配置文件
├── Dockerfile              # 主服务镜像
├── Dockerfile.detection    # 检测进程镜像
├── docker-compose.yml      # 编排文件
├── deploy.sh               # 部署脚本
├── CONFIG.md               # 配置详解
└── DEPLOY.md               # 部署指南
```

---

## API 文档

### HTTP 接口

| 方法 | 路径 | 说明 | 响应 |
|------|------|------|------|
| GET | `/` | Vue3 SPA | HTML |
| GET | `/api/clips` | 录像列表 | JSON |
| GET | `/api/clips/:name` | 获取录像 (Range) | MP4 |
| GET | `/api/clips/:name?start=0` | 指定偏移获取 | MP4 |
| GET | `/api/events` | 事件列表 | JSON |
| GET | `/api/events?type=fall` | 按类型筛选 | JSON |
| GET | `/api/storage` | 存储信息 | JSON |
| GET | `/api/settings` | 获取设置 | JSON |
| POST | `/api/settings` | 更新设置 | JSON |
| GET | `/api/version` | 版本信息 | JSON |
| GET | `/:path` | Vue Router fallback | HTML |

### WebSocket 接口

| 路径 | 说明 |
|------|------|
| `/api/ws/signal` | WebRTC 信令通道 |

### 响应格式

**录像列表** (`GET /api/clips`)
```json
{
  "clips": [
    {
      "name": "2026-03-26_10-30-00.mp4",
      "size": 15728640,
      "duration": 300,
      "created": "2026-03-26T10:30:00Z"
    }
  ],
  "total": 1
}
```

**事件列表** (`GET /api/events`)
```json
{
  "events": [
    {
      "id": 1,
      "type": "fall",
      "timestamp": 1711430400000,
      "clip_name": "2026-03-26_10-30-00.mp4",
      "clip_offset": 45,
      "screenshot": "/screenshots/event_1.jpg",
      "confidence": 0.87
    }
  ],
  "total": 1
}
```

**存储信息** (`GET /api/storage`)
```json
{
  "total_gb": 50.0,
  "used_gb": 12.5,
  "clips_count": 25,
  "events_count": 8
}
```

**设置** (`GET /api/settings`)
```json
{
  "max_storage_gb": 50,
  "detect_fall": true,
  "detect_cry": true,
  "detect_noise": true,
  "detect_intruder": true,
  "sensitivity": 0.7
}
```

---

## 配置

详细配置说明请参考 [CONFIG.md](CONFIG.md)。

### 快速配置

编辑 `config.yaml`:

```yaml
server:
  port: 8080

storage:
  max_gb: 50              # 最大存储 (GB)
  clip_dir: ./clips
  event_dir: ./events
  screenshot_dir: ./screenshots

capture:
  device: 0               # 摄像头 ID
  width: 640
  height: 480
  fps: 25

encoding:
  record_bitrate: 4000000  # 4Mbps 录制
  monitor_bitrate: 500000   # 500kbps 监看
  segment_min: 5
  hw_encoder: auto         # auto|qsv|nvenc|libx264

detection:
  enabled: true
  detect_fall: true
  detect_cry: true
  detect_noise: true
  detect_intruder: true
  sensitivity: 0.7
```

---

## 部署方式

详细部署说明请参考 [DEPLOY.md](DEPLOY.md)。

### Docker Compose (生产推荐)

```bash
docker compose up -d
docker compose logs -f
```

### 独立 Docker

```bash
docker build -t visionloop .
docker run -d -p 8080:8080 visionloop
```

### 部署脚本

```bash
./deploy.sh install   # 安装依赖
./deploy.sh start     # 启动
./deploy.sh stop      # 停止
./deploy.sh status    # 状态
```

---

## FAQ

**Q: 摄像头无法打开?**\nA: 检查 `/dev/video0` 权限，或修改 `config.yaml` 中的 `device` 配置。

**Q: 如何启用硬件编码?**\nA: 安装对应驱动后设置 `hw_encoder: auto` 或指定 `qsv`/`nvenc`。

**Q: 检测进程连接失败?**\nA: 确认 Python 环境已安装 YOLOv8 依赖，或在 `config.yaml` 中设置 `detection.enabled: false`。

**Q: 存储空间不足?**\nA: 降低 `max_gb` 配置，或手动清理 `clips/` 目录。

**Q: Docker 部署端口冲突?**\nA: 修改 `docker-compose.yml` 中 `8080:8080` 为 `8081:8080`。

---

## 性能指标

| 指标 | 目标 | 实测 |
|------|------|------|
| 监看延迟 | < 1s | ~800ms |
| 编码 CPU | < 5% (硬编码) | QSV/NVENC: <3% |
| 录制完整性 | 0 丢帧 | 无丢帧 |
| 启动时间 | < 5s | ~3s |
| 内存占用 | < 200MB | ~150MB |

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go, Gin, gocv, Pion WebRTC |
| 编码 | FFmpeg (H.264/H.265) |
| 检测 | Python, YOLOv8, ONNX Runtime |
| 前端 | Vue3, Vite, Pinia, video.js |
| 部署 | Docker, Docker Compose |

---

## 许可证

MIT License