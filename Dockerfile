# VisionLoop 主服务 Dockerfile
# 多阶段构建: Go build + runtime

# ============ Build Stage ============
FROM golang:1.21-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache \
    build-base \
    cmake \
    git \
    wget \
    ffmpeg \
    ffmpeg-dev \
    musl-dev \
    linux-headers \
    gcc \
    g++ \
    opencv-dev

# 设置工作目录
WORKDIR /build

# 复制 go mod 文件
COPY go.mod ./
RUN go mod download

# 复制源代码
COPY . .

# 复制 FFmpeg 库
COPY --from=alpine:3.18 /usr/lib/libavcodec.so* /usr/lib/
COPY --from=alpine:3.18 /usr/lib/libavformat.so* /usr/lib/
COPY --from=alpine:3.18 /usr/lib/libavutil.so* /usr/lib/
COPY --from=alpine:3.18 /usr/lib/libswscale.so* /usr/lib/
COPY --from=alpine:3.18 /usr/lib/libswresample.so* /usr/lib/

# 构建 Go 服务
ENV CGO_ENABLED=1
ENV CGO_LDFLAGS="-L/usr/lib"
RUN go build -ldflags="-s -w" -o visionloop ./cmd/server

# ============ Runtime Stage ============
FROM alpine:3.18 AS runtime

# 安装运行时依赖
RUN apk add --no-cache \
    ffmpeg \
    libstdc++ \
    libgcc \
    libwebp \
    ca-certificates \
    tzdata \
    bash \
    curl

# 安装 OpenCV (gocv 依赖)
RUN apk add --no-cache \
    openblas \
    libgomp

# 设置环境变量
ENV CGO_ENABLED=1
ENV GOCV_ENABLE_MIRROR=false
ENV GOCV_LOG_LEVEL=error

# 创建应用目录
WORKDIR /app

# 复制二进制文件
COPY --from=builder /build/visionloop .
COPY --from=builder /build/config.yaml .

# 复制前端资源 (如果存在)
COPY --from=builder /build/web/dist ./web/dist

# 复制检测进程 (如果存在)
COPY --from=builder /build/detection ./detection

# 创建必要目录
RUN mkdir -p clips events screenshots logs

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/storage || exit 1

# 暴露端口
EXPOSE 8080

# 启动命令
CMD ["./visionloop"]