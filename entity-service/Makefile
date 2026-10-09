# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
#
# This software is the property of WSO2 LLC. and its suppliers, if any.
# Dissemination of any information or reproduction of any material contained
# herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
# You may not alter or remove any copyright or other notice from copies of this content.

.PHONY: build test lint migrate schema-bootstrap-sql

SHELL := /bin/bash
.ONESHELL:

build:
	go build ./...

test:
	go test ./...

lint:
	go vet ./...

# Applies migrations/*.sql, tracked in csm_migration_applied_migration - same
# convention, same loop shape, as operations/csm-sync-service's own `make
# migrate`, since both services migrate the same shared database and must
# stay interchangeable here. Every migration file is plain SQL with no
# tracking statement of its own - this target records each one itself, in a
# separate statement, after that file's own statements succeed. Safe to
# re-run: a file already recorded is skipped.
#
# File order is NOT a plain alphabetical glob. Three phases, in order:
#   1. Every 4-digit NNNN_*.sql file (the convention), ascending numeric order.
#   2. Every 6-digit 1NNNNN_*.sql file numbered 100000 or above - the dedicated
#      RLS-migration track (CREATE POLICY/ALTER POLICY/ROW LEVEL SECURITY and
#      the functions that support them), kept in its own number space
#      specifically so it can never collide with, or be mistaken for, phase 1
#      or phase 3. Runs after every schema migration, since an RLS migration
#      always protects a table some 4-digit migration already created - see
#      each 1NNNNN file's own doc comment for why its specific dependencies are
#      satisfied by that ordering alone, with no interleaving needed.
#   3. The legacy 6-digit 0NNNNN_*.up.sql stragglers (numbered below 100000:
#      000020-000105) that predate this whole scheme and still depend on
#      tables the 4-digit files create (e.g. 000087 triggers on `outage`,
#      created by 0083). *.down.sql files are rollbacks and never run here.
# A plain `migrations/*.sql` glob sorts every 6-digit file *before* 0001
# (leading zeros), which both fails outright on a from-scratch database and
# stops a database that already has them applied by hand dead on the first
# untracked one. This mirrors scripts/csm-compose/migrate-and-seed.sh's own
# `apply_pending_migrations` ordering for phases 1 and 3 - see that script's
# own comment for the full reasoning; phase 2 is this repo's own addition, not
# mirrored there since the compose stack has no equivalent RLS/schema split.
#
# Review bundles: every file this run actually applies (not ones it skips as
# already-recorded) is also appended to one of two dist/ files -
# dist/rls_migrations_<ts>.sql for phase 2, dist/schema_migrations_<ts>.sql for
# phases 1 and 3 - so a deploy has a standalone, reviewable artifact of
# exactly what security-policy SQL shipped, separate from everything else.
# Purely a by-product for a human (or a security reviewer) to read: nothing
# re-applies these files, dist/ is gitignored, and a phase with nothing newly
# applied gets no file at all rather than an empty one.
migrate:
	set -a; [ -f .env ] && . ./.env; set +a; \
	if [ -n "$$DATABASE_URL" ]; then \
		conn="$$DATABASE_URL"; \
	else \
		export PGHOST="$$DB_HOST" PGPORT=$${DB_PORT:-5432} PGUSER="$$DB_USER" PGPASSWORD="$$DB_PASSWORD" PGDATABASE="$$DB_NAME" PGSSLMODE=$${DB_SSLMODE:-require}; \
		conn=""; \
	fi; \
	psql $$conn -v ON_ERROR_STOP=1 -c "CREATE TABLE IF NOT EXISTS csm_migration_applied_migration (filename TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now());" || exit 1; \
	mkdir -p dist; \
	ts=$$(date -u +%Y%m%d_%H%M%S); \
	rls_bundle="dist/rls_migrations_$${ts}.sql"; \
	other_bundle="dist/schema_migrations_$${ts}.sql"; \
	rls_count=0; other_count=0; \
	files=$$( { ls migrations/[0-9][0-9][0-9][0-9]_*.sql 2>/dev/null | sort; \
	            ls migrations/1[0-9][0-9][0-9][0-9][0-9]_*.sql 2>/dev/null | sort; \
	            ls migrations/0[0-9][0-9][0-9][0-9][0-9]_*.up.sql 2>/dev/null | sort; } ); \
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
		case "$$name" in \
			1[0-9][0-9][0-9][0-9][0-9]_*.sql) \
				{ echo "-- $$name"; cat "$$f"; echo; } >> "$$rls_bundle"; \
				rls_count=$$((rls_count + 1)) ;; \
			*) \
				{ echo "-- $$name"; cat "$$f"; echo; } >> "$$other_bundle"; \
				other_count=$$((other_count + 1)) ;; \
		esac; \
	done; \
	if [ "$$rls_count" -gt 0 ]; then echo "RLS review bundle: $$rls_bundle ($$rls_count file(s))"; else rm -f "$$rls_bundle"; fi; \
	if [ "$$other_count" -gt 0 ]; then echo "Schema review bundle: $$other_bundle ($$other_count file(s))"; else rm -f "$$other_bundle"; fi

# Combines migrations/*.sql into one file (dist/schema_bootstrap_<timestamp>.sql)
# for a manual run against a DB that has never run any of these before -
# first-time prod, or a staging reset. Mirrors
# operations/csm-sync-service's own script of the same name and output
# shape (including --since/--from/--to for a delta), so both repos' bootstrap
# files stay directly comparable. See scripts/generate_schema_bootstrap.sh.
schema-bootstrap-sql:
	./scripts/generate_schema_bootstrap.sh
