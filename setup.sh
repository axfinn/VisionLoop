#!/usr/bin/env bash
set -e

# ── 颜色 ──────────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[✓]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
error() { echo -e "${RED}[✗]${NC} $*"; exit 1; }

echo ""
echo "  VisionLoop 初始化脚本"
echo "────────────────────────────────────────"

# ── 1. 检查 Python ────────────────────────────────────────────────────────────
PYTHON=""
for cmd in python3.14 python3.13 python3.12 python3.11 python3.10 python3; do
  if command -v "$cmd" &>/dev/null; then
    VER=$("$cmd" -c "import sys; print(sys.version_info[:2])")
    if "$cmd" -c "import sys; sys.exit(0 if sys.version_info >= (3,10) else 1)" 2>/dev/null; then
      PYTHON="$cmd"
      break
    fi
  fi
done
[ -z "$PYTHON" ] && error "需要 Python 3.10+，请先安装：https://www.python.org/downloads/"
info "Python: $($PYTHON --version)"

# ── 2. macOS 依赖检查 ─────────────────────────────────────────────────────────
if [[ "$OSTYPE" == "darwin"* ]]; then
  if ! command -v cmake &>/dev/null; then
    warn "未检测到 cmake（dlib 编译可能需要）"
    if command -v brew &>/dev/null; then
      read -rp "  是否用 Homebrew 安装 cmake？[Y/n] " ans
      [[ "${ans:-Y}" =~ ^[Yy]$ ]] && brew install cmake && info "cmake 已安装"
    else
      warn "请手动安装 cmake：brew install cmake"
    fi
  else
    info "cmake: $(cmake --version | head -1)"
  fi
  # 设置 SDK 路径，避免 dlib 编译时找不到头文件
  export SDKROOT=$(xcrun --show-sdk-path 2>/dev/null || true)
  [ -n "$SDKROOT" ] && info "SDKROOT: $SDKROOT"
fi

# ── 3. 创建虚拟环境 ───────────────────────────────────────────────────────────
if [ ! -d ".venv" ]; then
  info "创建虚拟环境 .venv ..."
  $PYTHON -m venv .venv
else
  info "虚拟环境已存在，跳过创建"
fi

PIP=".venv/bin/pip"
PYTHON_VENV=".venv/bin/python"

# ── 4. 安装依赖 ───────────────────────────────────────────────────────────────
info "安装依赖（可能需要几分钟）..."
$PIP install --upgrade pip -q
$PIP install -r requirements.txt

# ── 5. 创建必要目录 ───────────────────────────────────────────────────────────
mkdir -p known_faces snapshots recordings
info "目录已就绪：known_faces / snapshots / recordings"

# ── 6. 生成默认配置 ───────────────────────────────────────────────────────────
if [ ! -f "config.yaml" ]; then
  cat > config.yaml << 'EOF'
camera:
  source: 0
  width: 1280
  height: 720
  fps: 30

detectors:
  motion:
    enabled: true
    min_area: 1500
    intrusion_seconds: 2.0
  face:
    enabled: true
    tolerance: 0.55
    scale: 0.5
  object:
    enabled: false
    model: yolov8n.pt
    confidence: 0.45
    classes: [0, 2, 15, 16]

alerts:
  cooldown_seconds: 30
  save_snapshots: true

recording:
  enabled: true
  segment_minutes: 10
  max_segments: 50
  max_locked: 20

web:
  host: 0.0.0.0
  port: 8888
  stream_quality: 75

paths:
  known_faces: known_faces
  snapshots: snapshots
  database: data.db
EOF
  info "已生成默认配置 config.yaml（YOLO 默认关闭，按需开启）"
else
  info "config.yaml 已存在，跳过生成"
fi

# ── 7. 完成 ───────────────────────────────────────────────────────────────────
echo ""
echo "────────────────────────────────────────"
info "初始化完成！启动方式："
echo ""
echo "    $PYTHON_VENV main.py"
echo ""
echo "  浏览器访问 http://localhost:8888"
echo ""
