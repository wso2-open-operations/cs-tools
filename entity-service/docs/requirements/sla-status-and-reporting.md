# SLA status and reporting — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check
are intended; they are collected in section 11.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related:** [sla-clocks.md](sla-clocks.md) (how clocks behave),
[sla-policies.md](sla-policies.md) (which clocks a case gets),
[sla-admin-actions.md](sla-admin-actions.md) (admin repair actions).

---

## 1. Purpose

Clocks are only useful if people and other systems can see them. This document covers every
way the service shows SLA data: the live list of running clocks, the state of a single
clock, the SLA tab on a case, the SLA filters in searches, the weekly breach report, and
the signals that keep breach alerting in step. It also covers a small catalogue lookup,
service offerings, that sits beside the SLA data.

## 2. Who can read SLA data

**SLA-310** The active-clock list (section 4), the single-clock state (section 5), the
SLA tab (section 6), the breach report (section 8) and the duration list
([sla-policies.md](sla-policies.md), section 8) shall be available to **internal callers
only**: internal services and signed-in internal users. A customer or partner is refused,
and a caller whose identity cannot be worked out is reported as such, not as "forbidden".
*Check:* a customer token calling the active-clock list gets a 403.

**SLA-311** The SLA filters in case and incident searches (section 7) are **not** limited
to internal callers by the service itself. They follow the access rules of the search
they are part of. (**Confirm**, 11.1)

## 3. Live figures

**SLA-320** Every figure read here (percentage elapsed, time left, stage, breached) shall
be worked out at the moment of the read. Nothing relies on a timer having updated stored
values.

**SLA-321** For a clock on the **WSO2 Business days** schedule that is still running, the
figures shall be corrected for weekends so that stage, breached flag and percentage never
disagree with each other.
*Check:* a business-day clock read on a Saturday does not report breached while showing a
percentage under 100.

**SLA-322** A paused or finished clock shall show the figures frozen when it paused or
finished, never recomputed from "now".

## 4. The list of active clocks

Used by the CSM portal's SLA admin table, and by the notification service when it starts up.

**SLA-330** The service shall return every **active** clock (in progress, paused or
breached) for case-like items: cases, engagements, service requests, security report
analyses and announcements. Incidents are not in this list.
*Check:* a finished clock never appears; an incident clock never appears.

**SLA-331** When a case has more than one open clock of the same kind, the list shall show
one entry per case and kind: the most recently started (the most recently updated if
start times are missing).

**SLA-332** Each entry shall show:

- the clock: its id, kind, percentage elapsed (not capped), breached, paused, when it
  started, when the current pause began, total paused seconds, allowed seconds, schedule;
- the case: number, WSO2 case id, title, type, engagement type, product with version,
  priority, state, assignee name and email;
- the routing facts: the account's team, team email, team lead's name, the project's
  onboarding status and whether the account is an evaluation account (project type
  "Evaluation Subscription").

**SLA-333** The list shall be ordered by case, then by kind of clock, and paged. The
default page size is **500** and the maximum is **2000**. A larger value shall be refused
with a message naming the limit; a missing, zero or negative value means 500; a negative
offset means 0; a non-numeric limit or offset is refused.

**SLA-334** The list shall accept optional filters. A request without filters behaves
exactly as it did before filters existed. A value outside the allowed set shall be refused
with a message listing the allowed values.

| Filter | Allowed values | Meaning |
|---|---|---|
| `severity` | catastrophic, critical, high, medium, low | The case's severity. Items with no severity, such as service requests, never match. |
| `caseType` | case, engagement, service_request, security_report_analysis, announcement | The type of item. |
| `search` | free text | Matches a case's id exactly, or its case number or WSO2 case id as a part of the text, ignoring letter case. It never searches the title, product or anything else. |
| `elapsedBucket` | under10, 10to20, 20to30, 30to40, 40to50, 50to60, 60to70, 70to80, 80to90, 90to100, over100 | Percentage elapsed in a band. The lower edge is included and the upper edge is not, except 90to100, which includes exactly 100, and over100, which is above 100. |
| `status` | in_progress, paused, breached | `paused` is every paused clock, `breached` is every breached one, and `in_progress` is neither. A clock paused after it ran out matches both `paused` and `breached`. |

**SLA-335** The `elapsedBucket` and `status` filters shall be applied **after** the
weekend correction in SLA-321, so a clock is never filtered on a figure that is wrong. Paging
and the total shall then count only the clocks that matched.

**SLA-336** A request that carries the old `source` parameter shall be accepted and the
parameter ignored.

**SLA-337** A caller who asks for the **admin view** (`admin=true`) gets the same list, but
only through the CSM portal ([sla-admin-actions.md](sla-admin-actions.md), SLA-410).

## 5. The state of one clock

Used by the notification service to re-check a clock just before it raises an alert.

**SLA-340** The service shall return the state of one clock, identified by a work item
(case or incident) and a kind of clock (response, workaround or resolution, in any letter
case). Both are required. A work item that is not a valid id, or a kind outside those
three, shall be refused with a message saying what is allowed.

**SLA-341** The service shall return the most recently started clock of that kind **even
if it has finished**, so a caller can tell "finished cleanly" from "never started" from
"replaced by a newer clock".

**SLA-342** If no clock exists, the answer shall say so (`found` is false) and nothing else
is filled in. This is a normal answer, not an error.

**SLA-343** A caller may pass a clock's own id. The answer is then about **that exact
clock**; an id that does not belong to that work item and kind is "not found". Leaving it
out gives the most recently started clock.

**SLA-344** The answer shall show: the clock's id, whether it is active, its stage,
breached, paused, started, paused since, total paused seconds, percentage elapsed, allowed
seconds, schedule, the assignee's name and email, the team's email, the team lead's name,
and the project's onboarding status and evaluation-account flag.

**SLA-345** For a project whose onboarding is **not started** or **in progress**, the
answer shall also give the Customer Onboarding team's email, so a breach email can reach
both the team working the case and the team running the onboarding. The case's own team
details stay as they were.

**SLA-346** For an incident, the answer shall also name the incident's current assignment
group and the team that group belongs to.

## 6. The SLA tab on a case

**SLA-350** The service shall list the SLA records of one or more work items and show one
record in full. Both are **internal only**.

**SLA-351** The list may be filtered by one or more work item ids, each a valid id, and
shall be paged. The default page size is 20 and the maximum is 50.

**SLA-352** Each record shall show: the clock's id, the policy's id, name and kind, the
work item, the stage in plain words (for example "In Progress"), percentage elapsed, time
elapsed and time left written as days, hours and minutes (for example "1 Day 2 Hours 5
Minutes"), and start and end times. The weekend correction of SLA-321 applies.

**SLA-353** The full record shall add: whether the clock is active, the schedule's name
and time zone, and the policy's detail: its duration, whether it is retroactive for start
and for pause, when it cancels and resumes, and its start, stop, pause and cancel rules.
Rules are shown as stored. Details the service has no source for are left blank.

**SLA-354** The duration shown shall be the policy's current duration. A stored copy of an
older duration shall not be used.
*Check:* a clock whose stored duration was wrong shows a sensible percentage, not 1000%.

**SLA-355** A record that does not exist shall be reported as not found, and a malformed
id shall be refused.

## 7. SLA filters in case and incident searches

**SLA-360** The case and incident searches shall accept two SLA filters:

- **`slaBreached`** — equals `true` or `false` (exactly those words, in lower case).
  `true` finds items with a currently breached clock, `false` items without one.
- **`taskSLABusinessElapsedPercent`** — at-least (`gte`) and at-most (`lte`), each a
  non-negative whole number with **no upper limit** (clocks far past due keep climbing). When
  both are given they describe **one** clock, not two.

**SLA-361** Only **active** clocks count (in progress, paused, breached). Finished clocks
never match, however they ended.
*Check:* a case whose clock finished long ago, past its time, is not found by
`slaBreached = true`.

**SLA-362** The figures used shall be the live, weekend-corrected ones (SLA-321), so
boundaries behave the same as on the SLA tab.

**SLA-363** A malformed value, an unsupported operator, or more than one value for
`slaBreached` shall be refused with a message naming the field.

**SLA-364** Where the service is set to read SLA data from the legacy case system, the SLA
tab and these two filters shall be answered from there instead. The active-clock list and the
single-clock state are never affected by that setting.

## 8. The weekly SLA breach report

A report listing clocks that breached in a week, mailed to internal teams by a scheduled task.

**SLA-370** The report shall be available to **internal callers only**.

**SLA-371** The request shall name the policies to include (`definitions`), given once as
a comma-separated list or repeated. Blanks are dropped. At least one is required, and at
most **50**. The report is never defaulted to "every policy".

**SLA-372** The report shall accept optional filters, each meaning "no restriction" when
left out, and each value outside its set shall be refused with a message:

- `stages` — in progress, completed, breached, cancelled, paused, achieved;
- `onboardingStatuses` — only projects with one of these onboarding statuses (not started,
  in progress, completed, on hold, not applicable, expired, cancelled);
- `excludeOnboardingStatuses` — everything except those. Cases with **no project**, and so
  no status, are **kept**.

**SLA-373** The week shall be a Monday-to-Monday window in the **Asia/Colombo** time zone.
With no `week`, it is the full calendar week **before** the one now falls in, so a late run
or a retry within the same week reports the same week. A run or retry after the next Monday
selects a different week, so for a delayed run or a retry the intended `week` should be given.
With `week` given (a date as `YYYY-MM-DD`), it is the week **containing** that date, which is
how a missed run is re-sent. Any other format is refused.

**SLA-374** A clock shall be included if it has breached and the moment it breached falls
in the week. This covers clocks still running past their time and clocks paused after they
ran out. It covers case-like items only.

**SLA-375** The moment of breach shall be the recorded one when there is one (from the
legacy system's own history). Otherwise it is worked out as the clock's start plus its
allowed time plus the time it has been paused. For a business-days clock the allowance
skips weekends, so a Friday-afternoon breach is not placed on the Saturday.

**SLA-376** Each row shall show: the team, WSO2 case id, case number, product, issue type,
priority, kind of clock, percentage elapsed, policy name, breach moment and project
onboarding status. Rows are ordered by team, then policy name, then breach moment.

**SLA-377** An empty week is a normal answer: the report is returned with no rows.

## 9. Keeping breach alerting in step

The notification service tracks breaches itself. This service keeps it in step by
publishing signals **after** the corresponding change has been made, so a consumer that
reads the clocks in reaction to a signal always sees the new clocks.

**SLA-380** The service shall publish these signals:

| Signal | When |
|---|---|
| Case created | After a case's clocks have been started. |
| Status changed | When a case's state genuinely changes, except for an administrator's reopen. |
| Severity changed | After the case's clocks have been revised for the new severity. |
| Type changed | When a type transfer revised the case's clocks. |
| Workaround provided | When a workaround is recorded as provided. Taking the workaround back is **not** published. |
| Comment added | Whenever a comment is added; it says whether the comment was a CS engineer reply, so the response clock can be stopped at once. |
| Recalculation requested | When an admin asks to rebuild alert tracking ([sla-admin-actions.md](sla-admin-actions.md), SLA-430). |

**SLA-381** The workaround signal shall carry only the case id, and the recalculation
request only who asked and when. They are tracking signals, not messages to anyone.

**SLA-382** A failed publish shall be logged and recorded as a failed publish, and shall
never fail or undo the case change
that caused it.

**SLA-383** The notification service shall be able to read the active clocks (section 4) at
start-up and to re-check a single clock (section 5) just before alerting. A restart recovers a
clock whose creation signal was lost. A clock whose creation signal is lost after start-up is not
recovered until the next restart or an admin-triggered SLA recalculation, so its breach can go
unalerted.

## 10. Service offerings

A small read-only catalogue lookup that sits beside the SLA data.

**SLA-390** The service shall list service offerings, each with its name and the parent
service it belongs to (if any), sorted by name.

**SLA-391** The list shall accept optional filters: parent service ids (each a valid id)
and a name search. The search matches any part of the name, ignores letter case, treats
`%` and `_` as ordinary characters, and may be at most 200 characters.

**SLA-392** The list shall be paged. The default page size is 20 and the maximum is 50.

**SLA-393** The service does not restrict this lookup to internal callers: any signed-in
caller may use it. (**Confirm**, 11.2)

## 11. Open questions for the product owner

1. **The two SLA search filters are open to whoever may search.** The service does not
   restrict them to internal callers, so a customer searching their own cases could filter by
   "SLA breached" or percentage elapsed, even though every other SLA read is internal only.
   Is that intended?
2. **Service offerings are open to every signed-in caller,** including customers. Is that
   intended?
3. **Breach report week can send a breach twice.** A clock that is paused after it
   breached has that later pause folded into its calculated breach moment, so the breach can
   move into the next week and be mailed again. Nothing in the stored clock distinguishes
   pause taken before the breach from pause taken after it.
4. **Incidents are not in the active-clock list,** though they have clocks and can be asked
   about one at a time.
5. **Reports need the policy name exactly.** A renamed policy silently drops out of a
   report. There is no check that a configured name still exists.

## 12. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Active list, single clock state | `internal/service/sla_status_service.go`, `internal/repository/sla_status_repo.go`, `GET /sla-status`, `GET /sla-status/clock-state` |
| Weekend correction | `businessDayCorrectedLiveState` in `internal/repository/sla_engine_repo.go`, migration `0279` |
| SLA tab | `internal/service/task_sla_service.go`, `internal/repository/task_sla_repo.go`, `GET /slas/{id}`, `POST /slas/search` |
| Search filters | `internal/service/sla_filters.go`, `internal/repository/sla_filter_sql.go` |
| Breach report | `internal/service/sla_breach_report_service.go`, `internal/repository/sla_breach_report_repo.go`, `GET /reports/sla-breaches` |
| Signals | `internal/events/events.go`, publishers in `case_service.go` and `case_type_transfer_service.go` |
| Service offerings | `internal/service/service_offering_service.go`, `POST /service-offerings/search` |
| Internal-only check | `internal/service/require_internal.go`, `internal/server/internal_only.go` |
| Settings | `SLA_DATA_SOURCE` |
| Tests | `sla_status_service_test.go`, `sla_status_repo_test.go`, `task_sla_repo_test.go`, `sla_breach_report_service_test.go`, `sla_filters_test.go` |
