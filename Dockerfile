# ============================================================
# Multi-stage build: frontend + backend → single binary image
# ============================================================

# Stage 1: Build frontend
FROM node:22-alpine AS frontend-builder

WORKDIR /app/frontend

# 使用国内 npm 镜像
RUN npm config set registry https://registry.npmmirror.com

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.24-alpine AS backend-builder

WORKDIR /app/backend

# 使用国内 Go 模块代理 + 允许自动下载所需 Go 版本
ENV GOPROXY=https://goproxy.cn,direct
ENV GOTOOLCHAIN=auto

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

# Copy frontend build output to static/
COPY --from=frontend-builder /app/frontend/dist ./static/

# Build static binary
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o server .

# Stage 3: Production image
FROM alpine:3.20

WORKDIR /app

# Install ca-certificates for HTTPS calls to RunningHub, timezone data
RUN apk add --no-cache ca-certificates tzdata

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
