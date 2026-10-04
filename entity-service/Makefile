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

.PHONY: build test lint migrate migrate-order schema-bootstrap-sql

SHELL := /bin/bash
.ONESHELL:

build:
	go build ./...

test:
	go test ./...

lint:
	go vet ./...

# Applies the migrations listed by scripts/migration_order.sh, in that order
# (not a plain migrations/*.sql glob: see that script for why), tracked in
# csm_migration_applied_migration - same convention, same loop shape, as
# operations/csm-sync-service's own `make migrate`, since both services
# migrate the same shared database and must stay interchangeable here.
# Every migration file is plain SQL with no tracking statement of its own -
# this target records each one itself, in a separate statement, after that
# file's own statements succeed. Safe to re-run: a file already recorded is
# skipped.
migrate:
	set -a; [ -f .env ] && . ./.env; set +a; \
	if [ -n "$$DATABASE_URL" ]; then \
		conn="$$DATABASE_URL"; \
	else \
		export PGHOST="$$DB_HOST" PGPORT=$${DB_PORT:-5432} PGUSER="$$DB_USER" PGPASSWORD="$$DB_PASSWORD" PGDATABASE="$$DB_NAME" PGSSLMODE=$${DB_SSLMODE:-require}; \
		conn=""; \
	fi; \
	psql $$conn -v ON_ERROR_STOP=1 -c "CREATE TABLE IF NOT EXISTS csm_migration_applied_migration (filename TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now());" || exit 1; \
	files=$$(./scripts/migration_order.sh) || exit 1; \
	for f in $$files; do \
		name=$$(basename "$$f"); \
		name_escaped=$$(printf '%s' "$$name" | sed "s/'/''/g"); \
		applied=$$(psql $$conn -v ON_ERROR_STOP=1 -tAc "SELECT 1 FROM csm_migration_applied_migration WHERE filename = '$$name_escaped';"); \
		if [ "$$applied" = "1" ]; then \
			echo "skipping $$f (already applied)"; \
			continue; \
		fi; \
		echo "applying $$f"; \
		psql $$conn -v ON_ERROR_STOP=1 -f "$$f" || exit 1; \
		psql $$conn -v ON_ERROR_STOP=1 -c "INSERT INTO csm_migration_applied_migration (filename) VALUES ('$$name_escaped');" || exit 1; \
	done

# Dry run: prints the files `migrate` would consider, in apply order.
migrate-order:
	./scripts/migration_order.sh

# Combines the migrations (same list and order as `migrate`) into one file (dist/schema_bootstrap_<timestamp>.sql)
# for a manual run against a DB that has never run any of these before -
# first-time prod, or a staging reset. Mirrors
# operations/csm-sync-service's own script of the same name and output
# shape (including --since/--from/--to for a delta), so both repos' bootstrap
# files stay directly comparable. See scripts/generate_schema_bootstrap.sh.
schema-bootstrap-sql:
	./scripts/generate_schema_bootstrap.sh
