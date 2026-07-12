#!/usr/bin/env bash
set -euo pipefail

IMAGE="${IMAGE:-game-server-scripts/gsm-panel:latest}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"
PUSH="${PUSH:-false}"

if [ "$PUSH" = "true" ]; then
  docker buildx build --platform "$PLATFORMS" -t "$IMAGE" --push .
else
  docker buildx build --platform "$PLATFORMS" -t "$IMAGE" .
fi
