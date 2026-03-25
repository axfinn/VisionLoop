package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"visionloop/internal/mp4"
	"visionloop/internal/storage"
	"visionloop/internal/webrtc"
)

// ServerConfig 服务配置
type ServerConfig struct {
	ClipsDir     string
	EventsDir    string
	Screenshots  string
	MaxStorageGB float64
	WebRTC       *webrtc.WebRTC
	Version      string
}

// Server HTTP服务器
type Server struct {
	config ServerConfig
	router *gin.Engine
	gc     *storage.GC
}

// NewServer 创建服务器
func NewServer(cfg ServerConfig) *Server {
	gin.SetMode(gin.ReleaseMode)

	s := &Server{
		config: cfg,
		gc:     storage.NewGC(cfg.ClipsDir, cfg.MaxStorageGB),
	}

	s.setupRouter()
	return s
}

func (s *Server) setupRouter() {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	r.Use(loggerMiddleware())

	// 静态文件
	r.Static("/static", "./web/dist")
	r.GET("/", s.serveIndex)

	// API
	api := r.Group("/api")
	{
		// clips
		api.GET("/clips", s.handleListClips)
		api.GET("/clips/*name", s.handleGetClip)
		api.HEAD("/clips/*name", s.handleGetClip)

		// events
		api.GET("/events", s.handleListEvents)
		api.POST("/events", s.handleCreateEvent)

		// storage
		api.GET("/storage", s.handleGetStorage)

		// webrtc signal
		api.GET("/ws/signal", s.handleWebRTCSignal)

		// settings
		api.GET("/settings", s.handleGetSettings)
		api.POST("/settings", s.handleUpdateSettings)
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "version": s.config.Version})
	})

	s.router = r
}

// Router 获取路由
func (s *Server) Router() *gin.Engine {
	return s.router
}

// handleListClips 列出clips
func (s *Server) handleListClips(c *gin.Context) {
	files, err := mp4.ListFiles(s.config.ClipsDir)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	result := make([]gin.H, len(files))
	for i, f := range files {
		result[i] = gin.H{
			"name":      f.Name,
			"size":      f.Size,
			"duration":  f.Duration.Seconds(),
			"created":   f.CreatedAt.Unix(),
			"createdAt": f.CreatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(200, gin.H{"clips": result})
}

// handleGetClip 获取clip (Range支持)
func (s *Server) handleGetClip(c *gin.Context) {
	name := c.Param("name")
	if name == "" || name == "/" {
		c.JSON(400, gin.H{"error": "name required"})
		return
	}

	name = strings.TrimPrefix(name, "/")
	path := filepath.Join(s.config.ClipsDir, name)

	// 安全检查
	if !strings.HasPrefix(filepath.Clean(path), s.config.ClipsDir) {
		c.JSON(403, gin.H{"error": "forbidden"})
		return
	}

	file, err := os.Open(path)
	if err != nil {
		c.JSON(404, gin.H{"error": "file not found"})
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Range请求
	rangeHeader := c.GetHeader("Range")
	if rangeHeader == "" {
		// 完整文件
		c.Header("Content-Type", "video/mp4")
		c.Header("Content-Length", strconv.FormatInt(stat.Size(), 10))
		c.Header("Accept-Ranges", "bytes")
		io.Copy(c.Writer, file)
		return
	}

	// 解析Range
	rangeStart, rangeEnd, err := parseRange(rangeHeader, stat.Size())
	if err != nil {
		c.JSON(416, gin.H{"error": "range not satisfiable"})
		return
	}

	// 发送部分内容
	c.Header("Content-Type", "video/mp4")
	c.Header("Content-Length", strconv.FormatInt(rangeEnd-rangeStart+1, 10))
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rangeStart, rangeEnd, stat.Size()))
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "no-cache")

	file.Seek(rangeStart, 0)
	io.CopyN(c.Writer, file, rangeEnd-rangeStart+1)
}

// parseRange 解析Range头
func parseRange(header string, size int64) (start, end int64, err error) {
	header = strings.TrimPrefix(header, "bytes=")
	parts := strings.Split(header, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range")
	}

	start, _ = strconv.ParseInt(parts[0], 10, 64)
	end, _ = strconv.ParseInt(parts[1], 10, 64)

	if end == 0 {
		end = size - 1
	}
	if start > end {
		return 0, 0, fmt.Errorf("invalid range")
	}
	return start, end, nil
}

// Event 事件
type Event struct {
	ID          int64   `json:"id"`
	Type        string  `json:"type"` // fall/cry/noise/intruder
	Timestamp   int64   `json:"timestamp"`
	ClipName    string  `json:"clip_name,omitempty"`
	ClipOffset  float64 `json:"clip_offset,omitempty"`
	Screenshot  string  `json:"screenshot,omitempty"`
	Confidence  float64 `json:"confidence,omitempty"`
	CreatedAt   int64   `json:"created_at"`
}

// handleListEvents 列出事件
func (s *Server) handleListEvents(c *gin.Context) {
	entries, err := os.ReadDir(s.config.EventsDir)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	var events []Event
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.config.EventsDir, entry.Name()))
		if err != nil {
			continue
		}

		var event Event
		if err := json.Unmarshal(data, &event); err == nil {
			events = append(events, event)
		}
	}

	c.JSON(200, gin.H{"events": events})
}

// handleCreateEvent 创建事件
func (s *Server) handleCreateEvent(c *gin.Context) {
	var event Event
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	event.ID = time.Now().UnixNano()
	event.CreatedAt = time.Now().Unix()

	// 保存到events目录
	data, _ := json.Marshal(event)
	filename := fmt.Sprintf("%d.json", event.ID)
	os.WriteFile(filepath.Join(s.config.EventsDir, filename), data, 0644)

	c.JSON(200, gin.H{"event": event})
}

// handleGetStorage 获取存储信息
func (s *Server) handleGetStorage(c *gin.Context) {
	used, max, err := s.gc.GetUsage()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"used":        used,
		"max":         max,
		"usedGB":      float64(used) / 1024 / 1024 / 1024,
		"maxGB":       float64(max) / 1024 / 1024 / 1024,
		"usedPercent": float64(used) / float64(max) * 100,
	})
}

// handleWebRTCSignal WebRTC信令
func (s *Server) handleWebRTCSignal(c *gin.Context) {
	if s.config.WebRTC == nil {
		c.JSON(503, gin.H{"error": "webrtc not available"})
		return
	}

	// 升级为WebSocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	webrtc := s.config.WebRTC
	done := make(chan struct{})

	// 读取客户端消息并转发到webrtc
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				close(done)
				return
			}

			var msg webrtc.SignalMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			if err := webrtc.HandleSignal(&msg); err != nil {
				log.Printf("handle signal error: %v", err)
			}
		}
	}()

	// 读取webrtc信号并转发到客户端
	for {
		select {
		case msg, ok := <-webrtc.GetSignalCh():
			if !ok {
				return
			}
			if err := conn.WriteJSON(msg); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

// handleGetSettings 获取设置
func (s *Server) handleGetSettings(c *gin.Context) {
	c.JSON(200, gin.H{
		"maxStorageGB":   s.gc.GetMaxGB(),
		"segmentMin":      5,
		"detectFall":      true,
		"detectCry":        true,
		"detectNoise":      true,
		"detectIntruder":   true,
		"sensitivity":      0.7,
	})
}

// handleUpdateSettings 更新设置
func (s *Server) handleUpdateSettings(c *gin.Context) {
	var settings map[string]interface{}
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if maxGB, ok := settings["maxStorageGB"].(float64); ok {
		s.gc.SetMaxGB(maxGB)
	}

	c.JSON(200, gin.H{"status": "ok"})
}

func (s *Server) serveIndex(c *gin.Context) {
	indexPath := "./web/dist/index.html"
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		c.Data(200, "text/html", []byte(`<!DOCTYPE html>
<html>
<head><title>VisionLoop</title></head>
<body>
<h1>VisionLoop v1.0.0</h1>
<p>Frontend not built. Run: cd web && npm install && npm run build</p>
</body>
</html>`))
		return
	}
	c.File(indexPath)
}

// 中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Range")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("%s %s %d %v", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start))
	}
}
