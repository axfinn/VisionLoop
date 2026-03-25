package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
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

	// 初始化采集
	cam, err := capture.NewCapture(0) // 默认摄像头
	if err != nil {
		log.Printf("WARNING: camera init failed: %v, using test pattern", err)
		cam = capture.NewTestPattern(640, 480, 25)
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
	wrtc := webrtc.NewWebRTC(enc.Width(), enc.Height())
	defer wrtc.Close()

	// 初始化Unix Socket IPC
	socketPath := "/tmp/visionloop_det.sock"
	detIPC, err := ipc.NewDetectionIPC(socketPath)
	if err != nil {
		log.Printf("WARNING: detection IPC init failed: %v", err)
	} else {
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
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-frameCh:
			if frame == nil {
				continue
			}
			// 编码
			recPacket, monPacket, err := enc.EncodeFrame(frame, true, true)
			if err != nil {
				log.Printf("encode error: %v", err)
				frame.Release()
				continue
			}

			// 录制路
			if recPacket != nil {
				// 转换为 mp4.EncoderPacket (两个包定义了相同字段的不同类型)
				mp4Pkt := &mp4.EncoderPacket{
					Data:      recPacket.Data,
					PTS:       recPacket.PTS,
					DTS:       recPacket.DTS,
					KeyFrame:  recPacket.KeyFrame,
					IsRecord:  recPacket.IsRecord,
					IsMonitor: recPacket.IsMonitor,
				}
				if err := mp4.WritePacket(mp4Pkt); err != nil {
					log.Printf("mp4 write error: %v", err)
				}
				recPacket.Release()
			}

			// 监看路
			if monPacket != nil {
				if err := wrtc.WriteVideoFrame(monPacket); err != nil {
					log.Printf("webrtc write error: %v", err)
				}
				monPacket.Release()
			}

			frame.Release()

		case <-ticker.C:
			// 定期检查存储GC
			if err := gc.CheckAndCleanup(); err != nil {
				log.Printf("gc error: %v", err)
			}
		}
	}
}
