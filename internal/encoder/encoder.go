package encoder

import (
	"fmt"
	"image"
	"log"
	"sync"
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

	// 软编码（作为兜底）
	recordEncoder  *softEncoder
	monitorEncoder *softEncoder
}

// softEncoder 软编码器
type softEncoder struct {
	width     int
	height    int
	bitrate   int
	framerate int
	frameBuf  []byte
}

// EncoderPacket 编码后的数据包
type EncoderPacket struct {
	Data      []byte
	PTS       int64
	DTS       int64
	KeyFrame  bool
	IsRecord  bool
	IsMonitor bool
}

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

	// 初始化软编码器
	e.recordEncoder = newSoftEncoder(cfg.Width, cfg.Height, cfg.RecordBitrate, cfg.Framerate)
	e.monitorEncoder = newSoftEncoder(cfg.Width, cfg.Height, cfg.MonitorBitrate, cfg.Framerate)

	log.Printf("encoder initialized: %dx%d, record=%d bps, monitor=%d bps",
		cfg.Width, cfg.Height, cfg.RecordBitrate, cfg.MonitorBitrate)

	return e, nil
}

func newSoftEncoder(width, height, bitrate, framerate int) *softEncoder {
	return &softEncoder{
		width:     width,
		height:    height,
		bitrate:   bitrate,
		framerate: framerate,
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

	// 转换为RGB
	rgb := gocv.NewMat()
	gocv.CvtColor(*img, &rgb, gocv.ColorBGRToRGB)
	defer rgb.Close()

	var recPkt, monPkt *EncoderPacket

	// 录制路编码
	if enableRecord {
		recPkt = e.recordEncoder.encode(rgb, e.frameCount)
		if recPkt != nil {
			recPkt.KeyFrame = e.frameCount%150 == 1
			recPkt.IsRecord = true
		}
	}

	// 监看路编码
	if enableMonitor {
		monPkt = e.monitorEncoder.encode(rgb, e.frameCount)
		if monPkt != nil {
			monPkt.KeyFrame = e.frameCount%150 == 1
			monPkt.IsMonitor = true
		}
	}

	return recPkt, monPkt, nil
}

func (se *softEncoder) encode(mat gocv.Mat, frameCount int64) *EncoderPacket {
	if mat.Empty() {
		return nil
	}

	// 获取图像数据
	data, err := mat.ToBytes()
	if err != nil {
		return nil
	}

	// 简化实现：直接复制数据作为H.264 NALU
	// 实际需要调用FFmpeg CGO进行H.264编码
	pkt := &EncoderPacket{
		Data:     data,
		PTS:      frameCount * int64(90000/se.framerate),
		DTS:      frameCount * int64(90000/se.framerate),
		KeyFrame: frameCount%150 == 1,
	}

	return pkt
}

// Close 关闭编码器
func (e *Encoder) Close() error {
	return nil
}

// GetFrameCount 获取编码帧数
func (e *Encoder) GetFrameCount() int64 {
	e.encMutex.Lock()
	defer e.encMutex.Unlock()
	return e.frameCount
}
