# Entity service — requirements

Plain-language requirements for what the entity service does. They describe current
behaviour, not original intent. Where the code does something a product owner should
confirm, the document says so (**Confirm**) and lists it under "Open questions".

The code stays the source of truth. If a requirement here and the code disagree, fix
this document in the same change.

## How to read a requirement

Each requirement has an ID, a "the service shall…" statement, and where useful a
**Check**: a simple way to see it is true.

An ID is a short **prefix for the feature area** plus a **number**: in `ESC-041`, `ESC`
is case escalation and `041` is one requirement (here, that a customer-triggered
escalation never emails the customer). Numbers are grouped by topic in tens (`010`s are
escalating, `020`s de-escalating, and so on). IDs are stable; when behaviour changes,
edit the requirement rather than renumbering.

Each document ends with a "Where this is implemented" table pointing at the code, routes,
migrations, settings and tests, so a requirement can be traced to both.

## Status

| Area | Document | Status |
|---|---|---|
| Case escalation and de-escalation | [case-escalation.md](case-escalation.md) | Done (pilot) |

## Proposed next areas

Grouped from what the service implements today. Order and grouping are open to change.

- **Cases:** create and numbering; search, filters and counts; update (assignee,
  acknowledge, parent, tags, watch list); state transitions; type transfer; auto-closure
  and reminders; feedback; reopen; volume trend
- **Case collaboration:** comments and attachments; call requests; announcements
- **Projects, accounts and deployments:** projects and closure state; accounts and
  contacts; deployments and deployed products; instances and usage metrics
- **People and access:** users and roles; memberships and registration; caller-scoped
  access; skills; team schedule and rota
- **SLA:** SLA status; the SLA clock engine; policy and duration rules
- **Changes and incidents:** change requests and the GitHub sync; incidents, problems
  and tasks; incident reports
- **Customer-facing data:** portal banners; saved dashboards; reports (MTTR, time cards)
- **Integrations and events:** Salesforce ingest; event publishing; notifications
- **Platform:** authentication and token validation; health; configuration

Related specs that already exist: [sla.md](../sla.md), [team-schedule.md](../team-schedule.md),
[github-cr-sync-spec.md](../github-cr-sync-spec.md), [usage-metrics.md](../usage-metrics.md).
