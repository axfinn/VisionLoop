#!/usr/bin/env bash
set -e

PIDFILE=".visionloop.pid"
LOGFILE="visionloop.log"
PYTHON=".venv/bin/python"

# ── 颜色 ──────────────────────────────────────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
info()  { echo -e "${GREEN}[✓]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
error() { echo -e "${RED}[✗]${NC} $*"; }

_check_venv() {
  [ -f "$PYTHON" ] || { error "未找到 .venv，请先运行 ./setup.sh"; exit 1; }
}

_pid() {
  [ -f "$PIDFILE" ] && cat "$PIDFILE" || echo ""
}

_is_running() {
  local pid=$(_pid)
  [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

cmd_start() {
  _check_venv
  if _is_running; then
    warn "已在运行（PID $(_pid)）"
    return
  fi
  nohup "$PYTHON" main.py >> "$LOGFILE" 2>&1 &
  echo $! > "$PIDFILE"
  sleep 1
  if _is_running; then
    info "已启动（PID $(_pid)），日志：$LOGFILE"
    info "浏览器访问 http://localhost:8888"
  else
    error "启动失败，查看日志：tail -20 $LOGFILE"
    exit 1
  fi
}

cmd_stop() {
  if ! _is_running; then
    warn "未在运行"
    rm -f "$PIDFILE"
    return
  fi
  local pid=$(_pid)
  kill "$pid" 2>/dev/null
  # 等待最多 5 秒
  for i in $(seq 1 10); do
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.5
  done
  if kill -0 "$pid" 2>/dev/null; then
    warn "进程未退出，强制终止..."
    kill -9 "$pid" 2>/dev/null || true
  fi
  rm -f "$PIDFILE"
  info "已停止（PID $pid）"
}

cmd_restart() {
  cmd_stop
  sleep 1
  cmd_start
}

cmd_kill() {
  # 强制杀掉所有 main.py 进程（包括孤儿进程）
  pkill -f "python.*main\.py" 2>/dev/null && info "已强制终止所有实例" || warn "没有找到运行中的实例"
  rm -f "$PIDFILE"
}

cmd_status() {
  if _is_running; then
    info "运行中（PID $(_pid)）"
    # 尝试获取 FPS
    local status
    status=$(curl -s --max-time 2 http://localhost:8888/api/status 2>/dev/null || true)
    [ -n "$status" ] && echo "  $status"
  else
    warn "未运行"
  fi
}

cmd_log() {
  [ -f "$LOGFILE" ] || { warn "日志文件不存在"; return; }
  tail -f "$LOGFILE"
}

# ── 入口 ──────────────────────────────────────────────────────────────────────
case "${1:-}" in
  start)   cmd_start   ;;
  stop)    cmd_stop    ;;
  restart) cmd_restart ;;
  kill)    cmd_kill    ;;
  status)  cmd_status  ;;
  log)     cmd_log     ;;
  *)
    echo "用法: $0 {start|stop|restart|kill|status|log}"
    echo ""
    echo "  start    后台启动"
    echo "  stop     优雅停止"
    echo "  restart  重启"
    echo "  kill     强制终止所有实例"
    echo "  status   查看运行状态"
    echo "  log      实时查看日志"
    exit 1
    ;;
esac
