# Lower-Environment Test Plan and Verification Record

Following current cs-tools convention, this repo does not gate PRs on automated
tests. Verification is performed manually against a Choreo lower environment
before merging. This document defines **what** must be verified and **how**,
and records **who verified it and when**.

The plan is exhaustive for the initial deploy; on subsequent PRs, verify only
the sections whose surface area has changed.

---

## Verification record

Fill this out before opening the PR.

| Field | Value |
|---|---|
| Environment | `<dev / staging>` |
| Choreo URL | `https://<...>.choreoapis.dev/api/v1` |
| Git commit tested | `<sha>` |
| Verified by | `<initials>` |
| Verification date | `<YYYY-MM-DD>` |
| ServiceNow instance | `<sn-instance-URL>` |
| Notes / deviations | `<free text — anything that didn't work as expected>` |

---

## 1. Ingestion

- [ ] SN scheduled job `MTTR - Push Resolved Cases` runs on schedule (every 10 min)
- [ ] After a run, `u_mttr_last_sync` system property advances (only when HTTP 200)
- [ ] After a run, a row appears in `ingestion_log` matching the batch (`GET /admin/ingestion-logs`)
- [ ] `POST /cases/batch` with a 500-record payload → HTTP 200 with counts matching DB
- [ ] `POST /cases/batch` with an invalid record (missing `case_sys_id`) → `rejected` count = 1 in response
- [ ] `POST /cases/batch` with 501 records → HTTP 400 `"cases must be an array of 1-500 items"`
- [ ] Same `case_sys_id` sent twice → first insert (`is_insert=true`), second update — no duplicate row
- [ ] `POST /cases/bulk-import` with a non-admin token → HTTP 403 `"Insufficient permissions"`
- [ ] `POST /cases/bulk-import` with an `mttr-admin` token + 5000 records → HTTP 200
- [ ] `POST /cases/bulk-import` with 5001 records → HTTP 400
- [ ] Ship a case whose `sys_updated_on` matches the last-shipped timestamp exactly → the case still lands (validates `>=` watermark)

## 2. Aggregation & MTTR reads

- [ ] `POST /admin/cache/reset` (no query param) → all 10 dimensions populated in `mttr_cache`
- [ ] `POST /admin/cache/reset?type=team_incidents` → only that dimension recomputed
- [ ] `POST /admin/cache/reset?type=<garbage>` → HTTP 400 with the accepted-types list
- [ ] `GET /mttr?type=<each of the 10 dimensions>` → each returns rows; `mttr_hours` spot-checked against manual query on `case_events`
- [ ] `GET /mttr?type=<unknown>` → HTTP 400
- [ ] `GET /mttr/types` → returns the dimension whitelist
- [ ] Fresh deploy (empty cache) + 5 concurrent `GET /mttr?type=X` from different terminals → logs show **one** `"Cache miss, computing aggregation on-the-fly"` and four `"Cache miss, awaiting in-flight aggregation"` for the same dimension (validates single-flight)

## 3. Historical summaries

- [ ] `POST /admin/retention/run` (with populated `case_events`) → `case_events_summary` rows appear
- [ ] `GET /summary/historical?case_type=Incident&group_by=team` → `{periods, series}` shape, periods sorted `YYYY-Qn`
- [ ] `GET /summary/historical?case_type=Incident&group_by=priority` → grouped by P1–P4
- [ ] `GET /summary/historical?case_type=Incident&group_by=product` → grouped by product
- [ ] `GET /summary/historical?case_type=Incident&group_by=<unknown>` → **HTTP 400 with the accepted-list message** (validates the `.has()` guard added on top of `.isIn()`)
- [ ] `GET /summary/historical` without `case_type` → HTTP 400
- [ ] `GET /summary/periods` → distinct quarters present

## 4. Authentication & authorisation

**Note:** Because the Choreo gateway enforces OAuth, most of these must be exercised **at the gateway**, not at the container. See `docs/design-document.md §11` for the trust-boundary rationale.

- [ ] Call any `/api/*` endpoint (except `/health`) **without** an `Authorization` header → HTTP 401 from the gateway
- [ ] Call with an invalid Bearer → HTTP 401 from the gateway
- [ ] Call `/admin/*` with a valid token that lacks the admin scope/role → HTTP 403
- [ ] Call `/admin/*` with a valid token that has the admin scope/role → HTTP 200
- [ ] Call `/cases/bulk-import` with a non-admin token → HTTP 403 (see §1)
- [ ] Call `/health` without a token → HTTP 200

## 5. Rate limiting

- [ ] Burst >1000 requests from one IP inside a 15-min window → HTTP 429 with `{ error: 'Too many requests, please try again later.' }`
- [ ] `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` headers present on every response
- [ ] Two callers from different real client IPs each get their own counter (validates `trust proxy: 1` — verify by watching `RateLimit-Remaining` diverge between the two)

## 6. Correlation IDs

- [ ] `curl -H "X-Correlation-Id: test-abc123" /api/v1/health` → response header `X-Correlation-Id: test-abc123` echoed back
- [ ] `curl` without the header → response carries a fresh UUID in `X-Correlation-Id`
- [ ] `curl -H "X-Correlation-Id: <newline>evil"` → server ignores the malformed value, echoes a UUID, and emits a WARN log line `"Rejected malformed X-Correlation-Id header"` with the source IP (not the payload)
- [ ] Trigger an SN scheduled job push → grep container log for the batch_id, verify the access log line, `"Batch ingested"`, and any error lines all quote the same `correlationId`
- [ ] Trigger a manual `POST /admin/cache/reset` and grep the log by `req.correlationId` → aggregation lines (`"Aggregated <dim>: N entries"`, `"Full aggregation completed"`) all carry the same ID
- [ ] Trigger any error path (e.g. bad `case_sys_id`) → response body includes `correlation_id`; that value appears in the ERROR log line

## 7. Health & error surface

- [ ] `GET /health` with DB up → HTTP 200 with `{ status: 'healthy', db_time, total_cases, last_ingestion_at, last_aggregation_at, cache_entries }`
- [ ] Kill DB pool / point `DB_HOST` at an unreachable address → HTTP 503 with body `{ status: 'unhealthy', error: 'Service temporarily unavailable' }` — **no driver text, no hostnames, no schema names** leaked
- [ ] Container log for the same event shows the underlying error message + stack (via `logger.error`)
- [ ] Any 500 response includes `correlation_id`; corresponding server log carries the same ID

## 8. Retention

- [ ] `POST /admin/retention/run` → responds `{ ingestion_log_purged, summarised, deleted }`
- [ ] `ingestion_log` rows older than `RETENTION_INGESTION_LOG_DAYS` (default 14) are gone
- [ ] `case_events` rows older than `RETENTION_CASE_EVENTS_MONTHS` (default 24) are gone
- [ ] Quarters older than the retention cutoff have summary rows in `case_events_summary`
- [ ] Re-run retention → idempotent (no new summary rows for quarters already summarised outside the cutoff)

## 9. Data & pool behaviour

- [ ] Kill a Postgres client from the server side (`SELECT pg_terminate_backend(...)`) → next `db.query` in the app succeeds after `DB_POOL_CONNECT_MS` at most, does **not** hang indefinitely
- [ ] Leave the app idle for > `DB_POOL_IDLE_MS` (default 30 s) → next query succeeds cleanly (pool reaped and re-established, no ECONNRESET)

## 10. ServiceNow dashboard end-to-end

- [ ] All 12 SN widgets render numbers (not `"0 Days 0 Hours 0 Minutes"` placeholders)
- [ ] Overall MTTR figures match a manual calculation from `case_events`
- [ ] Historical widgets render with the summarised quarters (§3)
- [ ] "Reset Cache" UI button (if exposed) works end-to-end (SN → Choreo → recompute → refetch)

---

## When any check above fails

Do not merge. Fix the underlying issue, re-verify the affected section only,
and update the record at the top of this file with the new commit SHA and
date. If the failure indicates a design bug (not just a wiring bug), open an
issue and mention the section number here.
