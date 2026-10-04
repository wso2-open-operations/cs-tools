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
# One-shot init: applies entity-service's raw SQL migrations (it doesn't wire
# up a migration tool -- see apps/csm-portal/README.md), then loads the fixed,
# minimal dummy seed data in seed-entity-service.sql into entity-service's
# database. Runs as the "migrate" compose service, which every dependent
# service waits on via `depends_on: condition: service_completed_successfully`.
#
# A broader, randomized set of dummy data is generated separately by the
# "seed-generator" compose service (scripts/csm-compose/seed-generator),
# chained to run after this script completes -- it isn't an extra step in
# this script because this container's image (postgres:16-alpine) has no Go
# toolchain to build or run that program.
set -eu

export PGPASSWORD="${POSTGRES_PASSWORD}"
PSQL="psql -h ${POSTGRES_HOST} -p ${POSTGRES_PORT} -U ${POSTGRES_USER} -v ON_ERROR_STOP=1"

echo "[migrate] waiting for postgres..."
until $PSQL -d postgres -c 'select 1' > /dev/null 2>&1; do
  sleep 1
done

echo "[migrate] ensuring database exists"
$PSQL -d postgres -tc "SELECT 1 FROM pg_database WHERE datname = '${ENTITY_DB_NAME}'" | grep -q 1 || \
  $PSQL -d postgres -c "CREATE DATABASE \"${ENTITY_DB_NAME}\""

# entity-service's own migrations/*.sql files (4-digit, single-file, no
# .up/.down split -- matching operations/csm-sync-service's own convention,
# see entity-service/CLAUDE.md's "Database migrations" section) are its
# source of truth, but ship with no migration tool of their own: `make
# migrate` there tracks against csm_migration_applied_migration on a real
# shared database, which this local compose stack has no equivalent
# connection to. So this script tracks which migrations have actually been
# applied in a per-database schema_migrations table of its own, applying
# only the ones missing from it on every `docker compose up`, rather than
# gating on a single application table's presence (e.g. entity-service's
# "work_item", created partway through the set by an early migration -- a
# later migration failing after that point would leave the schema
# incomplete but still look "migrated" forever after, since work_item
# already exists).
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

# entity-service/migrations currently carries TWO Team Schedule chains side by
# side (a merge collision upstream). The consolidated chain -- 0153_team_schedule_tables,
# 0154_team_schedule_catalogue, 0155_team_schedule_audit -- creates the
# team_schedule_* tables and, on a fresh database, first DROPs the un-prefixed
# schedule_* tables the older chain built. The older chain's remaining files
# (0152_team_schedule_tables .. 0168_team_schedule_audit, listed below) then
# ALTER those dropped tables and fail, so a fresh database can never finish
# migrating. They are superseded by the consolidated chain, so skip them here.
# Remove this list once the stale files are deleted from entity-service/migrations.
is_superseded() {
  case "$1" in
    0152_team_schedule_tables|0153_team_schedule_catalogue|0154_team_schedule_vocabulary|    0155_schedule_customer_allocation_split|0156_schedule_shift_is_rotation|    0157_schedule_shift_required_headcount|0158_schedule_assignment_activity|    0159_schedule_absence_activity|0160_schedule_absence_kind_consolidation|    0161_schedule_americas_weekend_names|0162_schedule_remove_invented_windows|    0163_schedule_rotation_short_codes|0164_schedule_integrity_constraints|    0165_schedule_team_key_catalogue|0166_schedule_absence_bucket_enum|    0167_team_schedule_table_prefix|0168_team_schedule_audit) return 0 ;;
  esac
  return 1
}

apply_pending_migrations() {
  db="$1"; dir="$2"
  ensure_migrations_table "$db"
  # Ordering -- must match entity-service/scripts/migration_order.sh, the
  # canonical order `make migrate` and the schema bootstrap use; it is
  # duplicated here, not called, because this container only mounts
  # entity-service/migrations. Change both together.
  #
  # The legacy 6-digit 000NNN_*.up.sql stragglers depend on objects the
  # 4-digit files create (000085 needs announcement_type_enum from 0088;
  # 000087 triggers on outage from 0083), so they cannot run first -- and
  # leading zeros would sort them first. They cannot run last either: 0149
  # replaces 000085's announcement_visibility policy, so 000085 run after it
  # fails with "policy already exists". So: 4-digit files below
  # STRAGGLERS_BEFORE, then the 6-digit *.up.sql files, then the remaining
  # 4-digit files, each group in version order. *.down.sql files are
  # rollbacks and are never applied on the way up.
  STRAGGLERS_BEFORE=149
  four_digit="$(ls "${dir}"/[0-9][0-9][0-9][0-9]_*.sql 2>/dev/null | LC_ALL=C sort -V)"
  files="$(
    for f in $four_digit; do n="$(basename "$f")"; n="${n%%_*}"; [ "$n" -lt "$STRAGGLERS_BEFORE" ] && echo "$f"; done
    ls "${dir}"/[0-9][0-9][0-9][0-9][0-9][0-9]_*.up.sql 2>/dev/null | LC_ALL=C sort -V
    for f in $four_digit; do n="$(basename "$f")"; n="${n%%_*}"; [ "$n" -ge "$STRAGGLERS_BEFORE" ] && echo "$f"; done
    true
  )"
  for f in $files; do
    version="$(basename "$f" .sql)"
    if is_superseded "$version"; then
      echo "[migrate]   skipping superseded $version"
      continue
    fi
    already="$($PSQL -d "$db" -tAc "SELECT 1 FROM schema_migrations WHERE version = '${version}'")"
    if [ "$already" != "1" ]; then
      echo "[migrate]   applying $f"
      if grep -qiE '(CREATE|DROP) INDEX CONCURRENTLY' "$f"; then
        # CONCURRENTLY cannot run inside a transaction block, so a migration
        # using it (e.g. 0152_work_item_type_updated_on_index.sql) is applied
        # without -1, then recorded separately. Such files are written
        # idempotent (IF NOT EXISTS), so a failure between the two steps just
        # retries harmlessly.
        $PSQL -d "$db" -f "$f"
        $PSQL -d "$db" -c "INSERT INTO schema_migrations (version) VALUES ('${version}')"
      else
        tmp="$(mktemp)"
        cat "$f" > "$tmp"
        printf "\nINSERT INTO schema_migrations (version) VALUES ('%s');\n" "$version" >> "$tmp"
        $PSQL -d "$db" -1 -f "$tmp"
        rm -f "$tmp"
      fi
    fi

    # A fixture named for this migration runs straight after it, in the same
    # pass. Some migrations backfill rows that only ServiceNow supplies and
    # RAISE EXCEPTION when they are absent -- fatal on a fresh local database,
    # and fatal for every migration queued behind them. A fixture supplies
    # those rows at the moment they first become insertable, so the real
    # migration runs unmodified. Local dev only; nothing reads /migrations/
    # fixtures outside this compose stack.
    #
    # A fixture is recorded as its own row, `fixture:<version>`, and checked
    # independently of its migration. Otherwise a fixture that failed after
    # its migration was recorded would never be retried, and a volume whose
    # migration ran before the fixture existed would never get it -- leaving
    # the later migration that needs its rows failing on every run. Still run
    # here, inside the loop, so it lands before any later migration does.
    # Fixtures must therefore be safe to re-run (ON CONFLICT DO NOTHING).
    fixture="/migrations/fixtures/${version}.sql"
    if [ -f "$fixture" ]; then
      loaded="$($PSQL -d "$db" -tAc "SELECT 1 FROM schema_migrations WHERE version = 'fixture:${version}'")"
      if [ "$loaded" != "1" ]; then
        echo "[migrate]     + fixture ${version}.sql"
        tmp="$(mktemp)"
        cat "$fixture" > "$tmp"
        printf "\nINSERT INTO schema_migrations (version) VALUES ('fixture:%s');\n" "$version" >> "$tmp"
        $PSQL -d "$db" -1 -f "$tmp"
        rm -f "$tmp"
      fi
    fi
  done
}

echo "[migrate] applying entity-service migrations (if any are pending)"
apply_pending_migrations "${ENTITY_DB_NAME}" /migrations/entity-service

echo "[migrate] loading entity-service seed data"
$PSQL -d "${ENTITY_DB_NAME}" -f /migrations/seed-entity-service.sql

# The Team Schedule roster: teams, engineers and a rota either side of today.
# Separate from the base seed because it is the only seed that depends on the
# schedule_* tables, and because it is the one a developer is likely to want to
# re-run on its own while working on the rota.
echo "[migrate] loading Team Schedule roster"
$PSQL -d "${ENTITY_DB_NAME}" -f /migrations/seed-team-schedule.sql

echo "[migrate] done"
