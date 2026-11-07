# syntax=docker/dockerfile:1

# --- 依存関係 ---
FROM node:24-slim AS deps

WORKDIR /app

# 依存関係のキャッシュ（ソースより先にコピーすることでキャッシュを効かせる）
COPY package.json package-lock.json ./
RUN npm ci

# --- 開発サーバー（docker compose 用） ---
# ソースは docker compose でバインドマウントし、Vite の HMR で即時反映する。
# node_modules はホスト (macOS) 用のネイティブバイナリと混ざらないよう、イメージ内のものを使う。
FROM deps AS dev

EXPOSE 3000
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0", "--port", "3000", "--strictPort"]

# --- ビルドステージ ---
FROM deps AS builder

COPY . .
RUN npm run build

# --- 実行ステージ ---
# --target を指定しない docker build（deploy/build-images.sh など）はこのステージを使う。
# 非 root ユーザーで動作する nginx イメージ（:8080 で待ち受け）
FROM nginxinc/nginx-unprivileged:stable-alpine

# /videos, /stream のプロキシ先。デフォルトは Kubernetes の Service 名で、docker compose では上書きする
ENV UPLOAD_SERVICE_URL=http://upload-service \
    STREAMING_SERVICE_URL=http://streaming-service

# /etc/nginx/templates 配下は起動時に envsubst で展開され /etc/nginx/conf.d に配置される
COPY nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 8080
