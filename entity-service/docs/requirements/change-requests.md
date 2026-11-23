# Change requests — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check
are intended; they are collected in section 14.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related documents:** the test inventory for this area is
[change-request-tests.md](../specs/change-request-tests.md). The GitHub side is described in
[github-cr-sync-spec.md](../specs/github-cr-sync-spec.md) and
[github-integration-deployment.md](../specs/github-integration-deployment.md); section 12
below says what the service actually does with GitHub today.

---

## 1. Purpose

A change request is a planned change to a customer's environment or to WSO2's own
services. It goes through an approval path before it is carried out, can be shown to
the customer (who may be asked to approve and to review it), and is closed once it has
been implemented and reviewed.

This document covers the rules of that path: how a change request is created, how it
moves between states, who approves it, what the customer can and cannot do, who can see
it, and who is emailed.

## 2. Who is involved

| Who | What they do |
|---|---|
| CS engineer | Creates change requests, edits them while they are open, requests approval, moves them through implementation, cancels or closes them. |
| Approver (internal) | Peer, CAB and review approvers: active internal staff who approve or reject an internal approval stage. |
| Customer contact | A registered contact of the change request's project. May be asked to approve and review it, and may propose a different implementation time. |
| WSO2 (as a whole) | Answers a customer's proposed time (accept, or propose another). |

## 3. Types, states and the three flows

### 3.1 Types

A change request has a type. Only three can be chosen when creating one: **Normal**,
**Standard** and **Emergency**. Other types exist on records that came from the legacy
case system (for example model, infra or cloud types) and are shown as they are. A
change request with a missing or unrecognised type follows the Normal flow.

### 3.2 States

New → Assess → Authorize → Customer Approval → Scheduled → Implement → Review →
Customer Review → Closed. The end states are **Closed**, **Canceled** and **Rollback**.

### 3.3 The three flows

| Type | What "Request Approval" does | Approval stages |
|---|---|---|
| Normal | Moves a New change to **Assess**. | Peer approval (in Assess), then CAB approval (in Authorize). CAB approval moves it on automatically. |
| Emergency | Moves it straight to **Authorize**. | One stage only: CAB approval. No peer stage. |
| Standard | Moves it straight to **Scheduled**. | None. It is pre-approved. |

When the customer's approval is required (section 7) the move to Scheduled goes through
**Customer Approval** first. When the customer's review is required, Review goes
through **Customer Review** before Closed.

## 4. Creating a change request

**CHG-010** The service shall let only internal callers create a change request. A
customer or partner caller is refused.
*Check:* create as a customer contact; refused.

**CHG-011** A new change request shall start in **New** and receive a number from the
portal's own change-request series (it starts with "CHG"). The creator is recorded as
the person who raised it.
*Check:* create one; it reads back in state New with a CHG number.

**CHG-012** A type shall be required and shall be one of Normal, Standard or Emergency.
Any other value, or none, is refused with a message naming the three.
*Check:* create without a type; refused.

**CHG-013** An Emergency change shall not be created with either customer box ticked
(customer approval required, customer review required). It is refused and nothing is
saved (see section 8).

**CHG-014** A planned start and end, when given, shall be valid date-times in one of two
layouts: RFC 3339 with a time zone, or "YYYY-MM-DD HH:MM:SS" with no zone, which is read
as UTC. Anything else (for example a date alone, a word such as "tomorrow", or
"infinity") is refused naming the field. Years before 2000 or after 2100 are refused.
*Check:* create with plannedStartDate "tomorrow"; refused.

**CHG-015** The Customer Project, when given, shall exist. Deployments, when given,
shall exist, be active and belong to that project, so deployments need a project. At most
100 deployments or deployment products may be named. The change's deployment products are
not chosen freely: they are always the active deployed products of the chosen
deployments, and a caller that names products must name exactly that set.
*Check:* name a deployment of another project; refused.

**CHG-016** The assignment group chosen on the form shall be saved with the change. An
unknown group is refused. (It was once silently dropped, which left the new change
unable to request approval.)

**CHG-017** The older "customer group" and "environment" fields shall no longer be
accepted on create or edit and are refused. The customer group is derived from the
project's registered contacts (section 7), and a deployment already carries its
environment.

**CHG-018** An optional customer-visible comment and an optional internal work note may
be given on create. Each becomes its own journal entry on the new change; a blank value is
ignored.

**CHG-019** Images embedded in the description shall be checked before the change is
created (a refusal saves nothing) and linked to it once it exists. Embedded images are
held to the same size limits as other user content.

## 5. Moving through the lifecycle

### 5.1 Request Approval

**CHG-020** "Request Approval" shall be the only way out of New (other than Cancel). The
caller sends the state "assess"; the state the change actually moves to is chosen by its
type (section 3.3), never by the caller.

**CHG-021** Request Approval shall be refused unless an assignment group is set. The
group supplies the peer approvers and, later, the review approvers.
*Check:* request approval on a change with no assignment group; refused.

**CHG-022** Request Approval shall be refused if a customer box is ticked and the change
has no Customer Project, or the project has nobody who can be asked (section 7).

**CHG-023** Request Approval shall be refused unless the change is in New. Asking again
for a change that has already moved is refused (re-sending the state it is already in is
not an error).

**CHG-024** A change request that has left New shall never return to it. The message says
to cancel it and clone it instead.

### 5.2 Staff moves

**CHG-025** For a CS engineer the service shall allow only these moves (anything else is a
refusal that names the moves still open):

| From | Staff may request |
|---|---|
| New | Assess (Request Approval), Canceled |
| Assess | Canceled (it waits for the peer approval) |
| Authorize | Canceled (it waits for the CAB approval) |
| Customer Approval | Re-schedule (section 9), Canceled |
| Scheduled | Implement, Canceled |
| Implement | Review, Canceled |
| Review | Closed, or Customer Review (decided by the customer-review box), Rollback, Canceled |
| Customer Review | Rollback (only while no customer review is waiting to be answered), Canceled |
| Closed, Canceled, Rollback | nothing |

**CHG-026** Assess→Authorize, Authorize→Scheduled (or Customer Approval) and Customer
Approval→Scheduled shall happen only as the result of an approval (section 6), never by a
staff request. A staff request for Scheduled is always refused.

**CHG-027** A change in Closed, Canceled or Rollback is final. Nothing moves it, and
nothing revives it.

**CHG-028** Rollback shall be requested only from Review or Customer Review, and from
Customer Review only while the customer's review request is not live: a failed review is
then given by a contact rejecting it, not by staff. Review moves to Closed when the
customer's review is not required, and to Customer Review when it is. A request for the
wrong one is refused with a message saying which applies.

**CHG-029** A change that is **on hold** shall not change state. The caller must take it
off hold (onHold false) first, or do both in one request. Editing other fields while on
hold is allowed. The hold reason is shown but has no effect of its own; putting a change on
hold without a reason clears any old reason, and taking it off hold always clears it.
*Check:* move a change on hold to Implement; refused until onHold is false.

## 6. Approvals

**CHG-030** Request Approval shall create the approval stage for the first checkpoint and
ask its approvers; each later checkpoint creates its stage when the change enters it:

| Stage | Entered when | Approvers |
|---|---|---|
| Peer Approval | Normal change enters Assess | Active internal members of the assigned group, other than the creator. If that group has none, the "Devops Approval" group is used instead. |
| CAB Approval | Peer approval resolves (Normal); Request Approval (Emergency) | Active internal members of the "CAB Approval" group, other than the creator. |
| Review | Change enters Review | Active internal members of the assigned group, other than the requester. |

If a pool has nobody eligible, the move that needs it is refused with a message that
says why (counts only, no names), and nothing is saved. Approving a peer stage fails the
whole approval if the CAB stage cannot be created, so a change is never stranded in
Authorize with nobody to approve it.

**CHG-031** Only **active internal users** may approve or reject an internal stage
(Peer, CAB, Review), even if an approver row exists for someone else.

**CHG-032** The creator of a change request (the person who raised it, or its named
requester) shall never be able to approve it, at any stage. They can still cancel it.

**CHG-033** A decision shall be "approved" or "rejected". The first person to decide a
stage resolves it, and every other still-waiting approver on that stage is cancelled
("first responder wins").

**CHG-034** Approving the Peer stage while the change is in Assess shall move it to
Authorize and create the CAB stage. Approving the CAB stage while in Authorize shall move
it to Scheduled, or to Customer Approval when the customer's approval is required and the
change is not Emergency.

**CHG-035** Rejecting an **internal** stage shall record the rejection and cancel the
other approvers but shall not change the state in either direction. Approving the Review
stage likewise records the decision and changes no state: staff then move the change on by
hand (section 5.2).

**CHG-036** A stage can be decided only while the change is in the state it belongs to.
A decision on a stage the change has already left is refused as a conflict. Every state
change cancels approvers still waiting on a stage of a state the change has left.

**CHG-037** A caller who is not an approver of the change, or has no waiting approval on
it, shall get "no pending approval found".

**CHG-038** The approvals of a change shall be readable (who was asked, who decided,
and for each viewer whether they may decide now). A customer sees the stage, that it
belongs to the change and where it stands for the internal stages (Peer, CAB, Review) but
not the approver names, group or user ids; the customer stages stay whole.

## 7. The customer's approval and review

Two boxes on the form decide whether the customer is involved: **customer approval
required** (an extra step before Scheduled) and **customer review required** (an extra
step before Closed).

**CHG-040** Whom the customer step asks shall be derived live from the Customer
Project's registered portal-user contacts. A contact who is deactivated, and the change's
creator, are never asked. There is no stored "customer group".

**CHG-041** Entering Customer Approval or Customer Review shall create a stage with one
waiting approver per eligible contact. The contacts answer in the Approvals tab or through
the customer portal's approve/reject buttons; both do the same thing. The first answer
resolves the stage.

**CHG-042** The customer's answer shall decide the outcome: Customer Approval approved →
Scheduled; rejected → Canceled. Customer Review approved → Closed; rejected → Rollback.
The matching "customer approved / reviewed" flag is stamped by the approving outcome.

**CHG-043** **No one at WSO2 shall be able to record the customer's approval or review on
their behalf.** A staff request carrying either answer is refused, and a staff request to
move a change out of a customer state (other than Cancel, Re-schedule or Rollback) is
refused. A change in a customer state moves on only through the customer's own answer.
*Check:* a CS engineer sends isCustomerApproved true; refused.

**CHG-044** A customer's answer shall count only from a registered contact of the
change's own project who has been asked (holds a waiting approval on the live stage).
Anyone else is refused.

**CHG-045** The customer may give their answer together with the planned time they were
shown. If the planned time has changed since, the answer is refused as a conflict, so a
customer never approves a time they did not see.

**CHG-046** The two customer boxes cannot be removed once approval has been requested;
see section 10.

**CHG-047** When a box is ticked, Request Approval shall be refused if the project has
nobody who can be asked (a registered contact other than the requester), and so shall
turning a box on after Request Approval. A change already beyond New is never re-judged:
a contact deactivated afterwards can leave a change waiting until a contact is registered.

**CHG-048** Whether the answer would be accepted right now shall be shown on the change
for each viewer, so the portals offer Approve, Reject and Propose exactly when they work.

## 8. Emergency changes

**CHG-050** An Emergency change is acted on without the customer's consent, so it shall
never enter Customer Approval or Customer Review. Neither box can be ticked on it: a
create, an edit that turns a box on, and an edit that changes a change with a box ticked
into Emergency are all refused.

**CHG-051** For an Emergency change that already carries a ticked box (created before the
rule, or came from the legacy case system), the stored value shall be left alone and shown
as it is, but the flow shall ignore it: CAB approval schedules it and Review closes it.

**CHG-052** An Emergency change that is already waiting in a customer state shall not be
taken out of the customer's hands: its contacts are asked, can answer, and are asked again
by a Re-schedule like any other change.

**CHG-053** An Emergency change has a single CAB approval stage, using the same "CAB
Approval" group as a Normal change's second stage. Any older stage labelled "ECAB
Approval" is still shown under that name and decided as the CAB stage.

## 9. The customer's proposed time

While a change waits in **Customer Approval**, a customer who has been asked may propose
a different start for the implementation, and WSO2 answers.

**CHG-060** A customer's proposal shall write only the proposed **start**; the change
stays in Customer Approval and the planned window stays what WSO2 planned. A proposal
keeps the planned **length**: an end, if sent, must equal the proposed start plus that
length. The proposed time must be in the future and different from the planned start.
*Check:* a customer proposes a past time; refused.

**CHG-061** A proposal shall be refused when: the change is on hold; another, non-customer
approval is waiting; the change has no planned window; the same time is already proposed
and waiting; or WSO2 already asked for that same time.

**CHG-062** A proposal and the customer's approval/review shall be sent in separate
requests.

**CHG-063** WSO2 may **accept** the proposed time. This shall apply the proposed start to
the planned window (keeping its length), record the agreement and move the change to
**Scheduled** in one step. Nothing goes through CAB again and the customer is not asked
again: the proposal is their own consent.

**CHG-064** Accepting shall be refused when: it is not in Customer Approval; nothing is
waiting; it is on hold; the proposed time has passed or lies beyond the year 2100; the
planned window has no length; or the proposed time changed after the page was opened (the
caller sends the proposed time and the planned window they saw, and a mismatch is a
conflict). The acceptance request cannot be combined with other fields.

**CHG-065** WSO2 may instead **propose a different time**, or keep the planned time and
so decline: this records a disagreement and asks the customer again (a fresh approval).
The state does not change. Asking again with no proposal waiting is the same re-ask.

**CHG-066** A Re-schedule shall need a start and/or end that differs from what is stored,
and the window must start before it ends and have a length. The customer's live request is
replaced by a fresh one; nothing internal is repeated.

**CHG-067** "Waiting" shall mean a proposed time that is stored, not answered, and
recorded as proposed by a registered contact of the project. A time written by WSO2, left
over from an earlier cycle, or whose proposer has since been overwritten by someone else
editing the change is "not recorded": it cannot be accepted (the customer must approve a
new time instead), and staff Re-schedule is then a plain Re-schedule.

**CHG-068** If nobody can be asked about the new time, the Re-schedule or counter-proposal
shall be refused as a whole rather than cancelling the customer's request and leaving
nobody to answer.

## 10. Edit locks

A change request can be edited until it has been approved. Rules judge what a request
*changes*: sending a field back with the value it already holds is accepted as a no-op,
so a client that sends the whole form back is not punished.

**CHG-070** From Request Approval on, the **Customer Project** and the **type** shall be
frozen. A request that changes either is refused and tells the caller to cancel the change
and clone it.

**CHG-071** After New the two customer boxes are add-only: turning one on is allowed
until the gate it controls has passed, and only on a change that has a Customer Project;
turning one off is always refused. Once the change has been approved (CHG-073, which starts
at Customer Approval) both boxes are read-only, so in practice a box can be turned on only
before approval, even though the review gate itself is still ahead in Customer Approval.
*Check:* in Customer Approval, turn on "customer review required"; refused as read-only.

**CHG-072** A new **requester** after Request Approval shall be judged with the new
requester in place of the old one, and refused when nobody would be left to ask about a
customer gate still ahead.

**CHG-073** Once a change request has been approved (in Customer Approval, Scheduled,
Implement, Review, Customer Review, Closed or Rollback) its content fields shall be
read-only: title, description, justification, impact and its description, the
implementation, rollback, test and communication plans, outage, affected services and
components, rollback duration, priority, category, the planned window, assignment group
and engineer, requester, case, service and offering, and the three flags (planning visible
to customers, the two customer boxes). Work notes and comments can still be added, and
the lifecycle still moves (state, hold, Re-schedule, the answer to a proposed time).
A Canceled change is not locked, because it can be cancelled before anyone approved it.
*Check:* edit the description of a Scheduled change; refused; add a work note; accepted.

**CHG-074** Deployments and deployment products shall be fixed from the same approved
states onward, and from then on the scope the change was approved for is fixed.

**CHG-075** At least one field must be given on an edit; a blank comment or work note is
refused.

## 11. Who can see a change request

**CHG-080** Internal staff shall see every change request.

**CHG-081** A customer shall see a change request only while all of these hold: they are
a registered contact of its **current** project, **and** either it was designated to them
(the customer's approval or review was asked of them, now or at any time, in any state) or
it is a legacy change request (below). A contact who registers after the stage was created
was never asked and does not see it. Moving a change to another project takes it away from
the old project's contacts.

**CHG-082** A **legacy** change request is one created before the cut-over instant set by
configuration (if none is set, every change request is legacy). It keeps being visible to
its project's registered contacts in the states customers saw before this rule existed:
everything past Authorize (Scheduled, Customer Approval, Implement, Review, Customer
Review, Rollback, Closed, Canceled), and never New, Assess, Authorize or no state.

**CHG-083** The rule shall be applied in one place and used by the list, counts, detail,
approvals, decisions, edits, comments, project statistics and the email audience, so it
cannot differ between them.

**CHG-084** A customer shall not read staff bookkeeping: tags are hidden from them, and
filtering by tag is available to internal callers only.

**CHG-085** A customer's edit shall be limited to exactly two things: the customer's
answer (approve or reject the approval or review) or a proposed time. Any other field in
the request is refused whatever the portal in front sends, and so is mixing the two.

## 12. Search, detail and GitHub

**CHG-090** The list shall be searchable by project, text, state, impact, closed dates,
number, and a generic filter array: created, updated, planned start and planned end date
ranges, assignment group, assigned user, approval, tag and type. A range whose "to" is
before its "from" is refused; an unknown value for a state or impact is refused. Sorting
includes the planned start and end.

**CHG-091** Counts grouped by a field (for dashboards) shall use the same filters and the
same visibility as the list; the number of groups is capped (10 by default) with the rest
folded into "others".

**CHG-092** The detail of a change shall include the states legally open next for staff
(so the portal offers only valid actions), who approved it and when (derived from the
customer approval stage), the customer proposal and whether the viewer may answer.

**CHG-093** A form lookup shall list a project's deployments and its registered customer
contacts, for **internal callers only**, so a customer cannot enumerate another project's
deployments or contacts.

**CHG-094** The service shall not create or change a change request from GitHub. A GitHub
issue whose title carries the change-request prefix creates a **service request** that
carries the class (Normal, Standard or Emergency) as its type; the change request itself
is raised later, by a person, with the approval path and planned window an issue cannot
supply. See the GitHub documents linked at the top.

## 13. Notices

Notices are triggered by changes in the change request's state or dates, not by the
request that made them. The notice service reads a log of changes and decides who to tell;
the notification service sends the mail. Failures never undo the change.

**CHG-100** When a change **moves into** Assess, Authorize, Review, Customer Approval or
Customer Review, one notice shall be sent saying it is waiting on the audience: the
members of "Devops Approval" (Assess), "CAB Approval" (Authorize) or "Devops Review"
(Review) for the internal states, and the designated project contacts for the customer
states. A change merely edited while already in such a state shall not notify again.

**CHG-101** The internal subject shall carry the owning team, derived from the change's
git reference (an unrecognised or missing reference is a default team, by design); the
customer subject shall not. People in more than one audience are told once.

**CHG-102** A customer notice shall go only to the contacts the change was designated to
(section 11), not every contact of the project, because the number and subject are
themselves a disclosure. For a legacy change request with no customer stage it goes to
every active contact of the project. A contact removed from the project (deactivated)
is not mailed.

**CHG-103** The plan-date conversation shall send: an internal notice when a customer
moves the proposed start date while the change is in Customer Approval, and a customer
notice when WSO2 answers (accepts, or disagrees/counter-proposes/declines; a disagreement
and a decline send the same notice and carry no time).

**CHG-104** A date move by a WSO2 user shall not produce the internal notice: it is the
team editing its own plan. **Confirm** (see 14.2): who made the move is read from the
change's last editor, not from the move itself.

**CHG-105** A notice with nobody to tell shall be dropped silently, not retried. A deleted
change request is a silent no-op.

**CHG-106** The notices are switched on by a setting (off by default) and polled at a
configurable interval (5 seconds by default).

## 14. Open questions for the product owner

1. **Review approval does not gate closing.** Deciding the Review stage records the
   decision and changes nothing; a CS engineer then moves the change to Closed (or
   Customer Review) by hand, even if the review was rejected or never answered. Is that
   intended?
2. **Who moved the date is a guess.** The "WSO2 user moved it" check reads the change's last
   editor at the time the notice is prepared, so a later edit can hide a genuine customer
   proposal or admit a WSO2 one.
3. **A rejected internal approval stalls the change.** It records the rejection and
   changes no state, so the change stays in Assess/Authorize until someone cancels it.
4. **A contact deactivated after Request Approval** can leave a change waiting with nobody
   to answer; there is no automatic recovery besides cancelling or Re-scheduling.
5. **Cut-over instant for customer visibility** is a single configured time. With it
   unset, every change request is legacy, which shows customers more than the strict
   rule would. Who owns setting it, and when?
6. **A legacy proposed date cannot be accepted.** A proposed time that nobody is recorded as
   having proposed can only be replaced by asking the customer to approve a new time.

## 15. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Create, edit, state and approval logic | `internal/repository/change_request_repo.go`, `internal/service/change_request_service.go` |
| Flows, pools, decisions | `internal/repository/change_request_approval_flow.go` |
| Allowed staff moves | `internal/repository/change_request_transitions.go` |
| Customer boxes and the edit lock | `internal/repository/change_request_customer_lock.go` |
| Approved-content lock | `internal/repository/change_request_approved_lock.go` |
| Customer answers and what a customer may send | `internal/repository/change_request_customer_outcome.go` |
| Proposed time and its answers | `internal/repository/change_request_customer_proposal.go` |
| Planned-window rules | `internal/repository/change_request_window.go` |
| Emergency rule | `internal/repository/change_request_emergency.go` |
| Project, deployments and products | `internal/repository/change_request_links.go` |
| Who can see a change | `internal/repository/change_request_visibility.go` |
| Notices | `internal/service/cr_notice_service.go`, `cr_notice_drainer.go`, `internal/repository/cr_notice_repo.go`, `internal/events/change_request.go` |
| Routes | `POST /change-requests` (internal only), `POST /change-requests/search`, `…/aggregate`, `…/link-options` (internal only), `GET`/`PATCH /change-requests/{id}`, `GET /change-requests/{id}/approvals`, `POST …/approvals/decision` |
| Settings | `CR_NOTICES_ENABLED`, `CR_NOTICE_POLL_INTERVAL`, `CR_STRICT_VISIBILITY_FROM` |
| Tests | `internal/repository/change_request_*_test.go`; scenario map in [change-request-tests.md](../specs/change-request-tests.md) |
