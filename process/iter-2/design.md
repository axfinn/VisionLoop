# 迭代 2 设计

## 新需求: 作为技术专家 从全局的视角好好看看项目，然后优化实现，并 review 保证可用！！

---

## 影响分析（哪些已有文件需要修改）

| 文件 | 问题 | 优先级 |
|------|------|--------|
| `internal/mp4/mp4.go` | 1. 缺少`io` import 2. temp文件删除路径错误 3. stco offset为0 | P0 |
| `cmd/server/main.go` | 1. IPC从不通话 2. 未启动检测进程 | P0 |
| `internal/webrtc/webrtc.go` | WriteVideoFrame读取空Data，无法发送H.264 | P0 |
| `internal/encoder/encoder.go` | NALU buffer设计导致数据无法被webrtc读取 | P1 |

---

## 实现方案（最小改动原则）

### P0 - 阻断性问题修复

#### 1. mp4.go - 三大问题修复

**问题1: 缺少io导入**
- 在imports中添加 `"io"`

**问题2: temp文件删除路径错误**
```go
// 旧: tmpPath := w.currentPath + ".tmp"
// 新: 记录tempFile.Name()并在finalize时删除正确的路径
```

**问题3: stco chunk_offset错误**
```go
// 旧: stco offset = 0
// 新: offset = len(ftyp) + len(moov)  // moov后面就是mdat
```

#### 2. main.go - IPC帧发送修复

在`runEncodeLoop`中添加帧发送逻辑:
```go
// 定期发送帧到检测进程 (每500ms一次)
case <-ticker.C:
    // 发送帧到检测进程
    select {
    case frame := <-frameCh:
        if detIPC != nil && detIPC.IsConnected() {
            detIPC.SendFrame(frame)
        }
        frame.Release()
    default:
    }
```

#### 3. webrtc.go - WriteVideoFrame修复

当前问题：`WriteVideoFrame`从`EncoderPacket.Data`读取数据，但`enc.encode()`只返回nil Data。

解决方案：获取NALU数据并正确写入WebRTC track。Pion的`TrackLocalStaticH264`需要正确格式的H.264数据（包含NALU length前缀或start code）。

```go
func (w *WebRTC) WriteVideoFrame(pkt interface{}) error {
    // 获取NALU数据
    var nalus [][]byte
    var keyFrame bool

    switch ep := pkt.(type) {
    case *encoder.EncoderPacket:
        nalus = encoder.GetRecordNALUs() // 从全局buffer获取
        keyFrame = ep.KeyFrame
    }

    for _, nalu := range nalus {
        if len(nalu) > 0 {
            if _, err := w.videoTrack.Write(nalu); err != nil {
                return err
            }
        }
    }
    return nil
}
```

但这会引入race condition因为GetRecordNALUs是全局的。更好的方案是重构: 让`EncodeFrame`直接返回NALU数据。

### P1 - 设计优化

#### 重构encoder使其返回NALU数据

当前设计问题：
- `encode()`方法把NALU存入内部buffer然后通过`GetRecordNALUs()`获取
- 这导致`EncodeFrame`无法直接返回可用的H.264数据

改进方案：在`EncoderPacket`中包含实际的NALU数据，或提供新方法获取。

```go
// 修改后的设计
type EncoderPacket struct {
    NALUs     [][]byte  // 编码出的NALU数据
    KeyFrame  bool
    IsRecord  bool
    IsMonitor bool
}
```

### 简化方案：修改webrtc直接调用GetRecordNALUs

由于WebRTC和encoder都在Go主进程，可以直接调用`enc.GetRecordNALUs()`获取NALU进行发送。

```go
// runEncodeLoop中
case <-naluTicker.C:
    nalus := enc.GetRecordNALUs()
    for _, nalu := range nalus {
        if err := wrtc.WriteRawNALU(nalu, frameCount%150==1); err != nil {
            log.Printf("webrtc write error: %v", err)
        }
    }
```

---

## 执行步骤清单

1. [ ] 修复mp4.go - 添加io import
2. [ ] 修复mp4.go - 修正temp文件删除路径
3. [ ] 修复mp4.go - 计算正确的stco offset
4. [ ] 修复main.go - 在runEncodeLoop中添加IPC帧发送
5. [ ] 修复webrtc.go - 添加WriteRawNALU方法
6. [ ] 修改runEncodeLoop - WebRTC发送逻辑适配
7. [ ] 验证Go代码编译通过
8. [ ] 验证Python语法通过

---

*设计完成时间: 2026-03-26*