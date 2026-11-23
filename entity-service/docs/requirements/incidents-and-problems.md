# Incidents, problems, incident tasks and conversations — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check
are intended; they are collected in section 14.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related documents:** what happens automatically after an incident changes (report
task, incident report, follow-up tasks, the workaround problem, the special-operations
page) is in [incident-reports.md](incident-reports.md). SLA clocks are covered by
[../specs/sla.md](../specs/sla.md). Change requests are in [change-requests.md](change-requests.md).

---

## 1. Purpose

An **incident** is a disruption to a service that CS engineers and the on-call teams must
restore. A **problem** is the underlying cause behind one or more incidents, worked
through its own states until it is fixed. An **incident task** is a follow-up piece of work
raised against an incident. A **conversation** is a chat a customer starts with WSO2 from
a project.

This document covers what each of them requires: how it is created, who may see and
change it, which values are allowed, what happens automatically, and what other services
are told.

## 2. Who is involved

| Who | What they do |
|---|---|
| CS engineer / SRE | Creates, updates, resolves and hands off incidents; works problems and tasks. |
| Monitoring and alert ingestion | Creates incidents as a machine caller, with no user token. |
| Customer contact | Starts and closes conversations in their own projects. They never see incidents, problems or tasks. |
| Paging and notification services | Receive events when incidents are created, taken or handed off. |

---

## 3. Creating an incident

**CHG-200** The service shall let only internal callers (CS engineers and internal
machine clients) create an incident. A customer or partner is refused.
*Check:* create as a customer contact; refused.

**CHG-201** A new incident shall require a subject, a caller (the person who reported
it), a category, a service, an impact and an urgency. A missing one is refused naming the
field.

**CHG-202** The allowed values shall be:

| Field | Allowed |
|---|---|
| Category | Inquiry, Service interruption, Security |
| Impact, Urgency | High, Medium, Low |
| Contact type | Self service, Email, Walk-in, Azure, Email (internal), Site 24x7, Direct, Phone, Sentinel, Virtual agent, Chat, Email (external) |

A subcategory is optional and must belong to the chosen category. Anything else is refused
with a message that names the field.

**CHG-203** An environment, when given, shall be at most 40 characters.

**CHG-204** A new incident shall start in **New**, receive a number from the portal's own
incident series (it starts with "INC"), and record the caller's email as its creator. For
a machine caller with no user token the creator is the machine client's identifier.

**CHG-205** The priority shall always be worked out from impact and urgency and not
taken from the caller:

| Impact \ Urgency | High | Medium | Low |
|---|---|---|---|
| High | Critical | High | Moderate |
| Medium | High | Moderate | Low |
| Low | Moderate | Low | Planning |

*Check:* create with impact Low and urgency Low; the priority is Planning.

**CHG-206** Optional details may be given on create and are stored with the incident:
service offering, configuration item, assigned engineer, watch list, parent, parent
incident, linked change request, linked problem, "caused by" and a **correlation id**.
The correlation id lets an external system find the incident it created by exact match.

**CHG-207** Optional "additional comments" (customer-visible) and "work notes"
(internal) given on create shall each become their own journal entry once the incident
exists.

**CHG-208** After the incident is saved, the service shall start its SLA clocks for its
priority. A failure to do so is logged and never fails the create.

**CHG-209** After the incident is saved, the service shall publish an **incident created**
event carrying what the paging ladder needs: the incident's number, title, short
description, when it was reported (its opened time), contact type, priority, team (the
assignment group's name), category, state, environment and service. A failure to publish
is logged and never fails the create. Under the older dual-write mode the event is
published only after both stores hold the incident.

## 4. The assignment group chosen on create

Every route into incident creation (the portal, the chat microapp, alert-born incidents,
any machine client) goes through one decision, made once, before anything is written.

**CHG-210** If an assignment group is sent, it shall be used if it is an active group
that supports at least one service, or is on the configured extra list. Otherwise the
create is refused with the code for "group not allowed". A value that is not an id is also
refused. A blank value counts as not sent.

**CHG-211** If none is sent, the incident shall get the **support group of its service**.

**CHG-212** If the service has no support group, the support group of the **default
service** (a setting) shall be used and a warning logged. A service id that names **no
service at all** is not rescued by this step: the group is still looked up as above, but the
incident is then refused as a bad request ("one or more referenced IDs do not exist"), because
the service id is saved as sent and must refer to a real service. No incident is created.

**CHG-213** If the service exists but has no support group, and there is no default service,
or it has none or does not exist, the incident shall be created **unassigned** and an error
logged (a misconfiguration, not a bad request). If a lookup itself fails the incident is not
created.

**CHG-214** The decision shall be recorded as an internal work note appended after the
caller's own notes ("Assignment group chosen by …", "set from service …'s support group",
or "service … has no support group; assigned to the default team …"). The caller's text is
kept as sent.

**CHG-215** A lookup shall tell the portal the default service and its support group, so a
form can show them before the user chooses. At startup the service shall log whether the
default service is usable (an error when it does not exist or has no group); this never
stops the service.

## 5. Updating an incident

**CHG-220** An update shall accept three kinds of change, alone or together: the
**state transition** (state, assigned engineer, resolution code, resolution notes,
resolved-by), the **record's own fields** (subject, category, subcategory, contact type,
impact, urgency, priority, parent, parent incident, assignment group, service, service
offering, configuration item, linked change request, linked problem, caused-by, incident
report, environment, watch list) and **notes** (work notes and additional comments). At
least one must be given. Fields sent as null are treated as not sent; only the
environment can be cleared.

**CHG-221** The incident's states shall be New, In Progress, On Hold, Resolved, Closed
and Cancelled. **No transition is checked for legality** (any state can be set from any
other), and no state needs an assignee or an assignment group, including In Progress. The
portal's list of allowed next states is guidance only.
**Confirm** (see 14.2).

**CHG-222** Moving to **Resolved** or **Closed** shall need a resolution code and
resolution notes, either in the request or already on the incident. Missing either is
refused and nothing is saved. Resolution codes are: Solved (workaround), Solved
(permanently), Not solved (not reproducible), False alarm, Duplicate, Not actionable alert.

**CHG-223** Entering Resolved shall stamp the resolved time and the resolver. The resolver
is the one named in the request, otherwise the caller.

**CHG-224** Moving to **Closed** or **Cancelled** shall also close the incident's open
incident tasks: every task not already closed and still active becomes "closed
incomplete" (for Closed) or "closed skipped" (for Cancelled), with an internal work note
on the task written as the acting user.

**CHG-225** A change to impact or urgency shall recompute the priority from the new value
and the one already stored, using the matrix in CHG-205. An explicit priority is written
only when neither impact nor urgency changes in the same request.

**CHG-226** A subcategory shall always belong to the incident's category. Setting a new
subcategory needs a category; a new category that does not hold the current subcategory
clears it.

**CHG-227** An edit's references (parent, group, service, offering, configuration item,
change request, problem, caused-by, assignee, resolved-by, watch list) shall be valid ids
of existing records, or the update is refused naming the field.

**CHG-228** The state change, the field changes, the notes and the watch-list replacement
shall be saved together: a failure in any leaves nothing saved.

## 6. SLA clocks and the activity feed

**CHG-230** When the state changes, the service shall apply the matching effects to the
incident's SLA clocks (complete, cancel, pause or resume, according to each policy's conditions), and shall
also start any clock that has just become applicable.

**CHG-231** When priority, service or category change, the service shall revise the
incident's clocks, and only when one of those values actually changed (a note alone, or a
failed read of the old value, never restarts a running clock).

**CHG-232** These SLA effects happen even where no event publishing is configured. They
are best-effort and logged on failure; they never fail the update.

**CHG-233** For each field a request actually changed the service shall write one entry to
the incident's **activity feed** (what it was, what it is, who changed it). Only fields
named in the request are compared, so a change someone else made in between is never
attributed to the caller; a field sent with the value it already had is not a change.
Values are shown the way a person reads them (for example "In Progress"). If the old
value could not be read, or there is no actor, nothing is written, and a failure never
fails the update.

**CHG-234** An internal caller shall be able to read an incident's activity feed (comments
and field changes), newest first, paged, saying whether more exist.

## 7. What the paging ladder is told

**CHG-240** When an incident moves **out of New** (to any other state) the service shall
publish an **incident acknowledged** event. A move that stays in New, or a move from any
state other than New, publishes nothing.

**CHG-241** When an engineer is set on an incident and differs from the one before, the
service shall publish an **incident assigned** event (assignee, team, who updated).
Clearing the assignee, or re-sending the same one, publishes nothing.

**CHG-242** The events shall be published after the update is saved, and a failure is
logged and never fails the update.

**CHG-243** On the native data source the service shall **not** publish the events that
start a ladder for a **priority elevation** or stop it on a **public comment**. Only the
older mode that talked to the legacy case system published them. **Confirm** (see 14.1).

## 8. Handing an incident to a specialist team

A CS engineer can escalate an incident to a specialist ("special operations") team of
the product the incident's service belongs to.

**CHG-250** The service shall refuse the handoff, as a conflict, unless **all** hold: the
incident's service belongs to a configured product; the incident is **In Progress**; it is
not already with one of that product's specialist groups; it has an **assignee** (the
assignee takes responsibility for the page to the on-duty engineer); and the chosen team
has an assignment group.

**CHG-251** A product with several teams shall need the caller to name the team (one of
the product's own keys); a product with one team takes it automatically. A missing or
unknown team is refused, naming the valid keys.

**CHG-252** The reason shall be one of two: "Runbook is not available" or "Runbook doesn't
solve the incident".

**CHG-253** In one step the service shall: move the incident to the specialist team's
group, clear the assignee, open a **runbook task** for the same group (its subject names
the reason and the incident), and write the reason as a work note. The handoff and the
reason shall be saved together.

**CHG-254** After that, and best effort, the service shall open an **internal GitHub
issue** in the product's repository (only products with a repository do) and write an
"Escalated to Special Ops team" work note naming who did it and linking the issue. A
GitHub failure never fails or undoes the handoff; the failure text is shown.

**CHG-255** The assignment-group change shall be recorded in the activity feed, and shall
raise the specialist page (see [incident-reports.md](incident-reports.md), section 5).

**CHG-256** A lookup shall list the specialist teams available for a given service, in the
configured order, and none for a service no product covers.

**CHG-257** Whether an incident can be handed off right now shall be shown on the incident
(using the same rules as CHG-250, read without a lock), so the portal offers the action
only when it can work.

## 9. Seeing and searching incidents

**CHG-260** Only internal callers shall be able to read, search or aggregate incidents.
They have no project and so no narrower scope for a customer.

**CHG-261** Search shall accept: free text, priority, number (exact), correlation id
(exact), parent, and filters on state, assignment group, business service, category,
created and updated date ranges, assigned user, whether the SLA was made and whether it
was breached. A range whose "to" is before its "from" is refused, and so is an unknown
state or priority.

**CHG-262** Counts grouped by state, assignment group or business service shall use the
same filters.

---

## 10. Problems

A problem moves through five states in order.

**CHG-300** The problem's states are New, Assess, Root cause analysis, Fix in progress,
Resolved and Closed. Only five forward moves exist, each one step:

| Move | From → To | Needs |
|---|---|---|
| assess | New → Assess | an assignee (in the request, or already on the problem) |
| confirm | Assess → Root cause analysis | |
| fix | Root cause analysis → Fix in progress | |
| resolve | Fix in progress → Resolved | fix notes (in the request, or already on the problem) |
| close | Resolved → Closed | resolution code not "risk accepted" |

A request that names a move from the wrong state, or any other move name, is refused
naming the valid ones.

**CHG-301** Resolving shall set the resolution code to "fix applied", the resolved time
and the resolver. Closing shall mark the problem no longer active and stamp the closed
time.

**CHG-302** A problem shall also accept plain-field changes, alone or together with a
move: assignee, assignment group, cause notes, fix notes, workaround and a target
resolution date ("YYYY-MM-DD HH:MM:SS" in UTC, or RFC 3339). At least one change must be
given. An assignee or group that does not exist is refused.

**CHG-303** A **new problem** shall start in New with a number from the "PRB" series. A
subject is required, at most 512 characters. The optional category is one of Software,
Hardware, Network or Database; optional fields are a description, subcategory, originating
case and primary incident. Impact and urgency start at Low, so the priority starts at
Planning.

**CHG-304** Only internal callers shall create, read, search, aggregate or change
problems. Search filters are state, assigned user and assignment group; grouping is by
state or assignment group.

---

## 11. Incident tasks

An incident task is follow-up work against an incident. Tasks are created by the incident
automation (see [incident-reports.md](incident-reports.md)) and by a specialist handoff,
not by a person through this service.

**CHG-330** A task shall be in one of: Pending, Open, Work in progress, Closed complete,
Closed incomplete, Closed skipped. The three "closed" states are final in effect (the task
is no longer active).

**CHG-331** A CS engineer shall be able to change a task's state and close notes. At least
one must be given, and the state is matched ignoring letter case. An unknown state is
refused naming the six.

**CHG-332** Moving a task into a closed state shall make it inactive and stamp the closed
time and the person who closed it, but only if those are not already set. Moving it back to
an open state makes it active again. No transition is checked for legality.

**CHG-333** Only internal callers shall search, aggregate, read or change tasks. Search
filters are state, incident and assignment group; counts group by state.

---

## 12. Conversations

**CHG-360** A member of a project shall be able to start a conversation in it with an
initial message, which is required. The service shall start it **Active**, give it a
number from the "CHAT" series, and set its subject to the first 100 characters of the
message. Internal callers may start one in any project. A non-member is refused, and an
unknown project is refused.

**CHG-361** Conversations shall be visible to internal staff and to members of the
project they belong to.

**CHG-362** A conversation's state may be changed to Active, Resolved, Converted,
Abandoned or Closed (not back to Open). A project member or internal staff may do so; for
anyone else the conversation is "not found".

**CHG-363** Search shall filter by project, state, free text, number, who started it ("by
me", or a list of people) and a window on the last update. "Resolved in the last 30 days"
is state Resolved with a last-update window.

**CHG-364** Each conversation shall show its first message and its total number of
messages.

---

## 13. Alert-to-incident mappings

**CHG-390** The service shall keep a durable record of which monitoring alert was grouped
onto which incident, so a later alert for the same condition can be added to the same
incident rather than creating a new one. A record holds the alert number, source,
status, optional correlation key, service and metric, and the incident.

**CHG-391** A record shall require an alert number, source, alert status and incident.
An alert number can be mapped once; mapping it again is a conflict.

**CHG-392** A lookup by source and correlation key shall return every mapping, newest
first, or an empty list when there is none. Both are required.

---

## 14. Open questions for the product owner

1. **Natively created incidents do not start or stop a ladder for an elevation.** The
   priority-elevated and public-comment events are only produced by the older mode that
   talked to the legacy case system. On the native data source, raising an incident's
   priority does not start the elevation ladder, and a public comment does not stop it.
   Is that intended, or still to be built?
2. **Any state can be set from any other**, including moving a Closed incident back to New.
   The portal hides this but the service does not stop it. Should the service enforce the
   transitions?
3. **Only a move out of New acknowledges an incident.** Moving On Hold→In Progress does
   not, and neither does an assignee change alone for the "acknowledged" signal (that
   publishes the separate "assigned" signal). Confirm this matches how each ladder stops.
4. **The alert mappings have no access check.** The routes are not limited to internal
   callers, and the table has no row-level protection. Any caller that holds a valid token
   can read or write them. Should they be internal only?
5. **A closing incident force-closes its open tasks** without asking, as "incomplete" or
   "skipped", and writes a work note as the acting user.
6. **The default service is configuration.** If it is unset or has no support group,
   incidents on a service with none are created unassigned (logged as an error, nothing
   shown to the caller).
7. **Conversations.** Any project member can change a conversation's state, including
   reopening a closed one to Active.

## 15. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Incident create, update, events | `internal/service/incident_service.go`, `internal/service/sn_incident_service.go` (shared checks, event helpers), `internal/repository/incident_repo.go` |
| Assignment group on create | `internal/service/incident_assignment_group.go` |
| Activity feed | `internal/service/incident_activity.go` |
| Specialist handoff | `internal/service/incident_handoff_service.go` |
| Problems | `internal/service/problem_service.go`, `internal/repository/problem_repo.go` |
| Incident tasks | `internal/service/incident_task_service.go`, `internal/repository/incident_task_repo.go` |
| Conversations | `internal/service/conversation_service.go`, `internal/repository/conversation_repo.go`, migrations `100007`, `100020` |
| Alert mappings | `internal/service/alert_incident_mapping_service.go`, `internal/repository/alert_incident_mapping_repo.go` |
| Numbers | migration `0180` (INC, PRB, CHAT, TASK series) |
| Routes | `POST /incidents`, `GET /incidents/create-defaults`, `POST /incidents/search`, `POST /incidents/aggregate`, `GET`/`PATCH /incidents/{id}`, `POST /incidents/{id}/activities/search`, `POST /incidents/{id}/specialist-handoffs`, `GET /specialist-handoff-teams`, `/problems…`, `/incident-tasks…`, `/conversations…`, `POST /alert-incident-mappings` and `…/lookup` |
| Settings | `INCIDENT_DEFAULT_SERVICE_ID`, `INCIDENT_EXTRA_ASSIGNMENT_GROUP_IDS`, `SPECIALIST_HANDOFF_CONFIG`, `SPECIALIST_HANDOFF_GITHUB_TOKENS` |
| Tests | `incident_create_group_test.go`, `incident_lifecycle_test.go`, `incident_lifecycle_integration_test.go`, `incident_create_decode_test.go`, `alert_incident_mapping_handler_test.go` and the matching service and repository tests |
