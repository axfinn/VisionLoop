<template>
  <div class="settings-view">
    <div class="card">
      <h2>存储设置</h2>
      <div class="setting-item">
        <label>最大存储空间</label>
        <div class="slider-group">
          <input type="range" min="1" max="100" v-model.number="settings.maxStorageGB" />
          <span>{{ settings.maxStorageGB }} GB</span>
        </div>
        <div class="storage-bar">
          <div class="storage-used" :style="{ width: storageInfo.usedPercent + '%' }"></div>
        </div>
        <div class="storage-text">
          已使用 {{ storageInfo.usedGB.toFixed(2) }} GB / {{ storageInfo.maxGB.toFixed(2) }} GB
        </div>
      </div>

      <div class="setting-item">
        <label>录像分段时长</label>
        <select v-model.number="settings.segmentMin">
          <option :value="1">1 分钟</option>
          <option :value="5">5 分钟</option>
          <option :value="10">10 分钟</option>
          <option :value="30">30 分钟</option>
        </select>
      </div>
    </div>

    <div class="card">
      <h2>检测设置</h2>
      <div class="setting-item">
        <label>事件检测</label>
        <div class="toggle-group">
          <label class="toggle">
            <input type="checkbox" v-model="settings.detectFall" />
            <span>摔倒检测</span>
          </label>
          <label class="toggle">
            <input type="checkbox" v-model="settings.detectCry" />
            <span>哭声检测</span>
          </label>
          <label class="toggle">
            <input type="checkbox" v-model="settings.detectNoise" />
            <span>异响检测</span>
          </label>
          <label class="toggle">
            <input type="checkbox" v-model="settings.detectIntruder" />
            <span>陌生人闯入</span>
          </label>
        </div>
      </div>

      <div class="setting-item">
        <label>检测灵敏度</label>
        <div class="slider-group">
          <input type="range" min="0.1" max="1.0" step="0.1" v-model.number="settings.sensitivity" />
          <span>{{ (settings.sensitivity * 100).toFixed(0) }}%</span>
        </div>
      </div>
    </div>

    <div class="card">
      <h2>系统信息</h2>
      <div class="info-grid">
        <div class="info-item">
          <span class="info-label">版本</span>
          <span class="info-value">{{ systemInfo.version }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">运行时间</span>
          <span class="info-value">{{ systemInfo.uptime }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">编码帧数</span>
          <span class="info-value">{{ systemInfo.frameCount }}</span>
        </div>
      </div>
    </div>

    <div class="actions">
      <button class="btn" @click="saveSettings">保存设置</button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import axios from 'axios'

const settings = ref({
  maxStorageGB: 50,
  segmentMin: 5,
  detectFall: true,
  detectCry: true,
  detectNoise: true,
  detectIntruder: true,
  sensitivity: 0.7
})

const storageInfo = ref({
  used: 0,
  max: 0,
  usedGB: 0,
  maxGB: 0,
  usedPercent: 0
})

const systemInfo = ref({
  version: '1.0.0',
  uptime: '0:00:00',
  frameCount: 0
})

onMounted(async () => {
  await loadSettings()
  await loadStorageInfo()
  await loadSystemInfo()
})

async function loadSettings() {
  try {
    const res = await axios.get('/api/settings')
    if (res.data) {
      Object.assign(settings.value, res.data)
    }
  } catch (err) {
    console.error('load settings error:', err)
  }
}

async function loadStorageInfo() {
  try {
    const res = await axios.get('/api/storage')
    storageInfo.value = res.data
  } catch (err) {
    console.error('load storage error:', err)
  }
}

async function loadSystemInfo() {
  try {
    const res = await axios.get('/health')
    systemInfo.value.version = res.data.version || '1.0.0'
  } catch (err) {
    console.error('load system info error:', err)
  }
}

async function saveSettings() {
  try {
    await axios.post('/api/settings', settings.value)
    alert('设置已保存')
  } catch (err) {
    console.error('save settings error:', err)
    alert('保存失败')
  }
}
</script>

<style scoped>
.settings-view { max-width: 800px; margin: 0 auto; }
.setting-item { margin-bottom: 1.5rem; }
.setting-item label { display: block; font-weight: 600; margin-bottom: 0.5rem; }
.slider-group { display: flex; align-items: center; gap: 1rem; }
.slider-group input[type="range"] { flex: 1; }
.toggle-group { display: flex; flex-wrap: wrap; gap: 1rem; }
.toggle { display: flex; align-items: center; gap: 0.5rem; cursor: pointer; }
.toggle input { width: 18px; height: 18px; }
.storage-bar { height: 8px; background: #333; border-radius: 4px; overflow: hidden; margin-top: 0.5rem; }
.storage-used { height: 100%; background: #00d4ff; transition: width 0.3s; }
.storage-text { font-size: 0.85rem; color: #888; margin-top: 0.25rem; }
select { padding: 0.5rem 1rem; min-width: 150px; }
.info-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 1rem; }
.info-item { background: #2a2a2a; padding: 1rem; border-radius: 4px; text-align: center; }
.info-label { display: block; color: #888; font-size: 0.85rem; margin-bottom: 0.25rem; }
.info-value { font-size: 1.2rem; font-weight: 600; }
.actions { margin-top: 1.5rem; }
.actions .btn { min-width: 120px; }
</style>
