#!/usr/bin/env bash
# Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
#
# Brings up the app locally (backend/docker-compose.yml's Postgres, the Go
# server, and the webapp) and runs an OWASP ZAP *active* full scan against it.
# This is the safe home for active scanning: everything it touches is
# localhost, so — unlike apps/git-internals-dashboard/.github's DAST
# workflow's `full` mode — it never needs platform-team clearance.
#
# The webapp is gated behind Asgardeo OIDC login (see AuthGuard). Without
# real IdP credentials this script cannot drive an authenticated session, so
# ZAP only crawls what's reachable pre-login: static assets, the login
# redirect, and response headers. The backend has no such gate — it is
# deliberately authless (the Choreo gateway owns JWT validation in every real
# deployment) — so it is the part of this scan that exercises real API
# surface, and openapi.yaml drives ZAP's coverage of it.
#
# Usage: apps/git-internals-dashboard/scripts/dast-local.sh
# Requires: docker, go, migrate (golang-migrate CLI), pnpm.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKEND_DIR="$APP_DIR/backend"
WEBAPP_DIR="$APP_DIR/webapp"
ZAP_WORK_DIR="$SCRIPT_DIR/.zap-local"

BACKEND_URL="http://localhost:8080"
WEBAPP_URL="http://localhost:4173"

mkdir -p "$ZAP_WORK_DIR"

SERVER_PID=""
PREVIEW_PID=""

cleanup() {
  echo "==> Cleaning up"
  [ -n "$SERVER_PID" ] && kill "$SERVER_PID" 2>/dev/null || true
  [ -n "$PREVIEW_PID" ] && kill "$PREVIEW_PID" 2>/dev/null || true
  (cd "$BACKEND_DIR" && docker compose down)
}
trap cleanup EXIT

echo "==> Starting Postgres (backend/docker-compose.yml)"
(cd "$BACKEND_DIR" && docker compose up -d)

if [ ! -f "$BACKEND_DIR/.env" ]; then
  echo "==> No backend/.env found — copying .env.example (synthetic seed, no GITHUB_TOKEN)"
  cp "$BACKEND_DIR/.env.example" "$BACKEND_DIR/.env"
fi

set -a
# shellcheck disable=SC1091
source "$BACKEND_DIR/.env"
set +a

echo "==> Waiting for Postgres to accept connections"
for _ in $(seq 1 30); do
  (cd "$BACKEND_DIR" && docker compose exec -T db pg_isready -U "${POSTGRES_USER:-gid}" -d "${POSTGRES_DB:-gid}" >/dev/null 2>&1) && break
  sleep 1
done

echo "==> Applying migrations"
(cd "$BACKEND_DIR" && migrate -path migrations -database "$DATABASE_URL" up)

echo "==> Seeding data (synthetic unless GITHUB_TOKEN is set in backend/.env)"
(cd "$BACKEND_DIR" && go run ./cmd/seed)

echo "==> Starting the backend server on $BACKEND_URL"
(cd "$BACKEND_DIR" && CORS_ALLOWED_ORIGINS="$WEBAPP_URL" go run ./cmd/server) &
SERVER_PID=$!

if [ ! -f "$WEBAPP_DIR/public/config.js" ]; then
  echo "==> No webapp/public/config.js found — copying .example with localhost defaults"
  echo "    (Asgardeo fields are placeholders: pre-login pages only, see file header)"
  cp "$WEBAPP_DIR/public/config.js.example" "$WEBAPP_DIR/public/config.js"
fi

echo "==> Building and previewing the webapp on $WEBAPP_URL"
(cd "$WEBAPP_DIR" && pnpm install --frozen-lockfile && pnpm run build && pnpm run preview -- --host 0.0.0.0 --port 4173) &
PREVIEW_PID=$!

echo "==> Waiting for both services to respond"
for url in "$BACKEND_URL/healthz" "$WEBAPP_URL"; do
  for _ in $(seq 1 60); do
    curl -sf "$url" >/dev/null 2>&1 && break
    sleep 1
  done
done

# Docker Desktop (macOS/Windows) exposes the host as host.docker.internal;
# native Linux Docker needs --network host instead to reach localhost.
ZAP_NETWORK_ARGS=(--add-host=host.docker.internal:host-gateway)
ZAP_HOST="host.docker.internal"

# ZAP's own exit codes (zap-api-scan.py / zap-full-scan.py): 0 clean, 1 at
# least one FAIL alert, 2 at least one WARN alert — all three mean the scan
# itself completed and wrote a report. 3 is ZAP's documented "any other
# failure" (the scan itself broke, e.g. it never reached the target).
# check_zap_result treats 0-2 as success and 3+ (or a missing report file,
# as a second safety net) as a real failure, instead of the old blanket
# `|| true`, which discarded a broken run indistinguishably from a clean one.
check_zap_result() {
  local status="$1" report="$2" label="$3"
  if [ "$status" -ge 3 ] || [ ! -f "$report" ]; then
    echo "==> $label ZAP scan failed (exit $status) or produced no report at $report" >&2
    return 1
  fi
  return 0
}

echo "==> Running ZAP full active scan against the backend (openapi.yaml)"
BACKEND_SCAN_STATUS=0
docker run --rm "${ZAP_NETWORK_ARGS[@]}" \
  -v "$ZAP_WORK_DIR:/zap/wrk/:rw" \
  -v "$BACKEND_DIR/openapi.yaml:/zap/wrk/openapi.yaml:ro" \
  -t ghcr.io/zaproxy/zaproxy:stable \
  zap-api-scan.py -t /zap/wrk/openapi.yaml -f openapi \
    -O "$ZAP_HOST:8080" \
    -r backend-full-scan-report.html || BACKEND_SCAN_STATUS=$?

echo "==> Running ZAP full active scan against the webapp (pre-login surface only)"
WEBAPP_SCAN_STATUS=0
docker run --rm "${ZAP_NETWORK_ARGS[@]}" \
  -v "$ZAP_WORK_DIR:/zap/wrk/:rw" \
  -t ghcr.io/zaproxy/zaproxy:stable \
  zap-full-scan.py -t "http://$ZAP_HOST:4173" \
    -r webapp-full-scan-report.html || WEBAPP_SCAN_STATUS=$?

SCAN_FAILED=0
check_zap_result "$BACKEND_SCAN_STATUS" "$ZAP_WORK_DIR/backend-full-scan-report.html" "Backend" || SCAN_FAILED=1
check_zap_result "$WEBAPP_SCAN_STATUS" "$ZAP_WORK_DIR/webapp-full-scan-report.html" "Webapp" || SCAN_FAILED=1

if [ "$SCAN_FAILED" -ne 0 ]; then
  echo "==> One or more ZAP scans failed to complete — see errors above" >&2
  exit 1
fi

echo "==> Reports written to $ZAP_WORK_DIR/{backend,webapp}-full-scan-report.html"
