#!/usr/bin/env bash

set -euo pipefail

PROJECT_NAME="${GOLDEN_PATH_PROJECT_NAME:-dungeon-master-golden-path}"
API_TEST_PORT="${API_TEST_PORT:-18085}"
WEB_TEST_PORT="${WEB_TEST_PORT:-13005}"
POSTGRES_TEST_PORT="${POSTGRES_TEST_PORT:-15435}"
REDIS_TEST_PORT="${REDIS_TEST_PORT:-16385}"
VISION_TEST_PORT="${VISION_TEST_PORT:-18090}"
KEEP_TEST_STACK="${KEEP_TEST_STACK:-false}"

compose=(docker compose -p "${PROJECT_NAME}" -f docker-compose.yml -f docker-compose.test.yml)

cleanup() {
  if [[ "${KEEP_TEST_STACK}" != "true" ]]; then
    "${compose[@]}" down --volumes --remove-orphans
  fi
}
trap cleanup EXIT

export API_PORT="${API_TEST_PORT}"
export WEB_PORT="${WEB_TEST_PORT}"
export POSTGRES_PORT="${POSTGRES_TEST_PORT}"
export REDIS_PORT="${REDIS_TEST_PORT}"
export VISION_PORT="${VISION_TEST_PORT}"

echo "==> Start isolated deterministic stack"
"${compose[@]}" up -d --build --wait

echo "==> API golden path"
API_BASE_URL="http://localhost:${API_TEST_PORT}" bash scripts/golden_path_api_test.sh

echo "==> Browser golden path"
PLAYWRIGHT_BASE_URL="http://localhost:${WEB_TEST_PORT}" npm run test:e2e

echo "==> Complete golden path passed"
