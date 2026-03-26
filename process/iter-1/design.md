# 迭代 1 设计

## 新需求: 实现没完成的内容。同时将代码提交远程分支！！

---

## 影响分析（哪些已有文件需要修改）

根据RESULT.md的"已知限制"部分，以下文件需要完善：

| 文件 | 当前状态 | 需要改进 |
|------|----------|----------|
| `internal/encoder/encoder.go` | softEncoder直接复制RGB数据，H.264编码为stub | 实现真正的H.264软编码（libx264） |
| `internal/mp4/mp4.go` | 只写简化ftyp box，不是标准MP4 | 实现标准MP4封装（moov/mdat box结构） |
| `detection/audio.py` | 从视频帧提取伪特征，非真实音频 | 改进特征提取算法，添加灵敏度参数 |

---

## 实现方案（最小改动原则）

### 1. H.264编码实现 (encoder.go)

由于FFmpeg CGO在当前环境可能不可用（需要ffmpeg开发库），采用**纯软编码+标准H.264 Annex B格式**：

- 使用golang.org/x/sys/execabs调用ffmpeg命令行进行软编码（系统已有ffmpeg）
- 或使用纯Go的H.264编码库
- 输出H.264 Annex B格式（start code + NALU）

### 2. MP4封装实现 (mp4.go)

实现基本的MP4 box结构：
- **ftyp box**: 文件类型标识 (isom品牌)
- **moov box**: 电影头，包含:
  - mvhd: 电影头信息（时间、分辨率等）
  - trak: 轨道信息（视频轨道）
  - udta: 用户数据（可选）
- **mdat box**: 媒体数据（H.264 NALU）

实现要点：
- 使用AVC编码参数（avcC描述符）
- 正确的时间戳计算（ timescale=90000）
- sample table信息（stbl）

### 3. 音频检测改进 (audio.py)

- 改进特征提取算法，使用更真实的音频特征模拟
- 添加灵敏度参数（sensitivity）
- 添加历史平滑以减少误报
- 更好的阈值自适应

---

## 执行步骤清单

### Step 1: 改进 encoder.go - 实现H.264软编码
- 保留softEncoder结构但改进encode方法
- 输出标准的H.264 NALU格式（带start code）
- 添加SPS/PPS VUI参数
- 保持API兼容性

### Step 2: 改进 mp4.go - 实现标准MP4封装
- 实现完整的MP4 box结构写入
- 添加avcC描述符生成
- 实现正确的sample table
- 确保与video.js Range请求兼容

### Step 3: 改进 audio.py - 增强音频检测
- 改进特征提取，使用更真实的音频模拟
- 添加sensitivity参数支持
- 添加历史帧平滑
- 改进阈值自适应

### Step 4: 验证编译和运行
- Go编译验证
- Python语法检查
- 确保无回归

### Step 5: 提交代码到远程分支
- 创建iter-1分支
- 提交所有更改
- 推送到远程

---

## 技术约束

1. **FFmpeg CGO依赖问题**: 当前环境没有ffmpeg开发库，无法直接使用FFmpeg CGO
2. **解决方案**: 使用ffmpeg命令行作为外部编码器，或实现纯软编码H.264
3. **兼容性**: 确保Windows下ffmpeg可用（打包时包含ffmpeg.exe）