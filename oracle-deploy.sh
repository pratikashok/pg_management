#!/usr/bin/env bash

set -euo pipefail

APP_DIR="/opt/pg-management-system"
ENV_FILE="$APP_DIR/.env"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required. Install Docker first."
  exit 1
fi

if ! command -v docker compose >/dev/null 2>&1; then
  echo "Docker Compose plugin is required."
  exit 1
fi

if [ ! -f "$ENV_FILE" ]; then
  echo "Missing $ENV_FILE"
  exit 1
fi

cd "$APP_DIR"
set -a
source "$ENV_FILE"
set +a

docker compose down
docker compose up -d --build
docker compose ps
