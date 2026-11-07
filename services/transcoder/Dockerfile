# syntax=docker/dockerfile:1

# --- ビルドステージ ---
FROM golang:1.23-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY pkg/ ./pkg/
COPY services/transcoder/ ./services/transcoder/

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /app/transcoder-service \
    ./services/transcoder

# --- 実行ステージ ---
# FFmpegが必要なのでalpineを使用
FROM alpine:3.21

# FFmpegとCA証明書をインストール
RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    tzdata

# 非rootユーザーを作成
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /app/transcoder-service /transcoder-service

# トランスコード作業用の一時ディレクトリ
RUN mkdir -p /tmp/transcode && chown appuser:appgroup /tmp/transcode

USER appuser

ENTRYPOINT ["/transcoder-service"]
