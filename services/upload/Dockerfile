# syntax=docker/dockerfile:1

# --- ビルドステージ ---
FROM golang:1.23-alpine AS builder

WORKDIR /build

# 依存関係のキャッシュ（ソースより先にコピーすることでキャッシュを効かせる）
COPY go.mod go.sum ./
RUN go mod download

# ソースコードをコピー
COPY pkg/ ./pkg/
COPY services/upload/ ./services/upload/

# バイナリをビルド（CGO無効で静的リンク）
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /app/upload-service \
    ./services/upload

# --- 実行ステージ ---
FROM gcr.io/distroless/static-debian12

COPY --from=builder /app/upload-service /upload-service

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/upload-service"]
