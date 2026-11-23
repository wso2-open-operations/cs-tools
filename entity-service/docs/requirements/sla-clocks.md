# SLA clocks — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check
are intended; they are collected in section 14.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related:** [sla-policies.md](sla-policies.md) (which rules decide what a clock does),
[sla-status-and-reporting.md](sla-status-and-reporting.md) (how clocks are read and
reported), [sla-admin-actions.md](sla-admin-actions.md) (admin repair actions). For the
storage-level view of the same engine see [../specs/sla.md](../specs/sla.md).

---

## 1. Purpose

A service level clock measures how long WSO2 takes to respond to, work around, or resolve
a customer's case, and says whether the promised time was met. The service runs these
clocks itself: it starts them when a case is created, pauses and resumes them as the
case moves through its states, stops them when the promise is kept, and records whether
they ran out first.

This document covers the life of a clock. Which clocks a particular case gets, and for
how long, is decided by policies (see [sla-policies.md](sla-policies.md)).

## 2. Who and what is involved

| Who / what | Role |
|---|---|
| CS engineer | Their first public reply to a customer stops the response clock. Moving a case between states pauses and resumes its clocks. |
| Customer | A reply on a case that is "Awaiting Info" or "Solution Proposed" moves it back to "Waiting on WSO2", and that state change re-checks the clocks (section 6). |
| The engine | Starts, pauses, resumes, stops, cancels and breaches clocks, automatically, as a side effect of case changes. |
| Notification service | Is told about the changes so it can raise breach alerts ([sla-status-and-reporting.md](sla-status-and-reporting.md), section 9). |

## 3. The three clocks

**SLA-010** The service shall measure three kinds of clock: **response** (time to a first
reply), **workaround** (time to provide a workaround) and **resolution** (time to resolve).
*Check:* a clock reports exactly one of these three kinds.

**SLA-011** A case may carry more than one clock of the same kind when more than one
policy applies to it. Each is independent: it has its own start, its own rules and its own
outcome.
*Check:* two policies match one case's resolution; the case shows two resolution clocks
that can finish at different times.

**SLA-012** Whether a case gets a clock of a given kind depends only on whether a policy
applies. If none applies, no clock is started for that kind, and this is never an error.
*Check:* a case matching no workaround policy simply has no workaround clock.

**SLA-013** Clocks shall be started at creation for cases, security report analyses and
service requests. Engagements and announcements get none at creation. (**Confirm**, 14.1)
*Check:* create an engagement; it has no clocks.

**SLA-014** Each clock shall record the policy it follows, when it started, how long it
allows (taken from the policy), the working-time schedule, its stage, how long it has been
paused, and when it ended.

## 4. Clock stages

| Stage | Meaning |
|---|---|
| In progress | Running. |
| Paused | Not counting. The case is in a state the policy says to wait in. |
| Breached | Its time has run out but the clock has not yet been satisfied. It is still open. |
| Achieved | Stopped because the policy's stop rule was met. The engine's own mark for a kept (or finally satisfied) promise. |
| Completed | Also a finished stage; treated exactly like Achieved. |
| Cancelled | Withdrawn because the policy no longer applies. |

**SLA-020** Achieved, Completed and Cancelled are final. A clock in one of them is no
longer active and shall never change again.

**SLA-021** In progress, Paused and Breached are all active.

**SLA-022** A breached clock shall stay open. It keeps counting, and it can still be
stopped or paused when its real event finally happens, with the true elapsed time
recorded however far past 100% it has climbed.
*Check:* a response clock breaches; a CS engineer replies later; the clock is finished
and shows its real overrun (for example 134%), not a frozen figure.

## 5. Starting clocks

**SLA-030** When a case is created, the service shall start one clock for every active
policy whose start rule matches the case.

**SLA-031** If a matching policy's stop rule is already true at that moment, no clock
shall be started for it.

**SLA-032** A clock starts at the moment it is created, unless its policy is marked
*retroactive*, in which case it starts at the case's own creation time.
*Check:* a retroactive response clock created a minute after the case shows a minute
already elapsed.

**SLA-033** For one case and one policy there shall be at most one open clock. Asking
twice shall not create a duplicate, even when two requests arrive at the same time.

**SLA-034** A policy whose clock has already finished on a case shall not be started again
for that case. A finished clock here means Achieved or Completed; for a response clock a
Breached one also counts. A Cancelled clock does not block a fresh one. (**Confirm**, 14.2)

**SLA-035** After a state change, or a change to the workaround or fix-ETA values, the
service shall also start clocks for any policy that now matches and had no clock, unless
that policy's cancel rule would cancel it straight away.

## 6. Stopping, cancelling, pausing and resuming

Each time a trigger in SLA-044 occurs, every **open** clock is judged against **its own
policy**, in this fixed order. The first step that applies wins.

**SLA-040** (1) If the stop rule is true, the clock is stopped. (2) Otherwise, if the
cancel rule applies, it is cancelled. (3) Otherwise, if the pause rule is true, it is
paused. (4) Otherwise, if the resume rule applies, it is resumed. (5) Otherwise it is left
exactly as it is.

**SLA-041** The cancel rule has three modes, chosen per policy: *cancel when the cancel
condition is met* (the default), *cancel when the start condition stops matching*, and
*never cancel*.

**SLA-042** The resume rule has two modes, chosen per policy: *resume when the resume
condition is met* (the default) and *resume when the pause condition stops being met*.

**SLA-043** A policy with no pause rule is never paused, whatever the case does.

**SLA-044** The rules shall be re-checked when a case's **state** changes, and also when
its **workaround provided** value or any of its three **fix ETA** values change.
*Check:* providing a workaround re-evaluates every open clock; a policy that pauses on
"workaround provided" pauses, one that stops on "fix ETA shared" stops.

**SLA-045** A clock that is stopped shall be marked Achieved, ended at that moment, with
its real elapsed time recorded and its breach flag set only if that elapsed time had
reached the allowed time. A clock stopped early shall therefore read as met and show how
much of its time it actually used, not 100%.
*Check:* a response clock stopped after ten minutes of a four-hour allowance shows about
4%, not breached.

**SLA-046** Pausing shall freeze the clock's elapsed figures at the moment of the pause.
Resuming shall add the finished pause to the clock's total paused time and continue.
*Check:* pause for two days then resume; the two days are not counted as elapsed.

**SLA-047** A clock stopped, cancelled or revised while paused shall have its open pause
added to its paused total first, so a finished clock is never left looking paused.

## 7. When a case closes

**SLA-050** Closing a case shall stop every open clock on it, even if a clock's own stop
rule does not mention closing. The exception is a clock whose cancel rule applies: that is
cancelled instead.
*Check:* close a case; none of its clocks remain open.

**SLA-051** Reopening a case as an administrator shall leave its clocks alone. (A closed
case has no open clocks to change.)

## 8. The response clock and CS engineer replies

**SLA-060** A public comment from a CS engineer shall stop the case's response clock.
Only public comments count; internal work notes never do.
*Check:* a work note does not stop the clock; a public comment from a CS engineer does.

**SLA-061** "CS engineer" means a user holding the role named in the service's
configuration. If that role is not configured, the author is unknown, or the role cannot
be looked up, the service shall treat the comment as *not* a CS engineer reply. It never
guesses, and the comment itself is saved either way.

**SLA-062** Where a rule asks whether a CS engineer has already replied, a copy of the
case description that the system posts as the first comment shall not count, even when a
CS engineer created the case on a customer's behalf.

**SLA-063** The same reply shall also be flagged on the case's "comment added" event, so
the notification service can stop its own response tracking at the same moment.

## 9. When severity or case type changes

**SLA-070** When a case's severity changes, every open clock shall be cancelled and a
fresh set started for the new severity, from zero. Nothing is carried over: no old start
time, no elapsed time, no old duration.

**SLA-071** The cancel-and-restart differs by kind of clock:

- **Response:** a clock that finished (Achieved) or whose first-reply window closed
  unanswered (Breached) stays as it is, and no new response clock is started. A severity
  change cannot reopen the question "are we still waiting for a first reply".
- **Workaround and resolution:** a Breached clock is cancelled and restarted under the
  new severity. Only a genuinely finished one (Achieved or Completed) is left.

**SLA-072** A kind of clock that no longer applies after the change (for example after a
downgrade) shall be cancelled and not replaced.

**SLA-073** If the case is currently in a state that pauses its clocks, the fresh clocks
shall be paused straight away, so a paused case does not start counting down.

**SLA-074** Moving a case into or out of type "case" shall revise its clocks the same way,
and the service shall tell the notification service that the type changed.

**SLA-075** The revision shall be retried once if it fails. If the policy lookup itself
fails, the revision is skipped and the old clocks are left untouched.

**SLA-076** A cancelled clock shall keep the elapsed figures it had at the moment of
cancellation.

## 10. Working time

**SLA-080** A clock on a **24 x 7** schedule counts every second, minus paused time.

**SLA-081** A clock on the **WSO2 Business days** schedule counts every second of
Monday to Friday and none of Saturday or Sunday. It does not use office hours, and public
holidays are not skipped. (**Confirm**, 14.3)

**SLA-082** Business days are decided in **New York time**, including on the days clocks
change, which are 23 and 25 hours long.
*Check:* a business-day clock started Friday 10:00 New York time is not reported breached
on Saturday: only 14 business hours have elapsed.

**SLA-083** A schedule the service does not recognise shall be counted as 24 x 7, and a
warning logged.

**SLA-084** The allowed time shall be read from the policy's **current** duration, not a
copy taken when the clock started. A change to a policy's duration therefore applies to
clocks still running. (**Confirm**, 14.4)

## 11. Breach

**SLA-090** A clock is breached when its elapsed working time reaches its allowed time.
Its percentage elapsed is not capped at 100.

**SLA-091** A clock that is paused after its time already ran out shall show as both
paused and breached.

**SLA-092** For a clock on the business-days schedule, breach shall be judged on business
time. It shall never be reported as breached purely because of calendar time that included
a weekend.

**SLA-093** Reads shall compute the elapsed time at the moment of the read. Nothing
depends on a timer having run recently. A paused or finished clock shows the figure frozen
when it paused or finished.

## 12. Incidents

**SLA-100** Incidents get clocks from their own policies: clocks start when an incident is
created, and are restarted when its priority, service or category genuinely changes. A
note-only update does not restart anything.

**SLA-101** On every state change the engine shall judge an incident's open clocks with
the same order as for cases (SLA-040), and closing an incident stops every open clock that
was not cancelled.

**SLA-102** Incident and case rules use different numeric codes for "state" and
"priority". They shall be translated separately so one can never be read as the other.

**SLA-103** There is no manual way to complete an incident clock. Its stop rule is met by
the incident reaching a state the policy names. At present only a resolution clock for one
class of cloud incident exists (see [sla-policies.md](sla-policies.md), section 6).

## 13. Failure behaviour and switches

**SLA-110** Every clock step is best-effort. A failure shall be logged and shall never
fail the case creation, comment, or update that caused it.

**SLA-111** The engine shall run whatever data source the service is configured for,
provided a database is available.

**SLA-112** The engine can be turned off with a setting; it is **on** unless the setting
is explicitly set to false. Turning it off stops *new* clock activity only. Clocks already
open are left exactly as they were and simply never change again. (**Confirm**, 14.5)

**SLA-113** Everything the engine writes shall be attributed to the system, not to a
person.

## 14. Open questions for the product owner

1. **Engagements and announcements get no clocks at creation.** Only cases, security
   report analyses and service requests do. Is that intended?
2. **A finished clock is never restarted for the same policy.** As far as the start rules
   show, a case that is closed and then reopened (by anyone other than an admin) does not
   get that policy's clocks again. Is that what the business expects?
3. **Public holidays are not skipped** on the business-days schedule, only weekends.
4. **A policy's duration is read live**, so editing a policy changes clocks already
   running. Is that wanted, or should running clocks keep the duration they started with?
5. **Switching the engine off leaves open clocks frozen** and still visible as active.
   Nothing retires them.

## 15. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Lifecycle (start, stop, pause, resume, cancel, revise) | `internal/service/sla_engine_service.go` |
| Clock storage and updates | `internal/repository/sla_engine_repo.go` |
| Business days | `internal/businessdays/businessdays.go`, migration `0279` (SQL port) |
| Live read view | migrations `0204`, `0245`, `0272`, `0279` |
| Triggers | `case_service.go` (create, comment, update), `case_type_transfer_service.go`, incident services |
| Rule language | `internal/slacondition/condition.go` |
| Settings | `CSM_SLA_ENGINE_ENABLED`, `CS_ENGINEER_ROLE` |
| Tests | `sla_engine_service_test.go`, `sla_engine_repo_*_test.go`, `case_service_sla_registration_test.go` |
