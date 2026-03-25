package ipc

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"visionloop/internal/capture"
)

// DetectionIPC Unix Socket IPC for detection process
type DetectionIPC struct {
	mu         sync.RWMutex
	socketPath string
	listener   *net.UnixListener
	conn       net.Conn
	connected  bool
	stopCh     chan struct{}
}

// FrameHeader 帧头
type FrameHeader struct {
	Width       int32
	Height      int32
	Channels    int32
	FrameType   int32
	TimestampNS int64
}

// NewDetectionIPC 创建IPC
func NewDetectionIPC(socketPath string) (*DetectionIPC, error) {
	// 清理旧socket
	os.Remove(socketPath)

	l, err := net.ListenUnix("unix", nil)
	if err != nil {
		return nil, fmt.Errorf("listen unix socket failed: %w", err)
	}
	if err := os.Chmod(socketPath, 0777); err != nil {
		log.Printf("chmod socket failed: %v", err)
	}

	ipc := &DetectionIPC{
		socketPath: socketPath,
		listener:   l,
		stopCh:     make(chan struct{}),
	}

	// 启动接受连接goroutine
	go ipc.acceptLoop()

	log.Printf("detection IPC listening on %s", socketPath)
	return ipc, nil
}

func (ipc *DetectionIPC) acceptLoop() {
	for {
		select {
		case <-ipc.stopCh:
			return
		default:
			ipc.listener.SetDeadline(time.Now().Add(1 * time.Second))
			conn, err := ipc.listener.Accept()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.Printf("accept error: %v", err)
				continue
			}

			ipc.mu.Lock()
			ipc.conn = conn
			ipc.connected = true
			ipc.mu.Unlock()

			log.Printf("detection process connected")
		}
	}
}

// SendFrame 发送帧到检测进程
func (ipc *DetectionIPC) SendFrame(frame *capture.Frame) error {
	ipc.mu.RLock()
	conn := ipc.conn
	connected := ipc.connected
	ipc.mu.RUnlock()

	if !connected || conn == nil {
		return fmt.Errorf("not connected to detection process")
	}

	header := FrameHeader{
		Width:       int32(frame.Width),
		Height:      int32(frame.Height),
		Channels:    3, // RGB
		FrameType:   0, // RGB
		TimestampNS: frame.Ts.UnixNano(),
	}

	// 发送header
	headerBuf := make([]byte, 24)
	binary.LittleEndian.PutUint32(headerBuf[0:4], uint32(header.Width))
	binary.LittleEndian.PutUint32(headerBuf[4:8], uint32(header.Height))
	binary.LittleEndian.PutUint32(headerBuf[8:12], uint32(header.Channels))
	binary.LittleEndian.PutUint32(headerBuf[12:16], uint32(header.FrameType))
	binary.LittleEndian.PutUint64(headerBuf[16:24], uint64(header.TimestampNS))

	if _, err := conn.Write(headerBuf); err != nil {
		ipc.mu.Lock()
		ipc.connected = false
		ipc.mu.Unlock()
		return fmt.Errorf("write header failed: %w", err)
	}

	// 发送图像数据
	if frame.Img != nil && !frame.Img.Empty() {
		data, err := frame.Img.ToBytes()
		if err != nil {
			return fmt.Errorf("mat to bytes failed: %w", err)
		}
		if _, err := conn.Write(data); err != nil {
			ipc.mu.Lock()
			ipc.connected = false
			ipc.mu.Unlock()
			return fmt.Errorf("write data failed: %w", err)
		}
	}

	return nil
}

// IsConnected 检查连接状态
func (ipc *DetectionIPC) IsConnected() bool {
	ipc.mu.RLock()
	defer ipc.mu.RUnlock()
	return ipc.connected
}

// Close 关闭
func (ipc *DetectionIPC) Close() error {
	close(ipc.stopCh)
	if ipc.conn != nil {
		ipc.conn.Close()
	}
	if ipc.listener != nil {
		ipc.listener.Close()
	}
	os.Remove(ipc.socketPath)
	return nil
}
