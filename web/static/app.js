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

// ── 时间轴回放 ────────────────────────────────────────────
const timelineCanvas = document.getElementById('timeline-canvas');
const timelineCtx = timelineCanvas.getContext('2d');
const timelineWrap = document.getElementById('timeline-wrap');
const timelineInfo = document.getElementById('timeline-info');
let timelineSegments = []; // [{filename, created, duration, start_ts, end_ts}, ...]
let timelineCurrentIdx = -1;

async function loadTimeline() {
  try {
    const r = await fetch('/api/recordings/timeline');
    const data = await r.json();
    if (!data.length) return;
    // 计算每段的绝对时间戳
    timelineSegments = data.map(rec => {
      const start = new Date(rec.created).getTime() / 1000;
      return { ...rec, start_ts: start, end_ts: start + (rec.duration || 0) };
    });
    drawTimeline();
    timelineWrap.style.display = 'block';
  } catch {}
}

function drawTimeline() {
  if (!timelineSegments.length) return;
  const W = timelineCanvas.parentElement.clientWidth || 600;
  timelineCanvas.width = W;
  const H = 48;
  timelineCtx.clearRect(0, 0, W, H);

  const tMin = timelineSegments[0].start_ts;
  const tMax = timelineSegments[timelineSegments.length - 1].end_ts;
  const span = tMax - tMin || 1;

  // 背景
  timelineCtx.fillStyle = '#161b22';
  timelineCtx.fillRect(0, 0, W, H);

  timelineSegments.forEach((seg, i) => {
    const x1 = Math.floor(((seg.start_ts - tMin) / span) * W);
    const x2 = Math.ceil(((seg.end_ts - tMin) / span) * W);
    const w = Math.max(x2 - x1, 3);
    timelineCtx.fillStyle = i === timelineCurrentIdx ? '#58a6ff' : '#238636';
    timelineCtx.fillRect(x1, 8, w, H - 16);
  });

  // 时间标签
  timelineCtx.fillStyle = '#8b949e';
  timelineCtx.font = '10px monospace';
  const fmt = ts => new Date(ts * 1000).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
  timelineCtx.fillText(fmt(tMin), 4, H - 4);
  const endLabel = fmt(tMax);
  timelineCtx.fillText(endLabel, W - timelineCtx.measureText(endLabel).width - 4, H - 4);
}

timelineCanvas.addEventListener('click', (e) => {
  if (!timelineSegments.length) return;
  const rect = timelineCanvas.getBoundingClientRect();
  const x = e.clientX - rect.left;
  const W = timelineCanvas.width;
  const tMin = timelineSegments[0].start_ts;
  const tMax = timelineSegments[timelineSegments.length - 1].end_ts;
  const span = tMax - tMin || 1;
  const clickTs = tMin + (x / W) * span;

  // 找到点击位置对应的片段
  let idx = -1;
  for (let i = 0; i < timelineSegments.length; i++) {
    if (clickTs >= timelineSegments[i].start_ts && clickTs <= timelineSegments[i].end_ts) {
      idx = i; break;
    }
  }
  // 如果点在间隙，找最近的片段
  if (idx === -1) {
    let minDist = Infinity;
    timelineSegments.forEach((seg, i) => {
      const d = Math.min(Math.abs(clickTs - seg.start_ts), Math.abs(clickTs - seg.end_ts));
      if (d < minDist) { minDist = d; idx = i; }
    });
  }
  if (idx === -1) return;

  const seg = timelineSegments[idx];
  const offset = Math.max(0, clickTs - seg.start_ts);
  playTimelineSegment(idx, offset);
});

function playTimelineSegment(idx, seekOffset = 0) {
  if (idx < 0 || idx >= timelineSegments.length) return;
  const seg = timelineSegments[idx];
  timelineCurrentIdx = idx;
  drawTimeline();

  playbackVideo.src = `/api/recordings/${seg.filename}`;
  playbackVideo.style.display = 'block';
  playbackPlaceholder.style.display = 'none';
  playbackControls.style.display = 'flex';
  playingName.textContent = seg.filename;

  playbackVideo.onloadedmetadata = () => {
    if (seekOffset > 0) playbackVideo.currentTime = seekOffset;
    playbackVideo.play();
  };

  const fmt = ts => new Date(ts * 1000).toLocaleString('zh-CN');
  timelineInfo.textContent = `${fmt(seg.start_ts)} — ${fmt(seg.end_ts)}  (${idx + 1}/${timelineSegments.length})`;

  // 播完自动跳下一段
  playbackVideo.onended = () => {
    if (timelineCurrentIdx + 1 < timelineSegments.length) {
      playTimelineSegment(timelineCurrentIdx + 1, 0);
    }
  };
}

// 切到回放 tab 时加载时间轴
document.querySelectorAll('.tab-btn').forEach(btn => {
  if (btn.dataset.tab === 'playback') {
    btn.addEventListener('click', () => {
      loadRecordings();
      loadTimeline();
    });
  }
});

// ── 设置页面 ──────────────────────────────────────────────
const settingsMsg = document.getElementById('settings-msg');

function showSettingsMsg(text, ok) {
  settingsMsg.textContent = text;
  settingsMsg.className = 'settings-msg ' + (ok ? 'msg-ok' : 'msg-err');
  settingsMsg.style.display = 'block';
  setTimeout(() => { settingsMsg.style.display = 'none'; }, 3000);
}

async function loadSettings() {
  try {
    const r = await fetch('/api/config');
    const cfg = await r.json();
    const g = (id, val) => { const el = document.getElementById(id); if (el) el.value = val ?? ''; };
    const gc = (id, val) => { const el = document.getElementById(id); if (el) el.checked = !!val; };

    g('cfg-camera-source', cfg.camera?.source ?? '');
    g('cfg-camera-res', `${cfg.camera?.width ?? 1280}x${cfg.camera?.height ?? 720}`);
    g('cfg-camera-fps', cfg.camera?.fps ?? 15);

    gc('cfg-motion-enabled', cfg.detectors?.motion?.enabled ?? true);
    g('cfg-motion-area', cfg.detectors?.motion?.min_area ?? 500);
    g('cfg-motion-intrusion', cfg.detectors?.motion?.intrusion_seconds ?? 3);

    gc('cfg-face-enabled', cfg.detectors?.face?.enabled ?? true);
    g('cfg-face-tolerance', cfg.detectors?.face?.tolerance ?? 0.6);

    gc('cfg-object-enabled', cfg.detectors?.object?.enabled ?? false);
    g('cfg-object-conf', cfg.detectors?.object?.confidence ?? 0.5);

    gc('cfg-recording-enabled', cfg.recording?.enabled ?? true);
    g('cfg-seg-minutes', cfg.recording?.segment_minutes ?? 10);
    g('cfg-max-segments', cfg.recording?.max_segments ?? 50);
    g('cfg-max-locked', cfg.recording?.max_locked ?? 20);

    g('cfg-cooldown', cfg.alerts?.cooldown_seconds ?? 30);
    gc('cfg-save-snapshots', cfg.alerts?.save_snapshots ?? true);

    g('cfg-stream-quality', cfg.stream?.jpeg_quality ?? 75);
  } catch (e) {
    showSettingsMsg('加载配置失败', false);
  }
}

document.getElementById('save-settings').addEventListener('click', async () => {
  const gv = id => document.getElementById(id)?.value;
  const gc = id => document.getElementById(id)?.checked;
  const res_parts = (gv('cfg-camera-res') || '1280x720').split('x');

  const body = {
    camera: {
      source: gv('cfg-camera-source'),
      width: parseInt(res_parts[0]) || 1280,
      height: parseInt(res_parts[1]) || 720,
      fps: parseInt(gv('cfg-camera-fps')) || 15,
    },
    detectors: {
      motion: {
        enabled: gc('cfg-motion-enabled'),
        min_area: parseInt(gv('cfg-motion-area')) || 500,
        intrusion_seconds: parseFloat(gv('cfg-motion-intrusion')) || 3,
      },
      face: {
        enabled: gc('cfg-face-enabled'),
        tolerance: parseFloat(gv('cfg-face-tolerance')) || 0.6,
      },
      object: {
        enabled: gc('cfg-object-enabled'),
        confidence: parseFloat(gv('cfg-object-conf')) || 0.5,
      },
    },
    recording: {
      enabled: gc('cfg-recording-enabled'),
      segment_minutes: parseInt(gv('cfg-seg-minutes')) || 10,
      max_segments: parseInt(gv('cfg-max-segments')) || 50,
      max_locked: parseInt(gv('cfg-max-locked')) || 20,
    },
    alerts: {
      cooldown_seconds: parseInt(gv('cfg-cooldown')) || 30,
      save_snapshots: gc('cfg-save-snapshots'),
    },
    stream: {
      jpeg_quality: parseInt(gv('cfg-stream-quality')) || 75,
    },
  };

  try {
    const r = await fetch('/api/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const d = await r.json();
    showSettingsMsg(d.message || (r.ok ? '已保存' : '保存失败'), r.ok);
  } catch {
    showSettingsMsg('保存失败', false);
  }
});

// 切到设置 tab 时加载配置
document.querySelectorAll('.tab-btn').forEach(btn => {
  if (btn.dataset.tab === 'settings') {
    btn.addEventListener('click', loadSettings);
  }
});
