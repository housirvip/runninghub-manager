# ============================================================
# Multi-stage build: frontend + backend → single binary image
# 优化: 前后端并行构建 / BuildKit 缓存 / 合并层数
# ============================================================

# Stage 1: Build frontend
FROM node:22-slim AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm install --registry=https://registry.npmmirror.com

COPY frontend/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.24-bookworm AS backend-builder

WORKDIR /app/backend

ENV GOPROXY=https://goproxy.cn,direct \
    GOTOOLCHAIN=auto

# 换国内 apt 源 + 安装编译依赖（合并为一层）
RUN sed -i 's|deb.debian.org|mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
    && apt-get install -y --no-install-recommends gcc libc6-dev \
    && rm -rf /var/lib/apt/lists/*

COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY backend/ ./

# 静态编译（前后端在此汇合）
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o server .

# Stage 3: Production image
FROM debian:bookworm-slim

WORKDIR /app

# 换国内 apt 源 + 安装运行时依赖（合并为一层）
RUN sed -i 's|deb.debian.org|mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

# 拷贝二进制
COPY --from=backend-builder /app/backend/server .

# 拷贝前端构建产物
COPY --from=frontend-builder /app/frontend/dist ./static/

# 创建数据目录
RUN mkdir -p /app/data /app/uploads /app/output

ENV PORT=:3060 \
    DB_DRIVER=sqlite \
    DB_PATH=/app/data/runninghub.db \
    UPLOAD_DIR=/app/uploads \
    OUTPUT_DIR=/app/output \
    GIN_MODE=release

EXPOSE 3060

VOLUME ["/app/data", "/app/uploads", "/app/output"]

ENTRYPOINT ["./server"]
