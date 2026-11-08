#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: Docker is not installed. Install Docker Engine or Docker Desktop first." >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "ERROR: Docker engine is not running. Start it and re-run this script." >&2
  exit 1
fi

echo "==> Building and starting containers (first run can take 5-10 minutes)"
docker compose up -d --build --remove-orphans --wait

echo "==> Ready"
echo "    UI:  http://localhost:3000"
echo "    API: http://localhost:8080/api/health"
echo "    Logs: docker compose logs -f backend"
