<template>
  <div class="live-view">
    <div class="card">
      <h2>实时监看</h2>
      <div class="video-container">
        <video ref="videoEl" class="video-js vjs-default-skin" autoplay muted playsinline></video>
        <div v-if="!connected" class="loading">正在连接WebRTC...</div>
        <!-- 事件气泡 -->
        <transition-group name="fade">
          <div
            v-for="event in recentEvents"
            :key="event.id"
            class="event-bubble"
            :class="'event-' + event.type"
          >
            {{ eventText(event.type) }}
          </div>
        </transition-group>
      </div>
    </div>

    <div class="card">
      <h2>实时事件</h2>
      <div v-if="recentEvents.length === 0" class="empty">暂无事件</div>
      <div v-else class="event-list">
        <div v-for="event in recentEvents" :key="event.id" class="event-item">
          <span class="event-badge" :class="'event-' + event.type">{{ eventText(event.type) }}</span>
          <span class="event-time">{{ formatTime(event.timestamp) }}</span>
          <img v-if="event.screenshot" :src="'/api/screenshots/' + event.screenshot" class="event-thumb" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import axios from 'axios'

const videoEl = ref(null)
const connected = ref(false)
const recentEvents = ref([])
let pc = null
let ws = null

const eventText = (type) => {
  const map = { fall: '摔倒', cry: '哭声', noise: '异响', intruder: '陌生人' }
  return map[type] || type
}

const formatTime = (ts) => {
  return new Date(ts).toLocaleTimeString()
}

onMounted(async () => {
  await connectWebRTC()
  await loadRecentEvents()

  // 定时刷新事件
  setInterval(loadRecentEvents, 5000)
})

onUnmounted(() => {
  if (pc) pc.close()
  if (ws) ws.close()
})

async function connectWebRTC() {
  try {
    const config = {
      iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
    }
    pc = new RTCPeerConnection(config)

    pc.ontrack = (e) => {
      if (videoEl.value) {
        videoEl.value.srcObject = e.streams[0]
      }
    }

    pc.onicecandidate = (e) => {
      if (e.candidate && ws) {
        ws.send(JSON.stringify({ type: 'ice-candidate', payload: e.candidate }))
      }
    }

    pc.onconnectionstatechange = () => {
      connected.value = pc.connectionState === 'connected'
    }

    // 发送offer
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)

    // WebSocket信令
    ws = new WebSocket(`ws://${location.host}/api/ws/signal`)
    ws.onopen = () => {
      ws.send(JSON.stringify({ type: 'offer', payload: { type: 'offer', sdp: pc.localDescription.sdp } }))
    }

    ws.onmessage = async (e) => {
      const msg = JSON.parse(e.data)
      if (msg.type === 'answer') {
        const desc = typeof msg.payload === 'string' ? JSON.parse(msg.payload) : msg.payload
        await pc.setRemoteDescription(desc)
      } else if (msg.type === 'ice-candidate') {
        const candidate = typeof msg.payload === 'string' ? JSON.parse(msg.payload) : msg.payload
        await pc.addIceCandidate(candidate)
      }
    }

  } catch (err) {
    console.error('WebRTC error:', err)
  }
}

async function loadRecentEvents() {
  try {
    const res = await axios.get('/api/events')
    recentEvents.value = (res.data.events || []).slice(-10).reverse()
  } catch (err) {
    console.error('load events error:', err)
  }
}
</script>

<style scoped>
.live-view { max-width: 1200px; margin: 0 auto; }
.video-container { position: relative; background: #000; border-radius: 8px; overflow: hidden; min-height: 400px; }
.video-js { width: 100%; height: 400px; }
.event-bubble {
  position: absolute; top: 20px; right: 20px; padding: 0.5rem 1rem;
  border-radius: 20px; font-weight: bold; animation: slideIn 0.3s ease;
}
.event-fall { background: #ff6b6b; }
.event-cry { background: #ffd93d; color: #000; }
.event-noise { background: #6bcb77; }
.event-intruder { background: #9b59b6; }
@keyframes slideIn { from { transform: translateX(100px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
.fade-enter-active, .fade-leave-active { transition: all 0.3s; }
.fade-enter-from, .fade-leave-to { opacity: 0; transform: translateY(-20px); }
.event-list { display: flex; flex-direction: column; gap: 0.5rem; }
.event-item { display: flex; align-items: center; gap: 1rem; padding: 0.5rem; background: #2a2a2a; border-radius: 4px; }
.event-time { color: #888; font-size: 0.85rem; }
.event-thumb { width: 60px; height: 40px; object-fit: cover; border-radius: 4px; }
.loading { position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); color: #888; }
</style>
