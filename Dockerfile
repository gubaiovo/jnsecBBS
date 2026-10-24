# 多阶段构建：bun 构建前端 + Go 构建单静态二进制
FROM oven/bun:1 AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/bunfig.toml ./
RUN bun install --frozen-lockfile
COPY frontend/ ./
RUN bun run build

FROM golang:1.25-alpine AS build
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,https://goproxy.io,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/internal/frontend/dist internal/frontend/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bbs ./cmd/bbs

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /bbs /app/bbs
# 配置与数据目录由卷挂载
VOLUME ["/app/data"]
EXPOSE 8080
ENTRYPOINT ["/app/bbs"]
CMD ["serve", "-c", "/app/config.yaml"]
