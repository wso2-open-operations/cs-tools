# Incident follow-up automation and paging ranks — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check
are intended; they are collected in section 7.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related documents:** incidents, problems and tasks themselves are in
[incidents-and-problems.md](incidents-and-problems.md). The team schedule and its rota
are in [../specs/team-schedule.md](../specs/team-schedule.md).

---

## 1. Purpose

When an incident changes state or moves to another team, a number of follow-up things
should happen without anyone having to remember them: a task to write the incident
report, the report text itself, follow-up tasks for alerts that turned out to be noise, a
problem for incidents closed with a workaround, and a page to the specialist team.

This document also covers the staffing data the call-escalation ladder reads: which rank
each person holds on their team.

## 2. How the automation runs

The automation is not triggered by the request that changed the incident. Each change to
an incident's state or assignment group is recorded in a change log, and a background
process reads that log and applies the follow-up.

**CHG-400** The service shall act on every recorded change, however late it is read. A
change that arrives after downtime is still acted on; there is no "off" setting.

**CHG-401** Each change shall be applied together with marking it done, so that changes
the service makes in its own database (tasks, problems) and the mark both happen or neither
does. Sending a message (the special-ops alert, CHG-464) cannot be undone, so it is delivered
**at least once**: if the message goes out and the commit then fails, the change is read again
and the message is sent again.

**CHG-402** If a change fails, it shall be retried with growing waits, up to 10 attempts
(about three hours in total). After that it is **parked** with the reason kept on the log
entry, and it is not retried again until someone resets it by hand.

**CHG-403** One change failing shall not hold up the others.

**CHG-404** If the database does not yet have the change this automation needs, the process
shall wait quietly (logging once, then hourly) and start by itself once it does, without a
restart.

**CHG-405** The log shall be read every few seconds (5 by default; a setting).

**CHG-406** A change to an incident that has since been deleted shall be skipped without
an error.

## 3. When an incident moves to In Progress

**CHG-410** Each move **into In Progress** shall create one follow-up task on the
incident: subject "[Incident Report] Create the incident report for <incident number>",
priority **Critical**, type "Incident Report", state Open and active, with the incident's
service, assignment group and assignee copied from it. The task is created by "system".

**CHG-411** The task shall get a number from the incident-task series.

## 4. When an incident is Resolved

**CHG-420** Each move **into Resolved** shall write the incident report text onto the
incident (the "incident report" field), overwriting whatever was there, even a report a
person had written.

**CHG-421** The report shall be a fixed template of nine headings: Incident Number,
Incident Severity Level, Incident Identification Time, Timeline, Affected Users or
Customers, Affected Functionality, Cause(s) if known, Initial Response Actions Taken and
Next Steps. The first three shall be filled in: the incident number; the priority as
"1 - Critical", "2 - High", "3 - Moderate", "4 - Low" or "5 - Planning" (or "-" when there
is none); and the time the incident was created, in UTC ("YYYY-MM-DD HH:MM:SS"). The
others are left as "-" for whoever writes the report up.

**CHG-422** The text shall be escaped, so a value never changes the layout of the report.

### 4.1 Follow-up tasks for noisy alerts

**CHG-430** When an incident on one of **two named cloud services** is resolved, one task
shall be created depending on the resolution code (at most one applies):

| Resolution code | Task subject starts with | Priority |
|---|---|---|
| False alarm | "[Alert Task][Falser Alarm]" | Critical |
| Duplicate | "[Alert Task][Duplicate Alert]" | Critical |
| Not actionable alert | "[Alert Task][Not Actionable Alert]" | High |

The task is for the SRE team's group (a fixed group; if that group does not exist in the
database the task is still created, unassigned), carries the incident's service and is
created by "system". Incidents on any other service get no alert task.
**Confirm** (see 7.2).

### 4.2 The workaround problem

**CHG-440** When **any** incident is resolved with the code "Solved (workaround)" and has
no problem linked, the service shall create a problem for it and link the incident to it.
(The rule for alert tasks is limited to two services; this one is deliberately not.)

**CHG-441** The problem shall be created with the subject "Fix the root cause of
<incident number>", the incident's service, impact and urgency (Low when missing) and the
priority derived from them (the matrix in CHG-205 of
[incidents-and-problems.md](incidents-and-problems.md)).

**CHG-442** The problem's assignment group shall depend on the service: a fixed group for
each of the two named cloud services, and the incident's own assignment group for any
other (none, when the incident has none). A fixed group that does not exist in the
database leaves the problem created without a group.

**CHG-443** The problem and the link shall be written together with the change being
processed. An incident that already has a problem gets no second one.

**CHG-444** In the older dual-write mode the resolve request itself creates the problem in
both stores, and this automation skips it so there is only one.

## 5. The special-operations page

**CHG-460** Whenever an incident's **assignment group changes to the group of a
specialist team** (any team in the handoff configuration), however it changed, the service
shall publish an **incident special-ops alert** so the notification service can page the
team and post to its room. It is not published when the group changes to any other group,
or when none is configured.

**CHG-461** The alert shall carry the incident's number, subject, description, state,
priority, impact, urgency, service, the product and team (key and label), the on-duty
rota team that answers for the specialist team (when configured), the group it moved to
and the group it left, who made the change and when.

**CHG-462** When several teams share one group, the alert shall be for the team named by
the handoff's reason note (the team picked in the dialog). If no note names one of them,
the first is used and a warning is logged.

**CHG-463** A team configured without a group id shall be matched by resolving its group
from the team schedule on demand, so a handoff is never missed because of a start-up
timing problem. If that lookup fails the change is retried (CHG-402).

**CHG-464** A failure to publish shall be retried like any other (CHG-402), and parked in
the end. The alert is published before the change is marked done, so it is delivered at least
once and can repeat after a failed commit (CHG-401); the notification service should tolerate
a repeated alert for the same incident.

## 6. Ranks for the call-escalation ladder

The call-escalation ladder has five rungs. Four of them are found from a person's **rank**
on a team: the sub leads and the lead of its own team (levels 1 and 2), and the
two heads (levels 3 and 4), who sit in a leadership team and not in a team of their own.
The fifth rung is whoever is on duty now and is a sub lead: the ladder asks for ranks and
for who is on duty, and intersects the two.

**CHG-470** Each team membership shall carry a **rank**, one of: engineer, sub lead, lead,
the Americas team lead (one above that team's three team leads), CRE head and CS head.
Ranks are a position on the team, not a granted permission: they are stored on the
membership, separate from the rota admin roles, so a person may hold either without the
other.

**CHG-471** There shall be at most one Americas team lead per team (at night the ladder
calls that person alone at its second level).

**CHG-472** A lead, an Americas team lead or a head shall never also hold a
first-responder alert tier (T1, T2 or T3).

**CHG-473** A caller shall be able to ask who holds which rank on given teams, by team
key or by team type (a whole group of teams such as "every team in this ABT", so the list
does not go stale when a team is added), optionally narrowed to some ranks and to some
alert tiers. For each person the answer shall give the team, the team's type, the rank,
the alert tier and the person's id, name and email.

**CHG-474** At least one team key or team type shall be required. An unfiltered read
would hand back every membership in the organisation and is refused.

**CHG-475** An unknown rank in the request shall be refused naming it. Blanks (for
example from a trailing comma) shall be ignored, and team types are compared ignoring
letter case.

**CHG-476** A team with nobody at the asked-for rank shall return an empty list, never a
failure: the ladder treats that as a rung that reaches nobody.

**CHG-477** Only internal callers (staff, and the ladder itself as an internal client)
shall read it. It returns names, addresses and ranks, which belong to no project and so
have no narrower scope a customer could be given.

---

## 7. Open questions for the product owner

1. **A second In Progress creates a second report task.** The report task is created on
   every move into In Progress, with no check for an existing one. An incident put on hold
   and then taken again gets another "[Incident Report]" task.
2. **The alert tasks are limited to two hard-coded services and one fixed group.** Other
   services' false alarms, duplicates and non-actionable alerts get no follow-up task.
   The workaround problem is not limited that way. Is the difference intended?
3. **The report is overwritten each time.** A person's edit to the incident report is
   replaced when the incident is resolved again.
4. **One rule is not carried over.** A "runbook solved the issue" follow-up task has no
   field to read in the portal, so that follow-up never happens.
5. **Parked changes need a manual reset.** After ten failures a change stops, and only a
   direct edit to the change log restarts it. Who watches for parked rows?
6. **The special-ops alert can be sent twice.** It goes out before the change is marked done, so a
   failed commit means the next pass sends it again. Should repeats be suppressed per change?
7. **A group change that matches several teams** alerts the first one when no note names
   one. A warning is the only trace.

## 8. Where this is implemented

| Area | Where (entity-service unless noted) |
|---|---|
| Change log reader and retries | `internal/service/incident_report_service.go` (`IncidentReportDrainer`), `internal/repository/incident_report_repo.go` |
| Report task, report text, alert tasks, workaround problem | `internal/service/incident_report_service.go`, `internal/service/workaround_problem.go` |
| Special-ops page | `internal/service/incident_report_service.go`, `internal/events/incident_special_ops.go` |
| Ranks | `internal/service/team_member_service.go`, `internal/handler/team_member_handler.go`, `internal/domain/team_member.go`, migrations `0170`, `0185` |
| Route | `GET /team-schedule/members` |
| Settings | `INCIDENT_REPORT_POLL_INTERVAL`, `SPECIALIST_HANDOFF_CONFIG` |
| Tests | `incident_report_service_test.go`, `post_resolution_tasks_test.go`, `post_resolution_tasks_integration_test.go`, `special_ops_alert_integration_test.go`, `workaround_problem_test.go` |
