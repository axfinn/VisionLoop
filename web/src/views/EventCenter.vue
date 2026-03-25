<template>
  <div class="event-center">
    <div class="card">
      <h2>事件中心</h2>
      <div class="filters">
        <button
          v-for="type in eventTypes"
          :key="type.value"
          class="filter-btn"
          :class="{ active: filters.includes(type.value) }"
          @click="toggleFilter(type.value)"
        >
          <span class="event-badge" :class="'event-' + type.value">{{ type.label }}</span>
        </button>
      </div>
    </div>

    <div v-if="loading" class="loading">加载中...</div>
    <div v-else-if="filteredEvents.length === 0" class="empty">暂无事件</div>
    <div v-else class="events-grid">
      <div v-for="event in filteredEvents" :key="event.id" class="event-card" @click="playEvent(event)">
        <div class="event-screenshot">
          <img v-if="event.screenshot" :src="'/api/screenshots/' + event.screenshot" />
          <div v-else class="no-image">无截图</div>
        </div>
        <div class="event-details">
          <span class="event-badge" :class="'event-' + event.type">{{ eventText(event.type) }}</span>
          <div class="event-time">{{ formatDateTime(event.timestamp) }}</div>
          <div v-if="event.clip_name" class="event-clip">
            <span class="clip-link" @click.stop="goToPlayback(event.clip_name, event.clip_offset)">
              {{ event.clip_name }}
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'

const router = useRouter()
const loading = ref(true)
const events = ref([])
const filters = ref([])

const eventTypes = [
  { value: 'fall', label: '摔倒' },
  { value: 'cry', label: '哭声' },
  { value: 'noise', label: '异响' },
  { value: 'intruder', label: '陌生人' }
]

const filteredEvents = computed(() => {
  if (filters.value.length === 0) return events.value
  return events.value.filter(e => filters.value.includes(e.type))
})

onMounted(async () => {
  await loadEvents()
})

async function loadEvents() {
  loading.value = true
  try {
    const res = await axios.get('/api/events')
    events.value = (res.data.events || []).sort((a, b) => b.timestamp - a.timestamp)
  } catch (err) {
    console.error('load events error:', err)
    events.value = []
  }
  loading.value = false
}

function toggleFilter(type) {
  const idx = filters.value.indexOf(type)
  if (idx >= 0) {
    filters.value.splice(idx, 1)
  } else {
    filters.value.push(type)
  }
}

function eventText(type) {
  const map = { fall: '摔倒', cry: '哭声', noise: '异响', intruder: '陌生人' }
  return map[type] || type
}

function formatDateTime(ts) {
  return new Date(ts).toLocaleString()
}

function playEvent(event) {
  if (event.clip_name) {
    router.push({ name: 'playback', query: { clip: event.clip_name, offset: event.clip_offset } })
  }
}

function goToPlayback(clipName, offset) {
  router.push({ name: 'playback', query: { clip: clipName, offset: offset } })
}
</script>

<style scoped>
.event-center { max-width: 1200px; margin: 0 auto; }
.filters { display: flex; gap: 0.5rem; flex-wrap: wrap; }
.filter-btn { background: none; border: 2px solid #333; padding: 0.25rem; border-radius: 4px; cursor: pointer; transition: all 0.2s; }
.filter-btn.active { border-color: #00d4ff; }
.events-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1rem; }
.event-card { background: #1a1a1a; border-radius: 8px; overflow: hidden; cursor: pointer; transition: all 0.2s; border: 1px solid #333; }
.event-card:hover { border-color: #00d4ff; transform: translateY(-2px); }
.event-screenshot { height: 160px; background: #0f0f0f; display: flex; align-items: center; justify-content: center; }
.event-screenshot img { width: 100%; height: 100%; object-fit: cover; }
.no-image { color: #666; }
.event-details { padding: 1rem; }
.event-badge { display: inline-block; padding: 0.25rem 0.5rem; border-radius: 4px; font-size: 0.75rem; font-weight: 600; }
.event-fall { background: #ff6b6b; }
.event-cry { background: #ffd93d; color: #000; }
.event-noise { background: #6bcb77; }
.event-intruder { background: #9b59b6; }
.event-time { color: #888; font-size: 0.85rem; margin-top: 0.5rem; }
.event-clip { margin-top: 0.5rem; }
.clip-link { color: #00d4ff; font-size: 0.85rem; }
.loading, .empty { text-align: center; padding: 3rem; color: #666; }
</style>
