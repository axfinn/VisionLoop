# VisionLoop 配置详解

本文档详细介绍 VisionLoop 的所有配置选项。

---

## 配置文件位置

配置文件位于项目根目录: `config.yaml`

---

## 配置结构

```yaml
server:
  port: 8080              # HTTP 服务端口

storage:
  max_gb: 50              # 最大存储空间 (GB)
  clip_dir: ./clips        # 录像文件存储目录
  event_dir: ./events      # 事件数据存储目录
  screenshot_dir: ./screenshots  # 事件截图存储目录

capture:
  device: 0               # 摄像头设备 ID (0=默认摄像头)
  width: 640              # 采集宽度 (像素)
  height: 480             # 采集高度 (像素)
  fps: 25                 # 采集帧率

encoding:
  record_bitrate: 4000000 # 录制码率 (4Mbps)
  monitor_bitrate: 500000 # 监看码率 (500kbps)
  segment_min: 5          # 录像分段时长 (分钟)
  hw_encoder: auto        # 硬件编码器: auto|qsv|nvenc|libx264

detection:
  enabled: true           # 是否启用事件检测
  socket: /tmp/visionloop_det.sock  # IPC socket 路径
  api_base: http://localhost:8080    # API 地址
  detect_fall: true       # 摔倒检测
  detect_cry: true        # 哭声检测
  detect_noise: true      # 异响检测
  detect_intruder: true   # 陌生人闯入检测
  sensitivity: 0.7         # 检测灵敏度 (0.0-1.0)
  frame_interval_ms: 500  # 抽帧间隔 (毫秒)
```

---

## 配置项详解

### server

HTTP 服务器配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `port` | int | 8080 | HTTP 服务监听端口 |

### storage

存储相关配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `max_gb` | float | 50 | 录像存储空间上限，超出后自动删除最旧文件 |
| `clip_dir` | string | ./clips | MP4 录像文件存储路径 |
| `event_dir` | string | ./events | 事件 JSON 数据存储路径 |
| `screenshot_dir` | string | ./screenshots | 事件截图 JPEG 文件存储路径 |

**注意**: 存储路径建议使用绝对路径，Docker 部署时需挂载卷。

### capture

摄像头采集配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `device` | int | 0 | 摄像头设备 ID，Linux 通常为 `/dev/video0` |
| `width` | int | 640 | 视频采集宽度 (像素)，影响画质和带宽 |
| `height` | int | 480 | 视频采集高度 (像素)，影响画质和带宽 |
| `fps` | int | 25 | 采集帧率，建议 25-30 |

**带宽估算**:
- 640x480 @ 25fps 原始数据: ~221 Mbps
- 编码后 (4Mbps): ~28x 压缩

### encoding

视频编码配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `record_bitrate` | int | 4000000 | 录制流 H.264 编码码率 (bps) |
| `monitor_bitrate` | int | 500000 | 监看流 H.264 编码码率 (bps) |
| `segment_min` | int | 5 | MP4 分段时长 (分钟) |
| `hw_encoder` | string | auto | 硬件编码器选择 |

**hw_encoder 选项**:
- `auto`: 自动探测可用编码器 (优先级: QSV > NVENC > libx264)
- `qsv`: Intel Quick Sync Video (需 Intel CPU)
- `nvenc`: NVIDIA NVENC (需 NVIDIA 显卡)
- `libx264`: CPU 软件编码 (兼容性最好，性能最低)

**码率建议**:
- 录制流: 4Mbps (1080p 可用 6-8Mbps)
- 监看流: 500kbps-1Mbps (网络带宽有限时降低)

### detection

事件检测引擎配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | true | 是否启用检测进程 |
| `socket` | string | /tmp/visionloop_det.sock | Unix Socket IPC 路径 |
| `api_base` | string | http://localhost:8080 | Go 服务 API 地址 |
| `detect_fall` | bool | true | 启用摔倒检测 (YOLOv8 姿态) |
| `detect_cry` | bool | true | 启用哭声检测 (音频频谱) |
| `detect_noise` | bool | true | 启用异响检测 (音频频谱) |
| `detect_intruder` | bool | true | 启用陌生人检测 (YOLOv8 人形) |
| `sensitivity` | float | 0.7 | 检测灵敏度 (0.0-1.0) |
| `frame_interval_ms` | int | 500 | 送入检测的抽帧间隔 |

**灵敏度说明**:
- 0.0: 最低灵敏度，只有最明显的事件触发
- 1.0: 最高灵敏度，可能有较多误报
- 建议值: 0.6-0.8

---

## 环境变量覆盖

配置项可以通过环境变量覆盖，优先级: 环境变量 > config.yaml

| 环境变量 | 对应配置 |
|----------|----------|
| `VL_PORT` | server.port |
| `VL_MAX_GB` | storage.max_gb |
| `VL_CLIP_DIR` | storage.clip_dir |
| `VL_DEVICE` | capture.device |
| `VL_WIDTH` | capture.width |
| `VL_HEIGHT` | capture.height |
| `VL_FPS` | capture.fps |
| `VL_RECORD_BITRATE` | encoding.record_bitrate |
| `VL_MONITOR_BITRATE` | encoding.monitor_bitrate |
| `VL_SEGMENT_MIN` | encoding.segment_min |
| `VL_HW_ENCODER` | encoding.hw_encoder |
| `VL_DETECTION_ENABLED` | detection.enabled |
| `DETECTION_SOCKET` | detection.socket |

---

## Docker 部署配置

Docker 部署时建议通过环境变量或挂载配置文件覆盖。

### docker-compose.yml 中的环境变量

```yaml
environment:
  - VL_PORT=8080
  - VL_MAX_GB=100
  - VL_DEVICE=0
  - DETECTION_ENABLED=true
```

### 挂载自定义配置

```yaml
volumes:
  - ./my-config.yaml:/app/config.yaml:ro
```

---

## 故障排除

### 摄像头无法打开

1. 检查设备是否存在: `ls -la /dev/video*`
2. 检查设备权限: `sudo chmod 666 /dev/video0`
3. 确认 device 配置正确

### 硬件编码不可用

1. QSV: 确认 Intel CPU 支持 Quick Sync
2. NVENC: 确认 NVIDIA 驱动已安装 `nvidia-smi`
3. 查看日志确认编码器探测结果

### 检测进程无法连接

1. 确认 socket 文件存在: `ls -la /tmp/visionloop_det.sock`
2. 确认 Python 检测进程已启动
3. 确认 API 地址可访问: `curl http://localhost:8080/api/storage`

### 存储空间不足

1. 检查实际使用: `df -h`
2. 降低 `max_gb` 配置
3. 手动清理旧录像: `rm -rf clips/*`