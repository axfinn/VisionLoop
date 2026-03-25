package mp4

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// EncoderPacket 编码后的数据包
type EncoderPacket struct {
	Data      []byte
	PTS      int64
	DTS      int64
	KeyFrame bool
	IsRecord  bool
	IsMonitor bool
}

// Release 释放数据
func (p *EncoderPacket) Release() {
	p.Data = nil
}

// MP4Writer MP4分段写入器
type MP4Writer struct {
	mu          sync.Mutex
	dir         string
	segmentMin  int
	width       int
	height      int
	bitrate     int

	currentFile *os.File
	currentPath string
	startTime   time.Time
	packetCount int64

	// 流上下文
	frameCount int
	closed     bool
}

// StreamContext 流上下文
type StreamContext struct {
	Index     int
	Timebase  Rational
	CodecType int
}

// Rational 有理数
type Rational struct {
	Num int64
	Den int64
}

// NewMP4Writer 创建MP4写入器
func NewMP4Writer(dir string, segmentMin, width, height, bitrate int) (*MP4Writer, error) {
	w := &MP4Writer{
		dir:        dir,
		segmentMin: segmentMin,
		width:      width,
		height:     height,
		bitrate:    bitrate,
	}

	if err := w.newSegment(); err != nil {
		return nil, err
	}

	return w, nil
}

// newSegment 创建新分段
func (w *MP4Writer) newSegment() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 关闭旧文件
	if w.currentFile != nil {
		w.currentFile.Close()
	}

	// 生成新文件名
	now := time.Now()
	filename := now.Format("2006-01-02_15-04-05") + ".mp4"
	w.currentPath = filepath.Join(w.dir, filename)
	w.startTime = now
	w.packetCount = 0

	// 创建文件
	f, err := os.Create(w.currentPath)
	if err != nil {
		return fmt.Errorf("create segment file failed: %w", err)
	}
	w.currentFile = f

	// 写入简化的MP4 header (实际项目使用FFmpeg CGO)
	// 这里写入一个简单的文件标记
	header := []byte{0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70} // ftyp box
	header = append(header, []byte("isom")...)
	header = append(header, 0x00, 0x00, 0x02, 0x00)
	if _, err := f.Write(header); err != nil {
		return err
	}

	log.Printf("new segment: %s", w.currentPath)
	return nil
}

// WritePacket 写入数据包
func (w *MP4Writer) WritePacket(pkt *EncoderPacket) error {
	if pkt == nil || !pkt.IsRecord {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// 检查是否需要切分 (5分钟 = 5*60*25帧 @25fps)
	maxFrames := int64(w.segmentMin * 60 * 25)
	if w.packetCount > 0 && w.packetCount%maxFrames == 0 {
		if err := w.newSegmentLocked(); err != nil {
			return err
		}
	}

	// 写入NALU长度前缀 + NALU数据
	if len(pkt.Data) > 0 {
		naluLen := make([]byte, 4)
		binary.BigEndian.PutUint32(naluLen, uint32(len(pkt.Data)))

		if _, err := w.currentFile.Write(naluLen); err != nil {
			return err
		}
		if _, err := w.currentFile.Write(pkt.Data); err != nil {
			return err
		}
		w.packetCount++
	}

	return nil
}

// newSegmentLocked 加锁版本
func (w *MP4Writer) newSegmentLocked() error {
	if w.currentFile != nil {
		w.currentFile.Close()
	}

	now := time.Now()
	filename := now.Format("2006-01-02_15-04-05") + ".mp4"
	w.currentPath = filepath.Join(w.dir, filename)
	w.startTime = now
	w.packetCount = 0

	f, err := os.Create(w.currentPath)
	if err != nil {
		return fmt.Errorf("create segment file failed: %w", err)
	}
	w.currentFile = f

	// 写入ftyp box
	header := []byte{0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70, 0x69, 0x73, 0x6f, 0x6d, 0x00, 0x00, 0x02, 0x00}
	f.Write(header)

	log.Printf("new segment: %s", w.currentPath)
	return nil
}

// Flush 刷新数据
func (w *MP4Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentFile != nil {
		return w.currentFile.Sync()
	}
	return nil
}

// Close 关闭写入器
func (w *MP4Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.closed = true
	if w.currentFile != nil {
		err := w.currentFile.Close()
		w.currentFile = nil
		return err
	}
	return nil
}

// CurrentPath 返回当前文件路径
func (w *MP4Writer) CurrentPath() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentPath
}

// MP4FileInfo MP4文件信息
type MP4FileInfo struct {
	Name      string
	Path      string
	Size      int64
	Duration  time.Duration
	CreatedAt time.Time
}

// ListFiles 列出所有MP4文件
func ListFiles(dir string) ([]MP4FileInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []MP4FileInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".mp4") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		files = append(files, MP4FileInfo{
			Name:      entry.Name(),
			Path:      path,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	// 按时间排序
	sort.Slice(files, func(i, j int) bool {
		return files[i].CreatedAt.After(files[j].CreatedAt)
	})

	return files, nil
}
