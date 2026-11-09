# SLA clocks — storage, pause/resume, breach

How entity-service stores and drives SLA clocks. Breach *alerting* (Chat/email) is not done here; see
[csm-notification-service spec, section 3.2.1](../../integrations/csm-notification-service/docs/csm-notification-service-spec.md).

## Storage

- **`sla_policy`** (migration `0047`) — the rules. One row per policy: `target` (`RESPONSE` / `WORKAROUND` /
  `RESOLUTION`), `duration`, `schedule` (a calendar name; `WSO2 Business days` skips weekends, anything else is
  wall-clock), and five condition columns: `start_condition`, `stop_condition`, `pause_condition`,
  `resume_condition`, `cancel_condition`. `when_to_cancel` / `when_to_resume` say how cancel/resume are decided
  (see below).
- **`sla`** (migration `0048`) — one row per clock instance: `work_item_id`, `sla_policy_id`, `stage`
  (`IN_PROGRESS`, `PAUSED`, `ACHIEVED`, `BREACHED`, `CANCELLED`, `COMPLETED`), `is_active`, `start_on`, `duration`,
  `paused_on`, `pause_duration` (completed pause segments), `has_breached`, elapsed/remaining columns.
  A work item can have several rows per target (two matching policies, or a reset); readers take the latest.
- **`sla_live`** (view, migrations `0204`/`0245`/`0272`) — the read path. It computes elapsed %, remaining time,
  `has_breached` and stage at query time (wall-clock since `start_on` minus `pause_duration`), so nothing writes
  a recompute on a timer. All SLA reads go through it.

## Conditions

Condition columns use a small predicate language (`internal/slacondition`): `^` is AND, `^OR` ORs a clause with
the previous one, `^NQ` starts a new OR'd group. It is evaluated against the case's current facts (state,
priority, case type, project type, product, whether a CS engineer has replied, workaround provided, fix ETA
shared, has-account, created date). An unknown field never matches and never panics.

## Lifecycle (`internal/service/sla_engine_service.go`)

| Trigger | What happens |
|---|---|
| Case created | Every active policy whose `start_condition` matches registers a clock (`RegisterClock`; unique per work item + policy). |
| Severity / case-type change | Existing clocks are cancelled and fresh ones registered (`ReviseClocks`) — a new `start_on`. |
| State change or field change | Each open clock is evaluated against its **own** policy, in this order: `stop_condition` true → complete; cancel rule → cancel; `pause_condition` true → pause; resume rule → resume. Then any policy that newly matches is registered. |
| Case closed | Every open clock is force-completed, as a safety net on top of the stop condition. |
| Qualifying CS-engineer comment | Completes the RESPONSE clock. |

Cancel rule (`when_to_cancel`): `CANCEL_CONDITIONS_ARE_MET` (default) cancels when `cancel_condition` is true;
`START_CONDITIONS_ARE_NOT_MET` cancels when `start_condition` stops matching; `NEVER` never cancels.
Resume rule (`when_to_resume`): `RESUME_CONDITIONS_ARE_MET` (default) resumes when `resume_condition` is true;
`PAUSE_CONDITIONS_ARE_NOT_MET` resumes when `pause_condition` stops matching.

## Pause

Pausing sets `stage = PAUSED` and `paused_on`. Resuming adds the finished segment to `pause_duration` and clears
`paused_on`, so elapsed time = wall-clock since `start_on` minus pauses. Business-day clocks do the same
arithmetic on the business calendar (`internal/businessdays`, `America/New_York`).

## Breach

A clock is breached when elapsed time reaches `duration` (`has_breached`, stage `BREACHED`). `BREACHED` stays
`is_active = TRUE` — a clock that ran out but is not yet satisfied is not finished. `ACHIEVED` and `CANCELLED`
set `is_active = FALSE`.

## Endpoints used by the notification service

- `GET /sla-status` — every active clock (internal callers only; page size default 500, max 2000). Used for the
  startup reconcile.
- `GET /sla-status/clock-state?workItemId=&target=` — the most recent row for one clock regardless of
  `is_active`, with `Found=false` when none exists. Used to re-verify a clock right before alerting.

## Caveats

- Every engine call is best-effort: a failure is logged and never fails the case request.
- `case_type` can only be `case`, `engagement`, `service_request`, `security_report_analysis` or `announcement`;
  a policy conditioned on a hosting type never matches.
- Service requests have SLA clocks but are UI-only; the notification service sends no breach alert for them.
