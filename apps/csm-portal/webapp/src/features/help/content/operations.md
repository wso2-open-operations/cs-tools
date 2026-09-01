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

## Change requests

The Change requests tab lists change requests with server-side search,
pagination, and filters for state, impact, and closed-date range, plus a
CSV export of the filtered results. Each row links to a detail page.

The detail page shows:

- An **overview** card: project, type, linked case, deployment, deployed
  product, assigned engineer/team, duration, planned start/end, and audit
  fields.
- Tabs for **Approval**, **Plan**, **Comments**, and **Attachments**.
  - **Approval** shows the customer-approval/review flags plus a full
    approval-stage breakdown (e.g. Assess, Authorize, Customer Approval) with
    each individual approver's status. These flags are read-only in this
    portal — see below.
  - **Plan** shows the change-review packet: description, justification,
    impact description, rollback plan, test plan, service outage notes, the
    communication plan, the implementation plan, and the affected
    services/components text and rollback duration. Below that, an **SRE
    details** card shows further read-only fields the backing system tracks:
    priority, category, requested by, customer group, change request type,
    likelihood, whether the plan is visible to customers, when the customer
    last updated it, work start/end, a git reference (if any), and any
    linked environments, deployment products, deployments, or labels. Most
    of these have no edit control anywhere yet — they're shown for context.

From the detail page a CS engineer can:

- **Change state**: the action bar's buttons are driven entirely by the
  record's own legal next states, so only valid transitions are ever offered.
  Moving to a destructive state (rollback, cancel) requires typing a reason
  first, which is recorded as an internal note before the state change is
  applied.
- **Approve or reject** a pending approval stage, if the engineer is listed
  as an approver on it: the Approve/Reject buttons only appear on that
  engineer's own pending approval.
- **Edit** the planned window, assignment group, requested by, customer
  group, rollback duration, and the implementation/rollback/test/affected-
  services/affected-components plans, or **Clone** the change request into a
  new one pre-filled with this one's values (useful for promoting the same
  change through another environment). The customer-approved/reviewed flags
  aren't editable here — they reflect an automation-only stage of the
  change's lifecycle and have no manual UI action in the backing system
  either. Category is also not editable — the backing change-request form
  has no real category control either, so this portal doesn't invent one.
- Add comments (public or internal) and upload/download attachments.

## Incidents

The Incidents tab lists incidents with server-side search, pagination, and
filters for priority, SLA-violated status, created-date range, and product,
plus a CSV export of the filtered results. Each row links to a detail page.

The detail page shows:

- An **overview** card: caller, assignment group, assigned to, opened date,
  created by, and last updated.
- Tabs for **Activities**, **Details**, **Related**, **Watchers**, and
  **Attachments**.
  - **Details** covers classification (category, subcategory, contact type,
    impact, urgency) and service/configuration-item information.
  - **Related** shows linked records (parent incident, change request,
    problem, and any linked service requests), plus a "caused by" reference
    shown as plain text since its target record type isn't confirmed.

From the detail page a CS engineer can:

- **Change state**: again driven by the incident's own legal next states.
  Moving to Resolved or Closed opens a dialog to collect a resolution code
  and notes, since those are required by the backing system for those two
  transitions.
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
- Manage the **watch list** (add or remove watchers).
- Add comments (public or internal) and upload/download attachments, with
  inline preview for supported attachment types.

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
