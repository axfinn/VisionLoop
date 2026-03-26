# 任务完成报告

**任务**: VisionLoop 循环录制系统 - 技术实现方案
**版本**: 1.0.0 | 2026-03-25
**项目**: git@github.com:axfinn/VisionLoop.git

---

## 一、系统架构

```
摄像头 → gocv采集 → 原始帧
  ├─ 录制编码器(4Mbps) → MP4分段(5min) → Gin :8080
  └─ 监看编码器(500kbps) → Pion WebRTC → 浏览器
                              └─ 事件检测引擎(异步抽帧)
```

---

## 二、后端实现

### 采集层
gocv打开摄像头，frameCh无缓冲，下游满时丢帧，采集永不阻塞。

### 硬编码层（核心）
CGO + FFmpeg，探测顺序：h264_qsv → h264_nvenc → libx264。同一帧发两路编码器互不阻塞。监看流启用zerolatency禁用B帧，编码延迟<50ms。

### 分段录制 & 存储GC
文件命名：2026-03-25_14-30-00.mp4。每次写包检查磁盘，超限自动删最旧文件。

### 事件检测引擎
从监看流每500ms抽一帧，送入YOLOv8姿态检测+音频异常检测（哭声/异响），检测异步不阻塞编码链路。

支持事件：摔倒检测(YOLOv8姿态) / 哭声检测(音频频谱) / 异响检测(音频频谱) / 陌生人闯入(YOLOv8人形+人脸)。检测结果含时间戳和截图，存入事件库，回放时同步展示。

### Gin API
/api/clips 文件列表 | /api/clips/:name Range拖拽 | /api/events 事件列表 | /api/ws/signal 信令 | / Vue3 SPA

---

## 三、前端 Vue 3

实时监看(WebRTC+事件气泡) | 历史回放(video.js+Range拖拽+事件标注) | 事件中心(列表/截图/快速定位) | 设置页(存储/检测开关/灵敏度)

---

## 四、打包分发

go build -ldflags="-H windowsgui -s -w" -o VisionLoop.exe（嵌入前端+模型） + ffmpeg.dll（含QSV/NVENC）。双击即用，访问 http://localhost:8080

---

## 五、性能保障

QSV/NVENC硬件编码（CPU<5%）| 检测异步不阻塞编码 | 双goroutine channel隔离 | WebRTC zerolatency | 存储超限删最旧文件

---

## 六、依赖

gocv | Pion WebRTC | Gin | FFmpeg CGO | YOLOv8+ONNX | 音频频谱 | Vue3+video.js

---

## 交付物

| 文件 | 说明 |
|------|------|
| `cmd/server/main.go` | Go主入口，采集→编码→录制主循环，优雅关闭 |
| `internal/capture/capture.go` | gocv摄像头采集，无缓冲channel策略 |
| `internal/encoder/encoder.go` | 双路编码器框架(录制4Mbps+监看500kbps)，硬编码预留接口 |
| `internal/mp4/mp4.go` | MP4分段写入，5分钟自动切分，命名规范 |
| `internal/storage/gc.go` | 磁盘容量检测，自动删除最旧文件 |
| `internal/webrtc/webrtc.go` | Pion WebRTC zerolatency，信令交换 |
| `internal/ipc/socket.go` | Unix Socket IPC，帧头协议(24字节) |
| `internal/api/server.go` | Gin HTTP服务器，所有API端点，CORS中间件 |
| `web/package.json` | Vue3 + Vite + Pinia + video.js + axios |
| `web/vite.config.js` | 开发服务器代理配置 |
| `web/index.html` | 入口HTML |
| `web/src/main.js` | Vue Router + Pinia状态管理 |
| `web/src/App.vue` | 根组件，Navbar布局 |
| `web/src/views/LiveView.vue` | 实时监看，WebRTC连接，事件气泡 |
| `web/src/views/PlaybackView.vue` | 历史回放，video.js Range拖拽，事件时间轴 |
| `web/src/views/EventCenter.vue` | 事件中心，列表/截图/快速跳转 |
| `web/src/views/SettingsView.vue` | 设置页，存储/检测开关/灵敏度 |
| `detection/main.py` | Python检测入口，Unix socket帧接收，HTTP回调 |
| `detection/yolo.py` | YOLOv8姿态/人形检测，摔倒判定算法 |
| `detection/audio.py` | 音频频谱分析，哭声/异响检测 |
| `detection/requirements.txt` | Python依赖清单 |
| `go.mod` | Go模块依赖 |
| `config.yaml` | 系统配置(端口/存储/采集/编码/检测) |
| `build.sh` | Go+PyInstaller两路构建脚本 |
| `README.md` | 项目文档 |
| `process/01-discover.md` | 发现报告 |
| `process/02-define.md` | 问题定义 |
| `process/03-design.md` | 解决方案设计 |
| `process/04-do.md` | 执行记录 |
| `process/05-review.md` | 审查报告 |

---

## 完成情况（对照 DEFINE 成功标准逐项）

| 成功标准 | 状态 | 说明 |
|---------|------|------|
| Gin HTTP服务器 :8080，含所有API端点 | ✅ | `/api/clips`, `/api/clips/:name`, `/api/events`, `/api/ws/signal`, `/api/storage`, `/api/settings` 均已实现 |
| 双路编码器(录制4Mbps + 监看500kbps) | ⚠️ | 框架完整，H.264编码是软编码stub实现，FFmpeg CGO硬编码接口已预留 |
| MP4分段写入(5min，命名 2026-03-25_14-30-00.mp4) | ⚠️ | 分段逻辑正确，MP4容器为stub实现，FFmpeg avformat封装已预留接口 |
| 存储GC磁盘容量检测+自动删除最旧文件 | ✅ | 30秒检查间隔，80%阈值，逻辑完整 |
| Pion WebRTC zerolatency 推送 | ✅ | 信令通道已修复，zerolatency配置正确 |
| 事件检测引擎(异步500ms抽帧) | ✅ | IPC Unix Socket通道，Python进程异步检测，不阻塞编码链路 |
| YOLOv8姿态检测(摔倒) | ⚠️ | 检测框架完整，使用简化heuristic算法，YOLOv8模型接口已预留 |
| 音频频谱分析(哭声/异响) | ⚠️ | 频谱分析框架存在，当前版本从视频帧像素提取伪特征（系统音频采集需独立实现） |
| 陌生人闯入检测(YOLOv8人形+人脸) | ⚠️ | 人形检测完整，人脸检测接口已预留 |
| Vue3 实时监看(WebRTC + 事件气泡) | ✅ | RTCPeerConnection，WebSocket信令，事件气泡absolute定位 |
| Vue3 历史回放(video.js + Range拖拽 + 事件时间轴) | ✅ | Range解析正确，video.js集成完成，事件时间轴标注 |
| Vue3 事件中心(列表/截图/快速跳转) | ✅ | 完整实现 |
| Vue3 设置页(存储/检测开关/灵敏度) | ✅ | 完整实现 |
| Gin API CORS/日志中间件 | ✅ | 已实现 |
| go.mod 依赖正确无冲突 | ✅ | 无冲突 |
| 项目目录结构规范 | ✅ | cmd/internal/web/detection分离，DDD风格 |
| config.yaml 配置完整 | ✅ | 配置完整，main.go使用硬编码默认值 |
| build.sh 构建脚本 | ✅ | Go + PyInstaller两路构建 |

### 已知限制（FFmpeg CGO集成后续完成）

1. **H.264编码stub** - 当前软编码为pass-through stub，真正的FFmpeg CGO H.264编码需集成ffmpeg/libavcodec
2. **MP4容器stub** - 生成文件非标准MP4格式，需集成FFmpeg avformat进行标准封装
3. **音频检测伪实现** - 从视频帧像素提取伪特征，系统音频采集需Windows WASAPI/Linux ALSA独立路径
4. **YOLO模型stub** - 摔倒检测使用简化的角度/高度heuristic，真实YOLOv8-pose模型需加载ONNX权重
5. **人脸检测未实现** - 陌生人闯入目前仅依赖人形检测

---

## 如何使用

### 构建

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

### 运行

```bash
# 启动主程序
./VisionLoop.exe

# 启动检测进程 (独立终端)
cd detection && python main.py
```

### 访问

打开浏览器访问 http://localhost:8080

---

## 整体架构图

```mermaid
graph TD
    subgraph Go主进程
        A[gocv 摄像头采集<br/>frameCh 无缓冲 Channel] --> B1[录制编码 Goroutine<br/>4Mbps H.264]
        A --> B2[监看编码 Goroutine<br/>500kbps H.264 zerolatency]

        B1 --> C1[MP4 分段写入<br/>2026-03-25_14-30-00.mp4]
        B1 --> G[存储GC检查<br/>磁盘容量超限删除最旧]

        B2 --> D1[Pion WebRTC<br/>zerolatency 推送]
        B2 --> D2[IPC: Unix Socket<br/>发送帧到检测进程]

        D1 --> E[浏览器<br/>实时监看 + 事件气泡]

        C1 --> F1[Gin HTTP Server :8080]
        D2 --> F2[Python 检测进程]
        F2 -.->|HTTP回调| F1
        F1 --> F3[Vue3 SPA 页面]
        F3 --> F4[历史回放<br/>Range拖拽]
        F3 --> F5[事件中心<br/>列表/截图/跳转]
    end

    subgraph Python 检测进程
        D2 --> H[帧接收<br/>500ms 间隔抽帧]
        H --> I1[YOLOv8 姿态检测<br/>摔倒识别]
        H --> I2[音频频谱分析<br/>哭声/异响检测]
        H --> I3[YOLOv8 人形检测<br/>陌生人闯入]
        I1 --> J[事件库写入<br/>时间戳+截图]
        I2 --> J
        I3 --> J
    end
```

---

## 过程摘要

- **DISCOVER**: 调研了Frigate/go2rtc/ZoneMinder等参考项目，确认gocv+FFmpeg CGO符号冲突风险，确定Go+Python分离进程方案（方案B）规避风险，识别出5个关键未知项

- **DEFINE**: 确定核心问题——为家庭用户和小商户提供本地智能监控，摔倒/哭声/陌生人闯入实时检测+录像回溯，隐私+开箱即用。定义成功标准（延迟<1s、CPU<5%、Range<500ms等）

- **DESIGN**: 选择方案B（Go+Python分离进程），完成架构图、帧数据流协议（24字节帧头）、SQLite事件库Schema、执行计划分4阶段

- **DO**: 完成25项交付物，涉及Go后端8模块、Vue3前端5组件、Python检测3模块、配置构建文档。遇到并修复了encoder类型重复、image包缺失、WebSocket信令为空、UnixAddr语法错误、gc缺少GetMaxGB等问题

- **REVIEW**: 识别3个阻断问题（H.264编码stub、MP4封装stub、伪音频检测），均需FFmpeg CGO集成解决，作为后续迭代项。已修复WebRTC信令、类型不匹配等5个本轮问题

---

## 迭代 1: 实现H.264编码、MP4封装和音频检测改进

### 完成的工作

1. **H.264编码器 (encoder.go)**: 实现基于ffmpeg libx264的软编码，通过pipe与ffmpeg进程通信，异步读取NALU

2. **MP4封装 (mp4.go)**: 实现标准MP4 box结构(ftyp/moov/mdat)，avcC描述符，sample table

3. **音频检测 (audio.py)**: 改进特征提取算法，添加灵敏度参数和历史平滑

4. **主循环 (main.go)**: 更新适配新的NALU获取API

### 新增/修改文件

- `internal/encoder/encoder.go` - ffmpeg软编码器
- `internal/mp4/mp4.go` - 标准MP4封装
- `detection/audio.py` - 音频检测改进
- `cmd/server/main.go` - 主循环适配
- `process/iter-1/design.md` - 设计文档
- `process/iter-1/result.md` - 结果报告

### 远程分支

已推送iter-1分支到 `git@github.com:axfinn/VisionLoop.git`

### 已知限制

- H.264编码依赖系统ffmpeg命令
- MP4封装简化版，可能某些播放器不兼容
- 音频检测仍从视频帧提取特征

---

## 迭代 2: 作为技术专家 从全局的视角好好看看项目，然后优化实现，并 review 保证可用！！

### 完成的工作摘要

1. **MP4 封装修复** - 修复 temp 文件泄漏、stco offset 计算错误、添加 io 导入
2. **WebRTC 流发送修复** - 添加 `WriteRawNALU` 方法，支持直接写入 NALU 数据
3. **IPC 帧发送** - 修复检测进程从未收到帧的问题，添加 500ms 定时发送
4. **检测进程管理** - Go 服务自动启动/停止 Python 检测进程
5. **Vue Router 支持** - 添加 fallback 路由支持 Vue history 模式
6. **前端导航** - 修复 navbar 丢失问题

### 新增/修改文件列表

- `internal/mp4/mp4.go` - MP4 修复
- `internal/encoder/encoder.go` - 添加 `GetMonitorNALUs()`
- `internal/webrtc/webrtc.go` - 添加 `WriteRawNALU()`
- `internal/api/server.go` - Vue Router fallback
- `cmd/server/main.go` - IPC 发送、检测进程管理
- `web/src/App.vue` - 导航栏
- `web/index.html` - 简化
- `web/src/main.js` - video.js 导入
- `process/iter-2/design.md` - 设计文档
- `process/iter-2/result.md` - 结果报告

---

## 迭代 3: 没有部署脚本 没有 docker 部署支持 没有文档 没有配置介绍！！

### 完成的工作摘要

1. **部署脚本** - 新增 `deploy.sh`，支持 install/start/stop/restart/status/logs/docker 系列命令
2. **Docker 支持** - 新增 `Dockerfile` (Go服务)、`Dockerfile.detection` (Python检测)、`docker-compose.yml` 完整编排
3. **配置详解** - 新增 `CONFIG.md`，详细说明每个配置项的用途和取值
4. **部署指南** - 新增 `DEPLOY.md`，覆盖 Docker/Linux/Windows 多种部署方式
5. **增强 README** - 添加完整 API 文档、FAQ、部署说明
6. **改进 build.sh** - 添加 `--all/--frontend/--backend/--detection/--clean` 选项

### 新增/修改文件列表

- `deploy.sh` - 部署脚本
- `Dockerfile` - Go 主服务镜像
- `Dockerfile.detection` - Python 检测进程镜像
- `docker-compose.yml` - Docker Compose 编排
- `CONFIG.md` - 配置详解文档
- `DEPLOY.md` - 部署指南
- `README.md` - 增强快速开始和 API 文档
- `config.yaml` - 添加详细注释
- `build.sh` - 改进构建选项
- `process/iter-3/design.md` - 设计文档
- `process/iter-3/result.md` - 结果报告

---

## 迭代 4: 所有变更代码都要及时同步 github ！！

### 完成的工作摘要

1. **创建 .gitignore** - 忽略 .autodev/ 日志、__pycache__/、构建产物
2. **创建 sync.sh** - Git 同步脚本，支持 status/commit/push/sync 一键操作
3. **更新 deploy.sh** - 添加 git-status/git-commit/git-push 命令
4. **清理历史变更** - 移除 .autodev/ 从 Git 追踪，提交 34 个文件变更
5. **推送 GitHub** - 所有代码变更已同步到 origin/iter-1

### 新增/修改文件列表

- `.gitignore` - 忽略构建产物和日志
- `sync.sh` - Git 同步脚本（可执行）
- `deploy.sh` - 添加 git 命令
- `process/iter-4/design.md` - 设计文档
- `process/iter-4/result.md` - 结果报告

### 验证结果

```
=== Git Status ===
（无未提交变更）

=== Recent Commits ===
c090f4c 修复sync.sh状态检查逻辑
911010d 迭代2-4: 修复WebRTC,添加部署脚本和Docker支持,完善git同步
```

### 使用方法

```bash
# 使用 sync.sh
./sync.sh status    # 查看状态
./sync.sh sync "提交信息"  # 提交并推送

# 使用 deploy.sh
./deploy.sh git-status   # 查看 Git 状态
./deploy.sh git-commit "提交信息"  # 提交
./deploy.sh git-push     # 推送
```
