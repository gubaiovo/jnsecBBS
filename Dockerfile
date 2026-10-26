FROM oven/bun:1.4.4-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/bun.lock frontend/bunfig.toml ./
RUN bun install --frozen-lockfile
COPY frontend/ ./
RUN bun --bun run build

FROM golang:1.25.4-alpine AS build
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,https://goproxy.io,direct GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/internal/frontend/dist internal/frontend/dist
RUN CGO_ENABLED=0 go build -mod=readonly -trimpath -ldflags="-s -w" -o /bbs ./cmd/bbs

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /bbs /app/bbs
VOLUME ["/app/data"]
EXPOSE 8080
ENTRYPOINT ["/app/bbs"]
CMD ["serve", "-c", "/app/config.yaml"]
