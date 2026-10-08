# Operations

Operations is mainly built for the SRE team, though any CS engineer can use
it. It's where the operational record types live: service requests, change
requests, incidents, problems, and outages. These aren't limited to
managed-cloud work; they also cover other SaaS offerings. Each has its own
tab in the Operations sidebar section, its own list, and its own detail
view: they are separate record types, not case sub-types, even though a few
of them link back to a case.

## Service requests

The Service requests tab lists service-request cases using the same shared
issues list and filters as the Support section (case type is locked to
"Service request" here, so the type filter is hidden). Clicking a row opens
the same case detail view used everywhere else in the portal: overview,
comments, attachments, watchers, and the rest of the standard case tabs.

Use **Create service request** to open a form and raise a new one. The form
renders each catalog item's own questions, and now respects that catalog
item's own rules where the backing system declares them: a question marked
mandatory is required, one marked read-only is shown but disabled, a
declared maximum length is enforced as you type, and a declared validation
pattern (e.g. a valid email format) is checked as soon as you leave the
field, with the backing system's own message shown if it doesn't match. A
question the catalog item doesn't currently use (inactive or hidden) isn't
shown at all.

Not every project can raise a service request — the form checks the selected
project's eligibility and shows an error instead of letting you submit
against one that isn't eligible. A project's **Create** menu on its detail
page hides **Create service request** the same way. For a cloud project
with a private data plane, the deployed-product picker is narrowed to that
project's eligible product categories automatically — you won't see
unrelated deployed products in the list.

## Change requests

The Change requests tab lists change requests with server-side search,
pagination, and filters for state, impact, and closed-date range, plus a
CSV export of the filtered results. Each row links to a detail page.

**Saved views**: save the current search/filter combination under a name for
one-click reuse later, from the same "Saved views" button as the Support
section's case list. Saved views here are scoped to this tab only (they
don't show up on Incidents, Problem management, or the case list, and vice
versa) and stay on this device/browser.

The detail page shows:

- A **lifecycle line** across the top, plotting the same eleven-stage workflow
  the customer sees in the Customer Portal: New, Assess, Authorize, Customer
  Approval, Scheduled, Implement, Review, Customer Review, **Rollback**,
  Closed and **Canceled**. Stages already passed are ticked and the current
  stage is highlighted. Rollback and Canceled are the two exits off the normal
  path, so they stay faint until the change really ends there, when the stage
  turns red. Customer Approval and Customer Review only appear when the change
  requires them. An **Emergency** change never takes Assess (it goes from New
  straight to Authorize, for the CAB alone), so Assess reads "not taken" on its
  line like Rollback and Canceled, and it never shows the two customer stages. A
  canceled change keeps no record of where it was canceled,
  so a stage the approvals cannot prove it passed is drawn faint (and read out
  as "history not recorded") rather than guessed. When the customer rejects the
  change at Customer Approval (canceled) or Customer Review (rolled back), that
  stage shows a red cross and, on a canceled change, the stages after it read
  "not taken". Hover a stage for what it means (it also says when a stage was
  not taken or its history is not recorded).
- An **overview** card: Customer Project, type, linked case, deployment,
  deployed product, the selected deployments and their deployment products,
  the Customer Group, category, assigned engineer/team, duration, planned
  start/end, and audit fields.
- Tabs for **Approval**, **Plan**, **Comments**, and **Attachments**.
  - **Approval** shows whether the change requires customer approval and
    customer review, what the customer has confirmed, and a full
    approval-stage breakdown (Peer Approval, CAB Approval, Customer
    Approval, Customer Review) with each individual approver's status. An
    Emergency change has one stage, CAB Approval, and the customer's part reads
    "Not applicable". (A change raised before this keeps the stage it already
    has, even one named "ECAB Approval", and the approvers it asked can still
    decide it.) The Assignment group of each stage is a link: it opens a dialog
    listing the group's members. What the customer has confirmed is read-only — it is
    recorded by the approval itself.
  - **Plan** shows the change-review packet: description, justification,
    impact description, rollback plan, test plan, service outage notes, the
    communication plan, the implementation plan, and the affected
    services/components text and rollback duration. Below that, an **SRE
    details** card shows further fields the backing system tracks:
    priority, requested by, change request type, likelihood, whether the
    Implementation Plan is visible to customers, when the customer last
    proposed a time and WSO2's answer to it, work start/end, a git reference (if any), and any labels. Most
    of these are read-only with no edit control anywhere yet — they're shown
    for context. "Implementation Plan visible to customers" is the exception:
    it's editable from Create/Edit (see below).
    The **Customer Group** is not picked: it is the change request's customer
    project's registered contacts (read-only), the people asked at Customer
    Approval / Customer Review.
  - Planned start and end times you enter when creating or editing a change
    are in your own time zone (the one on your profile, or your browser's if
    none is set) and are stored as UTC, so they mean the same instant to
    everyone.

From the detail page a CS engineer can:

- **Change state**: the action bar's buttons are driven entirely by the
  record's own legal next states, so only valid transitions are ever offered.
  The forward move (for example **Request Approval**, **Start implementation**,
  **Mark implemented**, **Send for customer review**) is the main button;
  **Re-schedule** (Customer Approval only; **Propose a different time** while a
  time the customer proposed waits for you) sits beside it; everything else is
  behind the **Change state** menu. Moving to a destructive state (**Roll back**
  from Review or Customer Review, **Cancel change**) requires typing a reason
  first, which is recorded as an internal note (the customer does not see it)
  before the state change is applied. **Go back** in that dialog leaves the
  change as it was.
- **The customer's answer is theirs to give.** Staff never record a customer's
  approval or review: while a change waits at Customer Approval or Customer
  Review, only the customer moves it on, by answering in the Customer Portal, so
  there is no action for it here, not even a greyed-out one. At
  Customer Approval you can **Re-schedule** (the customer is asked to approve
  the new time) or **Cancel change**; at Customer Review you can **Roll back** or **Cancel
  change**, and there is no Close. While the customer's review is still pending,
  **Roll back** is held back and says who the change is waiting on, since a
  failed review is the customer's to give in the Customer Portal.
- **When nobody can be asked.** The customer's request goes to the Customer
  Project's registered contacts, leaving out whoever raised the change and anyone
  no longer active. A change with Customer Approval and/or Customer Review ticked
  is therefore **not sent for approval while nobody can be asked**: **Request
  Approval** is refused, with a message that says so. Where the project has no
  registered contact at all the button is greyed out with the reason ("Register a
  contact for the Customer Project before requesting approval"); where it has
  contacts but none can be asked (its only contact is the person who raised the
  change, or its contacts are no longer active) the request goes out and the
  refusal appears as an error. Register a contact for the project, then request
  approval. Ticking a customer box on after approval was requested is refused the
  same way. The Customer Project cannot be changed once approval has been
  requested, which is why the contact has to be there first.
  Only an older change can still be left waiting at a gate with nobody to answer:
  one that reached Customer Approval or Customer Review before this was refused,
  one whose contacts left the project since, or one migrated from the previous
  system without a request. The Approval tab says so in a note, and since staff never
  answer for the customer the exits are the ones staff always have there. When
  nobody can be asked, **Cancel change** is the only way out of Customer
  Approval: **Re-schedule** asks the project's registered contacts again at
  once, and is refused, with the same message as Request Approval, while nobody
  can be asked (an older change with no request, on a project that has eligible
  contacts, is the case where it helps). **Roll back** or **Cancel change** are
  the ways out of Customer Review.
- **A time the customer proposed.** A customer who cannot make the planned
  window proposes a new **start** in the Customer Portal; the planned length
  stays the same. The change **stays in Customer Approval** and the planned
  window stays what you planned until you answer, so the page shows a banner
  under the stepper ("The customer proposed a new time") with the planned window
  beside the proposed one, who proposed it and when, and the header reads
  "Waiting for WSO2 to respond to the customer's proposed time". You answer it
  in one of two ways, Agree and Disagree:
  - **Accept proposed time** (after a confirmation) applies the proposal to the
    planned window and moves the change straight to **Scheduled**. No CAB
    approval is needed (the change itself has not changed) and the customer is
    not asked again: the proposal is their own consent. The Customer approved
    cell then reads "Proposed time accepted", because nobody on staff records a
    customer's approval. Accept is held back, with the reason, while the change
    is on hold, once the proposed time has passed, when there is no planned
    window to keep the length of, when the window it would give ends after the
    year 2100 (a date the previous system left far ahead), or when nobody is
    recorded as having proposed the time (see below).
  - **Propose a different time** asks the customer to approve the time you set
    instead (again with no CAB). Keeping the current time **declines** the
    proposal: only the answer is recorded and the customer keeps their request
    to approve the current time. The loop repeats with their next proposal. The
    optional reason you type is recorded as an internal note once the change has
    been updated, so a refused attempt leaves no note behind.

  A date a WSO2 user wrote in the previous system, or one left over from an
  earlier round, looks like a proposal too. The only thing on record about who
  proposed a time is who last changed the change request, and only when that is
  one of the project's registered contacts: so a customer's genuine proposal
  also reads "nobody is recorded" once anyone at WSO2 has edited the change
  request since (an unrelated edit too, such as a plan or the assigned
  engineer). When nobody is recorded as having proposed the stored time, it is
  not a proposal and nothing waits for your answer: the banner reads "A time is stored on this change request" ("A time is
  stored (...) but nobody is recorded as having proposed it."), the header stays
  at "Awaiting Customer Approval", and **Accept proposed time** is disabled with
  the reason, because no staff action stands in for the customer's own answer.
  **Propose a different time** stays available as a plain Re-schedule: there is
  no proposal to decline, so the window must change, and the customer is asked to
  approve the time you set (the stored time itself will do, when it is the one
  the customer wanted: it costs the customer one more approval).

  If the proposal or the planned window moves while one of these dialogs is open
  (the customer proposes again, or a colleague answers), the page does not send
  the request it was opened on: when the answer is refused for that reason the
  dialog closes, a notice at the top of the page says why and the page shows the
  current state.

  Proposals and your answers are kept in PostgreSQL only: they are not mirrored
  to the previous system (there is no field for them), Accept's mirror is best
  effort, and while the sync still runs it can rewrite these columns.
- **Approve or reject** a pending approval stage, if the engineer is listed
  as an approver on it: the Approve/Reject buttons only appear on that
  engineer's own pending approval.
- **Emergency changes proceed without the customer.** An Emergency change goes
  New -> Request Approval -> Authorize (one CAB Approval stage; no Peer stage) ->
  Scheduled, and is never sent to Customer Approval or Customer Review. In the
  create form and the Edit dialog the two customer checkboxes are therefore
  disabled and unticked while the type is Emergency ("Emergency changes proceed
  without customer approval or review."); choosing Emergency clears them, and
  choosing another type leaves them off for you to tick again. The CAB Approval
  stage is asked of the members of the **CAB Approval** group. A group may have no
  members: that is an operations matter (there is no screen for it), and where
  nobody maintains that group's membership in the portal database, Request
  Approval on a new Emergency change is refused with a message naming the group.
- **Edit** the Customer Project, deployments, category, planned window,
  assignment group, assigned engineer, requested by, rollback duration,
  whether the Implementation Plan is visible to customers, the Customer
  Approval / Customer Review checkboxes (until the step they control has
  passed), and the implementation/rollback/test/affected-services/
  affected-components plans, or **Clone** the change request into a new one
  pre-filled with this one's values (useful for promoting the same change
  through another environment). The Customer Project, deployments and
  deployment products can only be changed until implementation starts; the
  deployment products follow from the chosen deployments and are not picked.
  What the customer has confirmed isn't editable here — it is recorded by the
  customer's approval itself.
- Add comments (public or internal) and upload/download attachments. A comment's own author,
  or an admin, can edit or delete it from the **⋮** menu on that comment — deleting is a soft,
  audited removal (an admin can still read the original text; nobody else can).

## Incidents

The Incidents tab lists incidents with server-side search, pagination, and
filters for priority, SLA-violated status, created-date range, and product,
plus a CSV export of the filtered results. Each row links to a detail page.

Like Change requests, this tab has its own **Saved views** button for
naming and reapplying a filter combination — scoped to this tab, on this
device/browser.

A **Create incident** button (or a case's own **Create incident from case…**
action) opens a form for Caller, Service, and a classification (category,
subcategory (optional), channel, impact, urgency — Priority is computed live from
impact × urgency and not itself editable). **Assignment group** is not a
manual pick here: it's shown read-only, auto-filled from the selected
Service's ServiceNow support group, and blank with a hint if that service
has none set in ServiceNow.

The detail page shows:

- An **overview** card: caller, assignment group, assigned to, opened date,
  created by, and last updated.
- Tabs for **Activities**, **Details**, **Related**, **Watchers**, and
  **Attachments**.
  - **Details** covers classification (category, subcategory, channel,
    impact, urgency) and service/configuration-item information.
  - **Related** shows linked records (parent incident, change request,
    problem, and any linked service requests), plus a "caused by" reference
    shown as plain text since its target record type isn't confirmed.

From the detail page a CS engineer can:

- **Change state**: again driven by the incident's own legal next states.
  Moving to Resolved or Closed opens a dialog to collect a resolution code
  and notes, since those are required by the backing system for those two
  transitions. Resolution code is picked from a fixed list rather than typed
  freely: **Solved (Work Around)**, **Solved (Permanently)**, **Not Solved
  (Not Reproducible)**, **False Alarm**, **Duplicate**, and **Not Actionable
  Alert** — use **Duplicate** when closing an incident as a duplicate of
  another one.
- **Escalate to specialist team**: reproduces the backing system's own
  "Escalate to Special Ops" action. Pick a reason (runbook unavailable, or
  the runbook didn't solve the incident) and, for a Choreo incident
  specifically, optionally a team (Choreo Runtime or Choreo APIM). This
  moves the incident to the right specialist group, opens a runbook task,
  and files an internal GitHub issue for the receiving team. The action is
  always shown, regardless of the incident's own service or state — if it
  isn't eligible (wrong service, not In Progress, or already with the
  specialist group), the backing system's own rejection message is shown
  rather than the button being hidden or disabled. Once an incident has been
  handed off (through this button, or ServiceNow's own), a **Specialist
  handoff** card on the detail page shows which group it went to, why, when,
  by whom, a link to the runbook task, and a link to the GitHub issue if one
  exists. If the internal GitHub issue couldn't be created, that's called
  out explicitly rather than left to look like nothing happened — the
  handoff itself still went through; only the issue is missing.
- **Edit** the incident's fields.
- Manage the **watch list** (add or remove watchers). A **Follow incident
  updates** / **Unfollow incident updates** button on the Watchers tab also
  lets you add or remove yourself with one click.
- Add comments (public or internal) and upload/download attachments, with
  inline preview for supported attachment types. A comment's own author, or an admin, can
  edit or delete it from the **⋮** menu on that comment — deleting is a soft, audited removal
  (an admin can still read the original text; nobody else can).

Work notes on an incident often reference the alert or smart alert that
triggered it. Those references render as an inline **View alert** / **View
smart alert** link in the Activities timeline — click one to open a read-only
detail popup (severity, source, environment, and a link to the incident it's
tied to, if any) without leaving the page. A reference to an alert that's
since been removed shows a "could no longer be found" message in the popup
instead of an error.

## Problem management

The Problem management tab lists problems with server-side search,
pagination, free-text search, and a state filter. Each row links to a
detail page.

Like Change requests and Incidents, this tab has its own **Saved views**
button for naming and reapplying a filter combination — scoped to this tab,
on this device/browser.

The detail page shows an overview (priority, category, subcategory, assigned
to, opened/closed dates), any linked records (origin record, primary
incident, linked change request, and linked incidents), and, once resolved,
a resolution section with resolution code, resolved-by, resolved-on, cause
notes, fix notes, and workaround.

A **Create problem** button on the list opens a form to raise a new problem.

The detail page's action bar moves a problem through ServiceNow's own
Problem Management lifecycle, one step at a time: **New → Assess → Root
Cause Analysis → Fix In Progress → Resolved → Closed**. Only one transition
is ever available at once (the next step in the chain); once a problem is
Closed there is nothing further to do. Moving to Fix In Progress opens a
small dialog offering to record cause notes and fix notes — both optional,
and can be added or edited later — before the state changes.

An **Edit** button on the detail page opens a separate dialog for fields
that don't require a lifecycle transition: assigned engineer, assignment
group, workaround, and target resolution date. Two caveats:

- Assigning an engineer to a problem that has no owner yet automatically
  moves it to Assess, even without using the action bar — this is a
  ServiceNow business rule, not a portal quirk.
- Assignment group and target resolution date always start blank in the
  Edit dialog, even if a value was set previously — the portal can't read
  either one back from ServiceNow yet, so it doesn't guess. Target
  resolution date in particular is not shown anywhere on ServiceNow's own
  Problem form; it's a generic tracking field exposed here for the portal's
  own use.

## Outages

The Outages tab lists outage/degradation/planned-maintenance records, with
search, and filters for type, status, and a "published to status page only"
toggle. Status (In progress / Resolved) isn't a field you set directly — it
follows automatically from whether the outage has an end time. A public
outage — one whose linked configuration item is tracked on the public status
page — shows a public-page badge in the list and on its detail page.

**Create outage** opens a range-entry form: type, a short description
(required — this is what would appear on the public status page), a begin
time, and an optional end time (leave it blank for an outage that's still
ongoing; close it later from the detail page). You can optionally link a
configuration item and a related incident, and seed the first external and/or
internal communication entries.

Begin, end, and close times are entered in your own time zone (the one on
your profile, or your browser's if none is set) and stored as UTC, so they
mean the same instant to everyone.

**Linking a configuration item is the one choice that can make an outage
public.** If the item you pick is tracked on a monitored cloud's status
page, a warning appears as soon as you pick it — before you save, not after
a rejection — naming which clouds are monitored, and you'll need to
acknowledge it before the form lets you continue. The same warning and
acknowledgement appears again if you later post an **external** communication
on a public outage, since external entries are echoed verbatim on the public
page.

The detail page shows the outage's window, duration, links, and any affected
configuration items, plus its full communications journal (external,
internal, and additional-notes entries, each timestamped and attributed).
You can post a new communication on any channel from the same page. From
here you can also:

- **Close** the outage (records an end time — defaults to now, but you can
  record a different actual end) or **Reopen** a closed one.
- **Edit** the outage's type, short description, and its configuration-item
  or incident links.

An outage can also be created straight from an incident: the **Create
outage** action on an incident's own detail page opens the same form
pre-filled with that incident (and its configuration item, if it has one) —
both fields can still be changed before saving.
