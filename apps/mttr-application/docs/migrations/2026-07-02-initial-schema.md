# 2026-07-02 — Initial schema

## Context

First deployment of the MTTR application. This migration installs the four
tables the service reads and writes. There is no prior schema — this is
the baseline every future migration will `ALTER` from.

## Intent

- `case_events` — raw closed / resolved ServiceNow cases pushed in from
  the SN scheduled job. `case_sys_id` is the natural UPSERT key.
- `mttr_cache` — pre-computed P95-truncated-mean aggregations, one row
  per (dimension × group). Rebuilt nightly by the aggregation cron.
- `ingestion_log` — audit trail for each batch pushed by SN. 14-day
  retention (see `RETENTION_INGESTION_LOG_DAYS`).
- `case_events_summary` — quarterly summaries used by the historical
  dashboard endpoints. Populated by the retention job; the only place
  MTTR history survives once raw `case_events` are aged out past
  `RETENTION_CASE_EVENTS_MONTHS`.

Indexes cover the hot query paths documented in
`docs/logic-and-architecture.md`:

- `idx_case_events_state_closed` — supports every aggregation query
  (WHERE `case_state IN (...) AND closed_date BETWEEN ...`).
- `idx_ces_unique_slice` — enforces one summary row per
  (period, case_type, priority, cs_team, product, is_patched); powers
  the retention job's ON CONFLICT clause.

## Changed files

- **New**: `sql/init.sql` — full schema (CREATE TABLE IF NOT EXISTS +
  supporting indexes). Idempotent; safe to re-run.
- **New**: `src/scripts/initDb.js` — one-off runner invoked via
  `npm run db:init`.

## How to apply

```bash
# Local
psql -h localhost -U mttr_user -d mttr_db -f sql/init.sql
# or
npm run db:init

# Choreo — the same SQL is applied via the Choreo Postgres console
# against the managed database in each environment.
```

## Revert steps

Because every statement in `sql/init.sql` uses `CREATE ... IF NOT EXISTS`,
this migration is safe to re-run but cannot be automatically reverted.
To fully undo (destroys all data):

```sql
DROP TABLE IF EXISTS case_events_summary;
DROP TABLE IF EXISTS mttr_cache;
DROP TABLE IF EXISTS ingestion_log;
DROP TABLE IF EXISTS case_events;
```

For a partial rollback on a live environment, take a `pg_dump` of the
four tables before applying and restore from it. There is no `DOWN`
script.

## Verification

After applying, `GET /api/v1/health` should return 200 with
`total_cases: 0` and `cache_entries: 0` on a fresh database. If any
table is missing the endpoint returns 503 with the underlying SQL
error captured in the container log.

## Future migrations

**`sql/init.sql` is frozen as the baseline schema.** Once this migration has
been applied to any live environment, nobody edits `sql/init.sql` in place —
that file exists only to bootstrap a brand-new database. Every schema change
from now on lands as **two matching files**:

- `docs/migrations/YYYY-MM-DD-<short-name>.md` — context, intent, changed files, apply steps, revert plan (same shape as this file).
- `sql/migrations/YYYY-MM-DD-<short-name>.sql` — the `ALTER TABLE` / `CREATE INDEX` / etc. that the operator runs against each environment.

The date prefix keeps the folder listing chronological. Migrations are applied in that order and are expected to be idempotent (`IF EXISTS` / `IF NOT EXISTS` guards) so a partial re-run is safe.

If you find yourself wanting to edit `sql/init.sql` after this file exists, stop
and write a new migration instead. Editing the frozen baseline will silently
diverge new-environment installs from long-lived ones.
