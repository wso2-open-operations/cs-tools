# Pre-PR self-review checklist — csm-scheduled-tasks

Run this against the diff before opening or updating a PR touching this app. Check every item
below and call out anything that matches as a finding before proceeding to open the PR — don't
just silently fix and move on, say what was found and why.

Every item here comes from this codebase's own documented failure history and hard-won design
decisions (`CLAUDE.md` is explicit about several rejected earlier designs) — not generic
code-review advice. If a new failure class turns up during review that isn't on this list, add it
here in the same style (short, one line, grounded in what actually happened) so the next review
catches it too.

## Cross-cutting

- [ ] No secret, token, or service URL with embedded credentials staged in the diff.
- [ ] No PII in a log line — IDs and sanitized summaries only.
- [ ] New behavior has a test exercising it, not just a happy-path smoke test.
- [ ] If this diff introduces a new convention/pattern/gotcha, `CLAUDE.md` is updated in the same
      PR — don't leave the next reader to rediscover it.
- [ ] `go vet ./...` and `go test -race ./...` both clean before opening the PR.
- [ ] Commits/PR carry this session's attribution lines, not an older convention from memory.
- [ ] `git fetch upstream main` + `git merge-tree --write-tree HEAD upstream/main` checked clean
      before pushing — don't trust a merge that was clean an hour ago.
- [ ] No real name/email/URL/org-specific identifier copied from a screenshot, pasted log, or API
      response into code, tests, docs, or commit/PR text — use a generic placeholder.
- [ ] No ticket/issue number or repo name cited in a code comment — explain the why directly.

## `cmd/` — the single-main-package rule

- [ ] No second directory under `cmd/` and no second `package main` anywhere in this module — the
      Choreo buildpack picks the build target by `go list`-ing for `package main`, and a second
      result makes it silently fall back to the module root and fail with a misleading "no Go files
      in /workspace" that names neither offending directory. This has broken the build twice
      already (`cmd/availdiff`, `cmd/mockdashboard`). A dev tool needing its own entry point goes
      outside this module, as a test helper, or behind an explicit `GOOGLE_BUILDABLE` override.

## Adding or changing a sub-cron

- [ ] A task's `Name` is never renamed casually — it's the entity-service ledger key **and** the
      exact key both `SUB_CRON_SCHEDULES` and `SUB_CRON_RECIPIENTS` look it up by (two separate
      JSON maps, both keyed on this same string). A rename orphans the task's prior ledger history
      and silently stops matching any existing schedule/recipients override in either config unless
      both are updated in the same change.
- [ ] A new task's handler returns `func(ctx context.Context) error` and knows nothing about
      retries, periods, or the ledger — that's the engine's job, not the handler's.
- [ ] A new task's `To`/`Cc` are left unset in the `registry.Task` literal and wired through
      `recipientsFor`/`scheduleFor` from config, not hardcoded — matching every existing task.
- [ ] A new cron schedule's time-of-day assumption is checked against `TZ=UTC` — `PeriodKey`
      interprets every schedule in whatever `time.Time.Location()` the process's clock returns, so
      "03:00" only means 03:00 UTC if the deployment's container actually runs with `TZ=UTC`. Don't
      assume a documented local time without checking the deployment's env.
- [ ] A new report-style task (success sends an email) follows the established pattern — the
      handler closure sends its own report email directly, with its own template and its own
      `SUB_CRON_RECIPIENTS` entry — rather than adding a generic "task succeeded" hook to
      `engine.Engine`. A one-size-fits-all success-report mechanism was tried and deliberately
      dropped early on; don't reintroduce it for a new task's convenience.
- [ ] A report-style task fails the run (not just skips quietly) when its audience ends up empty
      and emptiness would be surprising/dangerous to miss silently — `migration_case_report` does
      this deliberately ("a scan nobody receives looks exactly like a quiet week"); decide
      per-task whether "no recipients configured" should be a silent skip (the common case,
      `stale_cases_report`/`open_cases_report`/the regression reports) or a hard failure.

## The ledger / retry model — don't fight it

- [ ] No `max_attempts`/give-up-state logic is added anywhere — this design has none on purpose: a
      broken job keeps retrying (and alerting) every eligible tick until it's fixed or its own next
      period supersedes it. A new "stop retrying after N failures" feature is a real design change,
      not a bug fix, and needs its own deliberate decision, not a quiet addition to one task.
- [ ] No "sweep" pass is added to rescue an old, unresolved period after the calendar has moved on
      — this was tried in an earlier design and removed once the rule became "only ever chase the
      newest period"; an old unresolved period is meant to be abandoned (superseded), not
      resurrected.
- [ ] A handler relying on "what did I already send" state for retry-correctness is checked against
      the fact that **this design keeps no record of partial progress** — a retry re-runs the
      handler from scratch and, for an email-sending task, re-sends everything, not just what
      failed last time (the rota reminder tasks document this explicitly: "a retry re-sends every
      window"). Don't write a handler that assumes idempotency it doesn't actually have, or that
      silently double-sends something that matters if sent twice.

## Alerting

- [ ] A new failure path doesn't bypass `ALERT_RECIPIENTS` — every failed attempt, for every task,
      alerts the standing audience unless both `ALERT_RECIPIENTS` and that task's own `To` are
      empty. Don't add a task-specific failure channel that silently replaces this.
- [ ] A new report-style task reads `ALERTS_ENABLED` directly for its own send gate (as
      `stale_cases_report`/`open_cases_report` do) — it is not routed through
      `Engine.recordFailure`/`recordSuccess` at all, so the global kill switch has to be checked
      explicitly inside the task's own handler, not assumed to apply automatically.
- [ ] A task's own `SUB_CRON_RECIPIENTS` entry is never confused with `ALERT_RECIPIENTS` — for a
      report-style task, `SUB_CRON_RECIPIENTS` is the actual report audience, unrelated to failure
      alerting; a deployment wanting the same people for both has to configure both explicitly.

## Dates, time zones, and "which clock"

- [ ] A new date/time cutoff states explicitly whether it's anchored to UTC (a calendar-day
      boundary like `open_cases_report`'s "before yesterday," always UTC regardless of deployment
      `TZ`) or to the process's local time (a cron schedule's own firing time, via `PeriodKey`) —
      these are two different, easily-confused clocks in this codebase and conflating them has
      already caused a documented ambiguity worth re-reading before adding a third kind.
- [ ] A retried task that announces "which day/period" computes it from the run's own period key
      (`Config.Anchor`), not from `time.Now()` at retry time — a Thursday run retried after
      midnight must still announce Friday, not Monday (the rota reminder tasks get this right
      deliberately; a new date-sensitive task should follow the same pattern).

## Security

- [ ] No app-level execution timeout is added to fight Choreo's own Scheduled Task limit — this
      component already cancels its context via `signal.NotifyContext` on `SIGTERM` so in-flight
      calls abort cleanly; don't add a second, possibly-conflicting timeout mechanism.
- [ ] A new external input reaching an email/calendar-invite template (names, notes, URLs) is
      escaped and validated the same way the rota reminder's ICS builder already does (`ics.go`:
      control-character stripping, `ValidInviteAddress`, `ValidScheduleURL`) — nothing from
      configuration or an upstream record should be able to inject a line into a generated
      invitation.
