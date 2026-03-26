import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import { createPinia } from 'pinia'
import 'video.js'  // 确保video.js被bundled
import App from './App.vue'

const routes = [
  { path: '/', name: 'live', component: () => import('./views/LiveView.vue') },
  { path: '/playback', name: 'playback', component: () => import('./views/PlaybackView.vue') },
  { path: '/events', name: 'events', component: () => import('./views/EventCenter.vue') },
  { path: '/settings', name: 'settings', component: () => import('./views/SettingsView.vue') }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

const pinia = createPinia()
const app = createApp(App)

app.use(pinia)
app.use(router)
app.mount('#app')
