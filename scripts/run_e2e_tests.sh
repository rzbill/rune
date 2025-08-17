#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT_DIR"

echo "[E2E] Building test server and CLI..."
go build -o bin/runed-test ./cmd/runed-test
go build -o bin/rune ./cmd/rune

# Increase HTTP health wait or allow override
export RUNE_E2E_HEALTH_TIMEOUT_SECONDS=${RUNE_E2E_HEALTH_TIMEOUT_SECONDS:-30}

echo "[E2E] Running tests with -tags=e2e"
go test ./test/e2e -tags=e2e -v
