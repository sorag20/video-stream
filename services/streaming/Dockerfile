# syntax=docker/dockerfile:1

# --- ビルドステージ ---
FROM golang:1.23-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY pkg/ ./pkg/
COPY services/streaming/ ./services/streaming/

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /app/streaming-service \
    ./services/streaming

# --- 実行ステージ ---
FROM gcr.io/distroless/static-debian12

COPY --from=builder /app/streaming-service /streaming-service

EXPOSE 8082

USER nonroot:nonroot

ENTRYPOINT ["/streaming-service"]
