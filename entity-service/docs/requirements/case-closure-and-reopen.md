# Cases: closing, reopening, auto-closure and feedback — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 9.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

Related documents: [case-creation-and-detail.md](case-creation-and-detail.md),
[case-updates-and-states.md](case-updates-and-states.md) (the state tables and every other
change), [case-escalation.md](case-escalation.md), and [sla.md](../specs/sla.md).

"Case" here means any of the five ticket kinds listed in
[case-creation-and-detail.md](case-creation-and-detail.md) section 3, unless a rule says
it is for cases only.

---

## 1. Purpose

A ticket ends when it is closed. It can come back through a reopen, and a ticket that the
customer has stopped answering can be closed for inactivity by an automated job. When a case
is closed the customer may rate the support they got. This document covers all of that.

## 2. Who is involved

| Who | What they do |
|---|---|
| CS engineer | Proposes a solution, closes a case with its resolution details. |
| Customer user | Closes their own case, accepts or rejects a proposed solution, leaves feedback. |
| CSM admin | Reopens a closed case, through the CSM portal only. |
| Scheduled job | Sends reminders and closes tickets that sit waiting on the customer. |

## 3. Closing a case

Which moves reach **Closed** is decided by the state tables in
[case-updates-and-states.md](case-updates-and-states.md) section 4. In short a CS engineer
closes from Work In Progress or Solution Proposed, and a customer closes from almost any
open state.

**CAS-400** Closing shall stamp the closing time and the person who closed it. The
case's SLA clocks are finished by the close (see [sla.md](../specs/sla.md)).

**CAS-401** A case shall **not** be closed while any of its child cases is still open. The
request shall be refused as a conflict, naming the open child's number, and nothing is saved.
A child counts as open unless it is Closed.
*Check:* a parent with one open child; closing the parent is refused.

**CAS-402** A close or a proposed solution may carry three **resolution details**: a
resolution code, a cause, and close notes. Cause must be one of the allowed values (the list
covers solution and deployment architecture, user error, customisation, documentation,
product, infrastructure, unknown and user mistake). An unknown resolution code or cause shall
be refused.

**CAS-403** Resolution details are only accepted together with a state of closed or solution
proposed (see CAS-203 in [case-updates-and-states.md](case-updates-and-states.md)).

**CAS-404** The resolution code is available for cases, service requests, security reports
and engagements, but **not** announcements. Announcements may be closed only as Open or
Closed and have no resolution code.

**CAS-405** While the old case system is still mirrored, closing or proposing a solution for a
**case** as an internal user shall **require** all three details, with non-blank close notes.
Without them the request is refused and nothing is saved. For the other ticket kinds they
stay optional.

**CAS-406** In the same mirrored mode a **customer** supplying any of the three details
shall be refused ("not allowed for external callers"), because the classification is WSO2's
own. A customer closing with none of them is accepted, and nothing is invented for them.

**CAS-407** When the old case system is not mirrored, the details are never required, and the
refusal in CAS-406 does not apply. (**Confirm**, see 9.1)

**CAS-408** A close attempt that is not allowed (wrong state for the caller, or an open child)
shall save nothing, including the resolution details.

## 4. What a closed case shows

**CAS-410** The detail of a closed case shall show the closing time, who closed it, when it
was resolved, the resolution code, the cause and the close notes.

**CAS-411** **Close notes** is the one field that holds the closing text. The separate
**resolution notes** field is always empty; it has no storage of its own. (**Confirm**, see
9.2)

**CAS-412** Who closed the case shall be the signed-in person. For an automatic close it is
the system user.

## 5. Reopening

**CAS-420** A customer may **reject** a proposed solution. The state table lets them move
Solution Proposed to **Reopened**, or to Waiting on WSO2. A CS engineer may then move a
Reopened case to Work In Progress.

**CAS-421** Entering Reopened shall clear the resolved-on time and restart the auto-closure
position of the ticket (see CAS-443).

**CAS-422** Once **Closed**, a case can only come back through an **admin reopen**. Nobody
else, and no other state change, can move it out of Closed.

**CAS-423** An admin reopen moves a Closed case to **any other state**, ignoring the state
tables.
*Check:* an admin reopens a Closed case to Work In Progress; it is now in Work In Progress.

**CAS-424** An admin reopen is honoured only when **all** of these hold: the caller is
internal; the call comes through the **CSM portal** (the service checks the caller's client
id, because the portal is where the "is this person an admin" decision is made); the case is
**currently Closed**; and the requested state is not Closed. The request shall also include a
state, and apply to a **case** (not a service request, engagement, security report or
announcement), otherwise it is refused.
*Check:* the same request sent from another client is not honoured, and is refused as an
illegal move out of Closed.

**CAS-425** A reopen may carry a **reason**. A reason is only accepted together with an admin
reopen. The reason is stored on the case.

**CAS-426** An admin reopen shall clear the closing time, the closer and the resolved-on
time, like any move out of Closed, and shall add a state change to the case's history.

**CAS-427** An admin reopen shall **not** send the "status changed" email to watchers, and
shall not touch the SLA clocks: a closed case has no running clocks to resume.

**CAS-428** The reopen reason is not shown anywhere: no screen or reply returns it.
(**Confirm**, see 9.3)

## 6. Auto-closure

Tickets that wait on the customer can be reminded twice and then closed for inactivity.
A scheduled job drives it by asking the service which tickets are due and then advancing them
one at a time. The service decides what is due; the job only decides when to ask.

### 6.1 Which tickets

**CAS-430** Only a **case** or a **security report analysis** in **Awaiting Info** or
**Solution Proposed** is eligible. Service requests, engagements and announcements never are.

**CAS-431** Only a **parent-level** ticket is eligible; a child case has no workflow of its
own.

**CAS-432** A ticket with an open child (a child case that is not Closed, or a child change
request that is not closed, cancelled or rolled back) is not due for a reminder or a close.

**CAS-433** Each ticket has an on/off flag for auto-closure, which is on by default. A ticket
with it off is never due. The service offers no way to change this flag. (**Confirm**, see 9.4)

### 6.2 The stages

A ticket moves through stages, each recorded as its position:

| Stage | What it does | Position after |
|---|---|---|
| Release hold | Ends a customer hold once its day is over. Notifies nobody. | Start |
| First reminder | Posts the first reminder comment. | First comment |
| Second reminder | Posts the second reminder comment. | Second comment |
| Close | Posts the closure comment and closes the ticket. | Second comment (closed) |

**CAS-440** A ticket is due for the **first reminder** when it has been waiting longer than
the interval, measured from the latest of: when it entered the state (for Solution Proposed,
when the solution was proposed); the last comment by a WSO2 user (Awaiting Info only, never
the system user's own reminders or work notes); and the last reset of its workflow. A ticket
with none of these uses its last update.

**CAS-441** It is due for the **second reminder** one interval after the first reminder, and
for **closing** one interval after the second reminder.

**CAS-442** The interval is a number of days, 1 to 365. The default is 7.

**CAS-443** A ticket's workflow shall **restart** from the beginning (with the clock set to
now) whenever it enters Awaiting Info, Solution Proposed, Waiting on WSO2 or Reopened from a
different state. Entering a waiting state is a fresh request to the customer and does not
inherit an earlier one's clock. This applies to every writer, including the portals and the
data sync, because it is a database rule.

**CAS-444** The reminder comments are **customer-visible comments**, written by the system
user, with a default text for each state and stage ("Dear Customer, … Kind regards, Customer
Success Team"). The caller of the job may supply its own text for any of them.

### 6.3 Holds

**CAS-450** A **hold** is set by giving the ticket a date (see CAS-288 in
[case-updates-and-states.md](case-updates-and-states.md)). The hold lasts to the end of that
UTC calendar day. A held ticket gets no reminders or close.

**CAS-451** When the hold day has passed, the **release hold** stage shall return the ticket
to the start with a fresh clock.

**CAS-452** A hold survives the ticket entering Awaiting Info or Solution Proposed, because
the customer asked for the ticket not to be closed. Moving to Waiting on WSO2 or Reopened ends
it.

**CAS-453** Announcements cannot be held.

### 6.4 Closing automatically

**CAS-460** The **close** stage shall move the ticket to Closed as the system user. This is
the one move from Awaiting Info to Closed that no CS engineer table allows.

**CAS-461** The closing details shall be: the existing close notes if there are any, otherwise
the closure comment as plain text; the existing cause, otherwise "unknown"; and a resolution
code of "abruptly closed due to non-responsiveness" for an Awaiting Info ticket (or any ticket
with no code), keeping the existing code for a Solution Proposed ticket that has one.

**CAS-462** The ticket is closed first and the **closure comment** is added afterwards, as a
customer-visible comment from the system user. If the comment fails, the ticket stays closed
and the failure is logged.

**CAS-463** A closed **case** shall be added to the customer's **Pending Verification** list,
with the reason "auto closed" and a note saying which kind of reminder went unanswered, when
the project has that feature switched on. Security reports are not queued. A ticket already
on the list is left as it is.

### 6.5 Safety limits

**CAS-470** Only internal callers may ask which tickets are due or advance them. Customers
shall be refused.

**CAS-471** Advancing a ticket **checks again** that it is still due. If it has since been
answered, held, moved, closed, or given an open child, the stage is skipped and nothing
changes. Doing the same stage twice is safe: the second finds the ticket already moved on.

**CAS-472** The step is claimed before the action, and given back if the action fails, so a
failed reminder is retried and a ticket is never left marked as reminded without a reminder.

**CAS-473** Reminders and closures share a **daily limit** (counted per UTC day, default 500
in the job), so a retried run cannot exceed it. Releasing holds notifies nobody and is not
counted. The limit is checked both when listing and right before each action.

**CAS-474** A ticket whose stage has **failed three times in seven days** is left out of the
list until the oldest failure ages out, so a ticket that can never succeed does not raise the
same alert every run. The list reports how many were left out.

**CAS-475** A **dry run** shall decide and record what would happen without changing
anything.

**CAS-476** Every automated action shall be recorded in an audit table: the ticket, stage,
position before and after, outcome (done, failed, failed temporarily, or dry run) and detail.
A temporary failure (a timeout or outage) does not count towards the three-failure limit.

**CAS-477** The workflow acts through the same case operations as a person, so its comments,
state change, history entries, SLA effects and "status changed" email all behave as for a
manual close.

**CAS-478** The job is **off** unless switched on, and can be limited to certain states. Where
the service cannot run it (it needs the case tables in the database) every request is answered
"service unavailable" with the reason.

## 7. Feedback

After a case is closed the customer may rate the support they received.

**CAS-480** Feedback can be submitted only for a case that is **Closed**. Before that the
request shall be refused as a conflict.

**CAS-481** The rating shall be one of the active ratings the catalogue offers (a face with
a set of reasons). A rating that is not a known catalogue rating shall be refused. Each
chosen reason must belong to that same rating; one from another rating is refused.

**CAS-482** An optional additional comment may be included.

**CAS-483** There shall be **one** feedback per case. A second submission is refused as a
conflict.

**CAS-484** A submitter must be signed in, and must be able to see the case; otherwise the
answer is "not found".

**CAS-485** Feedback applies to the five ticket kinds. It records the rating and its label,
the comment, the chosen reasons, who submitted it and when.

**CAS-486** Reading a case's feedback is for **internal users only**. If none was submitted
the answer is "not found" with a message saying so. The reply shows the rating, reasons,
comment, who submitted it and when.

Reports on feedback across cases are not covered here.

## 8. Auto-closure position in the case detail

**CAS-490** The case detail shall include the ticket's current auto-closure step and the time
it was entered, taken from the same stored position the workflow uses. The closing fields
themselves are described in section 4.

## 9. Open questions for the product owner

1. **Resolution details depend on mirroring.** They are required for internal closes, and
   refused from customers, only while the old case system is mirrored. Without mirroring any
   caller may send them and nobody must. Is that the intended end state?
2. **Resolution notes.** The field exists in the reply but is always empty; only close notes
   is stored. Is a separate resolution notes field needed?
3. **Reopen reason is write-only.** It is saved but never returned or shown. Should the portal
   show it?
4. **No way to switch auto-closure off for one ticket.** The flag exists and defaults to on,
   but the service never changes it. How is an exemption meant to be made?
5. **Admin reopen is limited to cases.** An admin cannot reopen a closed service request,
   engagement or security report through this route. Intended?
6. **Customers can close with no resolution details.** A customer-closed case then has no
   code, cause or notes. Reports that group by resolution code will see it as blank.
7. **Closing a parent.** The open-child rule applies to every close, including the automatic
   one, which skips such tickets silently rather than reporting them.

## 10. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Close rules, admin reopen, resolution checks | `internal/service/case_service.go` (update case) |
| Stamps, reopen reason, open-child rule | `internal/repository/case_repo.go` (update query) |
| Auto-closure workflow and closing details | `internal/service/autoclosure_service.go`, `internal/service/autoclosure_context.go` |
| Due list, claim and audit | `internal/repository/autoclosure_repo.go` |
| Hold, per-ticket flag, restart rule, audit table | migration `0249_case_autoclosure.sql`; hold in `internal/repository/case_repo.go` |
| Pending Verification queue | `internal/service/autoclosure_service.go` |
| Feedback | `internal/service/case_service.go`, `internal/repository/case_feedback_repo.go` |
| Reopen reason column | migration `0292_case_reopen_reason.sql` |
| Routes | `PATCH /cases/{id}`, `POST /autoclosure/candidates`, `POST /autoclosure/cases/{id}/advance`, `GET` and `POST /cases/{id}/feedback` |
| The job and its settings | `operations/csm-scheduled-tasks` (`AUTOCLOSURE_*`) |
| Tests | `internal/service/case_service_test.go`, `internal/service/autoclosure_service_test.go` |
