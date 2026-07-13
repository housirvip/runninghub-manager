# ============================================================
# Multi-stage build: frontend + backend → single binary image
# ============================================================

# Stage 1: Build frontend (Debian slim based)
FROM node:22-slim AS frontend-builder

WORKDIR /app/frontend

# 使用国内 npm 镜像
RUN npm config set registry https://registry.npmmirror.com

COPY frontend/package.json frontend/package-lock.json ./
RUN npm install

COPY frontend/ ./
RUN npm run build

# Stage 2: Build backend (Debian-based for better CGO/glibc compatibility)
FROM golang:1.24-bookworm AS backend-builder

WORKDIR /app/backend

# 使用国内 Go 模块代理 + 允许自动下载所需 Go 版本
ENV GOPROXY=https://goproxy.cn,direct
ENV GOTOOLCHAIN=auto

# 使用国内 apt 镜像源加速（清华大学）
RUN sed -i 's|deb.debian.org|mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list.d/debian.sources

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev && rm -rf /var/lib/apt/lists/*

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

# Copy frontend build output to static/
COPY --from=frontend-builder /app/frontend/dist ./static/

# Build static binary
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o server .

# Stage 3: Production image (Debian slim for glibc compatibility)
FROM debian:bookworm-slim

WORKDIR /app

# 使用国内 apt 镜像源加速（清华大学）
RUN sed -i 's|deb.debian.org|mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list.d/debian.sources

# Install ca-certificates for HTTPS calls to RunningHub, timezone data
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*

# Copy binary
COPY --from=backend-builder /app/backend/server .

# Create data directories
RUN mkdir -p /app/data /app/uploads /app/output

# Default environment
ENV PORT=:3060 \
    DB_DRIVER=sqlite \
    DB_PATH=/app/data/runninghub.db \
    UPLOAD_DIR=/app/uploads \
    OUTPUT_DIR=/app/output \
    GIN_MODE=release

EXPOSE 3060

VOLUME ["/app/data", "/app/uploads", "/app/output"]

ENTRYPOINT ["./server"]
