# Operations

Operations is mainly built for the SRE team, though any CS engineer can use
it. It's where the operational record types live: service requests, change
requests, incidents, and problems. These aren't limited to managed-cloud
work; they also cover other SaaS offerings. Each has its own tab in the
Operations sidebar section, its own list, and its own detail view: they
are separate record types, not case sub-types, even though a few of them link
back to a case.

## Service requests

The Service requests tab lists service-request cases using the same shared
issues list and filters as the Support section (case type is locked to
"Service request" here, so the type filter is hidden). Clicking a row opens
the same case detail view used everywhere else in the portal: overview,
comments, attachments, watchers, and the rest of the standard case tabs.

Use **Create service request** to open a form and raise a new one.

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
    impact description, rollback plan, test plan, service outage notes, and
    the communication plan.

From the detail page a CS engineer can:

- **Change state**: the action bar's buttons are driven entirely by the
  record's own legal next states, so only valid transitions are ever offered.
  Moving to a destructive state (rollback, cancel) requires typing a reason
  first, which is recorded as an internal note before the state change is
  applied.
- **Approve or reject** a pending approval stage, if the engineer is listed
  as an approver on it: the Approve/Reject buttons only appear on that
  engineer's own pending approval.
- **Edit** the planned window, assignment group, and rollback/test plans, or
  **Clone** the change request into a new one pre-filled with this one's
  values (useful for promoting the same change through another environment).
  The customer-approved/reviewed flags aren't editable here — they reflect an
  automation-only stage of the change's lifecycle and have no manual UI
  action in the backing system either.
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
- **Edit** the incident's fields.
- Manage the **watch list** (add or remove watchers). A **Follow incident
  updates** / **Unfollow incident updates** button on the Watchers tab also
  lets you add or remove yourself with one click.
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
