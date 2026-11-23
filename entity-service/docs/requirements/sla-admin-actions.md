# SLA admin actions — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check
are intended; they are collected in section 6.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related:** [sla-clocks.md](sla-clocks.md), [sla-status-and-reporting.md](sla-status-and-reporting.md).

---

## 1. Purpose

Clocks and alert tracking are automatic, but two things can drift: the notification
service's own copy of what is running, and a clock's stored paused time. The CSM portal
gives admins three actions to see and repair that. This document covers what the service
guarantees for each, and who may use them.

| Action | Route | What it does |
|---|---|---|
| View the SLA engine's running clocks | `GET /sla-status?admin=true` | Lists active clocks, with filters (section 3). |
| Rebuild alert tracking | `POST /sla-engine/recalculate` | Asks the notification service to rebuild its tracking from the stored clocks (section 4). |
| Repair one case's paused time | `POST /cases/{id}/sla/recalculate` | Corrects a case's stored paused-time totals from its state history (section 5). |

## 2. Who may use them

**SLA-410** All three actions shall be available **only through the CSM portal backend**,
for a **signed-in person**. The service shall check three things, in this order: the caller
is an internal caller; the call carries a signed-in person's identity (a service credential
alone is refused); and the caller is the CSM portal backend itself, identified by its
configured client id. Anything else is refused as forbidden, with a message that names the
action.
*Check:* a token that did not come through the portal is refused even if it holds an admin
role; a call through the portal with a signed-in person is allowed.

**SLA-411** The service shall **not** check the person's admin role a second time. Whether
the person is a CSM admin is decided once, by the CSM portal backend, before it calls. A
second list of admin roles in this service could silently disagree with the first, and did:
a real admin was refused by an unset copy.

**SLA-412** If the portal backend's client id is not configured, nobody shall be let
through.

**SLA-413** The plain, non-admin reads of the SLA data ([sla-status-and-reporting.md](sla-status-and-reporting.md))
are unaffected: internal services keep using them as before.

## 3. The admin view of the active-clock list

**SLA-420** A request for the active-clock list with the admin flag set to `true` shall
return the same list, with the same filters and paging, but be gated as in SLA-410. Any
other value, or no flag, is the ordinary internal read.
*Check:* the same call without the flag from an internal service still works.

## 4. Rebuild alert tracking

**SLA-430** An admin shall be able to ask for the notification service's SLA tracking to
be rebuilt from the stored clocks and policies. It exists to recover from a tracking state
that no longer agrees with reality.

**SLA-431** The request shall take no input. The service shall publish one
*recalculation requested* signal, recording who asked (their email) and when (UTC), and
answer "accepted" with no content.

**SLA-432** "Accepted" means **requested, not done**. The rebuild happens afterwards, in
the notification service. The stored clocks themselves are not changed by this action.

**SLA-433** If the service cannot publish signals in this deployment, the request shall be
refused as unavailable, with a message saying so, and never reported as accepted. If the
publish itself fails, the request fails.

**SLA-434** Every request shall use the same fixed key, because the rebuild is not about
any one case.

**SLA-435** The requester's email shall travel in the signal for the audit trail only. It
shall not be written to the service's own log.

## 5. Repair one case's paused time

A clock's total paused time can be wrong, for example when state changes were made while
the engine was not recording them. This rebuilds it from the case's own history.

**SLA-440** An admin shall be able to ask for one case's open clocks to have their stored
paused time recomputed. The case id must be valid, or the request is refused. The action
exists only when the SLA engine is enabled. The request takes no input.

**SLA-441** The service shall read the case's recorded **state changes** (and its severity
and type changes) and replay them. A case with **no recorded state changes** has nothing
to replay: the answer says there is no history, and no clock is touched.
*Check:* a case with no history returns "no history" and its clocks are unchanged.

**SLA-442** Only clocks that are **open** (in progress or paused) shall be considered. A
finished clock is final and is reported as skipped.

**SLA-443** Each clock shall be replayed with the **same decision order the live engine
uses** ([sla-clocks.md](sla-clocks.md), SLA-040), so the repair can never disagree with
what the engine would have done for the same history.

**SLA-444** A clock shall be **skipped, never guessed at**, and the answer shall say why,
when any of these is true:

- a severity or type change happened after the clock started, so the clock may be a stale
  leftover;
- its pause or resume rule depends on something other than state (for example a
  workaround), which the history cannot show;
- a recorded state is not one the service recognises;
- the history says the clock should already have stopped or been cancelled, or at some
  point should have stopped, while the engine still treats it as open;
- the history disagrees with the clock's current stage (history says paused but the clock
  runs, or the other way round);
- a correction was worked out but could not be saved.

**SLA-445** A correction shall be written only when the recomputed paused time differs
from the stored one by **more than 2 seconds**, so ordinary rounding does not create noise.

**SLA-446** For a clock on the **WSO2 Business days** schedule the business-time paused
total shall be recomputed and corrected as well, on its own 2-second test.

**SLA-447** For a clock that is **paused at the moment**, its frozen figures (percentage
elapsed, time elapsed, time left) shall be recomputed together with the paused total, so
the clock is never left inconsistent.

**SLA-448** The answer shall list every open clock with: its kind, its schedule, whether
it was skipped and why, the old and new paused seconds, the old and new business paused
seconds (business-days clocks only), and whether it was updated.

## 6. Open questions for the product owner

1. **No second admin check in this service.** Anyone able to reach the CSM portal backend's
   admin routes can run these actions, because this service trusts that backend. This is a
   deliberate choice, the same as for portal banners, but the single admin check now lives
   only in the portal.
2. **"Accepted" does not mean "rebuilt".** The portal cannot tell whether a rebuild
   succeeded, only that it was requested.
3. **A case id that does not exist** gets the same answer as a case with no history
   ("no history"), not "not found".
4. **Cases changed outside this service leave no history.** The per-case repair only works
   for state changes this service itself recorded, so a case handled through another route
   cannot be repaired this way.

## 7. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Gate (portal backend client id) | `requireCSMPortalBackendCaller` in `internal/service/sla_status_service.go` and `internal/service/case_sla_recalc_access_service.go` |
| Admin view of the list | `GET /sla-status?admin=true`, `internal/service/sla_status_service.go` |
| Rebuild alert tracking | `POST /sla-engine/recalculate`, `RequestRecalculation`; signal `sla.recalculation_requested` in `internal/events/events.go` |
| Per-case repair | `POST /cases/{id}/sla/recalculate`, `internal/service/sla_case_recalc_service.go`, `internal/handler/case_sla_recalc_handler.go` |
| Settings | `CSM_PORTAL_BACKEND_CLIENT_ID`, `CSM_SLA_ENGINE_ENABLED` |
| Tests | `sla_status_service_test.go`, `sla_case_recalc_service_test.go` |
