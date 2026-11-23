# Cases: changing a case and its state — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 14.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

Related documents: [case-creation-and-detail.md](case-creation-and-detail.md),
[case-closure-and-reopen.md](case-closure-and-reopen.md) (closing, reopening,
auto-closure), [case-escalation.md](case-escalation.md), and [sla.md](../specs/sla.md).

"Case" here means any of the five ticket kinds listed in
[case-creation-and-detail.md](case-creation-and-detail.md) section 3, unless a rule says
it is for cases only. Comments, attachments, tags and watch lists are not covered.

---

## 1. Purpose

A ticket changes over its life: it is picked up by an engineer, moves between states,
changes severity, gets fix estimates, is linked to other tickets, or is re-typed. This
document covers every change the service accepts through one operation, "update a case",
and the rules, side effects and notifications that come with each.

## 2. Who is involved

| Who | What they do |
|---|---|
| CS engineer | Moves cases through the working states, assigns, acknowledges, sets estimates. |
| Customer user | Closes their own case, answers a request for information, accepts or rejects a proposed solution. |
| CSM admin | Reopens a closed case (see [case-closure-and-reopen.md](case-closure-and-reopen.md)). |
| Integrations and scheduled jobs | Mark a fix as issued; the auto-closure job moves and closes waiting cases. |

## 3. What one update request may contain

Every change goes through one request that names the case and the fields to change.
Fields fall into groups, and the groups cannot be mixed.

**CAS-200** The service shall refuse a request with no field to change, and shall name the
fields that were expected.

**CAS-201** The following are each "exclusive" groups: (a) state, severity and work state
(these three may be sent together), (b) watch list, (c) assignee, (d) parent case,
(e) acknowledge, (f) mark fix issued. A request shall contain **at most one** exclusive
group and, if it does, **nothing else**.
*Check:* send a new assignee together with a new subject; refused with a message saying the
fields cannot be combined.

**CAS-202** The following "plain" fields may be sent together in one request, but not with
an exclusive group: subject, description, deployment, deployed product, the three fix
estimates, related case, auto-closure hold, and workaround provided.

**CAS-203** Resolution code, cause and close notes may only be sent together with a state of
closed or solution proposed. Sent alone, or with any other state, they shall be refused.

**CAS-204** The ticket id in the request must be a valid id. A case the caller may not see
or change, or that does not exist, shall be answered with "not found".

**CAS-205** A reply to a successful change shall give the case id and its new update time,
plus whichever of state, severity, work state, resolved-on, assignee, acknowledger or fix
estimates apply to the change.

**CAS-206** Some fields belong to the old case system only and shall be refused here, with
a message saying so: add public comment, product, public ticket, and (unless the case is
being re-typed) engagement type, payment type, issue type, catalog, catalog item and
variables.

## 4. States and the moves allowed

A ticket is always in one of seven states: **Open**, **Work In Progress**, **Waiting on
WSO2**, **Awaiting Info**, **Solution Proposed**, **Reopened** and **Closed**. Which move is
allowed depends on who is asking. "Internal" means a CS engineer or any identity with internal
access, including someone who is also a customer contact. Everyone else is treated as a
customer.

**CAS-210** For a **CS engineer** the service shall allow only these moves:

| From | To |
|---|---|
| Open | Work In Progress |
| Work In Progress | Waiting on WSO2, Awaiting Info, Solution Proposed, Closed |
| Awaiting Info | Waiting on WSO2 |
| Waiting on WSO2 | Work In Progress |
| Solution Proposed | Closed, Waiting on WSO2 |
| Reopened | Work In Progress |

**CAS-211** For a **customer** the service shall allow only these moves:

| From | To |
|---|---|
| Open, Work In Progress, Waiting on WSO2 | Closed |
| Solution Proposed | Closed, Reopened, Waiting on WSO2 |
| Awaiting Info | Closed, Waiting on WSO2 |
| Reopened | Closed |

**CAS-212** **Closed** has no onward move for anyone. The only way out is an admin reopen
(see [case-closure-and-reopen.md](case-closure-and-reopen.md)).

**CAS-213** Any other move shall be refused with the message "state cannot move from X to
Y", and nothing shall be saved.
*Check:* a CS engineer moves an Open case straight to Closed; refused.

**CAS-214** Two moves are allowed past the tables: the auto-closure job closing an Awaiting
Info or Solution Proposed case, and an admin reopen. Neither can be requested by an ordinary
caller.

**CAS-215** Asking for the state the case is already in shall succeed and change nothing
visible: no history entry and no email. It is not treated as a move. (The SLA clock effects
are re-applied, which is harmless; see CAS-224.)

**CAS-216** If the service cannot work out the current state or who is asking, it shall
refuse the state change rather than guess.

**CAS-217** The same tables apply to all five ticket kinds. An announcement can only be
Open or Closed. (**Confirm**, see 14.2)

## 5. What a state change does

**CAS-220** The service shall set the case's work state automatically: **paused** when the
case enters Awaiting Info; cleared when it enters any state other than Work In Progress;
unchanged when it enters Work In Progress. A work state sent in the same request takes
priority.

**CAS-221** Entering **Closed** shall stamp the closing time and the person who closed it.
Leaving Closed shall clear both.

**CAS-222** The **resolved-on** time shall be stamped when the case enters Solution Proposed
from any state other than Waiting on WSO2 (so each fresh proposal restarts it), kept when
the case closes, and cleared when the case is Reopened or leaves Closed.

**CAS-223** A genuine state change shall add an entry to the case's history, showing the old
and new state names, and attributed to the person who made it, or to the system user for an
automated change.

**CAS-224** A genuine state change shall update the case's SLA clocks (pause, resume, complete)
according to the new state, whether or not email is configured. Asking for the current state
again re-applies the same effect harmlessly. See [sla.md](../specs/sla.md).

**CAS-225** A genuine state change shall publish a "status changed" event so the notification
service can email the case's active watchers and the account's default watchers, showing
the previous and new status. Nothing is published if the case has no project or no watchers.

**CAS-226** Every change shall stamp the case's last-updated time.

**CAS-227** While the old case system is still mirrored, a state change is saved locally
first and then sent to the old system in the background. A failed send is recorded for
follow-up and never fails the change. Only a change that really moved the state is sent.

## 6. Customers answering a waiting case

**CAS-230** When a **customer** adds a customer-visible comment or completes an attachment on
a case that is **Awaiting Info** or **Solution Proposed**, the service shall move it to
**Waiting on WSO2**, using the normal state change (so history, email and SLA effects all
happen as for a manual move). Work notes never trigger it.
*Check:* a customer comments on an Awaiting Info case; the case becomes Waiting on WSO2.

**CAS-231** A comment or attachment from an internal user, or from a system or integration
caller, shall never trigger this move.

**CAS-232** A comment relayed from the code-hosting service onto an Awaiting Info case (its
text starts with the author handle and "GitHub Comment") shall move the case to Waiting on
WSO2 as the system user, whoever is shown as the author.

**CAS-233** The move is best effort. If it fails, the comment or attachment still stands and
the failure is logged.

**CAS-234** Two near-simultaneous replies may both try the move. Only the one that actually
changed the state sends the notification and the old-system update.

## 7. Severity

**CAS-240** Severity can only be changed on a **case**. For the other four kinds the request
shall be refused. Valid values are catastrophic, critical, high, medium and low.

**CAS-241** A severity change shall: add a history entry; cancel the case's old SLA clocks and
start new ones for the new severity, re-pausing them if the case is currently paused;
and publish a "severity changed" event for the watchers and the account's support team.

**CAS-242** Crossing the Low boundary shall re-mark the case's time cards in the same step:
becoming Low makes every time card billable, and leaving Low makes them non-billable, unless
the case carries a "patch" tag, which keeps them non-billable.

## 8. Work state

**CAS-250** Work state can be **ongoing** or **paused**, for cases, service requests, security
reports and engagements. Announcements have none.

**CAS-251** An engineer may have only **one** ongoing ticket (counting all four kinds, and
only those in Work In Progress). Setting a second one shall be refused as a conflict naming
the other ticket's number. Two requests at the same time for the same engineer are handled one
after the other. An unassigned ticket has no engineer to clash with.

## 9. Assignee

**CAS-260** A case's assignee is set by the assignee's email. The person must exist; an
unknown email shall be refused. A signed-in caller is required.

**CAS-261** Sending an empty (null) assignee shall clear the assignment. A blank email is
refused.

**CAS-262** Assigning the person already assigned shall succeed and change nothing.

**CAS-263** A real change shall add a history entry ("assigned to", old name to new name,
or "Unassigned") and publish a "case assigned" event, emailed to the active watchers and the
account's default watchers. Clearing the assignee publishes nothing.

**CAS-264** The reply shall give the new assignee's id, name and email.

## 10. Acknowledging

**CAS-270** A signed-in caller may **acknowledge** a case by sending "acknowledge: true".
Only true is accepted.

**CAS-271** Acknowledging is "first one wins". The first person to acknowledge is recorded;
a later acknowledge changes nothing and the reply says it was already
acknowledged and by whom. Two acknowledgements at the same moment cannot both win.

**CAS-272** The first acknowledgement shall add a history entry and publish a "case
acknowledged" event, which goes to the team chat only, with the case number, severity,
product, team and acknowledger. No email is sent.

## 11. Parent case, related case and fix details

**CAS-280** A case may be given a **parent** case (a "major case"), by id. The parent must
exist. A signed-in caller is required. A history entry shows the old and new parent numbers.
The ticket being re-parented must be one of the five kinds; any other work item is "not
found".

**CAS-281** A case cannot be closed while any of its child cases is still open (see
[case-closure-and-reopen.md](case-closure-and-reopen.md) section 3).

**CAS-282** A **related case** is a looser link with no closing rule. It can be set only on
a **case**, and must point at a case, service request, security report analysis or
engagement. Any other target or a missing one shall be refused.

**CAS-283** The **fix estimates** (best case, most likely, worst case) shall be dates in
YYYY-MM-DD form. A change to any of them shall stamp the "estimate shared" time, and add a
history entry for each estimate that changed.

**CAS-284** "Workaround provided: true" shall record the time and the person who gave the
workaround, and publish a "workaround provided" event. "False" shall recall it and clear both.
It needs a signed-in person; an integration with no user cannot set it.

**CAS-285** A change to the workaround or any estimate shall also update the case's SLA
clocks.

**CAS-286** "Mark fix issued: true" shall stamp when a fix was issued, once (first write
wins); only true is accepted, and it cannot be undone. It may also be sent by a trusted
integration on its own behalf, naming the actor to show in history. That actor is only
honoured from a trusted integration and never alongside a signed-in person.

**CAS-287** Subject, description, deployment and deployed product may be changed with the
other plain fields; deployment and product must be valid ids.

**CAS-288** The **auto-closure hold** may be set to a date. The date is taken as the UTC
calendar day and stored at midnight UTC. Announcements cannot be held. See
[case-closure-and-reopen.md](case-closure-and-reopen.md) section 6.

## 12. Changing a case's type

A case can be moved between four types: case, engagement, service request and security
report analysis.

**CAS-290** Only an **internal** user may change a case's type. Customers shall be refused.

**CAS-291** Announcements, and the system-managed types, are neither a source nor a target.
Moving a case to the type it already is shall be refused.

**CAS-292** A type change shall be the **only** thing in its request, apart from the fields
its target needs:

| Target | Must also send | May not send |
|---|---|---|
| Case | severity and issue type | engagement or catalog fields |
| Engagement | engagement type and payment type | severity, issue type, catalog fields |
| Service request | catalog, catalog item and at least one answer | severity, issue type, engagement fields |
| Security report analysis | nothing | any of the above |

**CAS-293** The case shall keep its number, project reference, project, deployment,
watchers, tags, comments and attachments. It shall carry over its state, close notes, cause,
closed-by and closed-on, resolved-on, auto-closure position, work state and resolution code.

**CAS-294** A type change shall add a history entry ("Type: Case → Engagement"). Moving a
case into the Case type shall start fresh SLA clocks for its severity; moving it out shall
cancel its clocks. Time cards are re-marked if the Low boundary is crossed.

**CAS-295** A type change shall be refused as a conflict when (a) other tickets name this
one as their related case and the database still ties that link to cases only (the message
names up to a few of them), or (b) the case has attachments still tied to its old type.
Nothing changes in either case.

**CAS-296** While the old case system is still mirrored it is asked first, from inside the
same transaction, so either both systems move or neither does. If its answer is lost, the
service asks it what type it holds before deciding.

## 13. Who may do what

**CAS-300** To change a case the caller must be internal or a member of the case's project,
enforced by the database. A customer changing another project's case is told "not found".

**CAS-301** Beyond that, the service itself adds only these checks: changing type needs an
internal user (CAS-290); an admin reopen needs the CSM portal (see
[case-closure-and-reopen.md](case-closure-and-reopen.md)); the state tables differ for
customers and internal users (section 4); resolution fields are refused from customers when
the old system is mirrored (see [case-closure-and-reopen.md](case-closure-and-reopen.md)).
The portals decide the rest. (**Confirm**, see 14.1)

## 14. Open questions for the product owner

1. **Few role checks here.** The service does not itself stop a customer from changing
   severity, assignee, work state, acknowledgement, fix estimates, parent or related case.
   Database rules only limit them to their own project. Is the portal's own check
   enough?
2. **Announcements and the state table.** Announcements can only be Open or Closed, but the
   state table lets a CS engineer move Open only to Work In Progress, so an engineer cannot
   close one through this operation. Is that intended, or are announcements closed another
   way?
3. **State, severity and work state may be sent together.** Comments in the code assume only
   one at a time; the code allows all three. When the old system is mirrored, only one of
   them is sent on. Should the combination be refused?
4. **No cycle or self-parent check.** A case can be made its own parent, or form a loop of
   parents; only existence is checked. Should it be refused?
5. **Blank subject or description.** The update accepts an empty subject or description.
   Should it?
6. **Hold date in the past.** A hold date earlier than today is accepted and simply expires
   at the next auto-closure run.
7. **Race in the customer-reply move.** A case moved to another state (for example Closed)
   between the check and the move can still be set to Waiting on WSO2. This is a known gap
   in the code.

## 15. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Update rules and side effects | `internal/service/case_service.go` (update case) |
| State tables | `internal/service/case_state_transitions.go` |
| State, work state, severity writes | `internal/repository/case_repo.go` (update query, one-ongoing rule) |
| Customer reply and attachment move | `internal/service/case_service.go` |
| Assignee, acknowledge, parent, fix issued | `internal/service/case_service.go`, `internal/repository/case_repo.go` |
| Type change | `internal/service/case_type_transfer_service.go`, `internal/repository/case_type_transfer_repo.go` |
| Status, assigned, acknowledged, severity events | `internal/service/sn_case_service.go`, `internal/service/case_service.go` |
| Write access | migration `100008_case_adjacent_rls.sql` |
| Route | `PATCH /cases/{id}` |
| Tests | `internal/service/case_service_test.go`, `internal/service/case_state_transitions_test.go` |
