# Query-hour port — test plan

The Go port of ServiceNow's `[Query Hour] UpdateTime Card` flow, plus the
`Set Project Query Hour State` and `Consumed Query Hour Update` business
rules, which between them did this work in three places.

Everything below was executed end to end on 2026-09-23 against a throwaway
PostgreSQL 15 cluster with **all 84 real csm-sync-service migrations** applied,
not a hand-written schema.

## What is in scope

| Piece | Where |
|---|---|
| `project_query_hours` table | `migrations/000084_create_project_query_hours.{up,down}.sql` |
| Aggregation + staleness queries | `internal/repository/query_hour_repo.go` |
| Thresholds, push decision, sweep | `internal/service/query_hour_service.go` |
| Outbound Choreo client | `internal/choreo/subscription_closure.go` |
| HTTP surface | `internal/handler/query_hour_handler.go`, `openapi.yaml` |
| Hourly sub-cron | `operations/csm-scheduled-tasks/internal/queryhours/` |

**The entitlement maths IS ported.** `sf_opportunity`, `sf_opportunity_product`
and `sf_opportunity_link` are now mirrored into Postgres by csm-sync-service
(digiops-cs migration 0080), and both `Consumption` and `WeeklyReportRows` in
`query_hour_repo.go` derive the entitlement from them — quantity x pack size,
over the lines whose service window covers today. An earlier version of this
plan said the opposite, from before those tables existed; it was wrong from the
moment 0080 merged, which matters because it is the paragraph a reader reaches
for at cutover.

What remains unported from `[Query Hour] Update Opportunity Line` is only its
**write-side trigger** — ServiceNow recomputes a project the moment an
opportunity line changes, whereas here the hourly sweep picks it up within the
hour. On a database without those tables the entitlement degrades to
ServiceNow's synced figure rather than failing, so a deployment where 0080 has
not been applied still works, just with the older number.

## Unit tests

```sh
go test ./internal/service/ -run 'TestQueryHourStateFor|TestRecompute|TestSweep' -v
cd ../operations/csm-scheduled-tasks && go test ./internal/queryhours/ -v
```

21 tests. The ones that pin the deliberate divergences from ServiceNow are
`TestRecompute_StateWalksBackDownWhenConsumptionDrops` and
`TestRecomputeForTimeCard_ScopesToTheCardsOwnProject`.

Note: `TestSNCaseService_CreateCase_PublishesCaseCreated` fails in
`internal/service`. That is pre-existing on this branch and on
`dev-app-csm-portal`, and unrelated to this work.

## End-to-end

### 1. A throwaway database

The Unix socket path limit (103 bytes) rules out deep temp directories, so use
a short one.

```sh
PGBIN=/opt/homebrew/opt/postgresql@15/bin
export LANG=C LC_ALL=C
BASE=/tmp/cqh; rm -rf $BASE; mkdir -p $BASE/data
$PGBIN/initdb -D $BASE/data -U postgres --auth=trust --locale=C --encoding=UTF8
$PGBIN/pg_ctl -D $BASE/data \
  -o "-p 55432 -k $BASE -c listen_addresses=127.0.0.1" -l $BASE/pg.log start
psql -h 127.0.0.1 -p 55432 -U postgres -c "CREATE DATABASE csm;"
psql -h 127.0.0.1 -p 55432 -U postgres -c "ALTER USER postgres WITH PASSWORD 'postgres';"
```

### 2. Schema — the real migrations, in order

```sh
SYNC=<digiops-cs>/operations/csm-sync-service/migrations
for f in $(ls $SYNC/*.sql | sort); do
  psql -h 127.0.0.1 -p 55432 -U postgres -d csm -v ON_ERROR_STOP=1 -q -f "$f"
done
psql -h 127.0.0.1 -p 55432 -U postgres -d csm -v ON_ERROR_STOP=1 \
  -f migrations/000084_create_project_query_hours.up.sql
```

All 84 apply cleanly, then 000084.

### 3. Seed

```sh
psql -h 127.0.0.1 -p 55432 -U postgres -d csm -v ON_ERROR_STOP=1 \
  -f testdata/query_hours_seed.sql
```

Five projects **on one shared account** — that is what makes the removal of
the account-wide fan-out observable — plus a SUBMITTED 9000-minute card that
must never be counted.

### 4. A mock Choreo

The port pushes to Choreo Sales Operations. Any server that accepts
`PUT /subscriptions/{sfId}/closure-state` and returns 2xx will do; the one used
for this run also exposes `GET /` to read back every push it received.

### 5. Run it

```sh
DATA_SOURCE=postgres \
DB_HOST=127.0.0.1 DB_PORT=55432 DB_USER=postgres DB_PASSWORD=postgres \
DB_NAME=csm DB_SSLMODE=disable SERVER_PORT=8091 \
QUERY_HOUR_CHOREO_BASE_URL=http://127.0.0.1:8099 \
QUERY_HOUR_CHOREO_API_KEY=test-key \
go run ./cmd/api
```

Leave `QUERY_HOUR_CHOREO_BASE_URL` unset to disable pushing without disabling
the recompute — safe by default, the same way `EVENT_PUBLISHING_ENABLED` is.

## Results, as observed

### Thresholds

`POST /projects/{id}/query-hours/recompute` on each seed project:

| Scenario | Entitlement | Consumed | Billable | Non-bill | % | State |
|---|---|---|---|---|---|---|
| NORMAL | 6000 | 600 | 600 | 0 | 10.0 | 0 |
| WARNING | 6000 | 4500 | 4000 | 500 | 75.0 | 1 |
| CRITICAL | 6000 | 5400 | 5400 | 0 | 90.0 | 2 |
| EXCEEDED | 6000 | 7500 | 7500 | 0 | 125.0 | 3 |
| NOQUOTA | 0 | 300 | 300 | 0 | 0.0 | 0 |

75 and 90 are hit exactly, confirming the inclusive boundaries. NORMAL reports
600 and not 9600, so the `state = 'APPROVED'` filter holds against the
SUBMITTED decoy. NOQUOTA returns 0% rather than dividing by zero — ServiceNow
skipped such projects entirely.

### Choreo payload

```json
PUT /subscriptions/a0d000000000000002/closure-state
{"consumedQueryTime":4500,"totalQueryTime":6000}
```

Field names and minute units match what ServiceNow's business rule already
sends, so the receiving service needs no change at cutover.

### The two deliberate divergences

**State walks back down.** SEED-EXCEEDED at 125% (state 3); the card is
recalled to 3000 minutes (50%); recompute returns `state=0`,
`stateChanged=true`, and pushes. ServiceNow's `if (newState > 0 && ...)` would
have left it at 3 forever.

**No account-wide fan-out.** All five projects backdated to stale, then
`POST /time-cards/{id}/query-hours/recompute` for a SEED-WARNING card:

```
SEED-WARNING  | RECOMPUTED
SEED-CRITICAL | untouched
SEED-EXCEEDED | untouched
SEED-NOQUOTA  | untouched
SEED-NORMAL   | untouched
```

ServiceNow would have recomputed all five.

### Idempotency, failure, retry

- Recompute with nothing changed: `pushed=false`, Choreo call count unchanged.
- Choreo stopped, then recompute: **HTTP 200**, position stored
  (`consumed=9700, state=3`), `last_pushed_state` left at the old `2`,
  `pushError` populated. The recompute does not fail because the push did.
- Choreo restarted, recompute again: `stateChanged=false` but `pushed=true` —
  the pending push is retried because `last_pushed_state != query_hour_state`.

### Sweep

`POST /query-hours/sweep?staleForMinutes=60&limit=50` → `requested=5
succeeded=5 failed=0`. Immediately again → `requested=0`: nothing stale, no
redundant work.

### Error paths

| Request | Status |
|---|---|
| unknown project | 404 |
| unknown time card | 404 |
| `limit=0` | 400 |
| `limit=99999` | 400 |

### The constraint that shaped the design

After every operation above:

```
SEED-WARNING | total_query_duration=100:00:00 | consumed_duration=NULL
```

`project.consumed_duration` is still NULL. That column belongs to
csm-sync-service and mirrors ServiceNow; anything Go wrote there would be
overwritten by the next sync. The port therefore keeps its own derived row in
`project_query_hours` and never writes `project`.

## Cutover

Per the double-fire rule, registering the sub-cron must, in the same change,
deactivate **all four** ServiceNow counterparts:

1. flow `[Query Hour] UpdateTime Card`
2. flow `Query Hour[Time Card Update]` (its subflow)
3. business rule `Set Project Query Hour State`
4. business rule `Consumed Query Hour Update` — this one also pushes to
   Choreo, so leaving it on means two systems telling Choreo different things

`Time card approved  :query hours closure` (note the double space) is already
`active=0` and needs nothing.

## Teardown

```sh
/opt/homebrew/opt/postgresql@15/bin/pg_ctl -D /tmp/cqh/data stop
rm -rf /tmp/cqh
```
