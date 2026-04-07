# VisionLoop 智能监控

基于电脑摄像头的本地智能监控系统，支持人脸识别、运动检测、YOLO 物体检测，带 Web 实时界面和录像回放。纯 CPU 运行，无需 GPU。

![Python](https://img.shields.io/badge/Python-3.10+-blue) ![FastAPI](https://img.shields.io/badge/FastAPI-0.110+-green) ![OpenCV](https://img.shields.io/badge/OpenCV-headless-orange)

---

## 功能

- **实时监控** — WebSocket 推流，浏览器直接看，带检测框和中文标注
- **运动检测** — MOG2 背景差分，持续运动超过阈值触发入侵告警
- **人脸识别** — 注册已知人脸，陌生人自动告警并截图
- **物体检测** — YOLOv8n 检测人/车/猫/狗等目标
- **循环录像** — 按时间分段录制，事件触发自动锁定相关片段
- **时间轴回放** — Canvas 时间轴可视化，点击跳转，自动续播，支持倍速
- **事件历史** — SQLite 存储，按类型/时间过滤，分页查询，截图可直接注册人脸
- **热更新配置** — 设置页面保存后立即生效，无需重启

---

## 快速开始

**安装依赖**

```bash
pip install -r requirements.txt
```

> 首次运行会自动下载 `yolov8n.pt`（约 6MB）。

**启动**

```bash
python main.py
```

浏览器打开 `http://localhost:8888`

---

## 项目结构

```
├── main.py                  # 入口
├── config.py                # Pydantic 配置模型
├── config.yaml              # 用户配置（自动生成）
├── core/
│   ├── camera.py            # 摄像头采集线程
│   └── frame_processor.py   # 检测编排 + 帧标注
├── detectors/
│   ├── motion.py            # 运动检测
│   ├── face.py              # 人脸识别
│   └── object_detector.py   # YOLOv8 物体检测
├── storage/
│   ├── database.py          # SQLite 事件存储
│   ├── recorder.py          # 循环录像
│   └── snapshot.py          # 截图保存
├── alerts/
│   └── alert_manager.py     # 冷却过滤 + 通知分发
├── web/
│   ├── app.py               # FastAPI 工厂
│   ├── broadcaster.py       # WebSocket 广播
│   └── routes/              # stream / events / faces / playback / config
├── known_faces/             # 已注册人脸图片（jpg/png）
└── snapshots/               # 事件截图
```

---

## 配置说明

首次启动后编辑 `config.yaml`，或直接在 Web 设置页修改（实时生效）。

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `camera.source` | `0` | 摄像头索引或 RTSP URL |
| `camera.fps` | `30` | 采集帧率 |
| `detectors.motion.enabled` | `true` | 运动检测开关 |
| `detectors.motion.min_area` | `1500` | 最小运动像素面积 |
| `detectors.motion.intrusion_seconds` | `2.0` | 持续运动多少秒触发入侵告警 |
| `detectors.face.enabled` | `true` | 人脸识别开关 |
| `detectors.face.tolerance` | `0.55` | 匹配阈值，越小越严格 |
| `detectors.object.enabled` | `true` | YOLO 检测开关 |
| `detectors.object.confidence` | `0.45` | 置信度阈值 |
| `recording.enabled` | `true` | 录像开关 |
| `recording.segment_minutes` | `1` | 每段录像时长（分钟） |
| `recording.max_segments` | `20` | 最多保留普通录像段数 |
| `alerts.cooldown_seconds` | `30` | 同类事件最小间隔（秒） |
| `web.port` | `8888` | Web 服务端口 |
| `web.stream_quality` | `80` | 推流 JPEG 质量（1-100） |

---

## 人脸注册

**方式一：上传图片**
在「人脸管理」页面填写姓名并上传正面照片（jpg/png）。

**方式二：从事件截图注册**
在「事件历史」中找到陌生人事件，点击「注册人脸」按钮直接注册。

**方式三：手动放置**
将图片命名为 `姓名.jpg` 放入 `known_faces/` 目录，重启或重新注册触发重载。

---

## 检测流程

```
摄像头帧
  └─ 运动检测（始终运行）
       └─ 有运动？
            ├─ 人脸识别（半分辨率）
            └─ YOLO 物体检测（缩放到 640px）
                  │
                  ├─ 标注帧 → JPEG → WebSocket 广播
                  └─ 检测事件 → 冷却过滤 → 截图 + 写库 + 桌面通知
```

运动门控设计：无运动时跳过人脸和 YOLO，降低 CPU 占用。

---

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| `WS` | `/ws/stream` | 实时视频流（JPEG 帧 + JSON 事件） |
| `GET` | `/api/status` | FPS、已注册人脸数 |
| `GET` | `/api/events` | 事件历史，支持 `type/since/until/limit/offset` |
| `GET` | `/api/faces` | 已注册人脸列表 |
| `POST` | `/api/faces` | 注册人脸（multipart） |
| `DELETE` | `/api/faces/{name}` | 删除人脸 |
| `POST` | `/api/faces/from-snapshot` | 从截图注册人脸 |
| `GET` | `/api/recordings` | 录像列表 |
| `GET` | `/api/recordings/timeline` | 带时长的时间轴数据 |
| `GET` | `/api/recordings/{filename}` | 播放录像（支持 Range） |
| `GET` | `/api/config` | 读取配置 |
| `POST` | `/api/config` | 保存配置（深度合并，立即生效） |

---

## 依赖

```
fastapi / uvicorn       Web 框架
opencv-python-headless  视频采集与处理
face-recognition        人脸检测与识别（基于 dlib）
ultralytics             YOLOv8 物体检测
aiosqlite               异步 SQLite
pydantic / pydantic-settings  配置模型
plyer                   桌面通知
PyYAML                  配置文件解析
Pillow                  中文文字渲染
```

> `face-recognition` 依赖 `dlib`，首次安装需要编译，建议提前安装 cmake：`brew install cmake`
