# CSM Scheduled Tasks

A single Choreo Scheduled Task that internally fans out to any number of independently-scheduled
sub-crons on one shared driver cadence. Go 1.26+, no HTTP surface at all — Choreo invokes this
binary fresh on its own trigger, `main` runs exactly one `engine.Engine.Tick` over the registered
task list, and exits. There is no internal ticker/cron loop; Choreo's own trigger IS the driver.

This process has no database of its own — durable claim/retry state lives in entity-service's
`scheduled_task_run` table (see that repo's own `CLAUDE.md`, "Scheduled task runs"), the same
division of labor `integrations/csm-notification-service`'s `internal/slaengine` uses.

## The core mechanism: period keys

For a sub-cron's cron expression `E` and a point in time `now`, `internal/schedule.PeriodKey(E,
now)` returns `E`'s most recent scheduled firing time ≤ `now` (via
`github.com/adhocore/gronx.PrevTickBefore`). A daily expression returns the *same* timestamp for
every call between that firing and its next one — every tick in between re-derives the identical
key, so retries land on the same ledger row instead of a new one, no matter how many hours or days
pass.

A row keeps retrying on every eligible tick until either it succeeds (done, forever, for that
period), or the *next* period comes due while it's still unresolved — at which point entity-service
marks it **superseded** (abandoned; only the newest period gets chased from then on). This is what
bounds the ledger to at most one open row per task at any time, and is why there is no
`max_attempts` anywhere in this design: a broken job just keeps retrying — and alerting — until
either it's fixed or its own next period supersedes it.

## Engine tick — one pass, one rule

`engine.Engine.Tick` does exactly one thing per registered task, once per invocation:

1. Compute the task's current period key.
2. Call `POST /scheduled-tasks/attempts` (`internal/ledger.Client.Attempt`) — entity-service
   atomically decides allow/deny: a period this task hasn't seen before first supersedes any other
   still-open row for the same task, then claims fresh; an existing row whose retry time has
   arrived is bumped and claimed; anything else is denied.
3. If allowed, run `Task.Handler`, then report back via `PATCH /scheduled-tasks/attempts/{id}`
   (one endpoint for both outcomes, `internal/ledger.Client.Complete`/`Fail` on the Go side) —
   `Complete` (success) or `Fail` (failure —
   sets `nextRetryOn`, never a permanent give-up state).

There is deliberately no second "sweep" pass here: an earlier design had one whose job was to
rescue an old failed period after the calendar moved past it, but once the rule became "only ever
chase the newest period," there is nothing left to rescue — an unresolved old period is meant to be
abandoned, not resurrected. See `internal/engine/engine.go`'s own package doc for the full
reasoning, and the design artifact linked from the PR that introduced this component for the
worked examples that led here.

## Adding a sub-cron

`internal/housekeeping` (registered as `"housekeeping_cleanup"` in `cmd/server/main.go`) is the
first real one, and doubles as the worked example for adding another:

1. Write the handler: `func(ctx context.Context) error` — return nil on success, a non-nil error
   (with enough detail to be useful in an alert email) on failure. It does not need to know
   anything about retries, periods, or the ledger — the engine handles all of that.
   `housekeeping.CleanupResolvedRuns(ledgerClient, retention)` returns exactly this shape, closing
   over the ledger client and a retention `time.Duration` rather than taking them as `Handler`
   arguments (the engine only ever calls `Handler(ctx)`, nothing else).
2. Add a `registry.Task{Name, Schedule, Handler, ...}` entry to the `tasks` slice in
   `cmd/server/main.go`, with a sensible default `Schedule` hardcoded in code. `Name` is the
   entity-service ledger key **and** the key both `SUB_CRON_SCHEDULES` and `SUB_CRON_RECIPIENTS`
   below look it up by — treat a rename like a breaking API change: it orphans the task's prior
   history in entity-service, and silently stops matching any existing schedule *or* recipients
   override in either config (they're two separate JSON maps — see step 4 — but both keyed on this
   same exact `Name`, so a rename has to be updated in both at once).
3. Leave `To`/`Cc` unset in the struct literal itself — they're populated from config, not
   hardcoded (see step 4 and "Alerting" below).
4. Wire up both config surfaces for it, by exact `Name`:
   - Schedule: `scheduleFor(scheduleOverrides, "<task.Name>", "<default>")` for the task's
     `Schedule` field, where `scheduleOverrides := parseSubCronSchedules(os.Getenv("SUB_CRON_SCHEDULES"))`
     is computed once, above the `tasks` slice, and reused for every task in it.
   - Recipients: `recipientsFor(recipientOverrides, "<task.Name>")` returns `(to, cc []string)`
     for the task's `To`/`Cc` fields, where `recipientOverrides := parseSubCronRecipients(os.Getenv("SUB_CRON_RECIPIENTS"))`
     is likewise computed once and reused — leave both unset (call returns `nil, nil`) unless
     `SUB_CRON_RECIPIENTS` actually mentions this task.

Every task's cadence and per-task recipients can both be set without a code change:
`SUB_CRON_SCHEDULES` is `{"<task.Name>": "<cron expression>"}`; `SUB_CRON_RECIPIENTS` is
`{"<task.Name>": {"to": [...], "cc": [...]}}` — both single JSON objects shared by the whole
registry, rather than a dedicated env var per task that would need inventing again for every new
sub-cron added here. `scheduleFor`/`recipientsFor` look up each task's override by its exact
`Name`; anything not mentioned just keeps its own hardcoded default (schedule) or gets nil (To/Cc).

## Retry backoff

`registry.Task.RetryBackoff` defaults to the driver's own interval (`DRIVER_INTERVAL`, see below)
when zero — there is no point backing off shorter than how often this process even runs. Set it
explicitly only if a specific task needs to back off harder than "every tick."

## Housekeeping

`internal/housekeeping.CleanupResolvedRuns`, registered as `"housekeeping_cleanup"`, is this
component's first real sub-cron and the thing that finally calls
`DELETE /scheduled-tasks/attempts?resolvedBefore=<ts>` — that endpoint existed in entity-service
from the start (see that repo's own CLAUDE.md, "Scheduled task runs"), but nothing called it until
this. Deletes every `scheduled_task_run` row that succeeded or was superseded more than
`HOUSEKEEPING_RETENTION_DAYS` (default 30) ago, by its own resolution time — a row still
failed/retrying is never touched, regardless of age. Default schedule `0 3 * * *` (daily at 03:00,
a low-traffic hour); override via `SUB_CRON_SCHEDULES` like any other task. `envDays`
(`cmd/server/main.go`) parses the retention setting as a plain integer number of days rather than
requiring Go duration syntax (`"720h"`) — friendlier for a "how many days of history" knob.

Cron expressions have no timezone of their own — `internal/schedule.PeriodKey` interprets every
schedule (this one included) in whatever `time.Time.Location()` the process's clock returns, i.e.
the container's local time. "03:00" only means 03:00 UTC if the deployment actually runs with
`TZ=UTC`; a non-UTC container silently shifts every period key by its own offset. Set `TZ=UTC` in
the Choreo component's environment rather than relying on the platform's default.

If a task is later removed from the registry entirely while it still has an open failed row, that
row is orphaned: nothing ever supersedes it (superseding only happens on a future `Attempt` call
for that same task name, which no longer exists), and `housekeeping_cleanup` only deletes resolved
(succeeded/superseded) rows, never an open one — so it sits in `status=failed` monitoring output
indefinitely. Deregistering a task requires manually resolving or deleting its open row in
entity-service; there is no automatic cleanup for this case.

## Stale cases report

`internal/stalecases.SendReport`, registered as `"stale_cases_report"`, is this component's first
sub-cron that sends an email on success rather than only on failure — see "Per-task report emails"
below for why that's not a generic engine feature. Queries entity-service's `POST /cases/search`
(via `internal/entitycases.Client.SearchOpenCasesOlderThan`) for every case whose `state` isn't
`closed` and whose `createdOn` is at least 30 days in the past — a fixed threshold, not an env var;
there's no operational need to tune it per deployment today (unlike
`HOUSEKEEPING_RETENTION_DAYS` — see `cmd/server/main.go`'s `staleCaseThreshold` constant if that
changes) — then emails a report table of them (oldest first) via `notify.RenderStaleCasesReport`.
Default schedule `0 7 * * *` (daily at 07:00, same `TZ=UTC` caveat as "Housekeeping" above);
override via `SUB_CRON_SCHEDULES`.

Recipients are **not** `ALERT_RECIPIENTS` — that standing audience is about failure alerting, not
report distribution. This task's report goes only to its own `SUB_CRON_RECIPIENTS` entry (`"to"`/
`"cc"`, the same config surface every task's failure-alert recipients use — see "Alerting" below);
a deployment that wants the same people getting both configures the same addresses in both places.
If this task isn't mentioned in `SUB_CRON_RECIPIENTS` at all (`to` is empty), the handler skips the
entity-service query entirely and just succeeds — no report to send means no reason to run the
query on every tick.

`internal/entitycases.Client` is a second entity-service client, separate from `internal/ledger`,
even though both point at the same `CUSTOMER_ENTITY_SERVICE_BASE_URL`/credentials — see that
package's own doc comment for why case search isn't just another method on `ledger.Client`.

## Open cases report

`internal/opencases.SendReport`, registered as `"open_cases_report"`, is a narrower, more urgent
sibling of "Stale cases report" above: it flags a case nobody has even started working on yet,
rather than any case that's merely been open a long time. Queries entity-service's
`POST /cases/search` (via `internal/entitycases.Client.SearchCasesInStateCreatedBeforeYesterday`)
for every case whose `state` is *exactly* `open` (not "any non-closed state" — a case that's since
moved to `work_in_progress` or anything else no longer belongs here) and whose `createdOn` falls
before the start of yesterday, then emails a report table of them (oldest first) via
`notify.RenderOpenCasesReport`.

"Before yesterday" is a calendar-day boundary computed in UTC (`time.Now().UTC().Truncate(24 *
time.Hour)`, minus 24 hours) — always UTC, regardless of the deployment's own `TZ` setting, unlike a
rolling duration such as `SearchOpenCasesOlderThan`'s: a case created at 23:59 yesterday (UTC) is
excluded, one created at 00:01 the day before is included, regardless of what time of day the task
itself runs. The "Housekeeping" section's `TZ=UTC` caveat is about something different — when the
*cron schedule itself* fires (`internal/schedule.PeriodKey`'s own local-time interpretation) — and
doesn't apply here: this cutoff is UTC-anchored no matter what `TZ` the process runs with.

Default schedule `0 8 * * *` (daily at 08:00, after "Stale cases report"'s 07:00 slot — this time
itself *is* subject to the same `TZ=UTC` caveat as "Housekeeping" above, since it's a cron schedule);
override via
`SUB_CRON_SCHEDULES`. Recipients work exactly like "Stale cases report" above: this task's own
`SUB_CRON_RECIPIENTS` entry, unrelated to `ALERT_RECIPIENTS`, no report sent (and no entity-service
query run) if empty. Shares `internal/entitycases.Client` and the row-rendering helpers in
`internal/notify` with "Stale cases report", but has its own template
(`internal/notify/templates/open_cases_report.html`) per this component's own "Per-task report
emails" below.

## Rota reminders

`internal/rotareminder`, registered as **`rota_weekday_reminder`**, **`rota_weekend_notice`**,
**`rota_weekend_reminder`** and **`rota_manual_reminder`** (the Rota Access card's "Send now" and
"Send a test to me"), emails the people rostered on a rotation shift in the Team Schedule, with a
calendar invitation for their window. **Turned on and off only on the Rota Access tab** (the
"CRE rota reminders" card): nothing in this component's configuration switches them on. The tasks
are registered whenever `EMAIL_BASE_URL` is set, so Send now and "Send a test to me" work without
a redeploy; a family that has never saved the card gets no scheduled reminder
(`rotareminder.withSettings`), and the card's weekday and weekend switches apply from the next run.
`ROTA_REMINDERS_ENABLED` is no longer read.

| Task | Default schedule | Announces |
|---|---|---|
| `rota_weekday_reminder` | `30 9 * * 1-5` (15:00 Colombo, Mon–Fri) | the next working day's weekday rotations (`CRE_MORNING`, `CRE_MORNING_OC`, `CRE_EVENING`): tomorrow from Monday to Thursday, **Monday from Friday** |
| `rota_weekend_notice` | `30 14 * * 0-4` (20:00 Colombo, Sun–Thu) | the coming weekend (`CRE_WEEKEND`, `CRE_WEEKEND_NIGHT`, `CRE_WEEKEND_NIGHT_OC`), **once it is fully marked**: the summary and the invitations, sent once; until then the rota admins are asked every evening |
| `rota_weekend_reminder` | `30 9 * * 5` (15:00 Colombo, Fri) | this weekend again: the summary, the invitations as updates, cancellations, gaps listed |
| `rota_manual_reminder` | `*/5 * * * *` (finer than the driver, so due at every run) | whatever was asked for on the Rota Access card since the last run: a weekday or weekend "Send now", or a test sent only to the person who asked (see "Send now" below) |

The schedule decides only *when* a task runs. Which days it announces comes from the day it runs
(`NextWorkingDay`, `ComingWeekend`), counted in `ROTA_REMINDER_TIME_ZONE` (default
`Asia/Colombo`), so a schedule moved by `SUB_CRON_SCHEDULES` still announces sensible days. The
schedules assume `TZ=UTC` like every other here.

- **The rota comes from entity-service** (`internal/teamschedule`): `GET /team-schedule/catalogue`
  for labels, windows and team names, and `POST /team-schedule/assignments/search` with
  `excludeAbsent: true`, so **people on leave are left out by entity-service**, by the same rule
  the pager uses. An entity-service that predates `excludeAbsent` answers 400 (unknown field) and
  the task fails and alerts rather than reminding somebody on leave: deploy entity-service first.
  The Team Schedule is internal-only, so this client's id must be in entity-service's
  `M2M_CLIENT_IDS`.
- **Weekday: one email per shift window**, sent to the people on that window only. A window is
  a shift and its on-call variant (`<CODE>_OC`), which share hours: Morning (with its on-call
  person) and Evening. The email lists its window's people.
- **Weekend: one summary email plus one invitation email per team per day.** The summary goes
  to everybody on the weekend, LK and Americas (with the LK on-call person) alike, in the layout the
  CRE weekend emails already had: "Hello team,", a Saturday Team and a Sunday Team list
  ("Name (Rotation)"), a Rotation Definitions table (Rotation with its member count | Local time on
  the rotation's own clock | UTC window | Role: "IST/PDT General support" or "On-call"), the zone
  legend and a daylight saving note, then the Handover paragraphs (`notify.RenderWeekendRota`).
  The Americas Weekend's hours, and the LK On-call's that cover it, are 08:00-18:00
  America/Los_Angeles (`ROTA_REMINDER_WINDOWS`), not the Team Schedule's 21:00-06:00 IST; the
  invitations use the same hours. Then each window (LK
  Saturday, Americas Saturday, LK Sunday, Americas Sunday) gets its own email to its own people,
  carrying its invitation with all of them as guests, so they can start a group chat from the
  event. One recipient is greeted by full name, several with "Dear all,".
- **Settings saved on the Rota Access tab win.** Every run first reads
  `GET /team-schedule/reminder-settings/{family}` (entity-service, migration 0242; the
  "CRE rota reminders" card, editable by a CSM admin or the CRE rota admin). When something
  is saved: `weekdayEnabled` / `weekendEnabled` turn that task off (the run succeeds and
  sends nothing), `invitesEnabled` can turn invitations off but never on where the deployment
  has them off (`ROTA_REMINDER_INVITES_ENABLED`, or no organizer), `weekendRequired` and
  `windows` override `ROTA_REMINDER_WEEKEND_REQUIRED` / `ROTA_REMINDER_WINDOWS` code by code
  (a code the saved settings do not name keeps the configured value; the card saves a code
  only once somebody changes it there), and `copyTo` is copied on top of the configured copy
  list. So saving one field never replaces a deployment setting nobody changed. **Nothing
  saved (a 404, also from an entity-service without the route) sends no scheduled reminder**:
  only the card's switches turn them on. Send now still goes out, with the environment's values.
  A failed read fails the run, so it is retried
  rather than sent with settings somebody turned off. Not on the card, on purpose: the
  send times, `ROTA_REMINDER_REDIRECT_TO`, the family, the shift codes and names, and the
  email service settings.
- **"Send now" (`rota_manual_reminder`, every run).** The card's Send now button queues a
  request in entity-service (`team_schedule_reminder_request`, migration 0242; asked for by a
  CSM admin or the family's rota admin, refused while one of the same kind is waiting and within
  10 minutes of the last that did not fail). This task has no address to call, so at every run it claims the
  waiting requests (`POST .../reminder-requests/{family}/claim`, up to 5 a run) and sends each
  with the code above: `weekday` is the next working day's windows, `weekend` the coming weekend
  (this one from a Saturday or a Sunday), sent even when not fully marked, as on Friday, and
  recorded like Friday's, so invitations update and cancellations follow. The saved on/off
  switches do not stop it; the saved minimums, hours, invitations switch and copy list apply.
  Each request is reported `SENT` ("Sent N emails.", counted where each reminder is sent: a
  weekday window's email by its final outcome, the weekend summary and each invitation, never the
  rota admins' email, cancellations or alerts; or "Nothing was sent: <why>.", the reason each
  window was skipped) or `FAILED` with the reason
  (`PUT .../{id}/result`); only a failed claim or report fails the run. A request claimed but
  never reported (the run died) is claimed again after 2 hours, so in that rare case its
  emails go out twice. How soon it is sent is the driver's cadence (`DRIVER_INTERVAL`, an hour by
  default); its schedule (`*/5 * * * *`) is finer than the driver, so it is due at every tick.
- **"Send a test to me".** The same queue with `test: true` (its own queue per person, 2 minutes
  apart, never held up by a real request). The run sends exactly what Send now would, but every
  email -- admins' and failure alerts included -- goes only to the person who asked, with
  "[Test]" in the subject and the "[Test copy] Not sent to the rota. ... would have been sent
  to: ..." line; nothing is copied; invitations carry test event ids (`...-test@csm-team-schedule`),
  so a test event never stands in for or blocks the real one; and the weekend's send record is
  neither read nor written, so the real reminders are unaffected. Works in every environment,
  production included, and needs no `ROTA_REMINDER_REDIRECT_TO`.
- **Only a fully marked weekend is announced.** `ROTA_REMINDER_WEEKEND_REQUIRED` (default
  `CRE_WEEKEND=3,CRE_WEEKEND_NIGHT=1`) is how many people each shift needs on each day; the on-call
  variant does not count. Until that holds, `rota_weekend_notice` emails the family's rota admins
  (entity-service `GET /team-schedule/rota-admins/{family}`; the task's alert audience when there
  are none) a table of what is missing, every evening, and sends nothing to the rota. Friday's
  `rota_weekend_reminder` sends anyway, listing the gaps, and tells the admins.
- **The weekend send is recorded in entity-service** (`team_schedule_reminder_sent`, migration
  0241, `GET`/`PUT /team-schedule/reminders/{family}/weekend_notice/{saturday}`): an evening that
  finds the record sends nothing, and Friday compares its invitations with the recorded ones and
  sends a cancellation (`METHOD:CANCEL`) to anybody taken off a window, or to everybody on a window
  that is gone. A failure to record is reported in the partial-failure alert; the emails are not
  re-sent for it.
- **Calendar invitations** (`ROTA_REMINDER_INVITES_ENABLED`, default true; needs
  `EMAIL_FROM_ADDRESS`, the organizer) are `text/calendar; method=REQUEST` attachments built in
  `ics.go`. Through the email service Gmail shows them as invitation cards and adds the event (an
  `.ics` attached by hand in Gmail compose does not). Each event's identity is the rota day and
  shift (`inviteUID`), and its sequence the minute it was sent, so a re-send updates it. The
  email service binds an attachment to `byte[]`, so `notify.Attachment` sends it as an array of
  byte values, not base64. Weekday emails carry their window's invitation in the same email. If the email service rejects a weekday email's invitation (HTTP 400), the email is sent again
  without it, so the reminder still arrives; a weekend invitation email has no such fallback.
  Nothing from the rota or the configuration can add a line to an invitation: names and notes
  are escaped and stripped of control characters, an attendee whose address is not a plain
  address is left out (`ValidInviteAddress`), and the event's URL is written only when it is
  http(s) (`ValidScheduleURL`). The organizer is `EMAIL_FROM_ADDRESS`'s address
  (`InviteOrganizer`): a plain `no-reply@example.com` as it is, or the address inside
  `Customer Service <no-reply@example.com>`, whose display name the emails keep. At startup an
  `EMAIL_FROM_ADDRESS` that is unset or holds no address turns invitations off, and a
  `ROTA_REMINDER_SCHEDULE_URL` that is not http(s) is dropped from every email, both with a
  warning.
- **The task's `SUB_CRON_RECIPIENTS` are not copied** unless
  `ROTA_REMINDER_COPY_TASK_RECIPIENTS=true`; they still get its failure alerts, as everywhere
  here. Nobody is ever both a recipient and copied, or copied twice (case-insensitive).
- **A plain, formal message, not a branded template** (by request). A weekday email, an invitation
  email and the rota admins' email (`notify.RenderRotaReminder`): a greeting, one line about the
  window -- its date and hours on the rotation's own clock (`clockOf`: IST, except a rotation in
  `ROTA_REMINDER_WINDOWS`, e.g. the Americas Weekend in PDT/PST) -- a Name | Rotation table, the
  swap instruction with the Team Schedule link written out, and a sign-off. The weekend summary
  (`notify.RenderWeekendRota`, described above): "Hello team,", the day team lists, the Rotation
  Definitions table, the zone note and the handover paragraphs. Rotation names come from config
  (`CODE=Name`). Names are escaped. Every subject starts `[<family> Operations]`
  (`[CRE Operations] Weekend Rotation – …`; a test adds `[Test] ` in front).
- **A retry announces what its scheduled run would have.** The engine retries a failed run every
  tick until the next period, which for `rota_weekend_notice` is days later. So the rota days are
  counted from the run's period key (`Config.Anchor`, from `schedule.PeriodKey`), not the clock:
  Thursday's run retried after midnight still announces Friday, not Monday, and Friday's weekend
  notice retried on Saturday announces this weekend, not the next. "Tomorrow" / "today" in the text
  are from the clock, as the reader sees them.
- **A window that has started is not sent** (a late retry); it is logged. A weekend email
  retried on Saturday goes to the people on the windows still to come.
- **The run fails only when no email went out.** The handler keeps no record of what it sent, so
  a retry re-sends every window. When some windows went out and some failed, the run succeeds and
  the handler itself emails the task's alert audience (its `SUB_CRON_RECIPIENTS` and
  `ALERT_RECIPIENTS`, as the engine would) a "PARTLY FAILED" alert listing each failed window and
  its people, to be told by hand. When nothing went out, the run fails and is retried as usual,
  which repeats nothing.
- **Nobody to email is a success**: an empty rota, or a shift whose people have no address (logged).
- **`ROTA_REMINDER_REDIRECT_TO`** sends every reminder to a test inbox instead, naming the intended
  recipients in its first line. Use it in any environment that holds the real rota but must not mail
  people. `ALERTS_ENABLED=false` silences these like every other email here.
- With `EMAIL_BASE_URL` unset the rota tasks are not registered (logged at startup): nothing
  could be sent.
- **Not built**: a "your shift changed" email when a lead edits the rota after the reminder went
  out (Friday's reminder is the only re-send; it needs entity-service to announce rota changes,
  see "Future: events"), SRE and SME rotas (set
  `ROTA_REMINDER_FAMILY` and the shift lists; the date rules assume CRE's weekday and weekend
  split), and cancelling a weekday invitation (weekday sends are not recorded). Unlike the report
  tasks, these own no template file.

## Availability recalculation

Recomputes every committed service offering's uptime and rewrites
`service_availability`. Task name **`availability_recalculation`**, default
schedule `0 3 * * *`. The Go port of ServiceNow's `Calculate Availability`
job, which has run nightly since 2022-11-01 and whose 212,904 rows the Cloud
Status Dashboard reads on every page load.

**Off unless `AVAILABILITY_RECALC_ENABLED=true`** (default false): neither the client nor the
task is created otherwise. ServiceNow's "Calculate Availability" job keeps writing the same
table through csm-sync-service, and `service_availability` has no unique constraint on a
period's natural key, so both running can leave two rows for one period. Turn it on in the
same change that switches ServiceNow's job off (the `CLOUD_STATUS_ENABLED` pattern).

*** THIS IS THE PRODUCER FOR THREE ALREADY-PORTED ENDPOINTS. ***
`/cloud-status/monitors`, `/availabilities` and `/availability-history` all
read that table and were ported long before anything wrote it: the rows come
from csm-sync-service mirroring ServiceNow's output. At cutover the
dashboard's uptime figures would simply stop advancing, with no error
anywhere — reading a table nobody updates looks exactly like reading a table
where nothing happened. This task is what takes over.

**The arithmetic is in entity-service, not here.** This task is a trigger.
The sweep reads every outage for ~146 subjects and writes up to eight period
rows each into a table the dashboard is concurrently reading; doing that over
HTTP would pull the whole working set across the wire every night, and this
component holds no database credentials. Same split as `outage_communication`
and `cloud_status`.

*** IT PORTS v2, AND THE INSTANCE RUNS v1. *** `com.snc.availability.v2` is
false on wso2sndev, so every stored row was written by the legacy calculator.
The two genuinely disagree — v1's "last 30 days" spans 29 under PRB1304264,
v2's spans 30 — so **the existing table is not a baseline to diff against.**

**It emits `LAST_90_DAYS`, which v2 does not define.** v2 registers seven
period types and that is not one of them; v1 writes it. But `/monitors` and
`/availabilities` both query it and both render a "Last 90 days" figure, so
shipping pure v2 would delete a number from the customer-facing status page
silently. `LAST_1_DAYS` is the mirror image — v1 writes it, nothing reads it,
not emitted.

**A partial run fails the task.** entity-service keeps going when one subject
fails and still returns 200, so the handler checks the `failed` count and the
subject count: a sweep that skipped offerings, or found none at all, is an
alert rather than a quiet success. Zero subjects is what an unmapped
`service_offering_commitment` looks like.

**It is inert until digiops-cs mirrors `service_offering_commitment`.** That
table is the join saying which offering answers to which commitment, and the
calculator builds its entire subject list from it. Everything else in the
family is already mirrored (`service_availability` migration 0084,
`service_commitment`, `outage_affected_ci`, `schedule`, `schedule_span`).

**Registering it is a paired change with disabling ServiceNow's `Calculate
Availability` job.** Two writers on one table, keyed differently — the sync
on the mirrored `sys_id`, this on the natural key — would double every
subject's rows.

Timeout is five minutes, not the sixty seconds the neighbouring sweeps use.
Volume is the normal case here, and cutting a healthy run off partway leaves
some subjects updated and the rest stale.

## Case auto-closure

`internal/autoclosure.Run`, registered as **`case_autoclosure`** (default schedule `0 8 * * *`, TZ=UTC
caveat as above), reminds and closes tickets that sit in Awaiting Info / Solution Proposed with no activity.
**Off unless `AUTOCLOSURE_ENABLED=true`**: it comments on and closes customers' tickets, so it is never on by
accident. This component's client id must be in entity-service's `M2M_CLIENT_IDS`. entity-service's `AUTOCLOSURE_SYSTEM_USER_EMAIL` is optional: empty acts as "System user" (no user row, no `closed_by`).

This task is a **trigger only**. Everything that decides anything (eligibility, whether a ticket is still due,
the comment text, how it is closed, the daily limit) is in entity-service; see its CLAUDE.md, "Case
auto-closure". Each run works through four stages in order -- `release_hold`, `close`, `second_reminder`,
`first_reminder` -- asking `POST /autoclosure/candidates` for what is due and `POST
/autoclosure/cases/{id}/advance` for each ticket. A ticket moves at most one stage per run.

- A failure on one ticket never stops the others; a failed listing does not stop the other stages.
- The run **returns an error if anything failed**, so the engine alerts and retries on the next tick. That is
  safe: tickets already advanced are no longer due, and entity-service takes what today has already done off the
  daily limit, so the limit holds across retries.
- The alert lists at most 10 failures and counts the rest.
- **Paging.** A listing returns at most one page, so each stage asks again until nothing new comes back (at most 20 passes of 500 new tickets, but entity-service never returns more than 5,000 in one listing, so at most 5,000 tickets are attempted per stage per run; the request limit grows with the tickets already attempted so ones that stay due -- skipped or failed -- do not hide those behind them). One known edge: entity-service also clamps a listing to the day's remaining budget, so when the budget is almost spent and the tickets at the front of the list are skipped or failed, the loop can stop early with later due tickets left; harmless because the budget is nearly gone. A ticket is attempted once per run. With a non-zero daily limit that limit bounds a day's work; with `AUTOCLOSURE_DAILY_LIMIT=0` (no cap) only the 20-pass page cap per stage and the schedule do, so a large backlog is then worked through at that pace.
- The summary log line carries evaluated / reminded / closed / holdsReleased / wouldAdvance / skipped / failed / **quarantined**. Quarantined is how many due tickets entity-service left out because they failed the stage 3 times in 7 days; when it is above zero the run also logs a WARN saying nobody will be reminded or closed for them until someone looks (`case_autoclosure_event`, outcome `FAILED`). It deliberately does **not** fail the run: a failed run alerts, and is retried and alerted again every tick, so a permanent quarantine would become the permanent alert noise the cap exists to remove. Getting this in front of a person by email needs the per-task report email that does not exist yet (see "Future: per-task report emails"); until then watch the log or alert on the WARN.
- **A dry run is not sign-off on closures.** It claims nothing, so it can only preview what is due *now*: first reminders for tickets still at `DEFAULT`, plus whatever second reminders and closes already-advanced tickets (for example from the old flow's steps) are due for. Tickets reach the second reminder and the close only after a real first reminder has moved them, so a clean dry run on a fresh backlog says nothing about the close stage. Validate the close rules on a small real batch first (`AUTOCLOSURE_STATES=solution_proposed`, a low `AUTOCLOSURE_DAILY_LIMIT`).

| Env var | Default | Meaning |
|---|---|---|
| `AUTOCLOSURE_ENABLED` | `false` | Register the task |
| `AUTOCLOSURE_INTERVAL_DAYS` | `7` | Days each stage waits (calendar days); above 365 refuses to start |
| `AUTOCLOSURE_DAILY_LIMIT` | `500` | Reminders + closures per UTC day, across all stages; `0` = no cap |
| `AUTOCLOSURE_DRY_RUN` | `false` | Decide and log only (entity-service records `DRY_RUN` audit rows, not counted against the budget; this task spends the daily limit itself, so the totals are what a real run would do). **A `TRYOUT_TASK=case_autoclosure` is always forced to a dry run**: a tryout skips the ledger but still talks to the real entity-service, and would otherwise comment on and close real tickets |
| `AUTOCLOSURE_STATES` | both | Comma list of `awaiting_info`, `solution_proposed`. Run `solution_proposed` alone first, add `awaiting_info` after watching it; an unknown value refuses to start |
| `AUTOCLOSURE_TEMPLATES` | built-in | JSON `{"<state>.<kind>": "text"}` overriding comment wording; kinds `first_reminder`, `second_reminder`, `closure`; a malformed value refuses to start (it would otherwise silently post the built-in wording) |

First run: expect a large backlog (thousands of eligible tickets); the daily limit spreads the notifications over
days. Set a hold on tickets that customers asked to keep open **before** enabling.

## ACP closure

`internal/acp.Run`, registered as **`acp_closure`** (default `0 4 * * 1-5`, 09:30 Colombo on
weekdays; TZ=UTC caveat as above), is the Account Closure Process, moved here from
`integrations/acp-closure-service` (still in the repo until this task runs in production). **Off
unless `ACP_ENABLED=true`**: it emails customers and suspends their projects, so it is never on by
accident, and while it is off no `ACP_*` setting is read, so a deployment without them still starts.
When it is on, a missing `ACP_SALES_ENTITY_*` setting stops startup.

The rules came over unchanged in `internal/acp/...` (`closure`, `sweep`, `suspensionstate`,
`recipients`, `notify` for the notice templates, `entity`, `salesentity`, and ACP's own `httpsec`
and `apierror`, which differ in shape from this component's). The service's own `CLAUDE.md` and
`README.md` describe the windows, recipients and invoice rules. entity-service's automatic reopen
keeps a copy of the invoice rules (`internal/service/acp_reopen_rules.go`); a rule change goes in
both places.

- **Reads and writes.** CSM (entity-service, `CUSTOMER_ENTITY_SERVICE_BASE_URL` with the shared
  `OAUTH2_*` client) gives the open-project list, closure states and notices sent, and takes the
  notice and suspension writes (`PATCH /projects/{id}`): **this component's client id must be in
  entity-service's `M2M_CLIENT_IDS`**. Salesforce, through sales-entity-service
  (`ACP_SALES_ENTITY_*`, its own credentials), gives the dates, people, opportunities and invoices,
  and takes `State__c=Suspended`. The standalone service reached the same entity-service endpoints
  through csm-integration-service, which forwards them unchanged.
- **Reopening is not here.** entity-service reopens an ACP-suspended project when a Salesforce
  event shows it renewed or paid, and once a day over every ACP-suspended project (the
  `acp_reopen_check` task below drives that); csm-notification-service sends the "Back to Open
  state" email (each one's CLAUDE.md, "ACP reopen"). entity-service keeps a copy of the invoice and
  date rules; `closure/rules_parity_test.go` and `sweep/rules_parity_test.go` run ACP's rules over
  the cases in `entity-service/internal/service/testdata/acp_rules_parity.json`, which entity-service
  runs over its copy too (skipped when that file is absent; `ACP_RULES_PARITY_FILE` points at
  another path). A rule changed here and not there makes ACP suspend a project every day and the
  reopen check reopen it, so change both and the file together.
- **Salesforce first.** A suspension sets Salesforce `State__c` to Suspended before the CSM
  suspension (`suspendInSalesforce`). A Salesforce failure then leaves the project Open in CSM, so
  the next run tries again (CSM first used to leave it for someone to fix by hand); and a reopen
  check running between the two writes finds nothing ACP-suspended yet instead of reopening a
  project that Salesforce is then set back to Suspended on.
- **Dry run by default** (`ACP_DRY_RUN`, unset or malformed is true): decide and log, write
  nothing to CSM or Salesforce. **It does not stop emails**: with `ACP_EMAIL_SEND_ENABLED=true`
  the notices are sent for real, and because the dry run records no notice in CSM the same
  notices are sent again on every run (startup logs a warning for that combination; test it
  with a `debug` list). Notices are only logged unless `ACP_EMAIL_SEND_ENABLED=true` and `ALERTS_ENABLED` is
  true (and then `EMAIL_BASE_URL` is required at startup). Addresses outside `@wso2.com` are dropped
  unless `ACP_ALLOW_NON_WSO2_RECIPIENTS=true` (production only). `ACP_STANDING_CC_RECIPIENTS` adds
  the distribution lists to every notice; leave it empty outside production.
- **Testing.** A `debug` list for `acp_closure` in `SUB_CRON_RECIPIENTS` sends every notice there
  instead (`notify.ForReport`, `[Test copy]` subject), as for a report task. `ACP_TEST_PROJECT_ID`
  scopes a run to one CSM project. A `TRYOUT_TASK=acp_closure` is forced to a dry run unless
  `ACP_TEST_PROJECT_ID` is set, since a tryout talks to the real CSM and Salesforce.
- **Weekends.** The schedule is weekdays only, and the handler also succeeds without doing anything
  on a Colombo Saturday or Sunday, for a retry that lands there. Monday's run catches up: windows and
  suspension are thresholds.
- **Failures.** A failed project never stops the others. A run in which any project failed (or the
  project list could not be read) returns an error, so the engine alerts and retries. The error
  names the run id and at most 20 failed project ids, never their errors (they can carry customer
  data); the log has each one. `RetryBackoff` is **4 hours**, not the driver interval: a retry
  repeats nothing already recorded in CSM, except a notice whose email went out while its CSM
  record failed, which is sent again.

## ACP reopen check

`acp.RunReopenCheck`, registered as **`acp_reopen_check`** (default `30 3 * * *`, 09:00 Colombo every
day, half an hour before `acp_closure`; TZ=UTC caveat as above), drives entity-service's daily safety
net for ACP suspensions: `POST /acp/reopen-check`, `{cursor, limit: 5}`, page after page while
the answer says `more` (a page short of time stops early, after at least one project, with `more`
set). entity-service checks every project ACP's record still shows suspended,
reopens the ones whose renewal or payment an event missed, and finishes a project a person set back
to Open (Salesforce Open, the "Back to Open state" email); see its CLAUDE.md, "ACP reopen". This task
decides nothing itself. **Off unless `ACP_REOPEN_CHECK_ENABLED=true`**, and it needs entity-service
with `CSM_ACP_REOPEN_ENABLED=true` (the route is not registered otherwise: the run fails with a 404
and alerts). Whether anything is written is entity-service's `CSM_ACP_REOPEN_DRY_RUN`, so after that
is switched off the next run reopens everything that was only logged as "would reopen" during the
dry run. Independent of `ACP_ENABLED`.

- Calls entity-service with the shared `OAUTH2_*` client (its id in entity-service's
  `M2M_CLIENT_IDS`, as for ACP), with a 2-minute timeout per page.
- A run in which a project failed returns an error naming at most 20 project ids (the errors are in
  entity-service's log); retried after 4 hours, like `acp_closure`. A page error, a page that makes no
  progress, or more than 2,000 pages stops the run with an error.

## Usage rollups

Two sub-crons that rebuild the usage summary tables: **`usage_rollup_daily`** (default `30 0 * * *`)
rebuilds the daily rows of the last `USAGE_ROLLUP_DAILY_LOOKBACK_DAYS` UTC days before today
(default 2, newest first), and **`usage_rollup_monthly`** (default `30 1 * * *`) rebuilds the
previous calendar month and then the current one. They are the Go replacement for ServiceNow's
`UsageAggregator` (`runAggregationForDate` / `runAggregationForMonth`).

**Off unless `USAGE_ROLLUP_ENABLED=true`** (default false): neither the client nor the tasks are created
otherwise. ServiceNow's aggregation jobs keep writing `u_daily_usage_summary` and
`u_monthly_usage_count`, which csm-sync-service mirrors into the same Postgres tables, so two
writers would fight over those rows, and csm-sync's `delete_sync` would delete rows ServiceNow
does not have. Turn it on in the same change that switches those ServiceNow jobs and csm-sync
jobs off (the `AVAILABILITY_RECALC_ENABLED` pattern).

**Backfill and late uploads.** These tasks rebuild only recent days and months. entity-service
rebuilds a late upload's months itself when its own `USAGE_ROLLUP_ENABLED` is on, and its
CLAUDE.md describes the one-time backfill to run when the rollups are switched on.

**The arithmetic is in entity-service, not here.** Each task is a trigger: it POSTs to
`/instances/usage-rollups/daily` or `/monthly`, which run set-based SQL in the database
(`rollup_daily_usage`, `rollup_monthly_usage`, migration 0245). Summing every node's hourly rows
over HTTP would drag the working set across the wire each night, and this component holds no
database credentials. The endpoints are internal-only: this component's client id must be in
entity-service's `M2M_CLIENT_IDS`. The rule (counters such as `TRANSACTION_COUNT`, `M2M_TOKEN` and
`AGENT_*` are summed; everything else takes its latest reading) is documented there.

Today is left out of the daily run because entity-service keeps the current day's rows current as
reports arrive; this run settles each day once it is over. Several days are rebuilt, not just
yesterday, so a report that reaches PCT late, or a night this task did not run, is picked up by the
next run. Both rollups are idempotent (a rerun over unchanged rows writes nothing), so the overlap
costs nothing. One failing day or month does not stop the others; the handler returns every failure
together and the engine retries the whole run. Days and months are UTC, so run with `TZ=UTC`.

## Unresolved products digest

`internal/unresolvedproducts.SendDigest`, registered as **`unresolved_products_digest`** (default
`0 6 * * *`), emails the deployment nodes seen in the last 24 hours whose reported product or
version is not in the product catalogue, so someone adds the missing catalogue entry. It replaces
ServiceNow's `PRODUCT_NOT_FOUND_ALERT_EMAIL`, which emailed one hard-coded address on every miss.

entity-service keeps what each node reported (`deployment_node.reported_product`/`reported_version`,
migration 0247) and lists the unresolved ones, grouped by what they reported, at the internal-only
`GET /instances/unresolved-products`; this task only fetches and sends. Same shape as the other
report sub-crons ("Per-task report emails"): its own template
(`internal/notify/templates/unresolved_products_report.html`), its own recipients from its
`SUB_CRON_RECIPIENTS` entry, a no-op without recipients or with `ALERTS_ENABLED=false`. Unlike
them it **sends nothing when the list is empty**: the email exists to prompt an action, and a daily
"nothing to do" would train its recipients to ignore it. The 24-hour window is fixed to match the
daily schedule; change both together.

**A node is listed every day until its catalogue entry is added**, on purpose: the digest is the
list of entries still missing, not a one-off alert (the previous system emailed on every miss).
Each row shows when the node went unresolved (`unresolvedSince`, entity-service's
`deployment_node.unresolved_since`), not when it first reported, and a list entity-service cut off
at its 5000-node limit says it is incomplete.

## Outage emails (moved out)

The two outage emails -- `outage_internal_notification` (internal
stakeholders) and `outage_communication` (SRE declaration/resolution) -- used
to be sub-crons here. They now run in entity-service's outage notice drainer,
which publishes them on the `outage-events` topic, and csm-notification-service
sends them: seconds after the change, as ServiceNow's record-triggered flows
do, instead of on this component's tick. Their recipients moved with them
(`OUTAGE_NOTIFICATION_RECIPIENTS` / `OUTAGE_COMMUNICATION_RECIPIENTS` on
entity-service); `SUB_CRON_RECIPIENTS` entries for the two old task names are
now ignored.

## Alerting

Two layers, combined:

- `ALERT_RECIPIENTS` (env var) is a standing ops/on-call audience — emailed on every failed
  attempt, for every task, unconditionally. Most tasks will leave `To`/`Cc` unset and rely on this
  alone.
- `registry.Task.To`/`Task.Cc` are additional recipients specific to that one task, populated from
  `SUB_CRON_RECIPIENTS` (see below) — set only for the sub-crons that need a different or extra
  audience, e.g. a billing job's own on-call team alongside the standing list. `To` gets merged
  into the same `To` line as `AlertRecipients` (not `Cc`), so a task's own recipients and the
  standing list both land as real primary recipients, not one buried in Cc; `Cc` stays purely
  per-task.

A task's failures send no email only if **both** `ALERT_RECIPIENTS` and that task's own `To` are
empty — it still fails and retries normally either way, just silently as far as email goes.

`ALERTS_ENABLED` (env var, default `true`) is a global kill switch above both layers —
`engine.Engine.AlertsEnabled` — for going quiet during a maintenance window or a known-noisy period
without editing any recipients config. Despite the name, it's not limited to failure alerts: it's
the single switch for every email this component sends, so `stale_cases_report`'s and
`open_cases_report`'s own report emails (see "Stale cases report"/"Open cases report" above) each
read the same underlying value directly too, since neither is sent through `Engine.recordFailure`
at all. For a failed task, `false` only silences the email — the failure is still recorded in
entity-service and logged either way, and retries proceed normally. For either report task
specifically, `false` skips the entity-service query and report rendering entirely, not just the
send (see `stalecases.SendReport`/`opencases.SendReport`'s own doc comments) — the task still
succeeds and its ledger row still updates, it just does no work that tick. When `ALERTS_ENABLED` is
`false`, `EMAIL_BASE_URL` is also no longer required at startup even if recipients are configured,
since no email will ever actually be sent — see cmd/server/main.go's startup check.

**Every single failed attempt sends an alert, not just a final give-up** — there is no "fully
failed" or exhausted state in this design (see "The core mechanism" above), so this fires each time
a handler returns an error, however many times that happens before the task either succeeds or gets
superseded by its own next period. A task retrying every tick for hours alerts every tick too; if
that's too noisy for a given task in practice, the fix is a notification-frequency policy layered
on top later, not a retry cap.

The email itself (`internal/notify/templates/alert.html`, rendered by `notify.RenderAlertEmail`) is
a plain HTML template in the same table-based, WSO2-branded style
`integrations/csm-notification-service`'s own templates use (`escapeHTML`/`escapeMultiline` mirror
that service's own functions of the same name) — task name, period, attempt count, next retry time,
and the failure's error message.

The engine itself still never sends a success email on any task's behalf — `stale_cases_report`'s
and `open_cases_report`'s own reports (see "Stale cases report"/"Open cases report" above) are each
sent from inside that task's own handler, not through this alerting path at all. See "Per-task
report emails" below for why that's not a generic engine feature.

## Environment variables

| Variable | Required | Description |
|---|---|---|
| `OAUTH2_CLIENT_ID` / `OAUTH2_CLIENT_SECRET` / `OAUTH2_TOKEN_URL` | Yes | Shared OAuth2 client credentials — used by the entity-service client and the email client below, and by any future service client this component grows. Mirrors `integrations/csm-notification-service`'s own `OAUTH2_*` convention: the real deployments these point at authenticate every caller through the same shared gateway app, scoped per-client via each client's own `*_SCOPES` var, not a separate app per consumer |
| `CUSTOMER_ENTITY_SERVICE_BASE_URL` | Yes | entity-service base URL |
| `CUSTOMER_ENTITY_SERVICE_SCOPES` | No | Comma-separated OAuth2 scopes for the entity-service client, using the shared credentials above |
| `EMAIL_BASE_URL` | No, but required if `ALERTS_ENABLED` is true and `ALERT_RECIPIENTS` or any task's `SUB_CRON_RECIPIENTS` "to" is non-empty (checked at startup) | Internal email notification service base URL (`POST /send-email`) — same service `integrations/csm-notification-service` uses. Authenticates with the same shared `OAUTH2_*` credentials, not its own |
| `EMAIL_SCOPES` | No | Comma-separated OAuth2 scopes for the email client, using the shared credentials above |
| `EMAIL_FROM_ADDRESS` | No | Fixed "From" address for every email this component sends |
| `ALERTS_ENABLED` | No (default `true`) | Global kill switch for every email this component sends — failure alerts and report-style tasks' own success emails alike — see "Alerting" above |
| `ALERT_RECIPIENTS` | No | Comma-separated email addresses alerted on every failed sub-cron attempt, for every task — see "Alerting" above |
| `TRYOUT_TASK` | No | Runs the named task's handler once, now, outside the schedule and the ledger, then exits (`runTryout` in `cmd/server/main.go`). Refuses unless that task has a `debug` list in `SUB_CRON_RECIPIENTS` |
| `DRIVER_INTERVAL` | No (default `1h`) | This component's own expected invocation cadence — must match the cron trigger configured on the Choreo Scheduled Task component itself |
| `SUB_CRON_SCHEDULES` | No | JSON object `{"<task.Name>": "<cron expression>"}` overriding any registered task's schedule by name — see "Adding a sub-cron" above. A task not mentioned keeps its own hardcoded default |
| `SUB_CRON_RECIPIENTS` | No | JSON object `{"<task.Name>": {"to": [...], "cc": [...], "debug": [...]}}` giving a registered task its own extra failure-alert audience, on top of `ALERT_RECIPIENTS` — or, for a report-style task, its report's actual recipients (see "Alerting" above for which tasks work which way). `debug` on a report task is debug mail mode for that report alone (`notify.ForReport`): it goes to that list only, `[Test copy]`-prefixed, with the intended To/Cc named in the body; startup warns naming every such task. A task not mentioned gets no per-task recipients |
| `ROTA_REMINDER_REDIRECT_TO` | No | Comma-separated test inbox(es) that receive every rota reminder instead of the people on the shift |
| `ROTA_REMINDER_SCHEDULE_URL` | No | Link to the CSM portal's Team Schedule page, shown in every reminder |
| `ROTA_REMINDER_COPY_TASK_RECIPIENTS` | No (default `false`) | Also copies each task's `SUB_CRON_RECIPIENTS` on every reminder |
| `ROTA_REMINDER_INVITES_ENABLED` | No (default `true`) | Attaches each window's calendar invitation; off when `EMAIL_FROM_ADDRESS` is unset or holds no address (`Name <address>` is fine) |
| `ROTA_REMINDER_WEEKEND_REQUIRED` | No (default `CRE_WEEKEND=3,CRE_WEEKEND_NIGHT=1`) | `CODE=N` pairs: how many people each weekend shift needs on each day before the weekend is announced |
| `ROTA_REMINDER_FAMILY` / `ROTA_REMINDER_WEEKDAY_SHIFTS` / `ROTA_REMINDER_WEEKEND_SHIFTS` | No (defaults `CRE` and the codes in "Rota reminders") | Which rota, and which shifts each task announces as `CODE=Name` pairs (the name the email shows) |
| `ROTA_REMINDER_WINDOWS` | No (default `CRE_WEEKEND_NIGHT=08:00-18:00 America/Los_Angeles,CRE_WEEKEND_NIGHT_OC=08:00-18:00 America/Los_Angeles`) | A rotation's hours on its own clock, used in the emails and invitations instead of the Team Schedule's (`CODE=HH:MM-HH:MM Zone`, comma-separated). The Americas Weekend and the LK On-call that covers it are 08:00-18:00 Pacific time, so it moves an hour in UTC and IST when US daylight saving changes; the weekend summary says so when a change is within five weeks. A malformed value stops startup |
| `ROTA_REMINDER_TIME_ZONE` | No (default `Asia/Colombo`) | The calendar "tomorrow" and "the coming weekend" are counted in |
| `ACP_ENABLED` / `ACP_DRY_RUN` / `ACP_EMAIL_SEND_ENABLED` / `ACP_ALLOW_NON_WSO2_RECIPIENTS` | No (defaults `false` / `true` / `false` / `false`) | The `acp_closure` sub-cron and its safety switches — see "ACP closure" above |
| `ACP_STANDING_CC_RECIPIENTS` / `ACP_TEST_PROJECT_ID` / `ACP_EXCLUDED_PROJECT_IDS` | No | Distribution lists on every notice; one CSM project to scope a run to; CSM project ids to skip |
| `ACP_SALES_ENTITY_BASE_URL` / `_TOKEN_URL` / `_CLIENT_ID` / `_CLIENT_SECRET` / `_SCOPES` | When `ACP_ENABLED=true` (scopes optional, space-separated) | sales-entity-service (Salesforce) and its own client credentials |
| `ACP_REOPEN_CHECK_ENABLED` | No (default `false`) | Registers `acp_reopen_check`, entity-service's daily ACP reopen check — see "ACP reopen check" above |
| `HOUSEKEEPING_RETENTION_DAYS` | No (default `30`) | Plain integer number of days of resolved history the `housekeeping_cleanup` sub-cron keeps — see "Housekeeping" above |
| `USAGE_ROLLUP_ENABLED` | No (default `false`) | Registers the `usage_rollup_daily` and `usage_rollup_monthly` sub-crons — see "Usage rollups" above |
| `USAGE_ROLLUP_DAILY_LOOKBACK_DAYS` | No (default `2`) | Plain integer number of days before today the daily rollup rebuilds each run |
| `MIGRATION_CASE_REPORT_OLDER_THAN_DAYS` | No (default `180`) | Plain integer number of days a migration must have been open before the `migration_case_report` sub-cron lists it — see "Per-task report emails" below |
| `MIGRATION_CASE_REPORT_LINK_BASE_URL` | No | The support portal's case page prefix (https), for the links in the migration case scan (`<prefix>/<case number>`). Unset renders plain text |

No app-level execution timeout is configured here — Choreo's own Scheduled Task execution-time
limit already bounds how long one invocation can run. `cmd/server/main.go` instead cancels its
context via `signal.NotifyContext` on `SIGTERM`, so however that signal arrives (Choreo's own
timeout firing, a redeploy, a manual stop), in-flight HTTP calls to entity-service abort promptly
instead of being cut off mid-request with no chance to react.

`.env` is auto-loaded from the working directory at startup if present (silently ignored if
absent), matching `integrations/csm-notification-service`'s own convention.

## Monthly product regression reports

`internal/regressionscan.SendReport`, registered three times -- `"product_regression_report_apim"`,
`"product_regression_report_iam"` and `"product_regression_report_integration"` -- is the port of
ServiceNow's three *Monthly Product Regression Report* scheduled reports. Each one was a plain list
report on cases filtered by the regression tag, "Fix ETA shared on last month", and that business
unit's product list, mailed as a PDF on the 1st at 09:00 India Standard Time to a fixed address
list. Here entity-service builds the list (`GET /reports/product-regressions?products=...`, client in
`internal/regressionreport`; the tag and the month window live there) and this task mails it as an
inline table grouped by product, with the introduction the ServiceNow mail carried, through its own
template (`internal/notify/templates/product_regression_report.html`,
`notify.RenderProductRegressionReport`).

The three reports are one handler constructor over a `regressionscan.Variant` (key, label, product
list): `regressionscan.DefaultVariants` holds the three product lists ServiceNow filtered on, and
`PRODUCT_REGRESSION_REPORT_PRODUCTS` (a JSON object of key to names) replaces any of them without a
code change -- an unknown key or an empty list refuses to start rather than mailing an empty report
every month. Three registry entries rather than one task sending three mails so that each report
keeps its own `SUB_CRON_SCHEDULES` and `SUB_CRON_RECIPIENTS` entry by exact task name, the way
every other task here is configured. Default schedule `30 3 1 * *`: 09:00 IST on the 1st, the hour
the reports have always arrived, in UTC (same `TZ=UTC` caveat as "Housekeeping" above). The month
reported is decided by entity-service in Asia/Colombo (the same +05:30), so a retry later in the
month still reports the same month.

Two behaviours are ServiceNow's, reproduced on purpose: a report whose month has no regression
cases is **not sent** (the scheduled reports had "Omit if no records" ticked), and the audience is
**only** the task's own `SUB_CRON_RECIPIENTS` entry, as the address lists were fixed per report
there. With no `to`, the task skips its query and succeeds, like the stale and open case reports.
Each `registry.Task` leaves `To`/`Cc` nil, as `migration_case_report` does: the audience is business
unit mail groups, and a failed period is retried on every tick, so failure alerts go to
`ALERT_RECIPIENTS` only.
`PRODUCT_REGRESSION_REPORT_LINK_BASE_URL` optionally turns each case id into a link
(`<prefix>/<case number>`); unset keeps the plain text the PDF had.

## Per-task report emails

`engine.recordSuccess` still only ever updates the ledger — it never sends anything, and
`registry.Task` still has no generic "report recipients" field. A generic one-size-fits-all "task
succeeded" template was tried and dropped early on: different sub-crons want genuinely different
report content (a usage report reads nothing like a billing summary), so one shared shape would
either stay generic to the point of being useless or grow special cases per task.

`stale_cases_report`, `open_cases_report` (see "Stale cases report"/"Open cases report" above),
`migration_case_report` and the three `product_regression_report_*` tasks (see "Monthly product regression
reports" above) are the real instances of the shape that replaced it. `migration_case_report` differs in
two ways worth knowing: entity-service builds the whole report (`GET /reports/migration-cases`,
client in `internal/migrationreport`) and derives part of the audience from the team table, so the
task merges those mail groups with its own `SUB_CRON_RECIPIENTS` entry (`migrationscan.Audience`)
and **fails the run when the merged audience is empty** rather than quietly skipping — a scan nobody
receives looks exactly like a quiet week. Its `registry.Task` leaves `To`/`Cc` nil so failure
alerts reach `ALERT_RECIPIENTS` only, never the team mail groups. Each sub-cron's own package
(`internal/stalecases`, `internal/opencases`) owns its own template
(`internal/notify/templates/{stale,open}_cases_report.html`, following `alert.html`'s pattern) and
its own recipients (its own `SUB_CRON_RECIPIENTS` entry, passed in as plain `to`/`cc` slices) — the
report is sent from inside the `Handler` closure itself, entirely outside `engine.Engine`'s own
success/failure path. What they *do* share is the case-search client (`internal/entitycases.Client`)
and the row-rendering helpers in `internal/notify` (`renderCaseRows`/`humanizeState`/etc.) — real,
already-duplicated logic worth sharing, as distinct from the report's own template/copy, which
stays owned per task on purpose. A future report-sending sub-cron follows the same split: reuse
whatever's genuinely identical (a case-search method, a rendering helper), but keep its own
template, its own render function, and its own recipients wired through in `cmd/server/main.go` —
there still isn't, and isn't meant to be, one shared "send a report" mechanism in `engine`.

## Future: events

Not built yet. `engine.Engine`'s `recordSuccess`/`recordFailure` are the one place a status
transition (succeeded/failed/superseded) is known — a future `onTransition(task, period, status)`
hook there, publishing to Event Hub, would be a small addition rather than a rework, and would let
email delivery move from "this component sends it" to "this component publishes,
`csm-notification-service` sends it" — the same division of labor the rest of this system already
uses for `case.*` events. `superseded` transitions aren't currently observable at all from this
component's own code (entity-service's `Attempt` response doesn't report whether it just
superseded something) — that's a real gap if a "period X was abandoned" notice is wanted later; it
would need a small addition to the `ClaimScheduledTaskRunResponse` contract, not just to this
component.

## `cmd/server` is the ONLY package main in this component

> **Do not add a second directory under `cmd/`, and do not add `package main`
> anywhere else in this module. It breaks the Choreo build.**

Choreo builds this with the Google Go buildpack, which picks the package to
build by running

```
go list -f '{{if eq .Name "main"}}{{.Dir}}{{end}}' ./...
```

With exactly one result it builds that. With two it cannot choose, falls back
to the module root, finds no `.go` files there and fails the build with

```
no Go files in /workspace
```

The failure names neither of the offending directories, so it reads like a
broken build path rather than an extra main package. It has happened twice:
`cmd/availdiff`, then `cmd/mockdashboard`.

A development tool that needs its own entry point belongs outside this
module, or as a test helper, or behind `GOOGLE_BUILDABLE=./cmd/server` set on
the Choreo build — but the default assumption here is one `cmd/` directory.

## Running locally

```bash
# from operations/csm-scheduled-tasks
go run ./cmd/server
```

Runs exactly one tick against whatever `CUSTOMER_ENTITY_SERVICE_BASE_URL` points at, then exits — there is
no server to leave running.

## Commands

```bash
go vet ./...              # vet
go test -race ./...       # race-detector tests
```
