package encoder

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"io"
	"log"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"gocv.io/x/gocv"
	"visionloop/internal/capture"
)

// EncoderConfig 编码器配置
type EncoderConfig struct {
	Width          int
	Height         int
	RecordBitrate  int // 4 Mbps
	MonitorBitrate int // 500 kbps
	Framerate      int
}

// Encoder 双路编码器
type Encoder struct {
	cfg     EncoderConfig
	width   int
	height  int
	encMutex sync.Mutex

	// 编码状态
	frameCount int64
	startTime  time.Time

	// 软编码（使用ffmpeg libx264）
	recordEncoder  *ffmpegEncoder
	monitorEncoder *ffmpegEncoder
}

// ffmpegEncoder 基于ffmpeg的H.264编码器
type ffmpegEncoder struct {
	width     int
	height    int
	bitrate   int
	framerate int

	// ffmpeg进程
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	// 编码缓冲
	bufLock sync.Mutex
	NALUs   [][]byte

	// 控制
	wg       sync.WaitGroup
	stopChan chan struct{}
	frameCount int64
}

// EncoderPacket 编码后的数据包
type EncoderPacket struct {
	Data      []byte
	PTS       int64
	DTS       int64
	KeyFrame  bool
	IsRecord  bool
	IsMonitor bool
	NALUType  byte
}

// SPSPPS 保存SPS和PPS数据
type SPSPPS struct {
	SPS []byte
	PPS []byte
}

// globalSPSPPS 全局SPS/PPS（用于MP4封装）
var globalSPSPPS = &SPSPPS{}
var spsPPSLock sync.Mutex

// Release 释放数据
func (p *EncoderPacket) Release() {
	p.Data = nil
}

// NewEncoder 创建编码器
func NewEncoder(cfg EncoderConfig) (*Encoder, error) {
	if cfg.Width == 0 {
		cfg.Width = 640
	}
	if cfg.Height == 0 {
		cfg.Height = 480
	}
	if cfg.RecordBitrate == 0 {
		cfg.RecordBitrate = 4 * 1024 * 1024
	}
	if cfg.MonitorBitrate == 0 {
		cfg.MonitorBitrate = 500 * 1024
	}
	if cfg.Framerate == 0 {
		cfg.Framerate = 25
	}

	e := &Encoder{
		cfg:    cfg,
		width:  cfg.Width,
		height: cfg.Height,
	}

	// 初始化ffmpeg编码器
	var err error
	e.recordEncoder, err = newFFmpegEncoder(cfg.Width, cfg.Height, cfg.RecordBitrate, cfg.Framerate)
	if err != nil {
		return nil, fmt.Errorf("failed to create record encoder: %w", err)
	}

	e.monitorEncoder, err = newFFmpegEncoder(cfg.Width, cfg.Height, cfg.MonitorBitrate, cfg.Framerate)
	if err != nil {
		e.recordEncoder.Close()
		return nil, fmt.Errorf("failed to create monitor encoder: %w", err)
	}

	log.Printf("encoder initialized: %dx%d, record=%d bps, monitor=%d bps",
		cfg.Width, cfg.Height, cfg.RecordBitrate, cfg.MonitorBitrate)

	return e, nil
}

// newFFmpegEncoder 创建ffmpeg编码器
func newFFmpegEncoder(width, height, bitrate, framerate int) (*ffmpegEncoder, error) {
	enc := &ffmpegEncoder{
		width:     width,
		height:    height,
		bitrate:   bitrate,
		framerate: framerate,
		NALUs:     make([][]byte, 0),
		stopChan:  make(chan struct{}),
	}

	if err := enc.startFFmpeg(); err != nil {
		return nil, err
	}

	// 启动读取goroutine
	enc.wg.Add(1)
	go enc.readOutput()

	return enc, nil
}

// startFFmpeg 启动ffmpeg进程进行H.264编码
func (enc *ffmpegEncoder) startFFmpeg() error {
	// ffmpeg命令行进行实时H.264编码
	// 输入: raw RGB像素 (通过pipe)
	// 输出: H.264 Annex B格式 (通过pipe)
	cmd := exec.Command("ffmpeg",
		"-re",                    // 实时输入模式
		"-f", "rawvideo",         // 输入格式: 原始视频
		"-pix_fmt", "rgb24",      // 像素格式: RGB24
		"-s", fmt.Sprintf("%dx%d", enc.width, enc.height),
		"-r", fmt.Sprintf("%d", enc.framerate),
		"-i", "pipe:0",           // 从stdin读取输入
		"-c:v", "libx264",        // H.264编码
		"-preset", "ultrafast",   // 最快编码速度
		"-tune", "zerolatency",   // 零延迟优化
		"-b:v", fmt.Sprintf("%d", enc.bitrate),
		"-pix_fmt", "yuv420p",    // 输出像素格式
		"-an",                    // 无音频
		"-f", "h264",             // 输出格式: H.264裸流
		"pipe:1"                  // 输出到stdout
	)

	cmd.Stderr = nil // 抑制ffmpeg错误输出

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	enc.cmd = cmd
	enc.stdin = stdin
	enc.stdout = bufio.NewReader(stdout)

	log.Printf("ffmpeg encoder started: %dx%d, bitrate=%d, framerate=%d",
		enc.width, enc.height, enc.bitrate, enc.framerate)

	return nil
}

// readOutput 异步读取ffmpeg输出
func (enc *ffmpegEncoder) readOutput() {
	defer enc.wg.Done()

	startCode := []byte{0x00, 0x00, 0x00, 0x01}
	var buffer []byte

	buf := make([]byte, 8192)
	for {
		select {
		case <-enc.stopChan:
			return
		default:
			n, err := enc.stdout.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("ffmpeg read error: %v", err)
				}
				return
			}
			if n > 0 {
				buffer = append(buffer, buf[:n]...)
				enc.processBuffer(&buffer, startCode)
			}
		}
	}
}

// processBuffer 处理缓冲区，提取NALU
func (enc *ffmpegEncoder) processBuffer(buffer *[]byte, startCode []byte) {
	for {
		// 查找start code
		startIdx := bytes.Index(*buffer, startCode)
		if startIdx == -1 {
			// 没有找到完整start code，保留部分数据
			if len(*buffer) > 4 {
				*buffer = (*buffer)[len(*buffer)-4:]
			}
			break
		}

		// 查找下一个start code
		remaining := (*buffer)[startIdx+4:]
		nextIdx := bytes.Index(remaining, startCode)

		var naluData []byte
		if nextIdx == -1 {
			// 没有下一个start code，数据不完整
			break
		} else {
			naluData = (*buffer)[startIdx : startIdx+4+nextIdx]
			*buffer = (*buffer)[startIdx+4+nextIdx:]
		}

		// 提取NALU类型
		if len(naluData) > 4 {
			nalType := naluData[4] & 0x1F

			// 保存SPS和PPS
			if nalType == 7 { // SPS
				spsPPSLock.Lock()
				globalSPSPPS.SPS = naluData[4:]
				spsPPSLock.Unlock()
				log.Printf("SPS captured, length: %d", len(globalSPSPPS.SPS))
			} else if nalType == 8 { // PPS
				spsPPSLock.Lock()
				globalSPSPPS.PPS = naluData[4:]
				spsPPSLock.Unlock()
				log.Printf("PPS captured, length: %d", len(globalSPSPPS.PPS))
			}

			// 添加到NALU列表
			enc.bufLock.Lock()
			enc.NALUs = append(enc.NALUs, naluData[4:]) // 去掉start code
			enc.bufLock.Unlock()
		}
	}
}

// encode 编码一帧
func (enc *ffmpegEncoder) encode(mat gocv.Mat, frameCount int64) *EncoderPacket {
	if mat.Empty() {
		return nil
	}

	// 转换颜色空间 BGR -> RGB
	rgb := gocv.NewMat()
	gocv.CvtColor(mat, &rgb, gocv.ColorBGRToRGB)
	defer rgb.Close()

	// 获取图像数据
	data, err := rgb.ToBytes()
	if err != nil {
		return nil
	}

	// 写入ffmpeg stdin
	if _, err := enc.stdin.Write(data); err != nil {
		log.Printf("ffmpeg write error: %v", err)
		return nil
	}

	atomic.AddInt64(&enc.frameCount, 1)

	// 计算PTS/DTS
	pts := atomic.LoadInt64(&enc.frameCount) * int64(90000/enc.framerate)

	return &EncoderPacket{
		Data:     nil, // 数据从NALU缓冲读取
		PTS:      pts,
		DTS:      pts,
		KeyFrame: atomic.LoadInt64(&enc.frameCount)%150 == 1,
	}
}

// GetNALUs 获取已编码的NALU列表
func (enc *ffmpegEncoder) GetNALUs() [][]byte {
	enc.bufLock.Lock()
	defer enc.bufLock.Unlock()

	if len(enc.NALUs) == 0 {
		return nil
	}

	nalus := enc.NALUs
	enc.NALUs = make([][]byte, 0)
	return nalus
}

// GetSPSPPS 获取SPS和PPS
func GetSPSPPS() (sps, pps []byte) {
	spsPPSLock.Lock()
	defer spsPPSLock.Unlock()
	return globalSPSPPS.SPS, globalSPSPPS.PPS
}

// Close 关闭编码器
func (enc *ffmpegEncoder) Close() {
	close(enc.stopChan)
	enc.wg.Wait()

	if enc.stdin != nil {
		enc.stdin.Close()
	}
	if enc.stdout != nil {
		enc.stdout = nil
	}
	if enc.cmd != nil {
		enc.cmd.Process.Kill()
		enc.cmd.Wait()
	}
}

// Width 返回宽度
func (e *Encoder) Width() int {
	return e.width
}

// Height 返回高度
func (e *Encoder) Height() int {
	return e.height
}

// EncodeFrame 编码一帧
func (e *Encoder) EncodeFrame(frame *capture.Frame, enableRecord, enableMonitor bool) (*EncoderPacket, *EncoderPacket, error) {
	if frame == nil || frame.Img == nil || frame.Img.Empty() {
		return nil, nil, fmt.Errorf("invalid frame")
	}

	e.encMutex.Lock()
	defer e.encMutex.Unlock()

	e.frameCount++

	// 获取图像
	img := frame.Img

	// 缩放到目标分辨率
	if img.Rows() != e.height || img.Cols() != e.width {
		tmp := gocv.NewMat()
		gocv.Resize(*img, &tmp, image.Point{X: e.width, Y: e.height}, 0, 0, gocv.InterpolationLinear)
		img = &tmp
		defer tmp.Close()
	}

	var recPkt, monPkt *EncoderPacket

	// 录制路编码
	if enableRecord {
		recPkt = e.recordEncoder.encode(img, e.frameCount)
		if recPkt != nil {
			recPkt.KeyFrame = e.frameCount%150 == 1
			recPkt.IsRecord = true
		}
	}

	// 监看路编码
	if enableMonitor {
		monPkt = e.monitorEncoder.encode(img, e.frameCount)
		if monPkt != nil {
			monPkt.KeyFrame = e.frameCount%150 == 1
			monPkt.IsMonitor = true
		}
	}

	return recPkt, monPkt, nil
}

// GetRecordNALUs 获取录制编码器的NALU
func (e *Encoder) GetRecordNALUs() [][]byte {
	if e.recordEncoder == nil {
		return nil
	}
	return e.recordEncoder.GetNALUs()
}

// GetMonitorNALUs 获取监看编码器的NALU
func (e *Encoder) GetMonitorNALUs() [][]byte {
	if e.monitorEncoder == nil {
		return nil
	}
	return e.monitorEncoder.GetNALUs()
}

// Close 关闭编码器
func (e *Encoder) Close() error {
	if e.recordEncoder != nil {
		e.recordEncoder.Close()
	}
	if e.monitorEncoder != nil {
		e.monitorEncoder.Close()
	}
	return nil
}

// GetFrameCount 获取编码帧数
func (e *Encoder) GetFrameCount() int64 {
	e.encMutex.Lock()
	defer e.encMutex.Unlock()
	return e.frameCount
}