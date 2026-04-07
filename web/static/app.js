// ── Tab 切换 ──────────────────────────────────────────────
document.querySelectorAll('.tab-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(s => s.classList.remove('active'));
    btn.classList.add('active');
    document.getElementById(`tab-${btn.dataset.tab}`).classList.add('active');
  });
});

// ── 实时视频流 (WebSocket) ────────────────────────────────
const canvas = document.getElementById('live-canvas');
const ctx = canvas.getContext('2d');
const noSignal = document.getElementById('no-signal');
const connStatus = document.getElementById('conn-status');
let ws = null;

function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  ws = new WebSocket(`${proto}://${location.host}/ws/stream`);
  ws.binaryType = 'arraybuffer';

  ws.onopen = () => {
    connStatus.textContent = '● 已连接';
    connStatus.className = 'connected';
    // 心跳
    setInterval(() => ws.readyState === WebSocket.OPEN && ws.send('ping'), 10000);
  };

  ws.onmessage = (e) => {
    if (typeof e.data === 'string') {
      // JSON 事件
      try { handleEvent(JSON.parse(e.data)); } catch {}
      return;
    }
    // 二进制 JPEG 帧
    noSignal.style.display = 'none';
    const blob = new Blob([e.data], { type: 'image/jpeg' });
    const url = URL.createObjectURL(blob);
    const img = new Image();
    img.onload = () => {
      canvas.width = img.width;
      canvas.height = img.height;
      ctx.drawImage(img, 0, 0);
      URL.revokeObjectURL(url);
    };
    img.src = url;
  };

  ws.onclose = () => {
    connStatus.textContent = '● 未连接';
    connStatus.className = 'disconnected';
    setTimeout(connectWS, 3000);
  };
}
connectWS();

// ── 实时事件面板 ──────────────────────────────────────────
const eventList = document.getElementById('event-list');
const TYPE_LABELS = {
  motion: '运动', intrusion: '⚠️ 入侵', face: '人脸', stranger: '🚨 陌生人', object: '物体'
};

function handleEvent(ev) {
  const li = document.createElement('li');
  li.className = `ev-${ev.type}`;
  const time = new Date(ev.timestamp || Date.now()).toLocaleTimeString('zh-CN');
  li.innerHTML = `<div class="ev-time">${time}</div>
    <div class="ev-label">${TYPE_LABELS[ev.type] || ev.type}: ${ev.label}</div>
    <div style="color:#8b949e;font-size:.72rem">置信度 ${(ev.confidence * 100).toFixed(0)}%</div>`;
  eventList.prepend(li);
  if (eventList.children.length > 100) eventList.lastChild.remove();
}

document.getElementById('clear-events').addEventListener('click', () => {
  eventList.innerHTML = '';
});

// ── 状态轮询 ─────────────────────────────────────────────
async function pollStatus() {
  try {
    const r = await fetch('/api/status');
    const d = await r.json();
    document.getElementById('fps-display').textContent = `FPS: ${d.fps}`;
    document.getElementById('faces-count').textContent = `已注册: ${d.known_faces} 人`;
  } catch {}
}
setInterval(pollStatus, 2000);
pollStatus();

// ── 录像回放 ─────────────────────────────────────────────
const recordingList = document.getElementById('recording-list');
const playbackVideo = document.getElementById('playback-video');
const playbackPlaceholder = document.getElementById('playback-placeholder');
const playbackControls = document.getElementById('playback-controls');
const playingName = document.getElementById('playing-name');
let currentRecording = null;

async function loadRecordings() {
  const r = await fetch('/api/recordings');
  const data = await r.json();
  recordingList.innerHTML = '';
  if (!data.length) {
    recordingList.innerHTML = '<li style="color:#8b949e;font-size:.8rem">暂无录像</li>';
    return;
  }
  data.forEach(rec => {
    const li = document.createElement('li');
    li.innerHTML = `<div class="rec-name">${rec.filename}${rec.locked ? ' 🔒' : ''}</div>
      <div class="rec-meta">${rec.size_mb} MB · ${rec.created.slice(0,16).replace('T',' ')}</div>`;
    li.addEventListener('click', () => startPlayback(rec, li));
    recordingList.appendChild(li);
  });
}

function startPlayback(rec, li) {
  document.querySelectorAll('#recording-list li').forEach(el => el.classList.remove('active'));
  li.classList.add('active');
  currentRecording = rec;
  playbackVideo.src = `/api/recordings/${rec.filename}`;
  playbackVideo.style.display = 'block';
  playbackPlaceholder.style.display = 'none';
  playbackControls.style.display = 'flex';
  playingName.textContent = rec.filename;
  playbackVideo.play();
}

document.getElementById('stop-playback').addEventListener('click', () => {
  playbackVideo.pause();
  playbackVideo.src = '';
  playbackVideo.style.display = 'none';
  playbackPlaceholder.style.display = 'block';
  playbackControls.style.display = 'none';
  currentRecording = null;
  document.querySelectorAll('#recording-list li').forEach(el => el.classList.remove('active'));
});

document.getElementById('refresh-recordings').addEventListener('click', loadRecordings);
loadRecordings();

// ── 人脸管理 ─────────────────────────────────────────────
const enrollForm = document.getElementById('enroll-form');
const enrollMsg = document.getElementById('enroll-msg');
const facesList = document.getElementById('faces-list');

async function loadFaces() {
  const r = await fetch('/api/faces');
  const d = await r.json();
  facesList.innerHTML = '';
  d.faces.forEach(name => {
    const li = document.createElement('li');
    li.innerHTML = `<span>👤 ${name}</span>
      <button data-name="${name}">删除</button>`;
    li.querySelector('button').addEventListener('click', async () => {
      await fetch(`/api/faces/${name}`, { method: 'DELETE' });
      loadFaces();
    });
    facesList.appendChild(li);
  });
}

enrollForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  const name = document.getElementById('face-name').value.trim();
  const file = document.getElementById('face-file').files[0];
  if (!name || !file) return;
  const fd = new FormData();
  fd.append('name', name);
  fd.append('file', file);
  try {
    const r = await fetch('/api/faces', { method: 'POST', body: fd });
    const d = await r.json();
    enrollMsg.className = r.ok ? 'msg-ok' : 'msg-err';
    enrollMsg.textContent = d.message || d.detail;
    if (r.ok) { enrollForm.reset(); loadFaces(); pollStatus(); }
  } catch (err) {
    enrollMsg.className = 'msg-err';
    enrollMsg.textContent = '注册失败';
  }
});

loadFaces();

// ── 事件历史 ─────────────────────────────────────────────
const eventsTbody = document.getElementById('events-tbody');
const filterType = document.getElementById('filter-type');
const filterSince = document.getElementById('filter-since');
const filterUntil = document.getElementById('filter-until');
const pageInfo = document.getElementById('page-info');
const pageTotal = document.getElementById('page-total');
const pagePrev = document.getElementById('page-prev');
const pageNext = document.getElementById('page-next');
const PAGE_SIZE = 20;
let eventsPage = 0;

function toLocalDatetimeInput(d) {
  const pad = n => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

document.querySelectorAll('.range-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.range-btn').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    const now = new Date();
    const range = btn.dataset.range;
    if (range === 'all') {
      filterSince.value = '';
      filterUntil.value = '';
    } else if (range === '30m') {
      filterSince.value = toLocalDatetimeInput(new Date(now - 30 * 60000));
      filterUntil.value = '';
    } else if (range === '1h') {
      filterSince.value = toLocalDatetimeInput(new Date(now - 60 * 60000));
      filterUntil.value = '';
    } else if (range === 'today') {
      const start = new Date(now); start.setHours(0, 0, 0, 0);
      filterSince.value = toLocalDatetimeInput(start);
      filterUntil.value = '';
    } else if (range === 'yesterday') {
      const start = new Date(now); start.setDate(start.getDate() - 1); start.setHours(0, 0, 0, 0);
      const end = new Date(start); end.setHours(23, 59, 0, 0);
      filterSince.value = toLocalDatetimeInput(start);
      filterUntil.value = toLocalDatetimeInput(end);
    } else if (range === '7d') {
      filterSince.value = toLocalDatetimeInput(new Date(now - 7 * 86400000));
      filterUntil.value = '';
    }
    loadEvents(0);
  });
});

// 手动改时间时取消快捷选中
[filterSince, filterUntil].forEach(el => {
  el.addEventListener('change', () => {
    document.querySelectorAll('.range-btn').forEach(b => b.classList.remove('active'));
  });
});

async function loadEvents(page = 0) {
  eventsPage = page;
  const type = filterType.value;
  const since = filterSince.value ? filterSince.value.replace('T', ' ') + ':00' : '';
  const until = filterUntil.value ? filterUntil.value.replace('T', ' ') + ':59' : '';
  let url = `/api/events?limit=${PAGE_SIZE}&offset=${page * PAGE_SIZE}`;
  if (type) url += `&type=${type}`;
  if (since) url += `&since=${encodeURIComponent(since)}`;
  if (until) url += `&until=${encodeURIComponent(until)}`;

  const r = await fetch(url);
  const data = await r.json();
  const items = data.items || [];
  const total = data.total || 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  eventsTbody.innerHTML = '';
  items.forEach(ev => {
    const tr = document.createElement('tr');
    const snapFile = ev.snapshot_path ? ev.snapshot_path.split('/').pop() : null;
    const snap = snapFile
      ? `<a class="snap-link" href="/snapshots/${snapFile}" target="_blank">查看</a>`
      : '-';
    const canEnroll = snapFile && (ev.type === 'stranger' || ev.type === 'face');
    const enrollBtn = canEnroll
      ? `<button class="enroll-btn" data-snap="${snapFile}" data-label="${ev.label}">注册人脸</button>`
      : '-';
    tr.innerHTML = `
      <td>${ev.timestamp.slice(0,19).replace('T',' ')}</td>
      <td><span class="badge badge-${ev.type}">${TYPE_LABELS[ev.type] || ev.type}</span></td>
      <td>${ev.label}</td>
      <td>${(ev.confidence * 100).toFixed(0)}%</td>
      <td>${snap}</td>
      <td>${enrollBtn}</td>`;
    eventsTbody.appendChild(tr);
  });

  pageInfo.textContent = `第 ${page + 1} / ${totalPages} 页`;
  pageTotal.textContent = `共 ${total} 条`;
  pagePrev.disabled = page === 0;
  pageNext.disabled = page >= totalPages - 1;

  eventsTbody.querySelectorAll('.enroll-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const snap = btn.dataset.snap;
      const defaultName = btn.dataset.label === '陌生人' ? '' : btn.dataset.label;
      const name = prompt('请输入姓名：', defaultName);
      if (!name || !name.trim()) return;
      const fd = new FormData();
      fd.append('name', name.trim());
      fd.append('snapshot_filename', snap);
      const res = await fetch('/api/faces/from-snapshot', { method: 'POST', body: fd });
      const d = await res.json();
      alert(res.ok ? d.message : (d.detail || '注册失败'));
      if (res.ok) pollStatus();
    });
  });
}

document.getElementById('load-events').addEventListener('click', () => loadEvents(0));
pagePrev.addEventListener('click', () => loadEvents(eventsPage - 1));
pageNext.addEventListener('click', () => loadEvents(eventsPage + 1));
loadEvents();
