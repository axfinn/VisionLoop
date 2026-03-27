package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"visionloop/internal/api"
	"visionloop/internal/capture"
	"visionloop/internal/encoder"
	"visionloop/internal/ipc"
	"visionloop/internal/mp4"
	"visionloop/internal/storage"
	"visionloop/internal/webrtc"
)

var (
	Version   = "1.0.0"
	BuildTime = time.Now().Format("2006-01-02")
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("VisionLoop v%s (built: %s) starting...", Version, BuildTime)

	// 工作目录
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatal("get workdir failed:", err)
	}

	// 确保必要目录存在
	clipsDir := filepath.Join(workDir, "clips")
	os.MkdirAll(clipsDir, 0755)
	os.MkdirAll(filepath.Join(workDir, "events"), 0755)
	os.MkdirAll(filepath.Join(workDir, "screenshots"), 0755)

	// 配置
	cfg := &Config{
		ClipsDir:     clipsDir,
		EventsDir:    filepath.Join(workDir, "events"),
		Screenshots:  filepath.Join(workDir, "screenshots"),
		MaxStorageGB: 50.0,
		SegmentMin:   5, // 5分钟分段
		RecordBitrate: 4 * 1024 * 1024,  // 4Mbps
		MonitorBitrate: 500 * 1024,       // 500kbps
	}

	// 初始化存储GC
	gc := storage.NewGC(cfg.ClipsDir, cfg.MaxStorageGB)

	// 初始化采集 - 优先使用视频文件，摄像头作为备选
	var cam capture.Capture
	videoPath := "/Users/finn/Movies/20260204_140354.mp4"
	if _, statErr := os.Stat(videoPath); statErr == nil {
		cam, err = capture.NewVideoFileCapture(videoPath, true)
		if err != nil {
			log.Printf("WARNING: video file failed: %v, trying camera", err)
			cam, err = capture.NewCapture(0)
			if err != nil {
				log.Printf("WARNING: camera failed: %v, using test pattern", err)
				cam = capture.NewTestPattern(640, 480, 25)
			} else {
				log.Println("camera initialized successfully")
			}
		} else {
			log.Println("video file initialized successfully")
		}
	} else {
		log.Printf("video file not found, trying camera")
		cam, err = capture.NewCapture(0)
		if err != nil {
			log.Printf("WARNING: camera failed: %v, using test pattern", err)
			cam = capture.NewTestPattern(640, 480, 25)
		} else {
			log.Println("camera initialized successfully")
		}
	}

	// 初始化编码器
	enc, err := encoder.NewEncoder(encoder.EncoderConfig{
		Width:          640,
		Height:         480,
		RecordBitrate:  cfg.RecordBitrate,
		MonitorBitrate: cfg.MonitorBitrate,
		Framerate:      25,
	})
	if err != nil {
		log.Fatal("encoder init failed:", err)
	}
	defer enc.Close()

	// 初始化MP4录制
	mp4Writer, err := mp4.NewMP4Writer(cfg.ClipsDir, cfg.SegmentMin, enc.Width(), enc.Height(), cfg.RecordBitrate)
	if err != nil {
		log.Fatal("mp4 writer init failed:", err)
	}
	defer mp4Writer.Close()

	// 初始化WebRTC
	wrtc, err := webrtc.NewWebRTC(enc.Width(), enc.Height())
	if err != nil {
		log.Fatal("webrtc init failed:", err)
	}
	defer wrtc.Close()

	// 初始化Unix Socket IPC
	socketPath := "/tmp/visionloop_det.sock"
	detIPC, err := ipc.NewDetectionIPC(socketPath)
	if err != nil {
		log.Printf("WARNING: detection IPC init failed: %v", err)
	}

	// 启动Python检测进程
	var detProc *exec.Cmd
	detProc, err = startDetectionProcess(socketPath)
	if err != nil {
		log.Printf("WARNING: detection process start failed: %v", err)
	} else {
		defer func() {
			if detProc != nil && detProc.Process != nil {
				detProc.Process.Kill()
			}
		}()
	}
	if detIPC != nil {
		defer detIPC.Close()
	}

	// 启动采集+编码+录制主循环
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 采集通道（无缓冲，下游满则丢帧）
	frameCh := make(chan *capture.Frame)

	// 启动采集goroutine
	go cam.CaptureLoop(ctx, frameCh)

	// 编码+录制goroutine
	go runEncodeLoop(ctx, frameCh, enc, mp4Writer, wrtc, detIPC, gc)

	// 初始化API服务
	server := api.NewServer(api.ServerConfig{
		ClipsDir:     cfg.ClipsDir,
		EventsDir:    cfg.EventsDir,
		Screenshots:  cfg.Screenshots,
		MaxStorageGB: cfg.MaxStorageGB,
		WebRTC:       wrtc,
		Version:      Version,
	})

	// 启动HTTP服务器
	addr := ":8080"
	srv := &http.Server{
		Addr:    addr,
		Handler: server.Router(),
	}

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("VisionLoop server listening on http://localhost%s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("server error:", err)
	}
	log.Println("VisionLoop stopped")
}

// Config 配置
type Config struct {
	ClipsDir      string
	EventsDir     string
	Screenshots   string
	MaxStorageGB  float64
	SegmentMin    int
	RecordBitrate int
	MonitorBitrate int
}

// runEncodeLoop 主编码循环
func runEncodeLoop(ctx context.Context, frameCh <-chan *capture.Frame, enc *encoder.Encoder, mp4 *mp4.MP4Writer, wrtc *webrtc.WebRTC, detIPC *ipc.DetectionIPC, gc *storage.GC) {
	// 500ms定时器用于GC和IPC
	ticker := time.NewTicker(500 * time.Millisecond)
	// 40ms定时器用于NALU获取 (25fps)
	naluTicker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	defer naluTicker.Stop()

	// 用于IPC的帧缓冲（非阻塞发送）
	var latestFrame *capture.Frame
	frameCount := int64(0)

	for {
		select {
		case <-ctx.Done():
			// 清理
			if latestFrame != nil {
				latestFrame.Release()
			}
			return
		case frame := <-frameCh:
			if frame == nil {
				continue
			}
			// 编码
			rec, mon, err := enc.EncodeFrame(frame, true, true)
			if err != nil {
				log.Printf("encode error: %v", err)
				frame.Release()
				continue
			}
			frameCount++
			if frameCount%100 == 0 {
				log.Printf("encoded %d frames, rec=%v, mon=%v", frameCount, rec != nil, mon != nil)
			}

			// 保存最新帧用于IPC（500ms发送一次）
			if latestFrame != nil {
				latestFrame.Release()
			}
			latestFrame = frame

		case <-naluTicker.C:
			// 获取录制NALU并写入MP4
			nalus := enc.GetRecordNALUs()
			for i, nalu := range nalus {
				isKeyFrame := (i == 0 && frameCount%150 == 1)
				if err := mp4.WriteNALU(nalu, isKeyFrame); err != nil {
					log.Printf("mp4 write error: %v", err)
				}
			}

			// 获取监看NALU并发送到WebRTC
			monitorNalus := enc.GetMonitorNALUs()
			if len(monitorNalus) > 0 {
				if err := wrtc.WriteRawNALU(monitorNalus, frameCount%150 == 1); err != nil {
					log.Printf("webrtc write error: %v", err)
				}
			}

		case <-ticker.C:
			// 发送最新帧到检测进程（非阻塞，每500ms发送一次）
			if latestFrame != nil && detIPC != nil && detIPC.IsConnected() {
				if err := detIPC.SendFrame(latestFrame); err != nil {
					// 检测进程断开连接时不阻塞主循环
				}
				// 发送后释放帧
				latestFrame.Release()
				latestFrame = nil
			}

			// 定期检查存储GC
			if err := gc.CheckAndCleanup(); err != nil {
				log.Printf("gc error: %v", err)
			}
		}
	}
}

// startDetectionProcess 启动Python检测进程
func startDetectionProcess(socketPath string) (*exec.Cmd, error) {
	// 查找Python检测脚本
	execPath, err := os.Executable()
	if err != nil {
		execPath = os.Args[0]
	}
	execDir := filepath.Dir(execPath)
	detScript := filepath.Join(execDir, "detection", "main.py")

	// 如果脚本不存在，尝试当前目录
	if _, err := os.Stat(detScript); os.IsNotExist(err) {
		detScript = filepath.Join("detection", "main.py")
	}

	// 检查脚本是否存在
	if _, err := os.Stat(detScript); os.IsNotExist(err) {
		return nil, fmt.Errorf("detection script not found: %s", detScript)
	}

	// 启动Python检测进程
	cmd := exec.Command("python", detScript,
		"--socket", socketPath,
		"--api", "http://localhost:8080")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start detection process failed: %w", err)
	}

	log.Printf("detection process started (PID: %d)", cmd.Process.Pid)
	return cmd, nil
}
