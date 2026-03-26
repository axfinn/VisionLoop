# VisionLoop 部署指南

本文档介绍 VisionLoop 的各种部署方式。

---

## 部署方式概览

| 部署方式 | 适用场景 | 难度 | 推荐度 |
|----------|----------|------|--------|
| Docker Compose | 生产环境 | 低 | ⭐⭐⭐⭐⭐ |
| Docker 单容器 | 生产环境 | 中 | ⭐⭐⭐⭐ |
| Linux 直接运行 | 开发/测试 | 中 | ⭐⭐⭐ |
| Windows 直接运行 | 开发/测试 | 低 | ⭐⭐ |

---

## 方式一: Docker Compose 部署 (推荐)

### 前置要求

- Docker 20.10+
- Docker Compose 2.0+ (或 `docker compose` 插件)

### 快速部署

```bash
# 克隆项目
git clone https://github.com/axfinn/VisionLoop.git
cd VisionLoop

# 创建必要目录
mkdir -p clips events screenshots logs

# 构建并启动
docker compose up -d

# 查看状态
docker compose ps

# 查看日志
docker compose logs -f
```

### 访问服务

打开浏览器访问: http://localhost:8080

### 启用检测服务 (可选)

检测服务需要较多资源，默认禁用。使用 `--profile detection` 启用:

```bash
docker compose --profile detection up -d
```

### 停止服务

```bash
docker compose down
```

### 数据持久化

当前配置数据存储在 Docker 卷中。如需持久化到宿主机:

```yaml
# 修改 docker-compose.yml 中的 volumes
volumes:
  - ./clips:/app/clips
  - ./events:/app/events
  - ./screenshots:/app/screenshots
```

---

## 方式二: Docker 单容器部署

### 构建镜像

```bash
docker build -t visionloop:latest .
```

### 运行容器

```bash
docker run -d \
  --name visionloop \
  -p 8080:8080 \
  -v $(pwd)/clips:/app/clips \
  -v $(pwd)/events:/app/events \
  -v $(pwd)/screenshots:/app/screenshots \
  visionloop:latest
```

### 查看日志

```bash
docker logs -f visionloop
```

---

## 方式三: Linux 直接部署

### 前置要求

- Go 1.21+
- Python 3.9+ (用于检测进程)
- FFmpeg (可选，推荐安装以启用硬件编码)

### 安装依赖 (Ubuntu/Debian)

```bash
# 系统依赖
sudo apt-get update
sudo apt-get install -y \
    build-essential \
    cmake \
    git \
    wget \
    curl \
    ffmpeg \
    libavcodec-dev \
    libavformat-dev \
    libswscale-dev \
    libgomp1 \
    python3-dev \
    python3-pip \
    python3-venv

# Python 虚拟环境
cd detection
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
deactivate
cd ..
```

### 构建项目

```bash
# 构建前端
cd web
npm install
npm run build
cd ..

# 构建 Go 服务
go mod download
go build -ldflags="-s -w" -o VisionLoop ./cmd/server

# 构建检测进程 (可选)
cd detection
pyinstaller --onefile --name visionloop_det main.py
cd ..
```

### 运行

```bash
# 创建数据目录
mkdir -p clips events screenshots logs

# 启动服务
./VisionLoop

# 或使用部署脚本
./deploy.sh install
./deploy.sh start
```

---

## 方式四: Windows 部署

### 前置要求

- Go 1.21+ (安装时勾选 "Add to PATH")
- Python 3.9+ (安装时勾选 "Add to PATH")
- FFmpeg (推荐，从 https://ffmpeg.org/download.html 下载)

### 构建

```batch
:: 构建前端
cd web
npm install
npm run build
cd ..

:: 构建 Go 服务
go mod download
go build -ldflags="-s -w" -o VisionLoop.exe .\cmd\server

:: 构建检测进程 (可选，需要 PyInstaller)
cd detection
pyinstaller --onefile --name visionloop_det main.py
cd ..
```

### 运行

双击 `VisionLoop.exe` 或在命令行中运行:

```batch
VisionLoop.exe
```

---

## 使用部署脚本

项目提供了 `deploy.sh` 脚本，简化部署操作:

```bash
# 安装依赖并构建
./deploy.sh install

# 启动服务
./deploy.sh start

# 停止服务
./deploy.sh stop

# 查看状态
./deploy.sh status

# 查看日志
./deploy.sh logs

# Docker 部署
./deploy.sh docker-build
./deploy.sh docker-start
./deploy.sh docker-stop

# 清理构建产物
./deploy.sh clean
```

---

## 配置

部署前建议根据实际情况修改 `config.yaml`:

```yaml
server:
  port: 8080

storage:
  max_gb: 50              # 根据磁盘空间调整
  clip_dir: ./clips
  event_dir: ./events
  screenshot_dir: ./screenshots

capture:
  device: 0              # 摄像头设备号
  width: 640
  height: 480
  fps: 25

encoding:
  record_bitrate: 4000000  # 4Mbps 录制
  monitor_bitrate: 500000   # 500kbps 监看
  segment_min: 5
  hw_encoder: auto         # 自动选择硬件编码

detection:
  enabled: true
  detect_fall: true
  detect_cry: true
  detect_noise: true
  detect_intruder: true
  sensitivity: 0.7
```

详细配置说明请参考 [CONFIG.md](CONFIG.md)。

---

## 验证部署

### 检查服务状态

```bash
curl http://localhost:8080/api/storage
```

返回示例:
```json
{
  "total_gb": 100.0,
  "used_gb": 12.5,
  "clips_count": 25,
  "events_count": 8
}
```

### 检查录像列表

```bash
curl http://localhost:8080/api/clips
```

### 检查事件列表

```bash
curl http://localhost:8080/api/events
```

---

## 故障排除

### 服务无法启动

1. 检查端口是否被占用: `lsof -i :8080`
2. 检查配置文件是否有效: `python -c "import yaml; yaml.safe_load(open('config.yaml'))"`
3. 查看日志获取详细错误信息

### Docker 部署问题

**问题**: `docker: Cannot connect to the Docker daemon`

**解决**: 启动 Docker 服务
```bash
sudo systemctl start docker
# 或
sudo service docker start
```

**问题**: `port is already allocated`

**解决**: 修改 `docker-compose.yml` 中的端口映射
```yaml
ports:
  - "8081:8080"  # 映射到 8081
```

### Linux 部署问题

**问题**: `ffmpeg: command not found`

**解决**: 安装 FFmpeg
```bash
sudo apt-get install ffmpeg
```

**问题**: 摄像头无法访问

**解决**: 检查设备权限
```bash
ls -la /dev/video0
sudo chmod 666 /dev/video0
```

### 性能问题

**问题**: CPU 占用过高

**解决**:
1. 降低分辨率: `width: 320, height: 240`
2. 降低帧率: `fps: 15`
3. 启用硬件编码: `hw_encoder: nvenc` 或 `qsv`

**问题**: 视频延迟高

**解决**:
1. 降低监看码率: `monitor_bitrate: 300000`
2. 检查网络带宽

---

## 安全建议

1. **网络访问控制**: 生产环境建议限制 `8080` 端口访问
2. **存储加密**: 敏感场景考虑对录像目录加密
3. **定期备份**: 定期备份 `clips`、`events` 目录
4. **日志审计**: 定期检查日志，发现异常访问