# DISCOVER - 发现报告

## 现状 (What exists)

### 项目路径
- **目标项目**: `git@github.com:axfinn/VisionLoop.git`
- **本地状态**: 项目尚未克隆到本地，代码库不存在
- **输出目录**: `/app/data/autodev/VisionLoop-----100--2026-03-2-1774439980/process/`

### 技术栈（从任务描述提取）
| 层级 | 技术 | 说明 |
|------|------|------|
| 视频采集 | gocv | OpenCV Go绑定，无缓冲channel，下游满时丢帧 |
| 硬件编码 | FFmpeg CGO | 探测顺序: h264_qsv → h264_nvenc → libx264 |
| 流媒体 | Pion WebRTC | zerolatency模式，禁用B帧，延迟<50ms |
| Web框架 | Gin | 提供 /api/clips, /api/events, /api/ws/signal |
| 事件检测 | YOLOv8+ONNX | 姿态检测、哭声检测、异响检测、人脸识别 |
| 前端 | Vue3 + video.js | 实时监看、历史回放、事件中心 |
| 分段格式 | MP4 | 5分钟分段，命名: 2026-03-25_14-30-00.mp4 |

### 架构要点
```
摄像头 → gocv采集 → 原始帧
  ├─ 录制编码器(4Mbps) → MP4分段(5min) → Gin :8080
  └─ 监看编码器(500kbps) → Pion WebRTC → 浏览器
                              └─ 事件检测引擎(异步抽帧)
```

## 外部参考 (What others have done)

### 相似项目调研

1. **Frigate NVR** (https://github.com/blakeblackshear/frigate)
   - Go+Python混合架构
   - TensorRT加速YOLOv8
   - WebRTC实时流
   - 设计参考: 事件检测与录像分离架构

2. **go2rtc** (https://github.com/AlexxIT/go2rtc)
   - Go原生WebRTC服务器
   - 支持多协议统一流媒体
   - 设计参考: Pion集成模式

3. **ZoneMinder** (https://github.com/ZoneMinder/ZoneMinder)
   - 传统Linux NVR方案
   - FFmpeg采集 + MySQL事件存储
   - 参考: 分段录制策略

### 关键开源组件

| 组件 | 仓库 | 用途 |
|------|------|------|
| gocv | github.com/hybridgroup/gocv | OpenCV Go绑定 |
| Pion WebRTC | github.com/pion/webrtc | WebRTC实现 |
| Gin | github.com/gin-gonic/gin | HTTP框架 |
| onnxruntime-go | github.com/ying32/onnxruntime-gateway | ONNX推理 |
| go-snap7 | github.com/simatheone/go-snap7 | 工业通信(备用) |

## 关键发现 (Key insights)

### 1. 双路编码架构
任务描述的核心是**同一帧发两路编码器互不阻塞**：
- 录制路: 4Mbps高画质，MP4分段存储
- 监看路: 500kbps低码流，WebRTC实时推送
- 实现关键: goroutine + channel隔离，帧复制

### 2. 硬件编码优先级
探测顺序 `h264_qsv → h264_nvenc → libx264` 表明：
- Intel QSV (Quick Sync Video) 优先
- NVIDIA NVENC 次之
- CPU软编码兜底
- CGO调用FFmpeg原生API实现

### 3. 事件检测异步化
检测引擎从监看流抽帧(500ms间隔)，不阻塞编码链路：
- YOLOv8姿态检测 → 摔倒识别
- 音频频谱分析 → 哭声/异响
- YOLOv8人形+人脸 → 陌生人闯入
- 检测结果含时间戳+截图，存入事件库

### 4. 存储GC策略
文件命名规范化 `2026-03-25_14-30-00.mp4`，每次写包检查磁盘， 超限自动删最旧文件。

### 5. 前端架构
Vue3 SPA + video.js Range拖拽:
- 实时监看: WebRTC + 事件气泡
- 历史回放: video.js + HTTP Range请求 + 事件标注同步
- 事件中心: 列表/截图/快速定位

## 未知项 (What we don't know yet)

### 技术风险
1. **gocv与FFmpeg CGO集成** - 两者同时使用CGO可能存在符号冲突
2. **Pion WebRTC zerolatency配置** - 具体API参数需验证
3. **YOLOv8 ONNX模型** - 需确定模型文件来源(自有训练/公开模型)
4. **音频检测方案** - 具体库未指定(频谱分析用什么库?)
5. **Windows打包** - CGO交叉编译+FFmpeg DLL嵌入的复杂度

### 架构疑问
1. **事件库存储** - SQLite? PostgreSQL? 时间序列数据库?
2. **帧复制策略** - 无缓冲channel如何实现帧复制给双编码器?
3. **WebRTC信令** - /api/ws/signal 的完整协议定义
4. **存储容量计算** - 5分钟分段的大小估算和删除阈值

### 待验证假设
1. gocv的VideoCapture可同时被两路读取(可能需要clone或重连)
2. FFmpeg的h264_qsv/h264_nvenc在Windows下可用
3. onnxruntime-go支持YOLOv8所有算子
4. video.js Range请求与Gin ETag缓存兼容

---

*报告生成时间: 2026-03-25*
*阶段: DISCOVER 发现