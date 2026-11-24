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

Each document ends with an "Open questions" section for the product owner and a
"Where this is implemented" table pointing at the code, routes, migrations, settings and
tests, so a requirement can be traced to both.

## Prefixes

| Prefix | Area |
|---|---|
| `CAS` | Cases: creation, changes, states, closure |
| `CSR` | Case search, dashboard figures, time cards |
| `ESC` | Case escalation and de-escalation |
| `COL` | Case collaboration: comments, attachments, call requests, announcements, tags |
| `PRJ` | Projects, accounts, deployments, engagements, services |
| `USR` | Users, access control, memberships, skills |
| `SLA` | Service levels |
| `CHG` | Change requests, incidents, problems |
| `CUS` | Portal features: banners, dashboards, knowledge base, outages, growth, health |
| `INT` | Salesforce ingest, events, data sources, scheduled tasks |
| `PLT` | Platform: authentication, conventions, health, settings, security |

## Documents (1578 requirements)

### Cases

| Document | Prefix | Requirements |
|---|---|---|
| [Cases: creation, numbering and detail](case-creation-and-detail.md) | `CAS` | 47 |
| [Cases: changing a case and its state](case-updates-and-states.md) | `CAS` | 59 |
| [Cases: closing, reopening, auto-closure and feedback](case-closure-and-reopen.md) | `CAS` | 55 |
| [Case escalation and de-escalation](case-escalation.md) | `ESC` | 44 |
| [Case search and counts](case-search.md) | `CSR` | 39 |
| [Dashboard figures and the search box](dashboard-reports.md) | `CSR` | 41 |
| [Time cards](time-cards.md) | `CSR` | 45 |

### Case collaboration

| Document | Prefix | Requirements |
|---|---|---|
| [Case comments and attachments](comments-and-attachments.md) | `COL` | 58 |
| [Call requests and service requests](call-requests-and-service-requests.md) | `COL` | 38 |
| [Announcements](announcements.md) | `COL` | 33 |
| [Tags, watch list, contacts and chat links](tags-watchers-contacts.md) | `COL` | 35 |

### Projects, accounts and deployments

| Document | Prefix | Requirements |
|---|---|---|
| [Projects](projects.md) | `PRJ` | 49 |
| [Accounts](accounts.md) | `PRJ` | 20 |
| [Deployments, products and instances](deployments-and-products.md) | `PRJ` | 53 |
| [Customer engagements and allocations](engagement-allocations.md) | `PRJ` | 25 |
| [IT services and service offerings](services-and-offerings.md) | `PRJ` | 4 |

### People and access

| Document | Prefix | Requirements |
|---|---|---|
| [Users](users.md) | `USR` | 47 |
| [Access control](access-control.md) | `USR` | 34 |
| [Project memberships and onboarding](memberships-and-onboarding.md) | `USR` | 62 |
| [Skills](skills.md) | `USR` | 28 |

### Service levels

| Document | Prefix | Requirements |
|---|---|---|
| [SLA clocks](sla-clocks.md) | `SLA` | 52 |
| [SLA policies](sla-policies.md) | `SLA` | 31 |
| [SLA status and reporting](sla-status-and-reporting.md) | `SLA` | 47 |
| [SLA admin actions](sla-admin-actions.md) | `SLA` | 20 |

### Changes and incidents

| Document | Prefix | Requirements |
|---|---|---|
| [Change requests](change-requests.md) | `CHG` | 75 |
| [Incidents, problems, incident tasks and conversations](incidents-and-problems.md) | `CHG` | 62 |
| [Incident follow-up automation and paging ranks](incident-reports.md) | `CHG` | 31 |

### Portal features

| Document | Prefix | Requirements |
|---|---|---|
| [Portal banners](portal-banners.md) | `CUS` | 22 |
| [Saved dashboards and saved filter views](custom-dashboards.md) | `CUS` | 34 |
| [Reference data and service lookups](reference-data-and-search.md) | `CUS` | 12 |
| [Case alert settings and public holidays](alert-settings-holidays-and-health.md) | `CUS` | 28 |
| [Knowledge base and runbooks](knowledge-base-and-runbooks.md) | `CUS` | 29 |
| [Outages and the cloud status page](outages-and-cloud-status.md) | `CUS` | 44 |
| [Product-led growth (PLG)](product-led-growth.md) | `CUS` | 39 |
| [Customer health: project risks and action items](customer-health.md) | `CUS` | 24 |
| [Commercial data and pending verification](commercial-data-and-verification.md) | `CUS` | 24 |

### Integrations and platform

| Document | Prefix | Requirements |
|---|---|---|
| [Salesforce ingest](salesforce-ingest.md) | `INT` | 76 |
| [Event publishing](event-publishing.md) | `INT` | 26 |
| [Data sources, mirroring and scheduled-task runs](data-sources-and-scheduled-tasks.md) | `INT` | 27 |
| [Platform and operations](platform-and-operations.md) | `PLT` | 59 |

Related specs that already exist: [sla.md](../specs/sla.md), [team-schedule.md](../specs/team-schedule.md),
[github-cr-sync-spec.md](../specs/github-cr-sync-spec.md), [usage-metrics.md](../specs/usage-metrics.md),
[github-integration-deployment.md](../specs/github-integration-deployment.md),
[change-request-tests.md](../specs/change-request-tests.md).
