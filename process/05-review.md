# REVIEW - 审查报告

## 验证清单（逐项 ✅/❌/⚠️）

| 成功标准 | 状态 | 说明 |
|---------|------|------|
| Gin HTTP服务器 :8080，含所有API端点 | ✅ | `/api/clips`, `/api/clips/:name`, `/api/events`, `/api/ws/signal`, `/` 均已实现 |
| 双路编码器(录制4Mbps + 监看500kbps) | ⚠️ | 框架存在但 H.264 编码是 stub 实现，未调用 FFmpeg CGO |
| MP4分段写入(5min，命名 2026-03-25_14-30-00.mp4) | ⚠️ | 分段逻辑正确但 MP4 容器是 stub，生成文件不完整无法播放 |
| 存储GC磁盘容量检测+自动删除最旧文件 | ✅ | 逻辑正确，30s检查间隔，80%阈值 |
| Pion WebRTC zerolatency 推送 | ⚠️ | 信令通道和 WriteVideoFrame 已修复(本轮review修复)，但 H.264 数据是 stub |
| 事件检测引擎(异步500ms抽帧) | ⚠️ | IPC Unix Socket 通道存在，但检测结果需要 H.264 流才有效 |
| YOLOv8姿态检测(摔倒) | ⚠️ | 代码存在但使用简化算法(无真实模型)，fall detection 召回率无法达标 |
| 音频频谱分析(哭声/异响) | ❌ | audio.py 从视频帧数据提取伪特征而非真实音频流，核心功能缺失 |
| 陌生人闯入检测(YOLOv8人形+人脸) | ⚠️ | 人形检测存在，人脸检测未实现 |
| Vue3 实时监看(WebRTC + 事件气泡) | ✅ | 组件完整，消息格式已修复 |
| Vue3 历史回放(video.js + Range拖拽 + 事件时间轴) | ✅ | Range 解析和 video.js 集成正确 |
| Vue3 事件中心(列表/截图/快速跳转) | ✅ | 完整实现 |
| Vue3 设置页(存储/检测开关/灵敏度) | ✅ | 完整实现 |
| Gin API CORS/日志中间件 | ✅ | 已实现 |
| go.mod 依赖正确无冲突 | ✅ | 重复的 gocv-io/x/go-cv 已移除，gorilla/websocket 已添加 |
| 项目目录结构规范 | ✅ | cmd/internal/web/detection 分离，DDD 风格 |
| config.yaml 配置完整 | ✅ | 但 main.go 未读取，依赖硬编码值(次要) |
| build.sh 构建脚本 | ✅ | Go + PyInstaller 两路构建 |

---

## 发现的问题及修复情况

### ✅ 本轮已修复的问题

1. **`internal/webrtc/webrtc.go` - WriteVideoFrame 是空操作**
   - 问题：`WriteVideoFrame` 收到帧后直接 `return nil`，不写入 WebRTC track
   - 修复：添加 `w.videoTrack.Write(ep.Data)` 将 H.264 数据写入 Pion track；添加 `encoder` 和 `mp4` 包引用以支持两种 EncoderPacket 类型

2. **`internal/api/server.go` - handleWebRTCSignal 是空操作**
   - 问题：函数体只有 `c.Next()`，未进行 WebSocket 升级，无任何信令转发
   - 修复：使用 `gorilla/websocket.Upgrader` 实现完整 WebSocket 升级；双向 goroutine 分别转发 客户端→webrtc 和 webrtc→客户端

3. **`web/src/views/LiveView.vue` - WebSocket 消息格式与后端不匹配**
   - 问题：前端发送 `{ type: 'offer', payload: pc.localDescription.sdp }`（裸 SDP 字符串），后端 webrtc.go 期望 `SignalMessage{ Payload: json.RawMessage }` 并尝试 `json.Unmarshal`
   - 修复：前端改为发送 `{ type: 'offer', payload: { type: 'offer', sdp: '...' } }` 结构化对象

4. **`internal/webrtc/webrtc.go` - HandleSignal offer/answer 解析错误**
   - 问题：offer/answer 处理时 `SDP: string(msg.Payload)` 直接将 JSON 字节转为字符串，而非从 JSON 对象中提取 `sdp` 字段
   - 修复：先用 `json.Unmarshal` 解析 payload 再提取 SDP 字段；保留兜底逻辑兼容旧格式

5. **`go.mod` - 重复依赖**
   - 问题：同时存在 `gocv.io/x/gocv` (direct) 和 `gocv-io/x/go-cv` (indirect)，潜在符号冲突
   - 修复：移除 `gocv-io/x/go-cv` 条目；添加 `github.com/gorilla/websocket` 以支持 WebSocket

6. **`cmd/server/main.go` - encoder.EncoderPacket 与 mp4.EncoderPacket 类型不匹配**
   - 问题：`runEncodeLoop` 调用 `mp4.WritePacket(recPacket)` 时传入 `*encoder.EncoderPacket`，但 `WritePacket` 签名是 `func(*mp4.EncoderPacket)`，类型不匹配无法编译
   - 修复：在调用处显式构造 `*mp4.EncoderPacket{...}` 进行字段拷贝

---

### ⚠️ 已识别但未修复的问题（需要 FFmpeg CGO 集成）

7. **encoder/encoder.go - H.264 编码是 stub**
   - 问题：`softEncoder.encode()` 只是复制原始图像字节作为"NALU"，注释明确写"实际需要调用FFmpeg CGO进行H.264编码"
   - 影响：录制路和监看路输出的都不是有效 H.264 码流，WebRTC 无法传输，录像无法播放
   - 建议：集成 FFmpeg CGO，实现真正的 H.264 编码（qsv/nvenc/libx264 按序探测）

8. **mp4/mp4.go - MP4 容器是 stub**
   - 问题：`WritePacket` 只写 NAL 长度前缀+原始数据，没有 moov/mdat/udat box，无 SPS/PPS，无视频时间戳
   - 影响：生成的文件不是合法 MP4，无法被任何播放器识别
   - 建议：集成 FFmpeg CGO 的 avformat API 进行标准 MP4 封装

9. **config.yaml 未被读取**
   - 问题：main.go 中配置全部硬编码，`config.yaml` 存在但未被 `gopkg.in/yaml.v3` 加载
   - 影响：用户修改 config.yaml 不会生效
   - 建议：main.go 启动时解析 config.yaml 覆盖硬编码默认值

10. **audio.py - 伪音频检测**
    - 问题：`AudioDetector.detect()` 从视频帧的像素数据计算"伪频谱特征"，而非真实音频 PCM 数据。注释也写明"当前版本从视频帧中提取音频信息，实际项目中音频应从独立音频流获取"
    - 影响：哭声/异响检测依赖视频图像的统计特征，物理意义不正确，无法达到声学检测精度要求
    - 建议：需要系统音频采集（Windows: WASAPI / Linux: ALSA）+ 独立音频流

11. **YOLO 摔倒检测算法是简化版**
    - 问题：`_detect_pose` 中的摔倒判定使用简单角度/高度比 heuristic，无真实 YOLOv8-pose 模型权重验证
    - 影响：无法保证摔倒检测召回率 >80% 的目标
    - 建议：使用真实 YOLOv8-pose 模型并通过测试集评估

12. **人脸检测未实现**
    - 问题：`yolo.py` 只有 `person` 类检测（YOLOv8n），无 face detection 模型，陌生人闯入依赖人形检测
    - 影响：无法区分已知人和陌生人
    - 建议：添加 YOLOv8-face 模型或人脸识别库

13. **storage/gc.go - GC 计算存在精度风险**
    - 问题：`g.deleteOldest(totalSize - maxBytes*80/100)` 使用整数运算，虽然数学上等价于 `totalSize - int64(float64(maxBytes)*0.8)`，但可读性差
    - 影响：边界情况下（如 maxGB=50.3）可能与预期有偏差
    - 建议：显式使用 `float64(maxBytes)*0.8` 计算目标删除量

---

## 最终质量评估

### 架构质量: 7/10
项目结构清晰，模块分离良好（capture/encoder/mp4/storage/webrtc/ipc/api），符合 Go 项目最佳实践。事件驱动设计（Unix Socket IPC、channel 隔离）架构上合理。

### 功能完整性: 4/10
核心录制链路（采集→编码→封装→存储）和 WebRTC 监看链路依赖 FFmpeg CGO 的 H.264 编码与 MP4 封装，当前 stub 无法产生可用视频。Vue3 前端四页面功能相对完整。

### 代码质量: 6/10
代码组织良好，注释充分。Go 代码遵循规范；Python 检测代码有完整错误处理和 fallback；Vue3 组件结构清晰。但存在多处 stub 实现和类型不匹配问题。

### 关键阻断问题:
1. **H.264 编码缺失** — 录制和 WebRTC 都依赖它，属于 MVP 核心功能
2. **MP4 封装缺失** — 录像文件无法播放
3. **音频检测不可用** — 使用视频像素替代音频数据，物理意义错误

### 结论: 需要重跑 DO 阶段

上述 3 个阻断问题（encoder stub、mp4 stub、伪音频检测）属于核心功能缺失而非局部 bug，修复需要 FFmpeg CGO 集成、系统音频采集、真实 YOLOv8 模型加载等重大工程工作，工作量超过 30% 文件重写。建议返回 DO 阶段完成以下关键任务：

1. FFmpeg CGO 集成（gocv + FFmpeg 混合编译）
2. 真正的 H.264 编码器（QSV/NVENC/libx264 按序探测）
3. 标准 MP4 容器封装（avformat）
4. 系统音频采集路径（独立于视频的音频流）
5. YOLOv8 真实模型加载与测试

已修复的信令/WebSocket、类型不匹配等问题作为本次 review 的产出保留。
