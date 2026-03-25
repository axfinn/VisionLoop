package storage

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GC 存储垃圾回收
type GC struct {
	dir      string
	maxGB    float64
	checkedAt time.Time
}

// NewGC 创建GC
func NewGC(dir string, maxGB float64) *GC {
	return &GC{
		dir:   dir,
		maxGB: maxGB,
	}
}

// CheckAndCleanup 检查并清理存储
func (g *GC) CheckAndCleanup() error {
	// 限制检查频率
	if time.Since(g.checkedAt) < 30*time.Second {
		return nil
	}
	g.checkedAt = time.Now()

	// 计算当前使用量
	totalSize, err := g.calculateSize()
	if err != nil {
		return fmt.Errorf("calculate size failed: %w", err)
	}

	maxBytes := int64(g.maxGB * 1024 * 1024 * 1024)
	if totalSize < maxBytes {
		return nil
	}

	// 超过阈值，删除最旧的文件
	log.Printf("storage GC triggered: %.2fGB / %.2fGB", float64(totalSize)/1024/1024/1024, g.maxGB)
	return g.deleteOldest(totalSize - maxBytes*80/100) // 清理到80%阈值
}

func (g *GC) calculateSize() (int64, error) {
	var total int64
	entries, err := os.ReadDir(g.dir)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".mp4") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		total += info.Size()
	}

	return total, nil
}

func (g *GC) deleteOldest(targetBytes int64) error {
	var deleted int64

	entries, err := os.ReadDir(g.dir)
	if err != nil {
		return err
	}

	// 按修改时间排序
	type fileInfo struct {
		name    string
		path    string
		size    int64
	 modTime time.Time
	}

	var files []fileInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".mp4") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{
			name:    entry.Name(),
			path:    filepath.Join(g.dir, entry.Name()),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	// 删除最旧的文件直到达到目标
	for _, f := range files {
		if deleted >= targetBytes {
			break
		}
		if err := os.Remove(f.path); err != nil {
			log.Printf("delete old file failed: %v", err)
			continue
		}
		deleted += f.size
		log.Printf("deleted old clip: %s (%.2fMB)", f.name, float64(f.size)/1024/1024)
	}

	return nil
}

// SetMaxGB 设置最大存储
func (g *GC) SetMaxGB(maxGB float64) {
	g.maxGB = maxGB
}

// GetMaxGB 获取最大存储
func (g *GC) GetMaxGB() float64 {
	return g.maxGB
}

// GetUsage 获取当前使用量
func (g *GC) GetUsage() (used int64, max int64, err error) {
	used, err = g.calculateSize()
	if err != nil {
		return 0, 0, err
	}
	max = int64(g.maxGB * 1024 * 1024 * 1024)
	return used, max, nil
}
