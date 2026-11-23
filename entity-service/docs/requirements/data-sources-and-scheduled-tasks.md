# Data sources, mirroring and scheduled-task runs — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 7.

**Audience:** product owners, CS leads, QA, operators and engineers new to the service.

Related: [event-publishing.md](event-publishing.md), [salesforce-ingest.md](salesforce-ingest.md),
[platform-and-operations.md](platform-and-operations.md) (the settings).

---

## 1. Purpose

The platform is moving case, project and related data from a legacy case system into the
CSM database. During the move the service can run in different **modes**, so a deployment can
be switched forward or back without a code change. This document says what each mode
guarantees. It also describes the **scheduled-task run record**, which lets a separate task
runner claim, retry and clear periodic jobs safely.

## 2. Data-source modes

**INT-300** The service shall run in exactly one of three modes, chosen at start-up by one
setting (`DATA_SOURCE`). The accepted values are `postgres` (the default; called **database**
mode here), `servicenow` (**legacy** mode) and `postgres-servicenow-dual-write` (**dual-write**
mode). Any other value shall stop the service from starting. The shorthand names in this document
are not valid setting values.

**INT-301** In **database** mode the CSM database is the system of record for every read and
write. No legacy system is called for them.

**INT-302** In **legacy** mode, reads and writes for the migrated entities shall go to the
legacy case system through its integration service. The CSM database is optional: it backs only
the features that have no legacy equivalent. With no database configured, features that need
one are not offered (their routes answer "not found"), and the service shall still start.

**INT-303** In **dual-write** mode the CSM database shall remain authoritative for **every**
read and write, and selected writes shall additionally be copied to the legacy system
afterwards. The legacy system is a rollback target, not a source: it is not read in this mode
(except where section 4 says otherwise), and is never authoritative.

**INT-304** Legacy and dual-write modes shall need the legacy integration service's address,
token address, client id and client secret. If any is missing the service shall refuse to
start. Dual-write needs a database; the three database settings (user, password, name) shall be
set together or not at all in every mode.

**INT-305** Features that exist only in the new platform (team schedules, the event queue,
scheduled-task runs, SLA status, failure records and similar) shall always be stored in the
database whatever the mode, and shall be offered only when a database is configured.

**INT-306** SLA reads shall be selectable separately (`postgres` or `servicenow`, by their own
setting), so SLA can move at its own pace. SLA status and clock-state reads have no legacy equivalent and always use the database.

## 3. Copying writes to the legacy system (dual-write)

**INT-310** Every dual-write copy shall be made **after** the CSM database write has committed.
The caller's request shall never wait for the legacy system and shall be answered from the
database result. (The one wait is the local failure-record write in INT-313.)

**INT-311** The copy shall run in the background on a small fixed pool, with a ten-second limit
per copy. Copies for the **same record** shall be applied in the order they were made, one at a
time, so an older change cannot overwrite a newer one; copies for different records run in
parallel.

**INT-312** A copy shall be tried **once**. On failure, the service shall try to store a failure
record (record kind, record id, the operation, the payload and the error) and shall log a warning.
Storing the record can itself fail; the service then only logs the error and carries on, so the
failure records are **not guaranteed to be a complete replay list** and operators must also check
the logs. The failure shall never be shown to the caller, and the copy shall not be repeated
automatically.

**INT-313** The queue limit (256) shall count **distinct records** waiting for a worker, not
copies. A later copy for a record that is already queued or running shall wait in that record's own
queue, which has no size limit. When the queue is full, the service shall not call the legacy
system for the dropped copy: it shall record it as a failure with a synchronous write to the CSM
database, so it can still be replayed. The caller waits for that local write (but never for the
legacy system).

**INT-314** The kinds of record copied are: cases and case tags, comments (including case
comments), time cards, change requests, projects, deployments and deployed products,
escalations, incidents, problems, call requests, and conversations.

**INT-315** Nothing shall read the failure records back automatically. They exist so an operator
can see exactly what the legacy system is missing, and replay it, before treating it as a live
rollback target. (**Confirm**, see 7.1.)

**INT-316** Some flows differ by design in dual-write mode:
- A project created by the Salesforce ingest shall take the id the legacy system already uses
  for it (see salesforce-ingest.md, INT-073).
- A service request raised from an external issue can be created first in the legacy system when
  that option is switched on.
- A service request raised by a portal is created in the legacy system first, and that system's
  own flow announces it (see event-publishing.md).

**INT-317** Where the legacy system also sends its own emails (case escalation, change-request
notices), the service shall publish its own equivalent only when explicitly allowed:
change-request notices are off until switched on, and the escalation email can be switched off.
Turning either on without turning the legacy flow off emails the same people twice.

## 4. Reads in dual-write mode

**INT-320** Instance and usage reads shall come from the CSM database by default. A deployment
can set them to `legacy` (the product-usage read setting; its other value is `postgres`), which sends those reads to the legacy system in dual-write mode, to
restore the earlier routing. The setting has no effect in the other two modes (legacy mode
always reads the legacy system; database mode always reads the database).

## 5. Scheduled-task runs

A separate task runner runs periodic jobs (reminders, sweeps, clean-up). Several copies of the
runner, or a retry after a crash, must not run the same job twice for the same period. The
service stores one **run record** per (job name, period) and decides, atomically, whether a job
may run now.

**INT-330** The service shall keep one run record per job name and period, with: the number of
attempts, the last error, when to retry, when it first and last ran, and when it succeeded or was
superseded. Status shall never be stored; it is read from which of those dates is set: succeeded
(done for this period, for good), superseded (abandoned because the next period came due first),
or failed/open (eligible for another try once its retry time has passed).

**INT-331** The runner shall **claim** a job by giving a job name and a period. A job name and a
period are both required. The answer shall say whether it may run now, together with the record
either way.
- A period the job has not seen before: any other still-open record for the same job is marked
  superseded, a new record is created, and the job is allowed.
- A record that has already succeeded or been superseded, or whose retry time is still in the
  future: not allowed, the record is returned unchanged.
- A record that is still open and whose retry time has arrived, or that looks like an orphaned
  claim (the runner crashed): the attempt count goes up and the job is allowed.
*Check:* claim the same job and period twice at once; exactly one is allowed.

**INT-332** Claims for the same job name, even for two different new periods, shall be handled
one at a time, so there is never more than one open record for a job.

**INT-333** A claim with no retry time that has been open longer than a limit shall count as
orphaned and be reclaimable. The limit is chosen by the runner (seconds), defaulting to one hour.

**INT-334** The runner shall report the outcome of an attempt, giving the attempt count it was
handed:
- **succeeded:** the record is marked done and its error and retry time cleared;
- **failed:** needs an error message and a retry time; the record stays open and eligible for
  another try or for being superseded by the next period.
An attempt count below one, a missing error or retry time on a failure, or any other status shall
be refused.

**INT-335** An outcome report shall be accepted only if the record is still open **and** the
reported attempt count still matches. A runner that stalled and was reclaimed by another therefore
cannot overwrite the newer attempt; its report is refused as "not found or no longer active".
This does **not** make a job run exactly once: a runner that is still working when its claim goes
stale can be overtaken by a second attempt, and both may perform the work. Jobs must therefore
tolerate duplicate effects.

**INT-336** The service shall list run records, optionally only failed, succeeded or superseded
ones. Any other filter shall be refused. The list is not paged, and is meant for monitoring, not
for the runner's own decisions.

**INT-337** The service shall delete finished records older than a given time on request (the
time must be a valid RFC 3339 timestamp, else refused). The age is measured from when the record
succeeded or was superseded, not from when it was created. A record that is still open is never
deleted, however old, because it is an unresolved problem. The runner calls this daily.

**INT-338** A job that is removed from the runner while it still has an open failed record leaves
that record behind forever; nothing clears it automatically. (**Confirm**, see 7.2.)

**INT-339** These actions are offered only when a database is configured, and the service does not
check which caller is using them. (**Confirm**, see 7.3.)

## 6. Work only a service may trigger

**INT-340** The work the runner drives against this service that writes to many records or
sends email shall be available only to a service credential, never to a person's token. A call
with a user token, even an internal user's, is refused. These are the daily project reopen check
(see salesforce-ingest.md), the public-holiday refresh, and the holiday-reminder "what is due"
and "record that it was sent" actions (see team-schedule.md).

## 7. Open questions for the product owner

1. **Failed mirror copies are never replayed.** Operators must find and replay them. Should there
   be a replay tool or job before the legacy system is retired as a rollback target?
2. **Retired jobs leave records behind.** Removing a job from the runner needs a manual clean-up.
3. **No caller check on run records.** Any caller that reaches the service can claim, report or
   delete runs. Only the gateway protects them.
4. **Dual-write copies are one-shot.** A legacy system that is down for a minute loses every
   write in that minute, and nothing retries it.
5. **Mode comments are out of date.** The code comment on dual-write says it is "piloted on the
   account entity only"; the service in fact copies fourteen kinds of record. Which statement is
   the plan?

## 8. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Modes and validation | `internal/config/config.go`, `internal/server/routes.go`, `cmd/api/main.go` |
| Mirror dispatcher | `internal/service/sn_writeback.go`, `internal/repository` (failure records) |
| Scheduled-task runs | `internal/handler/scheduled_task_run_handler.go`, `internal/service/scheduled_task_run_service.go`, `internal/repository/scheduled_task_run_repo.go` |
| Service-only guard | `internal/server/internal_only.go` |
| Routes | `POST /scheduled-tasks/attempts`, `PATCH /scheduled-tasks/attempts/{id}`, `GET /scheduled-tasks/attempts`, `DELETE /scheduled-tasks/attempts` |
| Migrations | scheduled task run table (`0108`) |
| Settings | `DATA_SOURCE`, `SLA_DATA_SOURCE`, `PRODUCT_CONSUMPTION_DATA_READ_MODE`, the four legacy integration-service connection settings, `DB_*` |
| Tests | `scheduled_task_run_*_test.go`, `sn_writeback_test.go`, `config_test.go` |
