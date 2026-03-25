<template>
  <div class="playback-view">
    <div class="card">
      <h2>录像列表</h2>
      <div v-if="loading" class="loading">加载中...</div>
      <div v-else-if="clips.length === 0" class="empty">暂无录像</div>
      <div v-else class="clips-grid">
        <div
          v-for="clip in clips"
          :key="clip.name"
          class="clip-card"
          :class="{ active: selectedClip === clip.name }"
          @click="selectClip(clip)"
        >
          <div class="clip-thumb">
            <video-icon />
          </div>
          <div class="clip-info">
            <div class="clip-name">{{ clip.name }}</div>
            <div class="clip-meta">{{ formatSize(clip.size) }} · {{ formatDuration(clip.duration) }}</div>
            <div class="clip-time">{{ formatDate(clip.created) }}</div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="selectedClip" class="card">
      <h2>回放: {{ selectedClip }}</h2>
      <div class="player-container">
        <video ref="playerEl" class="video-js vjs-default-skin" controls playsinline></video>
        <!-- 事件标注 -->
        <div class="event-markers">
          <div
            v-for="event in clipEvents"
            :key="event.id"
            class="event-marker"
            :class="'event-' + event.type"
            :style="{ left: (event.clip_offset / duration * 100) + '%' }"
            :title="eventText(event.type)"
          />
        </div>
      </div>
      <div class="event-timeline">
        <div v-for="event in clipEvents" :key="event.id" class="timeline-event" @click="seekTo(event.clip_offset)">
          <span class="event-badge" :class="'event-' + event.type">{{ eventText(event.type) }}</span>
          <span class="event-time">{{ formatTime(event.timestamp) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import axios from 'axios'

const loading = ref(true)
const clips = ref([])
const selectedClip = ref(null)
const clipEvents = ref([])
const duration = ref(0)
const playerEl = ref(null)
let videojsPlayer = null

onMounted(async () => {
  await loadClips()
})

onUnmounted(() => {
  if (videojsPlayer) videojsPlayer.dispose()
})

watch(selectedClip, async (name) => {
  if (name) {
    await loadClipEvents(name)
    initPlayer(name)
  }
})

async function loadClips() {
  loading.value = true
  try {
    const res = await axios.get('/api/clips')
    clips.value = res.data.clips || []
  } catch (err) {
    console.error('load clips error:', err)
    clips.value = []
  }
  loading.value = false
}

function selectClip(clip) {
  selectedClip.value = clip.name
}

async function loadClipEvents(clipName) {
  try {
    const res = await axios.get('/api/events')
    clipEvents.value = (res.data.events || []).filter(e => e.clip_name === clipName)
  } catch (err) {
    clipEvents.value = []
  }
}

function initPlayer(clipName) {
  if (videojsPlayer) {
    videojsPlayer.dispose()
    videojsPlayer = null
  }

  setTimeout(() => {
    if (!playerEl.value) return
    videojsPlayer = videojs.createPlayer(playerEl.value, {
      controls: true,
      autoplay: false,
      preload: 'auto',
      sources: [{ src: `/api/clips/${clipName}`, type: 'video/mp4' }]
    })
    videojsPlayer.on('loadedmetadata', () => {
      duration.value = videojsPlayer.duration()
    })
    videojsPlayer.on('error', () => {
      console.error('video error')
    })
    videojsPlayer.ready(() => {
      videojsPlayer.play()
    })
  }, 100)
}

function seekTo(offset) {
  if (videojsPlayer) {
    videojsPlayer.currentTime(offset)
  }
}

function eventText(type) {
  const map = { fall: '摔倒', cry: '哭声', noise: '异响', intruder: '陌生人' }
  return map[type] || type
}

function formatSize(bytes) {
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}

function formatDuration(seconds) {
  if (!seconds) return '00:00'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  return h > 0 ? `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}` : `${m}:${s.toString().padStart(2, '0')}`
}

function formatDate(unix) {
  return new Date(unix * 1000).toLocaleString()
}

function formatTime(unix) {
  return new Date(unix).toLocaleTimeString()
}
</script>

<style scoped>
.playback-view { max-width: 1200px; margin: 0 auto; }
.clips-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1rem; }
.clip-card { background: #2a2a2a; border-radius: 8px; padding: 1rem; cursor: pointer; transition: all 0.2s; border: 2px solid transparent; }
.clip-card:hover, .clip-card.active { border-color: #00d4ff; }
.clip-thumb { background: #1a1a1a; border-radius: 4px; height: 100px; display: flex; align-items: center; justify-content: center; margin-bottom: 0.75rem; color: #666; }
.clip-name { font-weight: 600; margin-bottom: 0.25rem; }
.clip-meta { font-size: 0.85rem; color: #888; }
.clip-time { font-size: 0.75rem; color: #666; margin-top: 0.25rem; }
.player-container { position: relative; background: #000; border-radius: 8px; overflow: hidden; }
.video-js { width: 100%; height: 400px; }
.event-markers { position: absolute; bottom: 40px; left: 0; right: 0; height: 20px; }
.event-marker { position: absolute; width: 8px; height: 16px; border-radius: 2px; cursor: pointer; transform: translateX(-50%); }
.event-fall { background: #ff6b6b; }
.event-cry { background: #ffd93d; }
.event-noise { background: #6bcb77; }
.event-intruder { background: #9b59b6; }
.event-timeline { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 1rem; }
.timeline-event { display: flex; align-items: center; gap: 0.5rem; padding: 0.25rem 0.5rem; background: #2a2a2a; border-radius: 4px; cursor: pointer; }
.timeline-event:hover { background: #3a3a3a; }
.loading, .empty { text-align: center; padding: 2rem; color: #666; }
</style>
