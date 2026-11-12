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

`registry.Task.MaxAttempts` (0, the default, is no cap) limits how many times one period is
attempted, the first run included. A failed attempt that reaches it gets no retry: the ledger has
no "give up" state, so the engine gives it a retry time ten years out, the next period's claim
supersedes the row as usual, and the failure alert says no retry is coming. Only for a task whose
retry repeats something outward (today `acp_closure`, 2: see "ACP closure").

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

`housekeeping.CleanupAttachmentUploads`, registered as `"attachment_upload_cleanup"` (daily at 03:30), is a
trigger only: it calls entity-service's `POST /attachments/uploads/cleanup`, which marks uploads that were never
confirmed `FAILED` and removes unlinked comment images and report drafts from Azure itself. This component's
client id must be in entity-service's `M2M_CLIENT_IDS` (the endpoint is internal-only). A file that could not be
removed is logged as a warning and tried again next run; only a failed call fails the run.

`housekeeping.ExpireClosedAttachments`, registered as `"attachment_retention_expire"` (daily at 04:00), removes the
files of work items that have been closed for `ATTACHMENT_RETENTION_DAYS` (default 90, at least 30). It is a trigger
only: it calls entity-service's `POST /attachments/retention/expire`, which removes the files from Azure and marks
the rows `EXPIRED`. It deletes customers' files, so it is **off unless `ATTACHMENT_RETENTION_ENABLED=true`** and it
**starts as a dry run** (`ATTACHMENT_RETENTION_DRY_RUN` defaults to `true`): read the `expired` count in the log,
then set it to `false`. It asks for `ATTACHMENT_RETENTION_BATCH_SIZE` (default 100) at a time while entity-service
says there are more, at most 20 pages a run. A value below 30 days refuses to start. A `TRYOUT_TASK=attachment_retention_expire` is always forced to a dry run. Deploy entity-service with
migrations 0284 and 0285 first, and this component's client id must be in its `M2M_CLIENT_IDS`.

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
sub-cron that sends an email on success rather than only on failure; see "Per-task report emails"
below for why that's not a generic engine feature. It mails entity-service's long-open cases report:
`internal/longopenreport.Client` calls `GET /reports/long-open-cases?olderThanDays=30` (same base URL
and OAuth client credentials as the other entity-service clients, no extra env vars) and
`notify.RenderLongOpenCasesReport` renders the result as two tables, in this order: "Ongoing cases"
(the report's `cases`, projects that are not being onboarded) and "Ongoing cases on onboarding
projects" (`onboardingCases`). Columns: Case, WSO2 Case ID, Integration CS Team, Assignee, State,
Severity (S0-S4), Last updated, Created. Which cases count as ongoing, the split between the tables, the
oldest-first order and the per-table row cap all live in entity-service; this task only fetches,
renders and sends. The 30-day window is a fixed constant (`staleCaseOlderThanDays` in
`cmd/server/main.go`), not an env var, and the subject is
`[Action-Required][Report] Cases open for more than 30 days`.

This depends on the entity-service long-open cases endpoint: deploy entity-service first. Against an
entity-service without it the task fails (404) and the engine alerts.

A table that entity-service cut at its cap carries a "Showing only the oldest N of TOTAL" note, and an
empty table shows "No cases." If both tables are empty the report omits itself: nothing is sent, a
count-only line is logged, and the run succeeds. Both case columns link to `<CSM_PORTAL_WEB_BASE_URL>/cases/<internal id>` (plain text when it is unset).

Default schedule `0 7 * * *` (daily at 07:00, same `TZ=UTC` caveat as "Housekeeping" above);
override via `SUB_CRON_SCHEDULES`.

Recipients are **not** `ALERT_RECIPIENTS`: that standing audience is about failure alerting, not
report distribution. This task's report goes only to its own `SUB_CRON_RECIPIENTS` entry (`"to"`/
`"cc"`, the same config surface every task's failure-alert recipients use; see "Alerting" below);
a deployment that wants the same people getting both configures the same addresses in both places.
If this task isn't mentioned in `SUB_CRON_RECIPIENTS` at all (`to` is empty), or `ALERTS_ENABLED` is
false, the handler skips the entity-service call entirely and just succeeds. A fetch or send failure
returns an error. `--run-now=stale_cases_report --recipients=... [--cc=...]` works through
`reportRunNow` like every report: it mails the override list only, even for a task with no `to` of its
own.

`internal/entitycases.Client` remains a separate entity-service client, used by `open_cases_report`,
even though both point at the same `CUSTOMER_ENTITY_SERVICE_BASE_URL`/credentials; see that
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
rolling duration: a case created at 23:59 yesterday (UTC) is
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
| `rota_manual_reminder` | `*/5 * * * *` (finer than the driver, so due at every run) | whatever was asked for on the Rota Access card since the last run: a Morning, Evening or weekend "Send now", or a test sent only to the person who asked (see "Send now" below) |

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
  to everybody on the weekend, LK and Americas (with the Americas On-call person) alike, in the layout the
  CRE weekend emails already had: "Hello team,", a Saturday Team and a Sunday Team list
  ("Name (Rotation)"), a Rotation Definitions table (Rotation with its member count | Local time on
  the rotation's own clock | UTC window | Role: "IST/PDT General support" or "On-call", with a note under a general-support
  window longer than 10 hours that has 2+ people every day asking them to split it into morning
  and evening shifts), the zone
  legend and a daylight saving note, then the Handover paragraphs (`notify.RenderWeekendRota`).
  The Americas Weekend's hours, and the Americas On-call's that cover it, are 08:00-18:00
  America/Los_Angeles (`ROTA_REMINDER_WINDOWS`), not the Team Schedule's 21:00-06:00 IST; the
  invitations use the same hours. Then each window (LK
  Saturday, Americas Saturday, LK Sunday, Americas Sunday) gets its own email to its own people,
  carrying its invitation with all of them as guests, so they can start a group chat from the
  event. One recipient is greeted by full name, several with "Dear all,".
- **The CRE weekend summary also lists the SRE weekend people** (`Config.Support`,
  `rotareminder.SREWeekendSupport`, set in `main.go` when `ROTA_REMINDER_FAMILY` is CRE): a
  second `POST /team-schedule/assignments/search` with `family: "SRE"` reads `SRE_WE_TZ1`
  ("SRE Weekend (TZ1 + TZ2)") and `SRE_TZ3` ("SRE Night (TZ3)") for the same two days. They
  appear in an "SRE Weekend Support" block after the day teams ("Name (SRE Weekend, L1)", the
  turn's `tier`), get one Rotation Definitions row per rotation (IST hours, "SRE support (L1 →
  L2)" from the tiers seen), and are added to the summary's To. That is all: they never count
  toward the weekend minimums, get no invitation or cancellation, and are not in the weekend
  record; IaaS and PaaS turns are not listed. The SRE read happens only when the summary is
  being sent, and a failed read is logged and the summary goes without the block.
- **Settings saved on the Rota Access tab win.** Every run first reads
  `GET /team-schedule/reminder-settings/{family}` (entity-service, migration 0242; the
  "CRE rota reminders" card, editable by a CSM admin or the CRE rota admin). When something
  is saved: `weekdayEnabled` / `weekendEnabled` turn that task off (the run succeeds and
  sends nothing), `invitesEnabled` can turn invitations off but never on where the deployment
  has them off (`ROTA_REMINDER_INVITES_ENABLED`, or no organizer), `inviteRotations`
  (migration 0271) turns them off for one rotation at a time, by its window's code
  (`CRE_MORNING`, `CRE_EVENING`, `CRE_WEEKEND`, `CRE_WEEKEND_NIGHT`; a code not named keeps
  them, `Config.invitesFor`): that rotation's reminder goes out without its invitation, the
  weekend summary mentions invitations only while one still follows, and an invitation already
  sent is neither updated nor withdrawn (it stays in the weekend record, so it is not cancelled
  as a window gone from the rota), `weekendRequired` and
  `windows` override `ROTA_REMINDER_WEEKEND_REQUIRED` / `ROTA_REMINDER_WINDOWS` code by code
  (a code the saved settings do not name keeps the configured value; the card saves a code
  only once somebody changes it there), and `copyTo` is copied on top of the configured copy
  list, `weekendCopyTo` (migration 0271) on the weekend summary only (`Config.WeekendCopy`: never on
  a weekday reminder, an invitation email or a cancellation). So saving one field never replaces a deployment setting nobody changed. **Nothing
  saved (a 404, also from an entity-service without the route) sends no scheduled reminder**:
  only the card's switches turn them on. Send now still goes out, with the environment's values.
  A failed read fails the run, so it is retried
  rather than sent with settings somebody turned off. Not on the card, on purpose: the
  send times, `ROTA_REMINDER_REDIRECT_TO`, the family, the shift codes and names, and the
  email service settings.
- **"Send now" (`rota_manual_reminder`, every run).** The card's Send now button queues a
  request in entity-service (`team_schedule_reminder_request`, migration 0242; asked for by a
  CSM admin or the family's rota admin, refused while one that sends any of the same emails is waiting
  and within 10 minutes of the last that did not fail: `weekday` overlaps `morning` and `evening`). This task has no address to call, so at every run it claims the
  waiting requests (`POST .../reminder-requests/{family}/claim`, up to 5 a run) and sends each
  with the code above: `morning` is the next working day's Morning with its on-call, `evening` its
  Evening (each found by its code ending `_MORNING` / `_EVENING` in `ROTA_REMINDER_WEEKDAY_SHIFTS`;
  `windowRotations`), `weekday` both (what the card sent before it offered them apart), `weekend` the coming weekend
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
  (`CODE=Name`). Names are escaped. Every subject starts `[<family>]`
  (`[CRE] Weekend Rotation – …`; a test adds `[Test] ` in front).
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

`internal/autoclosure.Run`, registered as **`case_autoclosure`** (default schedule `30 2 * * *`: 02:30 UTC =
08:00 IST, daily including weekends, the slot of the legacy job it replaces; TZ=UTC caveat as above, and the real
firing is the first driver start after the slot), reminds and closes tickets that sit in Awaiting Info / Solution Proposed with no activity.
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
| `AUTOCLOSURE_SCHEDULED` | `true` | `false` keeps the task configured but off the schedule: the normal tick never fires it, and it runs only on demand with `--run-now=case_autoclosure` (see "Running a task now"). Affects only this task. Needs `AUTOCLOSURE_ENABLED=true` to matter; anything but a boolean refuses to start |
| `AUTOCLOSURE_INTERVAL_DAYS` | `7` | Days each stage waits (calendar days); above 365 refuses to start |
| `AUTOCLOSURE_DAILY_LIMIT` | `500` | Reminders + closures per UTC day, across all stages; `0` = no cap |
| `AUTOCLOSURE_DRY_RUN` | `false` | Decide and log only (entity-service records `DRY_RUN` audit rows, not counted against the budget; this task spends the daily limit itself, so the totals are what a real run would do). **A `TRYOUT_TASK=case_autoclosure` is always forced to a dry run**: a tryout skips the ledger but still talks to the real entity-service, and would otherwise comment on and close real tickets |
| `AUTOCLOSURE_STATES` | both | Comma list of `awaiting_info`, `solution_proposed`. Run `solution_proposed` alone first, add `awaiting_info` after watching it; an unknown value refuses to start |
| `AUTOCLOSURE_TEMPLATES` | built-in | JSON `{"<state>.<kind>": "text"}` overriding comment wording; kinds `first_reminder`, `second_reminder`, `closure`; a malformed value refuses to start (it would otherwise silently post the built-in wording) |

First run: expect a large backlog (thousands of eligible tickets); the daily limit spreads the notifications over
days. Set a hold on tickets that customers asked to keep open **before** enabling.

## Incident auto-close

`internal/incidentautoclose.Run`, registered as **`incident_autoclose`** (default schedule `0 * * * *`, hourly;
TZ=UTC caveat as above), closes incidents that have been Resolved for `INCIDENT_AUTOCLOSE_DAYS` (default 14, the
threshold measured in the previous system: 399 of 400 of its auto-closes came 14 days after resolution), with no
comment and no work note. It follows the decisions made for "Case auto-closure" above rather than new ones:

- **Off unless `INCIDENT_AUTOCLOSE_ENABLED=true`.** It closes incidents, so it is never on by accident. Run it once
  per environment with `INCIDENT_AUTOCLOSE_DRY_RUN=true`, review the count (including the incidents with no
  `resolved_on`, measured from their last update), then enable it. Deploy entity-service with migration 0273 first.
- **The close is entity-service's own incident update**, as the system user `AUTOCLOSURE_SYSTEM_USER_EMAIL`
  (optional; empty acts as "System user" with no user row, so `closed_by` stays empty), the same user case
  auto-closure closes as. In dual-write the close is therefore mirrored to the backing data source like any other
  incident update. The previous system's own auto-close job still closes its copy until it is retired; while both
  run, a mirrored close of an incident that job already closed may be refused there, which is recorded in
  `sn_writeback_failures` and never undoes the close in Postgres (not verified against the backing data source).
  Switching that job off before enabling this under dual-write is a decision for the team, as it is for case
  auto-closure. Once it is retired, this task is what closes both copies.
- **Config is checked at startup**, as `autoclosure.RunConfig.Validate` is: a day count outside 1-3650 (what
  entity-service accepts) refuses to start rather than failing and alerting every hour. So does a daily limit
  that is negative, above 100000 or not a whole number, and a non-boolean `INCIDENT_AUTOCLOSE_SCHEDULED`.
- **Daily limit.** `INCIDENT_AUTOCLOSE_DAILY_LIMIT` is sent as `dailyLimit` on every request. entity-service
  subtracts the incidents closed since 00:00 UTC today from it and recomputes that from its database on every
  call, so no state is carried between pages or runs. **Manual closes count toward the limit**, so a busy day of
  hand-closing leaves the job less (or nothing). When the response says `dailyLimitReached` the run stops: it is
  logged at Info, is not an error and does not alert, and the run succeeds; the rest waits for the next run or the
  next UTC day. `0` or unset is no cap.
- **Staged rollout**, as for case auto-closure: `INCIDENT_AUTOCLOSE_SCHEDULED=false` keeps the task registered
  (so `--run-now` and `TRYOUT_TASK` find it) but off the engine tick, the same mechanism as
  `AUTOCLOSURE_SCHEDULED=false`. Then, from a Choreo Manual Task component, reading the logs between steps:
  1. `--run-now=incident_autoclose --dry-run` : review how many are due (a dry run asks one page only).
  2. `--run-now=incident_autoclose --limit=20` : a small real batch.
  3. Raise `--limit`, as far as `INCIDENT_AUTOCLOSE_DAILY_LIMIT` allows.

  Then set `INCIDENT_AUTOCLOSE_SCHEDULED=true` (or remove it) so the hourly run takes over. The task's run-now
  flags are `--dry-run` and `--limit` only (`--states` is refused: there are no states). They narrow and never
  loosen: `--dry-run` cannot turn off a configured `INCIDENT_AUTOCLOSE_DRY_RUN=true`, and `--limit=N` lowers the
  daily limit for that run and is refused when it exceeds a configured `INCIDENT_AUTOCLOSE_DAILY_LIMIT` (when that
  is `0` or unset, it sets a limit for the run, at most 100000). A run-now writes no ledger row; it is safe beside
  the scheduled run because entity-service re-checks each incident under its row lock and recomputes the daily
  budget on every call.

This component's client id must be in entity-service's `M2M_CLIENT_IDS`, and entity-service must run on Postgres:
under `DATA_SOURCE=servicenow` the endpoint answers 503 and every run fails and alerts, so leave the task off there.

This task is a **trigger only**. entity-service's `POST /incidents/autoclose` picks what is due (oldest resolved
first), closes each incident through its incident service (which re-checks it under the row lock and closes its
open tasks as a manual close would), and reports `closed`, `failed`, `more` (the page was full) and
`dailyLimitReached`. See
entity-service's CLAUDE.md, "Incident auto-close".

- **Paging.** Each call asks for 200 new incidents (`PageSize`); the handler asks again while `more` is true, at
  most 20 pages (`MaxPages`, 4,000 incidents) a run. Reaching the cap with `more` still true logs a WARN and the
  run succeeds; the next hourly run continues. entity-service lists the oldest due first with no offset, and an
  incident that fails stays Resolved and due, so it comes back at the head of every later page. The request limit
  therefore grows with the failures already seen (200 + failures, at most entity-service's 1,000), so failing
  incidents do not hide the ones behind them. If a full page still holds only incidents that already failed (more
  than 800 failing), the run stops and its error says that later due incidents were not reached.
- **Timeout.** A page must finish inside entity-service's `REQUEST_TIMEOUT` (60s by default); the client waits
  the same 60s. A page cut short comes back as an error with no `closed`/`failed` lists, so the run fails and
  alerts, but no work is lost: each incident commits on its own, and the rest are still due next run.
- **Failures fail the run.** A failed call, or any `failed` entry, returns an error naming the incidents (at most
  10, then a count; each message cut to 200 characters), so the engine alerts and retries on the next tick.
  Retrying is safe: closed incidents are no longer due. One incident failing does not stop the others on its page
  or later pages; it is named once per run even when later pages report it again.
- **A dry run asks once.** Nothing is closed, so a second call would return the same page; the run logs each
  incident it would close and, when `more` is true, that more are due. A response that says it was not a dry run
  when one was asked for fails the run.

| Env var | Default | Meaning |
|---|---|---|
| `INCIDENT_AUTOCLOSE_ENABLED` | `false` | Register the task |
| `INCIDENT_AUTOCLOSE_DAYS` | `14` | Days an incident stays Resolved (from `resolved_on`) before it is closed; a value outside 1-3650 or not a whole number refuses to start |
| `INCIDENT_AUTOCLOSE_DRY_RUN` | `false` | List what is due, close nothing. A malformed value refuses to start rather than falling back to a real run. **A `TRYOUT_TASK=incident_autoclose` is always forced to a dry run**, as for `case_autoclosure` |

## ACP closure

`internal/acp.Run`, registered as **`acp_closure`** (default `0 4 * * 1-5`, 09:30 Colombo on
weekdays; TZ=UTC caveat as above), is the Account Closure Process, moved here from
`integrations/acp-closure-service` (still in the repo until this task runs in production; it now
refuses to start unless it is a log-only dry run, so the two can never both run for real, and its
rules are frozen: change them here). **Off
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
- **Reopening is not here.** Until entity-service's ACP reopen (PR #3124) is deployed with
  `CSM_ACP_REOPEN_ENABLED=true`, a renewed or paid project stays suspended until someone reopens
  it by hand. Once it is, entity-service reopens an ACP-suspended project when a Salesforce
  event shows it renewed or paid, and once a day over every ACP-suspended project (the
  `acp_reopen_check` task below drives that); csm-notification-service sends the "Back to Open
  state" email (each one's CLAUDE.md, "ACP reopen"). entity-service keeps a copy of the invoice and
  date rules; `closure/rules_parity_test.go` and `sweep/rules_parity_test.go` run ACP's rules over
  the cases in `entity-service/internal/service/testdata/acp_rules_parity.json`, which entity-service
  runs over its copy too (skipped when that file is absent; `ACP_RULES_PARITY_FILE` points at
  another path). A rule changed here and not there makes ACP suspend a project every day and the
  reopen check reopen it, so change both and the file together.
- **Salesforce first, then one CSM update.** A suspension sets Salesforce `State__c` to Suspended
  (`suspendInSalesforce`), then writes the closure state **and** the suspension's record
  (`"event_type": "suspend"` with `"suspended_on": "YYYY-MM-DD"`, Colombo date) to CSM in one
  `PATCH` (`writeSuspension`). The day-0 notice before it is recorded as `"suspend_notice"`, which
  reads as window 0, so a retry sends no second notice and only retries the suspension.
  `"suspend"` is never written on its own: an Open project with a `"suspend"` record is what
  entity-service's reopen reads as "a person set it back to Open", and it would set Salesforce Open
  and email "Back to Open" for a project that was never suspended (PR #3124/#3126 review). So a
  Salesforce failure leaves the project Open in CSM with `"suspend_notice"` only, and the next run
  tries again (CSM first used to leave it for someone to fix by hand); and a reopen running between
  the two writes, after an event or the Salesforce echo of ACP's own write, finds nothing to reopen
  or finish. `suspended_on` is what entity-service checks before emailing for a project set back to
  Open by hand (only for a suspension in the last 7 days).
- **Dry run by default** (`ACP_DRY_RUN`, unset or malformed is true): decide and log, write
  nothing to CSM or Salesforce. Notices are only logged unless `ACP_EMAIL_SEND_ENABLED=true`
  (which needs `EMAIL_BASE_URL` and `EMAIL_FROM_ADDRESS`). Addresses outside `@wso2.com` are
  dropped unless `ACP_ALLOW_NON_WSO2_RECIPIENTS=true` (production only).
  `ACP_STANDING_CC_RECIPIENTS` adds the distribution lists to every notice; leave it empty
  outside production.
- **Startup refuses the unsafe combinations** (`acpConfig`, `cmd/server/acp_config.go`, table
  tests beside it), because each one either suspends customers who were never told or sends
  them the same notice on every run (PR #3126 review):
  - `ACP_DRY_RUN=false` without `ACP_EMAIL_SEND_ENABLED=true`. A real run records each notice and
    suspends on day 0 whatever the notifier did, so with emails off it would record notices nobody
    received and suspend customers who never got one. `acp.Run` refuses it again as a last guard.
  - `ACP_DRY_RUN=true` with `ACP_EMAIL_SEND_ENABLED=true` and no `debug` list. A dry run records
    nothing and windows are thresholds, so every run would send every due notice again (a project
    90 days out would get the 90-day notice on about 30 weekdays). With a debug list the
    repetition reaches only testers, which is the point.
  - A `debug` list with `ACP_DRY_RUN=false` and no `ACP_TEST_PROJECT_ID`. Notices would reach only
    the debug list while every project is recorded as notified and suspended on day 0.

  **`ALERTS_ENABLED=false` (this component's switch for all email) turns `acp_closure` into a dry
  run without emails** (logged at startup) instead of refusing: refusing would stop every other
  task too, and a real run with emails silenced is the first case above. Nothing is written, so no
  notice is recorded as sent; once alerts are back, the next real run sends the notice due then.
  **Notices are not caught up**: only the narrowest window reached is sent (`closure.dueWindow`),
  so after a pause longer than a window a project that crossed the 15- and 7-day windows meanwhile
  gets neither, and one whose end date passed gets the day-0 notice and is suspended in the same
  run. Keep a pause short, or check which projects crossed a window during it.
- **Testing.** A `debug` list for `acp_closure` in `SUB_CRON_RECIPIENTS` sends every notice there
  instead (`notify.ForReport`, `[Test copy]` subject), as for a report task. `ACP_TEST_PROJECT_ID`
  scopes a run to one CSM project. A `TRYOUT_TASK=acp_closure` is forced to a dry run unless
  `ACP_TEST_PROJECT_ID` is set, since a tryout talks to the real CSM and Salesforce, and it runs
  on weekends too (it emails only its debug list). A `--run-now=acp_closure` is a real run with
  the configured settings, the same as a scheduled one.
- **Before the first real run**, run once with `ACP_DRY_RUN=false` scoped to the test project
  (`ACP_TEST_PROJECT_ID`, a `debug` list) and read the project back from CSM and Salesforce: its
  `suspensionProcessState` and closure state must have changed. A 200 from the PATCH is not proof
  (this component calls entity-service directly, not through csm-integration-service as the
  standalone service did, so its write path is new).
- **Weekends.** The schedule is weekdays only, and the handler also succeeds without doing anything
  on a Colombo Saturday or Sunday, for a retry that lands there, logging a WARN that says the run
  did nothing. Monday's run catches up: windows and suspension are thresholds. A tryout runs
  anyway.
- **Fresh read.** The open projects are all listed first, then each one is read again from CSM
  (`GetProject`) right before it is decided: by the last project the listing is as old as the
  run, and every record write sends the whole `suspensionProcessState` rebuilt from what was read,
  so deciding from the listing could undo a change made since (entity-service's reopen, say). CSM
  reads its own database, so the re-read sees every committed write. The write itself is not
  conditional: entity-service has no version check on `PATCH /projects/{id}`.
- **Failures.** A failed project never stops the others, with one exception: a CSM write failure
  every project would hit (401/403, 429, 5xx, or no response; `writesFailingError` in
  `sweep/writer.go`) stops the run. Notices go out before they are recorded, so carrying on would
  send every remaining due notice unrecorded and the next run would send them all again. A project
  whose own record is unusable (no Salesforce id, one Salesforce does not have, a date that does
  not parse; `dataError`) is **skipped** with a WARN and does not fail the run: no retry can fix
  it, and failing would retry and alert every weekday until someone fixed the record. Any other
  failure makes the run return an error, so the engine alerts and retries. The error names the
  run id and at most 20 failed project ids, never their errors (they can carry customer data); the
  log has each one. `RetryBackoff` is **4 hours** and **`MaxAttempts` is 2** (one retry a day,
  `registry.Task.MaxAttempts`): a retry repeats nothing already recorded in CSM, except a notice
  whose email went out while its CSM record failed, which is sent again, so the rest waits for the
  next weekday's run.
- **A write is tried before every notice** (`probeRecordWrite`, `CheckRecordWritable`): the project's
  `suspensionProcessState` is written back unchanged (`{}` when it has none). If CSM refuses it for
  the project's own reason (400, 404, 409, 422), nothing is sent and the project is skipped as a
  `dataError`, on every run until its record is fixed. Without it, a project whose record CSM
  refuses would get the same notice every weekday, sent and never recorded (PR #3126 review). A
  refusal every project would get stops the run before the first notice. The cost is one extra CSM
  write per notice.

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

**Turn it on together with entity-service's live reopen.** `ACP_REOPEN_CHECK_ENABLED=true` here and
`CSM_ACP_REOPEN_ENABLED=true` with `CSM_ACP_REOPEN_DRY_RUN=false` in entity-service are a pair. With
entity-service's reopen live and this off, a reopen that fails past Service Bus's redeliveries is never
retried, and a project set back to Open by hand is never finished. With this on and entity-service's
reopen off, every run fails with a 404 (or a 403 if this client is not in `M2M_CLIENT_IDS`).

- **Runs before `acp_closure`**: registered first, so a tick that covers both (the driver late, or a
  restart after 04:00 UTC) still reopens a project before `acp_closure` could send it a reminder.
- Calls entity-service with the shared `OAUTH2_*` client (its id in entity-service's
  `M2M_CLIENT_IDS`, as for ACP), with a 2-minute timeout per page.
- A run in which a project failed returns an error naming at most 20 project ids (the errors are in
  entity-service's log); retried after 4 hours, like `acp_closure`, and at most once a day
  (`MaxAttempts` 2), so a failure that lasts (entity-service's reopen not on yet) alerts twice a day,
  not five times; the next day's run catches up. A page error, a page that makes no progress, or more
  than 2,000 pages stops the run with an error.

## Running a task now (`--run-now`)

`--run-now=<task name>` runs that task's handler once, right now, then exits (0 on success, 1 on failure), so an
operator does not have to edit an env var and remember to revert it. Any task registered in this process can be
named (`cmd/server/runnow.go`). It is **real**: unlike `TRYOUT_TASK` it is not forced to a dry run and not forced
to a debug audience. At start it logs one WARN with the task name and its effective settings, so it is visible
in Choreo logs.

Refusals (non-zero exit, a message, never a fall-through to the schedule): an unknown flag, a stray positional
argument, an empty or malformed value, `--run-now` together with `TRYOUT_TASK`, and a name that is not a
registered task. A task is registered only when its own enable flag and config allow it (for example
`case_autoclosure` only with `AUTOCLOSURE_ENABLED=true`), so a disabled task is refused as "not a registered
task", and the message lists the registered names.

| Argument | Meaning |
|---|---|
| `--run-now=<task>` | Run that registered task once now |
| `--dry-run` | Force a dry run. Only for tasks with a narrowing hook (today `case_autoclosure` and `incident_autoclose`) |
| `--limit=N` | Daily limit for this run, `N` > 0; at most the configured limit when that is non-zero. Same tasks as `--dry-run` |
| `--states=a,b` | Work only these states, each among the configured ones. `case_autoclosure` only |
| `--from-date=YYYY-MM-DD` `--to-date=YYYY-MM-DD` | Report an explicit inclusive window, plain dates in Asia/Colombo. Only `cs_worklogs_weekly_report` and `cs_worklogs_daily_report`. Both are required together, `--to-date` not before `--from-date`, at most 92 days (the entity-service cap), else a startup error. Weekly uses the range as is (any length up to 92 days); daily needs `--from-date` equal to `--to-date` |
| `--recipients=a@x,b@y` `[--cc=c@x]` | **Replace** the report's whole audience for this one run (To and Cc both; see "Recipient override" below). Every report task takes it. Plain comma list, no spaces or display names; an empty entry, whitespace, or a malformed or dotless-domain address is a startup error. `--cc` needs `--recipients` |
| `--project-key=KEY` `--to=a@x,b@y` `--cc=c@x` | **Only `time_card_report`** (see "Monthly time card report" below). `--project-key` runs one project, configured or not; `--to` replaces the project's derived contacts with your list (one mail, all in To); `--cc` adds copies. Same address rules as `--recipients`, at most 50 addresses per list, one bare key. `--to` and `--recipients` cannot be combined; every other task refuses `--to` and `--project-key` |

`--dry-run`, `--limit` and `--states` are valid only together with `--run-now`, and only for a task that
implements the optional `registry.Task.RunNowNarrow` hook; any other task exits non-zero with `<flag> is not
supported by task <name>`. The hook can only tighten the task's configured values, never loosen them. Do not add
per-task flags to a task that does not need them.

The window flags are valid only with `--run-now` and only for the two CS Worklogs report tasks, and the recipient
flags only for report tasks (each lists them in `registry.Task.RunNowFlags`; any other task exits non-zero with
`<flag> is not supported by task <name>`, and the report tasks refuse `--dry-run`, `--limit` and `--states` the same way). They are
refused together with `TRYOUT_TASK`, like every run-now, so they cannot weaken the tryout's debug-recipients rule.
`--recipients` wins over a `debug` list in `SUB_CRON_RECIPIENTS`. The subject of a window run is prefixed
`[Manual range]`, and `[Test copy]` when recipients are replaced; a scheduled run's subject is unchanged.

Example (test a custom week to one inbox, nothing reaches the real audience):

```
csm-scheduled-tasks --run-now=cs_worklogs_weekly_report --from-date=2030-01-07 --to-date=2030-01-15 \
  --recipients=jane.doe@example.com --cc=john.roe@example.com
```

**Recipient override (`--recipients`, `--cc`).** Valid for every report task: the two CS Worklogs reports,
`migration_case_report`, `open_cases_report`, `stale_cases_report`, `unresolved_products_digest`, the three
`product_regression_report_*` and the two `query_hours_*` tasks (`reportTaskNames` in `cmd/server/runnow.go`; startup
refuses to run if one of them is registered without the hook, `tasksMissingReportRunNow`). One mechanism: each task's
`RunNowNarrow` is `reportRunNow(normal, raw, build)` (the CS Worklogs ones `worklogsRunNow`, which also takes the window
flags), and `build` is the task's own handler constructor, so the scheduled `Handler` is untouched. With
`--recipients`, `build` gets `notify.ForRunNow(raw, recipients, cc)`: the mail goes to the given addresses only, the
Cc is the `--cc` list or empty, the subject is prefixed `[Test copy]`, and a notice at the top of the body names the
real To and Cc, which are never addressed. The To passed to the handler is the task's real To, or the override list
when it has none (`orRecipients`), so a task that skips or errors on an empty audience still sends. The explicit list
wins over the task's `debug` list (the override sender wraps the raw client, not the debug-mode one); without
`--recipients` a run-now still honours the debug list. Failure alerts to `ALERT_RECIPIENTS` are not affected. The
rota reminders send to the rota's people, not a report audience, and are not covered. Refused with `TRYOUT_TASK` and
without `--run-now`; `--cc` needs `--recipients`; `ALERTS_ENABLED=false` still silences the mail. The log carries
counts of replaced recipients, never the addresses. A report that sends nothing when it has nothing to report stays
silent, and a query-hour notification whose account has no internal owner is still skipped (that task also
recomputes and stores thresholds, as every run of it does, so a `--recipients` run of it consumes the crossings it
finds: the real owners never get those notices later, and the run-now WARN carries a `warning` key saying so).

Example: `csm-scheduled-tasks --run-now=migration_case_report --recipients=jane.doe@example.com`. It needs the normal
env (entity-service, email service, `ALERTS_ENABLED=true`) but no recipient config.

**Ledger.** An override run writes **no ledger row**, like every run-now: `runOnDemand` runs the handler and exits
without the engine, so `ledger.Attempt`, `Complete` and `Fail` are never called (`cmd/server/main.go`, the run-now
branch returns before `engine.New`). It therefore cannot mark a scheduled period done or take its claim, and the
next scheduled run is unaffected. The flip side is the one stated below: the report mails are not idempotent.

**Email.** A run-now of a task that sends email sends to its real recipients, or to its `debug` list in
`SUB_CRON_RECIPIENTS` if it has one, exactly as a scheduled run does. `ALERTS_ENABLED=false` still silences the
report emails. Use `TRYOUT_TASK` to see a report land in a debug inbox without touching the real audience.

**It writes no ledger row.** A row would either mark the period done (suppressing the scheduled run) or compete
with it for the claim. The consequence is plain: the scheduled run for the same period still fires, so a task
that is **not idempotent** sends or does its work again. Safe to run now are the idempotent tasks only. From
reading the handlers' code and doc comments (a judgement, not a test):

- **Idempotent (safe):** `case_autoclosure` (entity-service's per-ticket compare-and-set claim and its audit-table
  daily limit); `incident_autoclose` (entity-service's row-locked re-check and per-call daily budget); `acp_closure` (each notice and suspension is recorded in CSM, so the scheduled run does not repeat
  them; it is a real run, sending real notices and suspending, with the configured `ACP_*` settings);
  `acp_reopen_check` (entity-service records each reopen); `usage_rollup_daily`, `usage_rollup_monthly` (rebuild rows, documented idempotent);
  `housekeeping_cleanup` (deletes resolved rows older than retention); `publish_scheduled_announcements` and
  `cloud_status_webhooks` (a handled item leaves the due or pending set; documented idempotent per period);
  `rota_manual_reminder` (claims waiting requests, each reported once).
- **Not idempotent (will send again):** the report and digest emails, whose handlers send a mail on every run:
  `stale_cases_report`, `open_cases_report`, `migration_case_report`, `cs_worklogs_daily_report`,
  `cs_worklogs_weekly_report`, `query_hours_weekly_report`, the `product_regression_report_<key>` monthly reports,
  `time_card_report` (customer-facing: a repeat mails every contact of the project again).
  `unresolved_products_digest` also mails every run it finds anything.
- **Not verified (handler not read for this):** `availability_recalculation` (a recompute sweep, likely safe, not
  confirmed), `query_hours_usage_notifications` (mails crossings it derives in process; whether a repeat re-sends
  depends on stored state), `rota_weekday_reminder`, `rota_weekend_notice`, `rota_weekend_reminder` (the weekend
  notice records its send in entity-service, the weekday reminder was not checked for a repeat guard).

When unsure, treat it as not idempotent.

**Example: staged rollout of case auto-closure.** Set `AUTOCLOSURE_ENABLED=true` and `AUTOCLOSURE_SCHEDULED=false`
(the task stays configured but the normal tick never fires it; anything but a boolean refuses to start), then,
from a Choreo Manual Task component (or the scheduled task, if it accepts per-run arguments), run in order,
reading the logs between steps:

1. `--run-now=case_autoclosure --dry-run` : see what is due, change nothing.
2. `--run-now=case_autoclosure --states=solution_proposed --limit=20` : a small real batch of one state.
3. Raise `--limit`, then drop `--states` (or add `awaiting_info`), as far as `AUTOCLOSURE_DAILY_LIMIT` and
   `AUTOCLOSURE_STATES` allow.

When satisfied, set `AUTOCLOSURE_SCHEDULED=true` (or remove it) so the daily 02:30 UTC run takes over. For this task
the run-now is safe beside the scheduled run for the reason above: entity-service decides everything. Its
`--limit` may not exceed `AUTOCLOSURE_DAILY_LIMIT` unless that is `0`, and `--states` must be within
`AUTOCLOSURE_STATES` (both when unset). The price of no ledger row: a run-now is not in the run history, only in
the logs (`run-now: ...`).

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
| `ROTA_REMINDER_WINDOWS` | No (default `CRE_WEEKEND_NIGHT=08:00-18:00 America/Los_Angeles,CRE_WEEKEND_NIGHT_OC=08:00-18:00 America/Los_Angeles`) | A rotation's hours on its own clock, used in the emails and invitations instead of the Team Schedule's (`CODE=HH:MM-HH:MM Zone`, comma-separated). The Americas Weekend and the Americas On-call that covers it are 08:00-18:00 Pacific time, so it moves an hour in UTC and IST when US daylight saving changes; the weekend summary says so when a change is within five weeks. A malformed value stops startup |
| `ROTA_REMINDER_TIME_ZONE` | No (default `Asia/Colombo`) | The calendar "tomorrow" and "the coming weekend" are counted in |
| `ACP_ENABLED` / `ACP_DRY_RUN` / `ACP_EMAIL_SEND_ENABLED` / `ACP_ALLOW_NON_WSO2_RECIPIENTS` | No (defaults `false` / `true` / `false` / `false`) | The `acp_closure` sub-cron and its safety switches — see "ACP closure" above |
| `ACP_STANDING_CC_RECIPIENTS` / `ACP_TEST_PROJECT_ID` / `ACP_EXCLUDED_PROJECT_IDS` | No | Distribution lists on every notice; one CSM project to scope a run to; CSM project ids to skip |
| `ACP_SALES_ENTITY_BASE_URL` / `_TOKEN_URL` / `_CLIENT_ID` / `_CLIENT_SECRET` / `_SCOPES` | When `ACP_ENABLED=true` (scopes optional, space-separated) | sales-entity-service (Salesforce) and its own client credentials |
| `ACP_REOPEN_CHECK_ENABLED` | No (default `false`) | Registers `acp_reopen_check`, entity-service's daily ACP reopen check — see "ACP reopen check" above |
| `ATTACHMENT_RETENTION_ENABLED` / `ATTACHMENT_RETENTION_DAYS` / `ATTACHMENT_RETENTION_DRY_RUN` / `ATTACHMENT_RETENTION_BATCH_SIZE` | No (defaults `false` / `90` / `true` / `100`) | Registers and tunes the `attachment_retention_expire` sub-cron — see "Housekeeping" above |
| `HOUSEKEEPING_RETENTION_DAYS` | No (default `30`) | Plain integer number of days of resolved history the `housekeeping_cleanup` sub-cron keeps — see "Housekeeping" above |
| `INCIDENT_AUTOCLOSE_ENABLED` / `INCIDENT_AUTOCLOSE_DAYS` / `INCIDENT_AUTOCLOSE_DRY_RUN` / `INCIDENT_AUTOCLOSE_DAILY_LIMIT` / `INCIDENT_AUTOCLOSE_SCHEDULED` | No (defaults `false` / `14` / `false` / `0` / `true`) | Registers and tunes the `incident_autoclose` sub-cron — see "Incident auto-close" above |
| `USAGE_ROLLUP_ENABLED` | No (default `false`) | Registers the `usage_rollup_daily` and `usage_rollup_monthly` sub-crons — see "Usage rollups" above |
| `USAGE_ROLLUP_DAILY_LOOKBACK_DAYS` | No (default `2`) | Plain integer number of days before today the daily rollup rebuilds each run |
| `MIGRATION_CASE_REPORT_OLDER_THAN_DAYS` | No (default `180`) | Plain integer number of days a migration must have been open before the `migration_case_report` sub-cron lists it — see "Per-task report emails" below |
| `TIME_CARD_REPORT_PROJECT_KEYS` | No (default empty) | Comma-separated project keys the `time_card_report` sub-cron mails each month (spaces trimmed, empties and duplicates dropped). Empty or unset: the task is registered but does nothing, with a log line, so a deployment sends nothing until it names its projects. Needs `EMAIL_BASE_URL` when set (checked at startup) — see "Monthly time card report" below |
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

## Monthly time card report

`internal/timecardreport.SendReports`, registered as `"time_card_report"`, emails each project's contacts the
previous calendar month's approved effort, per case. It is **customer-facing**: the recipients are the project's
own contacts, not an internal list. Layering is the usual one: `internal/timecards` (entity-service client and wire
types), `internal/timecardreport` (the task), `internal/notify` (`RenderTimeCardReport`, `TimeCardReportSubject`,
`FormatMinutes`, template `templates/time_card_report.html`: table layout, inline styles, no vendor name or logo).

- **Projects.** `TIME_CARD_REPORT_PROJECT_KEYS`, comma-separated keys. Empty or unset is a logged no-op, never an
  error. Each key is resolved to a project id with `POST /projects/search` (a substring match on name, key and
  subscription type), keeping only the result whose `key` equals the configured one ignoring case; none, or more than
  one, fails that project.
- **Window.** The calendar month before the **period key's** month, in UTC (`PreviousMonth`): the first through the
  last day, so January reaches back to December of the previous year and February is 28 or 29 days. It derives
  from the period key (the cron firing), never the clock, so every retry of a period asks for the same window.
  Default schedule `30 5 2 * *` (05:30 UTC on the 2nd), overridable in `SUB_CRON_SCHEDULES`. Consequence for
  `--run-now`: it uses the period the schedule is in *now*, so run on the 1st before the 05:30 firing of the 2nd and
  it reports the month before last. The run-now log line has the exact from and to.
- **Report data.** `GET /projects/{id}/time-cards/report?from=&to=` (approved cards only, durations in whole minutes).
  A month with no approved time is **still sent**, with a "No time card data found for the selected period." card in
  place of the summary cards and table. Durations are formatted once, from integer minutes, as `<h>h <m>m`
  (`FormatMinutes`: 119 is `1h 59m`, 60 is `1h 0m`, never `60m`). Case type is humanised (`service_request` is
  `Service request`). Subject: `Time Card Report - <project name> - <Month YYYY>` (the key stands in for a missing name).
- **Recipients.** `POST /projects/{id}/contacts/search` (all pages): every contact with a non-empty email,
  de-duplicated ignoring case. One filter beyond legacy's none: integration users (`isCsIntegrationUser`, machine
  accounts that never read mail) are left out. Deactivated memberships are already excluded by entity-service.
  Invited-but-unregistered contacts ARE included, as legacy did. A project with no recipients is skipped with a
  WARN (project key and counts, never addresses) and does not fail the run.
- **Privacy.** The mail client has no Bcc, so a derived audience gets **one mail per address**: no customer ever sees
  another's address. Any Cc (manual runs only) is on every one of those mails.
- **Failures and retries.** Projects are independent: one failing never stops the rest, and the handler returns a
  single error naming every failed project key (`N of M projects failed (PRJ-2, PRJ-4)`), with each project's reason
  (status codes only, never addresses). A project whose mails partly failed counts as failed. **The handler has no
  memory of what it sent**, so when any project fails the engine retries the whole run on a later tick and the
  projects that already succeeded are mailed again. The only brake is `MaxAttempts: 2` on the task (one retry; the
  failure alert says no retry is coming), the same device `acp_closure` uses; there is no per-project ledger.
  To re-send one project after that, use `--run-now=time_card_report --project-key=KEY`.
- **Failure alerts.** `SUB_CRON_RECIPIENTS` `to`/`cc` for this task are the failure-alert audience, as for every
  task (the customers' addresses are never configured anywhere). A `debug` list redirects every mail to it (so one
  copy per would-be recipient), with `[Test copy]` in the subject.
- **`ALERTS_ENABLED=false`** silences it like every report.

**On-demand run (`--run-now=time_card_report`).** The one task whose run-now flags can **widen** a run, which is why
it is the deliberate exception to the "narrowing only" rule of `registry.Task.RunNowNarrow`: an operator has to be
able to send one customer's report (a missed month, a retry) or send a test to themselves, and neither is a subset of
the configured projects and contacts.

| Flags | Projects | Mailed to |
|---|---|---|
| none | `TIME_CARD_REPORT_PROJECT_KEYS` | each project's derived contacts (as scheduled) |
| `--project-key=KEY` | only `KEY`, whether configured or not | its derived contacts |
| `--project-key=KEY --to=a@x,b@y` | only `KEY` | the list, one mail, all in To |
| `--to=a@x,b@y` | every configured project | the list, one mail per project |
| any of the above plus `--cc=c@x` | unchanged | unchanged, plus Cc on every mail |

`--to` goes out with the **raw mail client**: no `[Test copy]` rewrite and it wins over a `debug` list (the operator
chose the audience, and may be re-sending to a customer on purpose). Without `--to` the task's normal sender
(debug mode included) is used. Each address is validated with `net/mail` (bare address, no display name, any invalid
one is refused, at most 50 per list); contacts are not fetched when `--to` is given. Like every run-now it writes **no
ledger row**, so it can mail a period the schedule already sent, and the start-of-run WARN says so. The window,
project count and recipient counts are logged (counts only, never addresses). The other tasks refuse `--to` and
`--project-key` as `not supported by task <name>`.

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
