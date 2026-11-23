# Case escalation and de-escalation — requirements

**Status:** pilot. Describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check
are intended; they are collected in section 13.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

---

## 1. Purpose

CS engineers can raise a case to a higher management level when it needs more
attention (an escalation), and bring it back down once the pressure is off (a
de-escalation). Each step is recorded, the right internal people are told, and the case
shows its current level.

This document covers what the entity service does for escalation: the rules, who can do
what, who is told, and what is recorded. The screens in the portals are out of scope
except where they decide who is allowed to act (section 6).

## 2. Who is involved

| Who | What they do |
|---|---|
| CS engineer | Escalate a case, or de-escalate it if allowed (section 6). |
| Customer user | Can escalate a case in a project they belong to (section 7). |
| Team lead of the case's account team | May de-escalate cases for their account (section 6). |
| CSM admin | Names extra people who may de-escalate any case (section 8). |
| Management recipients | Receive an email when a case is escalated (section 4). |

## 3. Escalation levels

A case is always at one of six levels, **EL0 to EL5**. EL0 means "not escalated". A case
that has never been escalated is treated as EL0. A case counts as *escalated* whenever it
is at EL1 or above.

**ESC-001** The service shall keep the current escalation level and an "is escalated" flag
on every case, so any screen can show them without reading the history.
*Check:* escalate a new case once; the case now shows level 1 and is flagged escalated.

## 4. Escalating a case

**ESC-010** The service shall let a caller escalate a case by naming the case and giving
a reason.
*Check:* escalating a case at EL1 with a reason moves it to EL2.

**ESC-011** Each escalation shall raise the case by exactly one level.
*Check:* EL0 → EL1 → EL2, never skipping a level.

**ESC-012** A reason is required when escalating. A missing or blank reason shall be
refused with a clear message, and nothing shall be recorded.
*Check:* escalate with an empty or whitespace-only reason; the request is refused and the
case level is unchanged.

**ESC-013** A case already at EL5 cannot be escalated further. The request shall be
refused as a conflict and nothing shall be recorded.
*Check:* escalate an EL5 case; refused, level stays EL5.

**ESC-014** If no action is given the service shall treat the request as an escalation.
The action may be written in any letter case (for example `escalate` or `ESCALATE`).
*Check:* a request with no action and a reason escalates the case.

**ESC-015** Any action other than escalate or de-escalate shall be refused.
*Check:* send an unknown action; refused with a message naming the two allowed values.

## 5. De-escalating a case

**ESC-020** The service shall let a caller de-escalate a case.
*Check:* de-escalating an EL3 case succeeds.

**ESC-021** A de-escalation shall always return the case straight to **EL0**, whatever
level it was at. It does not step down one level at a time. (**Confirm**, see 13.1)
*Check:* de-escalate an EL4 case; it is now EL0 and no longer flagged escalated.

**ESC-022** A de-escalation does not need a reason. If one is given it shall be recorded.
*Check:* de-escalate with no reason; it succeeds.

**ESC-023** A case already at EL0 cannot be de-escalated. The request shall be refused as
a conflict.
*Check:* de-escalate an EL0 case; refused.

## 6. Who may de-escalate

This service records a de-escalation for any caller who is allowed to see the case. It
does **not** decide whether that person ought to be allowed to de-escalate. The CSM
portal makes that decision and only then calls this service. The rule below is what the
portal enforces.

**ESC-030** A person may de-escalate a case if they are on the list of people an admin has
named (section 8). They need no escalation role.

**ESC-031** A person may also de-escalate a case if they lead the account's team *and*
hold the escalation permission in the CSM portal.

**ESC-032** Escalating always needs the escalation permission in the CSM portal.

**ESC-033** For each case the service shall tell the portal who may de-escalate it, in the
same response that carries the case's history: the account team's leads, and the named
people. Named people who are no longer active internal staff are left out.
*Check:* the history of a case returns both lists; a deactivated named person is absent.

**ESC-034** If the service cannot work out who may de-escalate, the portal shall refuse
rather than guess.

## 7. Customers

**ESC-040** A customer may escalate a case only in a project they belong to. For a case
outside their projects the answer shall be "not found", the same as a case that does not
exist, so they cannot learn that it is there.
*Check:* a customer escalating another project's case gets "not found".

**ESC-041** A customer-triggered escalation shall never email the customer. Only internal
staff addresses are used as recipients (section 9).
*Check:* a customer escalates; no customer address appears in the notified list or email.

## 8. Naming extra de-escalators (admin)

A CSM admin can keep a list of people who may de-escalate any case, in addition to the
account team leads.

**ESC-050** An admin shall be able to view the list. Each entry shows the person's name,
email, who granted it and when, and whether they are still active internal staff.

**ESC-051** An admin shall be able to add a person. Only active internal staff can be
added; anyone else is refused. Adding someone already on the list changes nothing.

**ESC-052** An admin shall be able to remove a person. Removing someone who is not on the
list changes nothing and is not an error.

**ESC-053** Adds and removals shall be recorded with the admin who made them.

**ESC-054** A person who stops being active internal staff stays visible in the admin's
list (marked inactive) so the admin can remove them, but they no longer count as a
de-escalator.

**ESC-055** These three actions work **only when called through the CSM portal**, which
checks the person is a CSM admin before calling. The service does not check admin roles a
second time. Any other caller is refused, even one holding an admin role, and so is a
person who has been deactivated.
*Check:* call the list directly with an admin token that did not come through the portal;
refused. Call it through the portal as an admin; allowed.

The list is stored as a role named `case_deescalator`. It is created the first time
someone is added.

## 9. Who is told about an escalation

Each escalation produces a **notification list**, stored with the escalation record. The
more senior the level, the more people are on it, and the list is **cumulative**: an EL3
escalation also reaches everyone an EL1 and EL2 escalation would.

| Level reached | Added to the list |
|---|---|
| EL1 | The account team's leads; a configured set of regional team leads; the account owner; the technical owner; the renewal account manager; the secondary technical owner. |
| EL2 | A configured set of regional technical contacts, plus one product contact chosen from the case's product (below). |
| EL3 | The CRE head, plus the account's customer success manager. |
| EL4 | Everyone holding the `case_escalation_el4` role. |
| EL5 | Everyone holding the `case_escalation_el5` role. |

**ESC-060** The product contact at EL2 is chosen as follows: a services product uses the
services contact; the Identity Server product uses the Identity Server contact; any other
product, and a case with no product, uses the default contact.

**ESC-061** Each person shall appear on the list once, even if they qualify several ways.

**ESC-062** A slot that is not configured, an address that matches no user, or a role
nobody holds shall simply add nobody. It shall never make the escalation fail.

**ESC-063** For a de-escalation the list is built for the level the case **left**, so it
shows the people the escalation had reached.

**ESC-064** Who is reached at EL3 to EL5 is controlled by data, not by deployment: by
who holds the head position and the two roles. Changing it needs a grant, not a release.

## 10. The escalation email

**ESC-070** When a case is escalated, the service shall publish an event so that the
notification service emails the escalation's recipients. One email goes to all of them.

**ESC-071** Only internal staff addresses (those ending in the company's own domain)
shall receive it. If that leaves nobody, no email is sent.
*Check:* escalate a case whose list holds only customer addresses; nothing is published.

**ESC-072** A de-escalation shall not send an email.

**ESC-073** The email shall have the subject "Case Escalation To EL*n*" and show:
case number, who escalated it, customer account, the path (for example EL1 → EL2), the
time, the reason, the case title, product, priority, environment, and a "View Case" link.
The time is shown in the Asia/Colombo zone.

**ESC-074** A failure to publish or send shall be logged and shall **never** fail or undo
the escalation.

**ESC-075** Email can be switched off with a setting; it is on by default.

**ESC-076** If email is in debug mode, mail goes to the debug recipients instead, with a
"Sent to" line showing who it was meant for. If sending is disabled, nothing is sent.

## 11. What is recorded

**ESC-080** Every escalate and de-escalate shall be stored as a new record and never
edited afterwards: the case, the new and previous level, the reason, who did it, and when.

**ESC-081** The record, its notification list, the case's new level and flag, and the
case's "last updated by / on" shall be saved together. If any part fails, none is saved.

**ESC-082** Two people acting on the same case at once shall be handled one after the
other, so levels never skip or repeat.

**ESC-083** After a successful action taken through the case's own escalation route, the
service shall add an **internal work note** to the case, for example "Case escalated from
EL1 to EL2. Reason: …". The note is internal only. If adding it fails, the escalation
still stands.
*Check:* escalate a case; its notes show the entry, and a customer cannot see it.

**ESC-084** The result of an action shall include the new record, the levels, and the
notification list.

## 12. Viewing history

**ESC-090** The service shall return a case's full escalation history, newest first, with
the people notified at the current level, the account team leads and the named
de-escalators.
*Check:* a case escalated three times returns three records, newest first.

**ESC-091** The service shall also allow searching escalations across cases, filtered by
case and by current level (0 to 5), sorted by created or updated time. A level outside 0
to 5 shall be refused.

**ESC-092** A person shall only see escalations of cases they are allowed to see:
internal staff see all; customers see their projects' only.

## 13. Open questions for the product owner

1. **De-escalation goes straight to EL0.** The wording in a code comment suggests a step
   down (for example "EL2 to EL1"). The behaviour is a full reset. Which is intended?
2. **Who may de-escalate is enforced in the portal, not here.** Any caller who can see a
   case could record a de-escalation by calling this service directly. Is that acceptable?
3. **Email goes only to addresses in the company domain.** Internal staff whose address
   uses another domain would not be told. Should this follow user type instead?
4. **Customers can read the notified people.** The history returns names and emails of
   internal staff to any caller who can see the case. Is this meant to reach customers?
5. **Regional and product contacts** come from configuration with no default. Who owns
   keeping those lists current?
6. **No acknowledgement.** The email links to the case but nothing records that anyone
   read or acted on it.
7. **Duplicate emails during migration.** While the old system is still mirrored, both
   systems can email the same escalation unless the old one is switched off.

## 14. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Level rules, recipients, record | `internal/repository/escalation_repo.go` |
| Validation, scoping, email event | `internal/service/escalation_service.go` |
| Case history, work note | `internal/service/case_escalation_service.go` |
| De-escalator list | `internal/service/case_escalation_access_service.go`, `internal/repository/case_escalation_access_repo.go` |
| Email event / email | `internal/events/case_escalation.go`; notification service `dispatch/case_escalation.go` |
| Routes | `GET/POST /cases/{id}/escalations`, `POST /escalations`, `POST /escalations/search`, `GET /case-escalation-access/deescalators`, `PUT/DELETE …/{userId}` |
| Row-level security | migrations `100002`, `100011` |
| Recipient roles | migrations `0170`, `0209` |
| Settings | `ESCALATION_EL1_*`, `ESCALATION_EL2_*`, `CASE_ESCALATION_NOTICES_ENABLED` |
| Portal rules | CSM portal backend `internal/handler/cases.go` |
| Tests | `escalation_repo_test.go`, `escalation_service_test.go`, `case_escalation_*_test.go` |
