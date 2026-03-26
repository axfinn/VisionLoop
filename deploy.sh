#!/bin/bash
# VisionLoop 部署脚本
# 支持: Linux native 部署 | Docker 部署 | Git 同步
set -e

VERSION=${VERSION:-"1.0.0"}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 帮助信息
show_help() {
    cat << EOF
VisionLoop 部署脚本 v${VERSION}

用法: ./deploy.sh [命令] [选项]

命令:
    install       安装依赖并构建
    start         启动服务
    stop          停止服务
    restart       重启服务
    status        查看服务状态
    logs          查看日志
    clean         清理构建产物
    docker-build  构建 Docker 镜像
    docker-start 使用 Docker Compose 启动
    docker-stop  停止 Docker Compose
    git-status    查看 Git 状态
    git-commit    提交所有变更
    git-push      推送到远程仓库

选项:
    --help        显示帮助信息
    --version     显示版本信息

示例:
    ./deploy.sh install          # 安装并构建
    ./deploy.sh docker-start     # 使用 Docker 启动
    ./deploy.sh start            # 直接启动 (需要 Go 环境)

EOF
}

# 检查依赖
check_dependencies() {
    log_info "检查系统依赖..."

    # Docker 检查
    if command -v docker &> /dev/null; then
        DOCKER_VERSION=$(docker --version | grep -oP '\d+\.\d+\.\d+' | head -1)
        log_info "Docker: $DOCKER_VERSION"
    else
        log_warn "Docker 未安装"
    fi

    # Docker Compose 检查
    if command -v docker-compose &> /dev/null || docker compose version &> /dev/null 2>&1; then
        if docker compose version &> /dev/null 2>&1; then
            COMPOSE_VERSION=$(docker compose version --short 2>/dev/null || echo "unknown")
        else
            COMPOSE_VERSION=$(docker-compose --version | grep -oP '\d+\.\d+\.\d+' | head -1)
        fi
        log_info "Docker Compose: $COMPOSE_VERSION"
    else
        log_warn "Docker Compose 未安装"
    fi

    # Go 检查
    if command -v go &> /dev/null; then
        GO_VERSION=$(go version | grep -oP 'go\d+\.\d+\.\d+' | head -1)
        log_info "Go: $GO_VERSION"
    else
        log_warn "Go 未安装，将无法进行本地构建"
    fi

    # Python 检查
    if command -v python3 &> /dev/null; then
        PYTHON_VERSION=$(python3 --version | grep -oP '\d+\.\d+\.\d+' | head -1)
        log_info "Python: $PYTHON_VERSION"
    else
        log_warn "Python 未安装，将无法构建检测进程"
    fi

    # FFmpeg 检查
    if command -v ffmpeg &> /dev/null; then
        FFMPEG_VERSION=$(ffmpeg -version | grep -oP 'ffmpeg \d+' | head -1)
        log_info "FFmpeg: $FFMPEG_VERSION"
    else
        log_warn "FFmpeg 未安装，将使用软编码"
    fi
}

# 安装系统依赖 (Ubuntu/Debian)
install_ubuntu_deps() {
    log_info "安装系统依赖 (Ubuntu/Debian)..."
    sudo apt-get update
    sudo apt-get install -y \
        build-essential \
        cmake \
        git \
        wget \
        curl \
        ffmpeg \
        libavcodec-dev \
        libavformat-dev \
        libswscale-dev \
        libgomp1 \
        python3-dev \
        python3-pip \
        python3-venv
}

# 安装系统依赖 (CentOS/RHEL)
install_centos_deps() {
    log_info "安装系统依赖 (CentOS/RHEL)..."
    sudo yum groupinstall -y "Development Tools"
    sudo yum install -y \
        cmake \
        git \
        wget \
        curl \
        ffmpeg \
        ffmpeg-devel \
        python3-devel \
        python3-pip
}

# 安装 Python 虚拟环境
install_python_venv() {
    log_info "设置 Python 虚拟环境..."
    cd "$PROJECT_DIR/detection"
    python3 -m venv venv
    source venv/bin/activate
    pip install --upgrade pip
    pip install -r requirements.txt
    deactivate
    cd "$PROJECT_DIR"
}

# 构建前端
build_frontend() {
    if ! command -v npm &> /dev/null; then
        log_warn "npm 未安装，跳过前端构建"
        return
    fi

    log_info "构建前端..."
    cd "$PROJECT_DIR/web"
    npm install
    npm run build
    cd "$PROJECT_DIR"
}

# 构建 Go 服务
build_go() {
    if ! command -v go &> /dev/null; then
        log_error "Go 未安装，无法构建"
        return 1
    fi

    log_info "构建 Go 服务..."
    cd "$PROJECT_DIR"
    go mod download
    go build -ldflags="-s -w" -o VisionLoop ./cmd/server
}

# 构建检测进程
build_detection() {
    if ! command -v python3 &> /dev/null; then
        log_warn "Python 未安装，跳过检测进程构建"
        return
    fi

    log_info "构建检测进程..."
    cd "$PROJECT_DIR/detection"

    if command -v pyinstaller &> /dev/null; then
        pyinstaller --onefile --name visionloop_det main.py
    else
        log_warn "PyInstaller 未安装，跳过检测进程打包"
    fi

    cd "$PROJECT_DIR"
}

# 本地安装
do_install() {
    log_info "开始安装 VisionLoop..."

    check_dependencies

    # 检测系统类型
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        case "$ID" in
            ubuntu|debian)
                install_ubuntu_deps
                ;;
            centos|rhel|fedora)
                install_centos_deps
                ;;
            *)
                log_warn "未知系统类型: $ID"
                ;;
        esac
    fi

    # 构建项目
    build_frontend
    build_go
    build_detection

    # 创建必要目录
    mkdir -p "$PROJECT_DIR/clips"
    mkdir -p "$PROJECT_DIR/events"
    mkdir -p "$PROJECT_DIR/screenshots"

    log_info "安装完成!"
    log_info "运行 ./deploy.sh start 启动服务"
}

# 启动服务 (本地)
do_start() {
    log_info "启动 VisionLoop 服务..."

    if [ ! -f "$PROJECT_DIR/VisionLoop" ]; then
        log_error "VisionLoop 可执行文件不存在，请先运行 ./deploy.sh install"
        return 1
    fi

    cd "$PROJECT_DIR"

    # 后台运行
    nohup ./VisionLoop > logs/visionloop.log 2>&1 &
    echo $! > logs/visionloop.pid

    log_info "服务已启动 (PID: $(cat logs/visionloop.pid))"
    log_info "访问 http://localhost:8080"
}

# 停止服务 (本地)
do_stop() {
    log_info "停止 VisionLoop 服务..."

    if [ -f "$PROJECT_DIR/logs/visionloop.pid" ]; then
        PID=$(cat "$PROJECT_DIR/logs/visionloop.pid")
        if ps -p "$PID" > /dev/null 2>&1; then
            kill "$PID"
            rm -f "$PROJECT_DIR/logs/visionloop.pid"
            log_info "服务已停止"
        else
            log_warn "服务未运行"
        fi
    else
        # 尝试查找进程
        PIDS=$(pgrep -f "VisionLoop")
        if [ -n "$PIDS" ]; then
            kill $PIDS 2>/dev/null
            log_info "服务已停止"
        else
            log_warn "未找到运行中的服务"
        fi
    fi
}

# 查看状态
do_status() {
    if [ -f "$PROJECT_DIR/logs/visionloop.pid" ]; then
        PID=$(cat "$PROJECT_DIR/logs/visionloop.pid")
        if ps -p "$PID" > /dev/null 2>&1; then
            log_info "服务运行中 (PID: $PID)"
        else
            log_warn "PID 文件存在但服务未运行"
        fi
    else
        PIDS=$(pgrep -f "VisionLoop")
        if [ -n "$PIDS" ]; then
            log_info "服务运行中 (PIDs: $PIDS)"
        else
            log_info "服务未运行"
        fi
    fi
}

# 查看日志
do_logs() {
    if [ -f "$PROJECT_DIR/logs/visionloop.log" ]; then
        tail -f "$PROJECT_DIR/logs/visionloop.log"
    else
        log_warn "日志文件不存在"
    fi
}

# 清理构建产物
do_clean() {
    log_info "清理构建产物..."
    cd "$PROJECT_DIR"

    rm -f VisionLoop
    rm -f VisionLoop.exe
    rm -rf detection/dist
    rm -rf detection/build
    rm -rf detection/__pycache__
    rm -rf web/dist
    rm -rf web/node_modules
    rm -rf web/.vite

    log_info "清理完成"
}

# 构建 Docker 镜像
do_docker_build() {
    log_info "构建 Docker 镜像..."
    cd "$PROJECT_DIR"

    docker build -t visionloop:${VERSION} .
    docker build -f Dockerfile.detection -t visionloop-detect:${VERSION} .

    log_info "Docker 镜像构建完成"
}

# Docker Compose 启动
do_docker_start() {
    log_info "使用 Docker Compose 启动 VisionLoop..."
    cd "$PROJECT_DIR"

    # 确保目录存在
    mkdir -p clips events screenshots

    if docker compose version &> /dev/null 2>&1; then
        docker compose up -d
    else
        docker-compose up -d
    fi

    log_info "服务已启动"
    log_info "访问 http://localhost:8080"
}

# Docker Compose 停止
do_docker_stop() {
    log_info "停止 Docker Compose..."
    cd "$PROJECT_DIR"

    if docker compose version &> /dev/null 2>&1; then
        docker compose down
    else
        docker-compose down
    fi

    log_info "服务已停止"
}

# Git 状态
do_git_status() {
    log_info "Git 状态..."
    cd "$PROJECT_DIR"
    git status --short
    echo ""
    git log --oneline -5
}

# Git 提交
do_git_commit() {
    local MSG="${2:-Auto sync $(date '+%Y-%m-%d %H:%M:%S')}"
    log_info "提交 Git 变更..."
    cd "$PROJECT_DIR"
    git add -A
    if git diff --cached --quiet; then
        log_warn "没有变更需要提交"
    else
        git commit -m "$MSG"
        log_info "提交完成"
    fi
}

# Git 推送
do_git_push() {
    log_info "推送 Git 变更..."
    cd "$PROJECT_DIR"
    git push origin HEAD
    log_info "推送完成"
}

# 主入口
main() {
    COMMAND=${1:-help}

    # 显示帮助不需要创建日志目录
    if [ "$COMMAND" = "help" ] || [ "$COMMAND" = "--help" ]; then
        show_help
        exit 0
    fi

    # 创建日志目录
    mkdir -p "$PROJECT_DIR/logs"

    case "$COMMAND" in
        install)
            do_install
            ;;
        start)
            do_start
            ;;
        stop)
            do_stop
            ;;
        restart)
            do_stop
            sleep 2
            do_start
            ;;
        status)
            do_status
            ;;
        logs)
            do_logs
            ;;
        clean)
            do_clean
            ;;
        docker-build)
            do_docker_build
            ;;
        docker-start)
            do_docker_start
            ;;
        docker-stop)
            do_docker_stop
            ;;
        git-status)
            do_git_status
            ;;
        git-commit)
            do_git_commit "$@"
            ;;
        git-push)
            do_git_push
            ;;
        --version)
            echo "VisionLoop Deploy Script v${VERSION}"
            ;;
        help|--help)
            show_help
            ;;
        *)
            log_error "未知命令: $COMMAND"
            show_help
            exit 1
            ;;
    esac
}

main "$@"