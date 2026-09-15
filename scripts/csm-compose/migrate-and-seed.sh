#!/bin/sh
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
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
#
# One-shot init: applies entity-service's and sre-alert-ingestion-service's
# raw SQL migrations (neither service wires up a migration tool -- see
# apps/csm-portal/README.md), then loads dummy seed data into entity-service's
# database. Runs as the "migrate" compose service, which every dependent
# service waits on via `depends_on: condition: service_completed_successfully`.
set -eu

export PGPASSWORD="${POSTGRES_PASSWORD}"
PSQL="psql -h ${POSTGRES_HOST} -p ${POSTGRES_PORT} -U ${POSTGRES_USER} -v ON_ERROR_STOP=1"

echo "[migrate] waiting for postgres..."
until $PSQL -d postgres -c 'select 1' > /dev/null 2>&1; do
  sleep 1
done

echo "[migrate] ensuring databases exist"
$PSQL -d postgres -tc "SELECT 1 FROM pg_database WHERE datname = '${ENTITY_DB_NAME}'" | grep -q 1 || \
  $PSQL -d postgres -c "CREATE DATABASE \"${ENTITY_DB_NAME}\""
$PSQL -d postgres -tc "SELECT 1 FROM pg_database WHERE datname = '${SRE_ALERT_DB_NAME}'" | grep -q 1 || \
  $PSQL -d postgres -c "CREATE DATABASE \"${SRE_ALERT_DB_NAME}\""

# entity-service ships no migration tool and its raw .up.sql files are not
# all safely re-runnable (most guard with IF NOT EXISTS, but at least one
# ADD CONSTRAINT does not -- a real gap in those files, not something to
# patch here). So this script tracks which migrations have actually been
# applied in a per-database schema_migrations table, applying only the ones
# missing from it on every `docker compose up`, rather than gating on a
# single application table's presence (e.g. entity-service's "work_item",
# created partway through the set by migration 000016 -- a later migration
# failing after that point would leave the schema incomplete but still look
# "migrated" forever after, since work_item already exists).
#
# Each migration file is applied and recorded in one transaction (`psql -1`):
# if the file's SQL fails partway through, nothing from it is recorded, so
# the same migration is retried -- from scratch -- on the next run instead
# of being silently skipped with a half-applied schema.
ensure_migrations_table() {
  db="$1"
  $PSQL -d "$db" -c \
    "CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())"
}

apply_pending_migrations() {
  db="$1"; dir="$2"
  ensure_migrations_table "$db"
  for f in $(ls "${dir}"/*.up.sql | sort); do
    version="$(basename "$f" .up.sql)"
    already="$($PSQL -d "$db" -tAc "SELECT 1 FROM schema_migrations WHERE version = '${version}'")"
    if [ "$already" = "1" ]; then
      continue
    fi
    echo "[migrate]   applying $f"
    tmp="$(mktemp)"
    cat "$f" > "$tmp"
    printf "\nINSERT INTO schema_migrations (version) VALUES ('%s');\n" "$version" >> "$tmp"
    $PSQL -d "$db" -1 -f "$tmp"
    rm -f "$tmp"
  done
}

echo "[migrate] applying entity-service migrations (if any are pending)"
apply_pending_migrations "${ENTITY_DB_NAME}" /migrations/entity-service

echo "[migrate] applying sre-alert-ingestion-service migrations (if any are pending)"
apply_pending_migrations "${SRE_ALERT_DB_NAME}" /migrations/sre-alert-ingestion-service

echo "[migrate] loading entity-service seed data"
$PSQL -d "${ENTITY_DB_NAME}" -f /migrations/seed-entity-service.sql

echo "[migrate] done"
