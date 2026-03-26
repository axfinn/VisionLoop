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

	"visionloop/internal/encoder"
)

// EncoderPacket 编码后的数据包（兼容encoder包的定义）
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
	framerate   int

	currentFile *os.File
	currentPath string
	startTime   time.Time
	frameCount  int64
	closed      bool

	// MP4结构
	moov    *moovBox
	samples []sampleEntry
}

// MP4 box类型
const (
	BoxTypeFTYP = 0x66747970 // "ftyp"
	BoxTypeMOOV = 0x6d6f6f76 // "moov"
	BoxTypeMVHD = 0x6d766864 // "mvhd"
	BoxTypeTRAK = 0x7472616b // "trak"
	BoxTypeTKHD = 0x746b6864 // "tkhd"
	BoxTypeMDIA = 0x6d646961 // "mdia"
	BoxTypeMDHD = 0x6d646864 // "mdhd"
	BoxTypeMINF = 0x6d696e66 // "minf"
	BoxTypeSTBL = 0x7374626c // "stbl"
	BoxTypeSTSD = 0x73747364 // "stsd"
	BoxTypeSTTS = 0x73747473 // "stts"
	BoxTypeSTSC = 0x73747363 // "stsc"
	BoxTypeSTCO = 0x7374636f // "stco"
	BoxTypeMDAT = 0x6d646174 // "mdat"
	BoxTypeAVC1 = 0x61766331 // "avc1"
	BoxTypeAVCC = 0x61766343 // "avcC"
)

// Box MP4 box基础结构
type Box struct {
	Type   uint32
	Size   uint64
	Offset uint64
	Data   []byte
	Children []Box
}

// moovBox moov box结构
type moovBox struct {
	mvhd []byte
	trak []byte
}

// sampleEntry 样本条目
type sampleEntry struct {
	size      uint32
	duration  uint32
	offset    uint32
	isKeyFrame bool
	data      []byte
}

// NewMP4Writer 创建MP4写入器
func NewMP4Writer(dir string, segmentMin, width, height, bitrate int) (*MP4Writer, error) {
	framerate := 25 // 默认帧率

	w := &MP4Writer{
		dir:        dir,
		segmentMin: segmentMin,
		width:      width,
		height:     height,
		bitrate:    bitrate,
		framerate:  framerate,
		samples:    make([]sampleEntry, 0),
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
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
	w.frameCount = 0
	w.samples = make([]sampleEntry, 0)

	// 创建临时文件用于mdat数据
	tmpFile, err := os.CreateTemp(w.dir, "mp4tmp_*.dat")
	if err != nil {
		return fmt.Errorf("create temp file failed: %w", err)
	}
	tmpFile.Close()
	os.Remove(tmpFile.Name())
	w.currentFile = tmpFile

	log.Printf("new segment: %s", w.currentPath)
	return nil
}

// WritePacket 写入数据包
func (w *MP4Writer) WritePacket(pkt *encoder.EncoderPacket) error {
	if pkt == nil || !pkt.IsRecord || len(pkt.Data) == 0 {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// 检查是否需要切分 (5分钟 = segmentMin * 60 * framerate)
	maxFrames := int64(w.segmentMin * 60 * w.framerate)
	if w.frameCount > 0 && w.frameCount%maxFrames == 0 {
		if err := w.finalizeSegmentLocked(); err != nil {
			return err
		}
		if err := w.newSegmentLocked(); err != nil {
			return err
		}
	}

	// 写入NALU长度前缀 + NALU数据 (AVC格式)
	naluLen := make([]byte, 4)
	binary.BigEndian.PutUint32(naluLen, uint32(len(pkt.Data)))

	if _, err := w.currentFile.Write(naluLen); err != nil {
		return err
	}
	if _, err := w.currentFile.Write(pkt.Data); err != nil {
		return err
	}

	// 记录sample信息
	w.samples = append(w.samples, sampleEntry{
		size:      uint32(len(pkt.Data) + 4), // 包含长度前缀
		duration:  uint32(90000 / w.framerate), // 采样持续时间
		offset:    uint32(w.frameCount), // 简化：实际offset需要计算
		isKeyFrame: pkt.KeyFrame,
		data:      pkt.Data,
	})

	w.frameCount++
	return nil
}

// WriteNALU 直接写入NALU数据（从encoder获取）
func (w *MP4Writer) WriteNALU(naluData []byte, isKeyFrame bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentFile == nil {
		return fmt.Errorf("no current file")
	}

	// 写入4字节长度 + NALU数据
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(naluData)))

	if _, err := w.currentFile.Write(length); err != nil {
		return err
	}
	if _, err := w.currentFile.Write(naluData); err != nil {
		return err
	}

	// 记录sample信息
	w.samples = append(w.samples, sampleEntry{
		size:       uint32(len(naluData) + 4),
		duration:   uint32(90000 / w.framerate),
		offset:     uint32(len(w.samples)), // 简化
		isKeyFrame: isKeyFrame,
		data:       naluData,
	})

	w.frameCount++
	return nil
}

// finalizeSegmentLocked 完成分段写入（加锁版本）
func (w *MP4Writer) finalizeSegmentLocked() error {
	if w.currentFile == nil {
		return nil
	}

	// 获取当前数据大小
	fileInfo, err := w.currentFile.Stat()
	if err != nil {
		return err
	}
	mdatSize := fileInfo.Size()

	// 创建MP4文件
	mp4File, err := os.Create(w.currentPath)
	if err != nil {
		return fmt.Errorf("create mp4 file failed: %w", err)
	}
	defer mp4File.Close()

	// 生成avcC描述符
	avcC := w.generateAVCC()

	// 生成moov box
	moovData := w.generateMOOV(uint32(mdatSize), avcC)

	// 写入ftyp box
	ftyp := w.generateFTYP()
	if _, err := mp4File.Write(ftyp); err != nil {
		return err
	}

	// 写入moov box
	if _, err := mp4File.Write(moovData); err != nil {
		return err
	}

	// 复制mdat数据
	w.currentFile.Seek(0, 0)
	_, err = io.Copy(mp4File, w.currentFile)
	if err != nil {
		return err
	}

	// 关闭临时文件
	w.currentFile.Close()

	// 删除临时文件
	tmpPath := w.currentPath + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		os.Remove(tmpPath)
	}

	log.Printf("segment finalized: %s, frames: %d", w.currentPath, len(w.samples))
	return nil
}

// generateFTYP 生成ftyp box
func (w *MP4Writer) generateFTYP() []byte {
	// ftyp box: major_brand(4) + minor_version(4) + compatible_brands[]
	brand := []byte("isom") // isom品牌
	version := []byte{0x00, 0x00, 0x02, 0x00} // minor version
	compatible := []byte("isomavc1") // 兼容品牌

	boxSize := 8 + 4 + 4 + len(compatible)
	box := make([]byte, boxSize)
	binary.BigEndian.PutUint32(box[0:4], uint32(boxSize))
	binary.BigEndian.PutUint32(box[4:8], BoxTypeFTYP)
	copy(box[8:12], brand)
	copy(box[12:16], version)
	copy(box[16:], compatible)

	return box
}

// generateAVCC 生成avcC描述符
func (w *MP4Writer) generateAVCC() []byte {
	sps, pps := encoder.GetSPSPPS()

	if len(sps) == 0 || len(pps) == 0 {
		// 默认值（如果没有获取到SPS/PPS）
		sps = []byte{0x67, 0x42, 0xc0, 0x0d, 0xda, 0x0f, 0x2a, 0x7e, 0x44}
		pps = []byte{0x68, 0xce, 0x06, 0xe2}
	}

	// avcC格式:
	// version(1) + profile(1) + profile_compatibility(1) + level(1) + 0xFF (6 bits reserved + 3 bits nal size length - 1) + 0xFF (5 bits reserved + 3 bits num of SPS) + SPS length(2) + SPS + 0xFF (8 bits num of PPS) + PPS length(2) + PPS
	avcC := make([]byte, 0)
	avcC = append(avcC, 0x01) // version
	avcC = append(avcC, sps[1]) // profile
	avcC = append(avcC, sps[2]) // profile_compatibility
	avcC = append(avcC, sps[3]) // level
	avcC = append(avcC, 0xFF) // 0xFF = 6 bits reserved (111111) + nal size length - 1 (3 bits, 0 = 1 byte, 3 = 4 bytes)
	avcC = append(avcC, 0xE1) // 0xE1 = 3 bits reserved (111) + num of SPS (5 bits) = 1 SPS
	// SPS length
	avcC = append(avcC, byte(len(sps)>>8), byte(len(sps)&0xFF))
	// SPS
	avcC = append(avcC, sps...)
	// PPS count
	avcC = append(avcC, 0x01) // 1 PPS
	// PPS length
	avcC = append(avcC, byte(len(pps)>>8), byte(len(pps)&0xFF))
	// PPS
	avcC = append(avcC, pps...)

	return avcC
}

// generateMOOV 生成moov box
func (w *MP4Writer) generateMOOV(mdatSize uint32, avcC []byte) []byte {
	// 计算时间
	timescale := uint32(90000) // H.264 timescale
	duration := uint32(len(w.samples)) * uint32(90000/uint32(w.framerate))

	// mvhd box
	mvhd := w.generateMVHD(timescale, duration)

	// tkhd box
	tkhd := w.generateTKHD(timescale, duration)

	// mdhd box
	mdhd := w.generateMDHD(timescale, duration)

	// stbl box
	stbl := w.generateSTBL(avcC)

	// minf box
	minf := w.generateMINF(stbl)

	// mdia box
	mdia := w.generateMDIA(mdhd, minf)

	// trak box
	trak := w.generateTRAK(tkhd, mdia)

	// 组合moov
	moovSize := 8 + len(mvhd) + len(trak)
	moov := make([]byte, 0, moovSize)
	moov = append(moov, 0x00, 0x00, 0x00, 0x00) // size (后填充)
	moov = append(moov, 0x6d, 0x6f, 0x6f, 0x76) // "moov"
	moov = append(moov, mvhd...)
	moov = append(moov, trak...)

	// 填充moov size
	size := uint32(len(moov))
	moov[0] = byte(size >> 24)
	moov[1] = byte((size >> 16) & 0xFF)
	moov[2] = byte((size >> 8) & 0xFF)
	moov[3] = byte(size & 0xFF)

	return moov
}

// generateMVHD 生成mvhd box
func (w *MP4Writer) generateMVHD(timescale, duration uint32) []byte {
	box := make([]byte, 96+8)

	// box size
	binary.BigEndian.PutUint32(box[0:4], uint32(len(box)))
	// box type "mvhd"
	box[4] = 0x6d; box[5] = 0x76; box[6] = 0x68; box[7] = 0x64

	// version 0
	box[8] = 0x00
	// flags
	box[9] = 0x00; box[10] = 0x00; box[11] = 0x00

	// creation time
	binary.BigEndian.PutUint32(box[12:16], 0)
	// modification time
	binary.BigEndian.PutUint32(box[16:20], 0)
	// timescale
	binary.BigEndian.PutUint32(box[20:24], timescale)
	// duration
	binary.BigEndian.PutUint32(box[24:28], duration)

	// rate (fixed point 16.16)
	binary.BigEndian.PutUint32(box[28:32], 0x00010000)
	// volume (fixed point 8.8)
	binary.BigEndian.PutUint16(box[32:34], 0x0100)
	// reserved
	box[34] = 0x00; box[35] = 0x00

	// matrix (identity)
	identity := []uint32{0x00010000, 0, 0, 0, 0x00010000, 0, 0, 0, 0x40000000}
	for i, v := range identity {
		binary.BigEndian.PutUint32(box[36+i*4:40+i*4], v)
	}

	// pre-defined
	for i := 72; i < 84; i++ {
		box[i] = 0x00
	}

	// next track ID
	binary.BigEndian.PutUint32(box[84:88], 1)

	return box
}

// generateTKHD 生成tkhd box
func (w *MP4Writer) generateTKHD(timescale, duration uint32) []byte {
	box := make([]byte, 80+8)

	binary.BigEndian.PutUint32(box[0:4], uint32(len(box)))
	box[4] = 0x74; box[5] = 0x6b; box[6] = 0x68; box[7] = 0x64 // "tkhd"

	box[8] = 0x00 // version
	box[9] = 0x00; box[10] = 0x00; box[11] = 0x03 // flags (track enabled)

	binary.BigEndian.PutUint32(box[12:16], 0) // creation time
	binary.BigEndian.PutUint32(box[16:20], 0) // modification time
	binary.BigEndian.PutUint32(box[20:24], 1) // track ID
	binary.BigEndian.PutUint32(box[24:28], 0) // reserved
	binary.BigEndian.PutUint32(box[28:32], duration) // duration

	binary.BigEndian.PutUint64(box[32:40], 0) // reserved
	binary.BigEndian.PutUint16(box[40:42], 0) // layer
	binary.BigEndian.PutUint16(box[42:44], 0) // alternate group
	binary.BigEndian.PutUint16(box[44:46], 0x0100) // volume (sound track)
	binary.BigEndian.PutUint16(box[46:48], 0) // reserved

	// matrix
	identity := []uint32{0x00010000, 0, 0, 0, 0x00010000, 0, 0, 0, 0x40000000}
	for i, v := range identity {
		binary.BigEndian.PutUint32(box[48+i*4:52+i*4], v)
	}

	// width/height (fixed point 16.16)
	binary.BigEndian.PutUint32(box[76:80], uint32(w.width)<<16)
	binary.BigEndian.PutUint32(box[80:84], uint32(w.height)<<16)

	return box
}

// generateMDHD 生成mdhd box
func (w *MP4Writer) generateMDHD(timescale, duration uint32) []byte {
	box := make([]byte, 20+8)

	binary.BigEndian.PutUint32(box[0:4], uint32(len(box)))
	box[4] = 0x6d; box[5] = 0x64; box[6] = 0x68; box[7] = 0x64 // "mdhd"

	box[8] = 0x00 // version
	box[9] = 0x00; box[10] = 0x00; box[11] = 0x00 // flags

	binary.BigEndian.PutUint32(box[12:16], 0) // creation time
	binary.BigEndian.PutUint32(box[16:20], 0) // modification time
	binary.BigEndian.PutUint32(box[20:24], timescale) // timescale
	binary.BigEndian.PutUint32(box[24:28], duration) // duration

	binary.BigEndian.PutUint16(box[28:30], 0x55C4) // language (und)
	binary.BigEndian.PutUint16(box[30:32], 0) // pre-defined

	return box
}

// generateMINF 生成minf box
func (w *MP4Writer) generateMINF(stbl []byte) []byte {
	box := make([]byte, 0, 8+len(stbl))
	// hdlr box (minimal)
	hdlr := []byte{0x00, 0x00, 0x00, 0x21, 0x68, 0x64, 0x6c, 0x72, // size + "hdlr"
		0x00, 0x00, 0x00, 0x00, // version + flags
		0x00, 0x00, 0x00, 0x00, // pre_defined
		0x76, 0x69, 0x64, 0x65, // "vide"
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // reserved
		0x00} // name (empty)
	box = append(box, hdlr...)
	box = append(box, stbl...)
	return box
}

// generateSTBL 生成stbl box
func (w *MP4Writer) generateSTBL(avcC []byte) []byte {
	box := make([]byte, 0)

	// stsd (sample description)
	stsd := w.generateSTSD(avcC)
	box = append(box, stsd...)

	// stts (time to sample)
	stts := w.generateSTTS()
	box = append(box, stts...)

	// stsc (sample to chunk)
	stsc := w.generateSTSC()
	box = append(box, stsc...)

	// stco (chunk offset) - 使用段落偏移
	stco := w.generateSTCO()
	box = append(box, stco...)

	// 计算stbl大小
	stblLen := 8 + len(box)
	stbl := make([]byte, 4)
	binary.BigEndian.PutUint32(stbl, uint32(stblLen))
	stbl = append(stbl, 0x73, 0x74, 0x62, 0x6c) // "stbl"
	stbl = append(stbl, box...)

	return stbl
}

// generateSTSD 生成stsd box
func (w *MP4Writer) generateSTSD(avcC []byte) []byte {
	// avc1 box
	avc1 := make([]byte, 78+len(avcC))

	binary.BigEndian.PutUint32(avc1[0:4], uint32(len(avc1))) // size
	binary.BigEndian.PutUint32(avc1[4:8], BoxTypeAVC1) // "avc1"

	// reserved
	for i := 8; i < 16; i++ {
		avc1[i] = 0x00
	}
	// data-reference-index
	binary.BigEndian.PutUint16(avc1[16:18], 1)
	// pre-defined
	for i := 18; i < 24; i++ {
		avc1[i] = 0x00
	}
	// width/height
	binary.BigEndian.PutUint16(avc1[24:26], uint16(w.width))
	binary.BigEndian.PutUint16(avc1[26:28], uint16(w.height))
	// horizresolution (72 dpi)
	binary.BigEndian.PutUint32(avc1[28:32], 0x00480000)
	// vertresolution (72 dpi)
	binary.BigEndian.PutUint32(avc1[32:36], 0x00480000)
	// reserved
	binary.BigEndian.PutUint32(avc1[36:40], 0)
	// frame_count
	binary.BigEndian.PutUint16(avc1[40:42], 1)
	// compressor name (empty)
	for i := 42; i < 54; i++ {
		avc1[i] = 0x00
	}
	// depth
	binary.BigEndian.PutUint16(avc1[54:56], 0x0018)
	// pre_defined
	binary.BigEndian.PutUint16(avc1[56:58], 0xFFFF)

	// avcC描述符
	copy(avc1[78:], avcC)

	// stsd box
	stsdLen := 8 + len(avc1)
	stsd := make([]byte, 4)
	binary.BigEndian.PutUint32(stsd, uint32(stsdLen))
	stsd = append(stsd, 0x73, 0x74, 0x73, 0x64) // "stsd"
	stsd = append(stsd, 0x00, 0x00, 0x00, 0x01) // entry_count = 1
	stsd = append(stsd, avc1...)

	return stsd
}

// generateSTTS 生成stts box
func (w *MP4Writer) generateSTTS() []byte {
	// 简化：所有sample有相同duration
	sttsData := make([]byte, 8+8) // 1 entry
	binary.BigEndian.PutUint32(sttsData[0:4], 1) // entry_count
	binary.BigEndian.PutUint32(sttsData[4:8], uint32(90000/uint32(w.framerate))) // sample_count
	binary.BigEndian.PutUint32(sttsData[8:12], 1) // sample_delta

	sttsLen := 8 + len(sttsData)
	stts := make([]byte, 4)
	binary.BigEndian.PutUint32(stts, uint32(sttsLen))
	stts = append(stts, 0x73, 0x74, 0x74, 0x73) // "stts"
	stts = append(stts, sttsData...)

	return stts
}

// generateSTSC 生成stsc box
func (w *MP4Writer) generateSTSC() []byte {
	// 简化：所有sample在同一个chunk
	stscData := make([]byte, 8+12) // 1 entry
	binary.BigEndian.PutUint32(stscData[0:4], 1) // entry_count
	binary.BigEndian.PutUint32(stscData[4:8], 1) // first_chunk
	binary.BigEndian.PutUint32(stscData[8:12], uint32(len(w.samples))) // samples_per_chunk

	stscLen := 8 + len(stscData)
	stsc := make([]byte, 4)
	binary.BigEndian.PutUint32(stsc, uint32(stscLen))
	stsc = append(stsc, 0x73, 0x74, 0x73, 0x63) // "stsc"
	stsc = append(stsc, stscData...)

	return stsc
}

// generateSTCO 生成stco box
func (w *MP4Writer) generateSTCO() []byte {
	// 简化：chunk offset从moov后面开始
	stcoData := make([]byte, 8+4) // 1 entry
	binary.BigEndian.PutUint32(stcoData[0:4], 1) // entry_count
	binary.BigEndian.PutUint32(stcoData[4:8], 0) // chunk_offset (需要在finalize时计算)

	stcoLen := 8 + len(stcoData)
	stco := make([]byte, 4)
	binary.BigEndian.PutUint32(stco, uint32(stcoLen))
	stco = append(stco, 0x73, 0x74, 0x63, 0x6f) // "stco"
	stco = append(stco, stcoData...)

	return stco
}

// generateMDIA 生成mdia box
func (w *MP4Writer) generateMDIA(mdhd, minf []byte) []byte {
	box := make([]byte, 0, 8+len(mdhd)+len(minf))
	box = append(box, mdhd...)
	box = append(box, minf...)

	mdiaLen := 8 + len(box)
	mdia := make([]byte, 4)
	binary.BigEndian.PutUint32(mdia, uint32(mdiaLen))
	mdia = append(mdia, 0x6d, 0x64, 0x69, 0x61) // "mdia"
	mdia = append(mdia, box...)

	return mdia
}

// generateTRAK 生成trak box
func (w *MP4Writer) generateTRAK(tkhd, mdia []byte) []byte {
	box := make([]byte, 0, 8+len(tkhd)+len(mdia))
	box = append(box, tkhd...)
	box = append(box, mdia...)

	trakLen := 8 + len(box)
	trak := make([]byte, 4)
	binary.BigEndian.PutUint32(trak, uint32(trakLen))
	trak = append(trak, 0x74, 0x72, 0x61, 0x6b) // "trak"
	trak = append(trak, box...)

	return trak
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
	w.frameCount = 0
	w.samples = make([]sampleEntry, 0)

	// 创建临时数据文件
	tmpFile, err := os.CreateTemp(w.dir, "mp4tmp_*.dat")
	if err != nil {
		return fmt.Errorf("create temp file failed: %w", err)
	}
	tmpFile.Close()
	os.Remove(tmpFile.Name())
	w.currentFile = tmpFile

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

	w.closed := true
	if w.currentFile != nil {
		// 完成当前分段
		w.finalizeSegmentLocked()
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

// closed 标记是否已关闭
func (w *MP4Writer) IsClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

var wMu sync.Mutex

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