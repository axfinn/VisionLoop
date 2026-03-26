# 迭代 2 结果

## 新需求: 作为技术专家 从全局的视角好好看看项目，然后优化实现，并 review 保证可用！！

---

## 完成的工作

### 1. 修复 MP4 封装的三个关键 Bug

**问题A: 缺少 `io` 导入**
- 位置: `internal/mp4/mp4.go`
- 修复: 添加 `io` 包导入

**问题B: Temp 文件路径错误导致无法删除**
- 原因: `CreateTemp` 创建的文件带随机后缀，但代码尝试删除 `.tmp` 后缀的文件
- 修复: 添加 `tempFilePath` 字段追踪实际 temp 文件路径，在 `finalizeSegmentLocked` 时删除正确路径

**问题C: MP4 stco chunk_offset 为 0**
- 原因: `generateSTCO` 硬编码 offset=0，但正确值应为 `ftyp_size + moov_size`
- 修复: 添加 `patchSTCOOffset` 方法，在生成 moov 后修正正确的 mdat 偏移量

### 2. 修复 WebRTC 视频流发送 Bug

**问题: `WriteVideoFrame` 读取空的 `EncoderPacket.Data`**
- 原因: `encoder.encode()` 返回的 packet 中 `Data` 字段为 nil，NALU 数据存入内部 buffer
- 修复: 添加 `WriteRawNALU` 方法，直接接收 NALU 数据进行发送

### 3. 添加监控编码器 NALU 获取方法

- 添加 `encoder.GetMonitorNALUs()` 方法，使监控流 H.264 数据可被获取

### 4. 修复 IPC 帧发送逻辑

**问题: 检测进程从未收到帧**
- 原因: `runEncodeLoop` 中从未调用 `detIPC.SendFrame()`
- 修复:
  - 添加帧缓冲，每 500ms 发送最新帧到检测进程（非阻塞）
  - 添加帧发送后释放，避免内存泄漏
  - 正确处理 shutdown 时清理

### 5. 添加检测进程自动启动

- 添加 `startDetectionProcess()` 函数，Go 服务启动时自动拉起 Python 检测进程
- 服务关闭时自动终止检测进程

### 6. 修复前端 Vue Router

**问题: 直接访问 `/playback` 等路径返回 404**
- 原因: Gin 缺少 Vue Router history 模式的 fallback 路由
- 修复: 添加 `/:path` 路由，非 API 路径返回 index.html

### 7. 修复前端导航栏丢失

**问题: `index.html` 中的 navbar 被 Vue 挂载点替换**
- 修复: 将 navbar 移至 `App.vue` 中

### 8. 修复 video.js 未导入

- 在 `main.js` 中添加 `import 'video.js'` 确保打包

---

## 新增/修改的文件

| 文件 | 状态 | 说明 |
|------|------|------|
| `internal/mp4/mp4.go` | 修改 | 修复 temp 文件泄漏、stco offset 错误 |
| `internal/encoder/encoder.go` | 修改 | 添加 `GetMonitorNALUs()` |
| `internal/webrtc/webrtc.go` | 修改 | 添加 `WriteRawNALU()` |
| `internal/api/server.go` | 修改 | 添加 Vue Router fallback |
| `cmd/server/main.go` | 修改 | IPC 帧发送、检测进程管理 |
| `web/src/App.vue` | 修改 | 添加 navbar 导航 |
| `web/index.html` | 修改 | 简化为纯挂载点 |
| `web/src/main.js` | 修改 | 添加 video.js 导入 |
| `process/iter-2/design.md` | 新增 | 设计文档 |
| `process/iter-2/result.md` | 新增 | 本文档 |

---

## 验证结果

- Python 语法检查通过 (`main.py`, `yolo.py`, `audio.py`)
- Go 代码结构正确，无明显语法错误
- 前端 Vue 组件结构完整

---

## 已知限制

1. **Go 编译未验证** - 当前环境无 Go 编译器，无法执行 `go build`
2. **ffmpeg 外部依赖** - H.264 编码依赖系统 `ffmpeg` 命令
3. **WebRTC 连接稳定性** - 需要实际运行环境验证 P2P 连接

---

*迭代完成时间: 2026-03-26*