package capture

import (
	"context"
	"log"
	"math"
	"time"

	"gocv.io/x/gocv"
)

// Frame 采集到的帧
type Frame struct {
	Img     *gocv.Mat
	Ts      time.Time
	Width   int
	Height  int
}

// NewFrame 创建帧
func NewFrame(img *gocv.Mat) *Frame {
	if img == nil {
		return nil
	}
	return &Frame{
		Img:    img,
		Ts:     time.Now(),
		Width:  img.Cols(),
		Height: img.Rows(),
	}
}

// Release 释放帧
func (f *Frame) Release() {
	if f.Img != nil {
		f.Img.Close()
		f.Img = nil
	}
}

// Capture 采集接口
type Capture interface {
	CaptureLoop(ctx context.Context, frameCh chan<- *Frame)
	Width() int
	Height() int
	Close()
}

// VideoCapture gocv摄像头采集
type VideoCapture struct {
	cap     *gocv.VideoCapture
	width   int
	height  int
	fps     float64
	device  int
}

var _ Capture = (*VideoCapture)(nil)

// NewCapture 创建摄像头采集
func NewCapture(device int) (*VideoCapture, error) {
	// 尝试直接打开，不指定API，让gocv选择默认后端
	cap, err := gocv.OpenVideoCapture(device)
	if err != nil {
		return nil, err
	}

	// 等待摄像头稳定
	time.Sleep(500 * time.Millisecond)

	// 获取摄像头参数
	width := int(cap.Get(gocv.VideoCaptureProperties(3)))
	height := int(cap.Get(gocv.VideoCaptureProperties(4)))
	fps := cap.Get(gocv.VideoCaptureProperties(5))

	if width == 0 || height == 0 {
		width, height = 640, 480
		cap.Set(gocv.VideoCaptureProperties(3), float64(width))
		cap.Set(gocv.VideoCaptureProperties(4), float64(height))
	}
	if fps == 0 {
		fps = 25
	}

	log.Printf("camera opened: %dx%d @ %.2f fps", width, height, fps)

	return &VideoCapture{
		cap:    cap,
		width:  width,
		height: height,
		fps:    fps,
		device: device,
	}, nil
}

func (c *VideoCapture) Width() int  { return c.width }
func (c *VideoCapture) Height() int { return c.height }

func (c *VideoCapture) Close() {
	if c.cap != nil {
		c.cap.Close()
		c.cap = nil
	}
}

// CaptureLoop 采集循环，无缓冲channel，下游满则丢帧
func (c *VideoCapture) CaptureLoop(ctx context.Context, frameCh chan<- *Frame) {
	defer close(frameCh)

	// 目标帧率
	frameDuration := time.Duration(1e9 / int(c.fps))
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	consecutiveFails := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			img := gocv.NewMat()
			if ok := c.cap.Read(&img); !ok {
				img.Close()
				consecutiveFails++
				if consecutiveFails%100 == 0 {
					log.Printf("camera read failed %d times, retrying...", consecutiveFails)
				}
				continue
			}
			if img.Empty() {
				img.Close()
				continue
			}
			consecutiveFails = 0

			select {
			case frameCh <- NewFrame(&img):
				// 成功写入，不阻塞
			default:
				// channel满，丢帧
				img.Close()
			}
		}
	}
}

// VideoFileCapture 视频文件采集
type VideoFileCapture struct {
	cap    *gocv.VideoCapture
	width  int
	height int
	fps    float64
	loop   bool
}

var _ Capture = (*VideoFileCapture)(nil)

// NewVideoFileCapture 创建视频文件采集
func NewVideoFileCapture(path string, loop bool) (*VideoFileCapture, error) {
	cap, err := gocv.OpenVideoCapture(path)
	if err != nil {
		return nil, err
	}

	width := int(cap.Get(gocv.VideoCaptureProperties(3)))
	height := int(cap.Get(gocv.VideoCaptureProperties(4)))
	fps := cap.Get(gocv.VideoCaptureProperties(5))

	if fps == 0 {
		fps = 25
	}

	log.Printf("video file opened: %dx%d @ %.2f fps, loop=%v", width, height, fps, loop)

	return &VideoFileCapture{
		cap:    cap,
		width:  width,
		height: height,
		fps:    fps,
		loop:   loop,
	}, nil
}

func (c *VideoFileCapture) Width() int  { return c.width }
func (c *VideoFileCapture) Height() int { return c.height }

func (c *VideoFileCapture) Close() {
	if c.cap != nil {
		c.cap.Close()
		c.cap = nil
	}
}

// CaptureLoop 视频文件采集循环
func (c *VideoFileCapture) CaptureLoop(ctx context.Context, frameCh chan<- *Frame) {
	defer close(frameCh)

	frameDuration := time.Duration(1e9 / int(c.fps))
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			img := gocv.NewMat()
			if ok := c.cap.Read(&img); !ok {
				img.Close()
				if c.loop {
					c.cap.Set(gocv.VideoCaptureProperties(1), 0) // seek to beginning
					continue
				}
				return
			}
			if img.Empty() {
				img.Close()
				if c.loop {
					c.cap.Set(gocv.VideoCaptureProperties(1), 0)
					continue
				}
				return
			}

			select {
			case frameCh <- NewFrame(&img):
			default:
				img.Close()
			}
		}
	}
}

// TestPattern 测试图案生成器
type TestPattern struct {
	width  int
	height int
	fps    float64
	t      float64
}

var _ Capture = (*TestPattern)(nil)

// NewTestPattern 创建测试图案
func NewTestPattern(width, height int, fps float64) *TestPattern {
	return &TestPattern{
		width:  width,
		height: height,
		fps:    fps,
	}
}

func (t *TestPattern) Width() int  { return t.width }
func (t *TestPattern) Height() int { return t.height }

func (t *TestPattern) Close() {}

// CaptureLoop 生成测试图案
func (t *TestPattern) CaptureLoop(ctx context.Context, frameCh chan<- *Frame) {
	defer close(frameCh)

	frameDuration := time.Duration(1e9 / int(t.fps))
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			img := t.generateFrame()
			select {
			case frameCh <- NewFrame(img):
			default:
				img.Close()
			}
			t.t += 1.0 / t.fps
		}
	}
}

func (t *TestPattern) generateFrame() *gocv.Mat {
	img := gocv.NewMatWithSize(t.height, t.width, gocv.MatTypeCV8UC3)
	if img.Empty() {
		return nil
	}

	// 生成渐变测试图案
	for y := 0; y < t.height; y++ {
		for x := 0; x < t.width; x++ {
			phase := float64(x)*0.02 + t.t*2.0
			v := uint8((math.Sin(phase) + 1) * 127)
			img.SetUCharAt(y, x*3, v)
			img.SetUCharAt(y, x*3+1, v)
			img.SetUCharAt(y, x*3+2, v)
		}
	}

	return &img
}
