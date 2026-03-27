package webrtc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	pionwebrtc "github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"visionloop/internal/encoder"
	"visionloop/internal/mp4"
)

// SignalMessage 信令消息
type SignalMessage struct {
	Type    string          `json:"type"` // offer/answer/ice-candidate
	Payload json.RawMessage `json:"payload,omitempty"`
}

// WebRTC Pion WebRTC实现
type WebRTC struct {
	mu           sync.RWMutex
	width        int
	height       int
	peerConn     *pionwebrtc.PeerConnection
	videoTrack   *pionwebrtc.TrackLocalStaticSample
	connected    bool
	signalCh     chan *SignalMessage
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewWebRTC 创建WebRTC
func NewWebRTC(width, height int) (*WebRTC, error) {
	ctx, cancel := context.WithCancel(context.Background())

	w := &WebRTC{
		width:    width,
		height:   height,
		signalCh: make(chan *SignalMessage, 10),
		ctx:      ctx,
		cancel:   cancel,
	}

	if err := w.setupPeerConnection(); err != nil {
		cancel()
		return nil, err
	}

	return w, nil
}

func (w *WebRTC) setupPeerConnection() error {
	// 创建配置
	config := pionwebrtc.Configuration{
		ICEServers: []pionwebrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// 创建PeerConnection
	pc, err := pionwebrtc.NewPeerConnection(config)
	if err != nil {
		return fmt.Errorf("create peer connection failed: %w", err)
	}
	w.peerConn = pc

	// 创建H264视频track
	track, err := pionwebrtc.NewTrackLocalStaticSample(
		pionwebrtc.RTPCodecCapability{
			MimeType:  "video/h264",
			ClockRate: 90000,
			// SDP Fmtp line for baseline profile
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		},
		"video",
		"visionloop",
	)
	if err != nil {
		return fmt.Errorf("create video track failed: %w", err)
	}
	w.videoTrack = track

	// 添加track到peer connection
	if _, err = pc.AddTrack(track); err != nil {
		return fmt.Errorf("add track failed: %w", err)
	}

	// 设置ICE候选者处理
	pc.OnICECandidate(func(candidate *pionwebrtc.ICECandidate) {
		if candidate != nil {
			w.signalCh <- &SignalMessage{
				Type:    "ice-candidate",
				Payload: mustMarshal(candidate),
			}
		}
	})

	// 设置连接状态变化
	pc.OnConnectionStateChange(func(state pionwebrtc.PeerConnectionState) {
		w.mu.Lock()
		w.connected = (state == pionwebrtc.PeerConnectionStateConnected)
		w.mu.Unlock()

		log.Printf("peer connection state: %s", state.String())
	})

	return nil
}

// WriteVideoFrame 写入视频帧
func (w *WebRTC) WriteVideoFrame(pkt interface{}) error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if !w.connected || w.videoTrack == nil {
		return nil
	}

	// 从 EncoderPacket (encoder包) 或 mp4.EncoderPacket 获取数据
	var data []byte
	var duration time.Duration
	switch ep := pkt.(type) {
	case *encoder.EncoderPacket:
		data = ep.Data
		duration = time.Second / 25 // 假设25fps
	case *mp4.EncoderPacket:
		data = ep.Data
		duration = time.Second / 25
	}
	if len(data) > 0 {
		sample := media.Sample{
			Data:     data,
			Duration: duration,
		}
		if err := w.videoTrack.WriteSample(sample); err != nil {
			return fmt.Errorf("write video frame failed: %w", err)
		}
	}
	return nil
}

// WriteRawNALU 写入原始NALU数据到WebRTC
func (w *WebRTC) WriteRawNALU(nalus [][]byte, keyFrame bool) error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if !w.connected || w.videoTrack == nil {
		return nil
	}

	duration := time.Second / 25
	for _, nalu := range nalus {
		if len(nalu) > 0 {
			sample := media.Sample{
				Data:     nalu,
				Duration: duration,
			}
			if err := w.videoTrack.WriteSample(sample); err != nil {
				return fmt.Errorf("write NALU failed: %w", err)
			}
		}
	}
	return nil
}

// HandleSignal 处理信令
func (w *WebRTC) HandleSignal(msg *SignalMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.peerConn == nil {
		return fmt.Errorf("peer connection not initialized")
	}

	switch msg.Type {
	case "offer":
		// 解析 offer payload: { type: 'offer', sdp: '...' }
		var offerPayload struct {
			Type string `json:"type"`
			SDP  string `json:"sdp"`
		}
		if err := json.Unmarshal(msg.Payload, &offerPayload); err != nil {
			// 兜底: 直接当SDP字符串处理（旧格式兼容）
			offerPayload.SDP = string(msg.Payload)
		}
		if err := w.peerConn.SetRemoteDescription(pionwebrtc.SessionDescription{
			Type: pionwebrtc.SDPTypeOffer,
			SDP:  offerPayload.SDP,
		}); err != nil {
			return err
		}

		// 创建answer
		answer, err := w.peerConn.CreateAnswer(nil)
		if err != nil {
			return err
		}
		if err := w.peerConn.SetLocalDescription(answer); err != nil {
			return err
		}

		// 发送answer
		w.signalCh <- &SignalMessage{
			Type:    "answer",
			Payload: mustMarshal(answer),
		}

	case "answer":
		// answer payload: { type: 'answer', sdp: '...' }
		var answerPayload struct {
			Type string `json:"type"`
			SDP  string `json:"sdp"`
		}
		if err := json.Unmarshal(msg.Payload, &answerPayload); err != nil {
			answerPayload.SDP = string(msg.Payload)
		}
		return w.peerConn.SetRemoteDescription(pionwebrtc.SessionDescription{
			Type: pionwebrtc.SDPTypeAnswer,
			SDP:  answerPayload.SDP,
		})

	case "ice-candidate":
		var candidate pionwebrtc.ICECandidateInit
		if err := json.Unmarshal(msg.Payload, &candidate); err != nil {
			return err
		}
		return w.peerConn.AddICECandidate(candidate)
	}

	return nil
}

// GetSignalCh 获取信令通道
func (w *WebRTC) GetSignalCh() <-chan *SignalMessage {
	return w.signalCh
}

// Close 关闭
func (w *WebRTC) Close() error {
	w.cancel()
	// videoTrack is closed automatically when peerConn is closed
	if w.peerConn != nil {
		return w.peerConn.Close()
	}
	return nil
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
