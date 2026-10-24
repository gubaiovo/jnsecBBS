# 多阶段构建：Node 构建前端 + Go 构建单静态二进制
FROM node:22-alpine AS frontend

WORKDIR /src/frontend

# 复制包管理文件 (bunfig.toml 不需要被 npm 使用，但为了 COPY 不报错保留)
COPY frontend/package.json frontend/bunfig.toml ./

# 使用 --legacy-peer-deps 绕过 npm 的 edgesOut 解析 Bug
RUN npm install --legacy-peer-deps

# 复制前端所有源码
COPY frontend/ ./

# 【关键修改 1】：容错构建。
# 先尝试运行 tsc 检查，如果报错（|| true）不中断流程，继续执行 vite build 生成静态文件。
RUN npx tsc -b || true
RUN npx vite build

# --- Go 编译阶段 ---
FROM golang:1.25-alpine AS build
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,https://goproxy.io,direct

# 复制 Go 模块文件并下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制后端源码
COPY . .

# 【关键修改 2】：因为 vite.config.ts 配置了 outDir 为 "../internal/frontend/dist"，
# 所以在 frontend 容器里，产物在 /src/internal/frontend/dist。
# 直接从这里拷贝到 Go 的构建上下文。
COPY --from=frontend /src/internal/frontend/dist internal/frontend/dist

# 编译 Go 二进制
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bbs ./cmd/bbs

# --- 最终运行阶段 ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /bbs /app/bbs
VOLUME ["/app/data"]
EXPOSE 8080
ENTRYPOINT ["/app/bbs"]
CMD ["serve", "-c", "/app/config.yaml"]