#!/usr/bin/env bash
# Docker Desktop のKubernetes用にローカルイメージをビルドするスクリプト
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TAG="${TAG:-latest}"

echo "==> Building images from: $REPO_ROOT"

build() {
  local name="$1"
  local dockerfile="$2"
  local context="${3:-.}"  # ビルドコンテキスト（リポジトリルートからの相対パス）
  echo "--> Building video-stream/${name}:${TAG}"
  docker build \
    -f "${REPO_ROOT}/${dockerfile}" \
    -t "video-stream/${name}:${TAG}" \
    "${REPO_ROOT}/${context}"
}

build "upload-service"    "services/upload/Dockerfile"
build "transcoder-service" "services/transcoder/Dockerfile"
build "streaming-service" "services/streaming/Dockerfile"
build "web"               "web/Dockerfile" "web"

echo ""
echo "==> Done. Images built:"
docker images | grep "video-stream/"
