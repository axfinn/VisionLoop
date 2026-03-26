#!/bin/bash
# VisionLoop 构建脚本
# 构建前端、Go 服务和 Python 检测进程
set -e

VERSION=${VERSION:-"1.0.0"}
BUILD_DIR=${BUILD_DIR:-"./build"}
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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
VisionLoop 构建脚本 v${VERSION}

用法: ./build.sh [选项]

选项:
    --all           构建所有组件 (默认)
    --frontend      仅构建前端
    --backend       仅构建 Go 服务
    --detection     仅构建检测进程
    --clean         清理构建产物
    --help          显示帮助

示例:
    ./build.sh              # 构建全部
    ./build.sh --frontend   # 仅构建前端

EOF
}

# 清理构建产物
clean() {
    log_info "清理构建产物..."
    cd "$PROJECT_ROOT"

    rm -rf build/
    rm -f VisionLoop VisionLoop.exe
    rm -rf detection/dist detection/build
    rm -rf detection/__pycache__ detection/*.egg-info
    rm -rf web/dist web/node_modules web/.vite

    log_info "清理完成"
}

# 构建前端
build_frontend() {
    log_info "构建前端..."
    cd "$PROJECT_ROOT/web"

    if ! command -v npm &> /dev/null; then
        log_warn "npm 未安装，跳过前端构建"
        return 0
    fi

    if [ ! -f "package.json" ]; then
        log_warn "package.json 不存在，跳过前端构建"
        return 0
    fi

    npm install
    npm run build

    if [ ! -d "dist" ]; then
        log_error "前端构建失败，dist 目录不存在"
        return 1
    fi

    log_info "前端构建完成: web/dist"
    cd "$PROJECT_ROOT"
}

# 构建 Go 服务
build_backend() {
    log_info "构建 Go 服务..."
    cd "$PROJECT_ROOT"

    if ! command -v go &> /dev/null; then
        log_error "Go 未安装，请先安装 Go 1.21+"
        return 1
    fi

    # 检查 go.mod
    if [ ! -f "go.mod" ]; then
        log_error "go.mod 不存在"
        return 1
    fi

    # 下载依赖
    go mod download

    # 确定输出文件名
    OS=$(go env GOOS)
    if [ "$OS" = "windows" ]; then
        OUTPUT="VisionLoop.exe"
    else
        OUTPUT="VisionLoop"
    fi

    # 构建
    CGO_ENABLED=1 go build -ldflags="-s -w -X main.Version=${VERSION}" -o "$OUTPUT" ./cmd/server

    if [ ! -f "$OUTPUT" ]; then
        log_error "Go 构建失败"
        return 1
    fi

    log_info "Go 服务构建完成: $OUTPUT"
    cd "$PROJECT_ROOT"
}

# 构建检测进程
build_detection() {
    log_info "构建检测进程..."
    cd "$PROJECT_ROOT/detection"

    if ! command -v python3 &> /dev/null; then
        log_warn "Python 未安装，跳过检测进程构建"
        return 0
    fi

    if [ ! -f "requirements.txt" ]; then
        log_warn "requirements.txt 不存在，跳过"
        return 0
    fi

    # 检查 PyInstaller
    if ! command -v pyinstaller &> /dev/null; then
        log_warn "PyInstaller 未安装，跳过打包"
        log_info "如需打包请运行: pip install pyinstaller"
        return 0
    fi

    # 创建虚拟环境 (可选)
    if [ -d "venv" ]; then
        log_info "使用虚拟环境..."
        source venv/bin/activate 2>/dev/null || true
    fi

    # 构建
    pyinstaller --onefile --name visionloop_det main.py

    if [ -f "dist/visionloop_det" ] || [ -f "dist/visionloop_det.exe" ]; then
        log_info "检测进程构建完成: detection/dist/"
    fi

    cd "$PROJECT_ROOT"
}

# 主入口
main() {
    BUILD_ALL=true
    BUILD_FRONTEND=false
    BUILD_BACKEND=false
    BUILD_DETECTION=false
    CLEAN=false

    # 解析参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --all)
                BUILD_ALL=true
                BUILD_FRONTEND=true
                BUILD_BACKEND=true
                BUILD_DETECTION=true
                shift
                ;;
            --frontend)
                BUILD_ALL=false
                BUILD_FRONTEND=true
                shift
                ;;
            --backend)
                BUILD_ALL=false
                BUILD_BACKEND=true
                shift
                ;;
            --detection)
                BUILD_ALL=false
                BUILD_DETECTION=true
                shift
                ;;
            --clean)
                CLEAN=true
                shift
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                log_error "未知选项: $1"
                show_help
                exit 1
                ;;
        esac
    done

    # 创建构建目录
    mkdir -p "$BUILD_DIR"

    # 清理
    if $CLEAN; then
        clean
    fi

    # 构建
    if $BUILD_ALL || $BUILD_FRONTEND; then
        build_frontend || true
    fi

    if $BUILD_ALL || $BUILD_BACKEND; then
        build_backend || true
    fi

    if $BUILD_ALL || $BUILD_DETECTION; then
        build_detection || true
    fi

    log_info "构建完成!"
}

main "$@"