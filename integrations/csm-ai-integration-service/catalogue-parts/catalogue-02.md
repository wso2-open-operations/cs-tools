## 8. House rules for reading English

| Words | Means |
|---|---|
| "my", "assigned to me" | `assignedUserId in ["__current_user__"]` |
| "I raised", "created by me" | `createdBy eq ["__current_user_email__"]` |
| "raised by person@example.com", "created by <email>" | `createdBy in ["person@example.com"]` (several people: one `in` with all their emails) |
| "raised by Alex" (a name, no email) | not expressible: reply with `{"error": ...}` asking for the email address |
| no creator mentioned | no `createdBy` filter at all (records raised by anyone) |
| "open", "new" (state) | `state in ["open"]` — the Open state only |
| "active", "outstanding", "not closed", "ongoing", "unresolved" | `state notIn ["closed"]` — every state except Closed |
| "CSAT", "customer satisfaction", "feedback", "ratings", "survey responses" | `resourceType: "case_feedback"` (section 5d) |
| "CSAT trend", "CSAT by month / by week", "average rating over time" | a `bar` with `"groupBy": {"bucket": "month"}` (or `week`) |
| "feedback by rating", "rating distribution" | a `pie` with `"groupBy": {"bucket": "rating"}` |
| "why customers were dissatisfied / very dissatisfied / satisfied", "reasons for low ratings" | a `bar` with `groupBy.bucket` `reasons_dissatisfied` / `reasons_very_dissatisfied` / `reasons_satisfied` (for "low ratings", one widget each for very dissatisfied and dissatisfied) |
| "very dissatisfied feedback", "1-star ratings", "unhappy customers' comments" | a `list` with `rating 1` (or `1` and `2` as two widgets); "5-star" is `rating 5` |
| "number of feedback responses", "how many ratings" | a `count` with `{}` |
| CSAT for a period ("this quarter", "last 30 days"), by account, team or engineer | not expressible yet: `{"error": ...}` suggesting the same view over all feedback |
| "accounts", "customers" (as organisations), "clients" | `resourceType: "account"` (section 5c); "customers" adds `lifeCycle "Customer"` |
| "partners" / "prospects" / "lost customers" (accounts) | `lifeCycle` `"Partner"` / `"Prospect"` / `"Lost Customer"` |
| "Rigel's accounts", "accounts of team Apollo" | `abtTeam` with that team's key |
| "accounts at risk", "at-risk customers" | `healthStatus "at_risk"`; "healthy" / "to be reviewed" likewise |
| "accounts by risk", "risk flags" | a `bar` with one slice per risk: Recent escalations, EOL product, No go-live in 6 months, No cases in 6 months, Migration delays, Abandoned migrations |
| "accounts with EOL products" / "not live yet" / "no cases in 6 months" / "with recent escalations" | `risks` `["hasEolProduct"]` / `["hasNoGoLive"]` / `["noSupportCases6mo"]` / `["hasRecentEscalations"]` |
| "projects", "subscriptions" | `resourceType: "project"` (section 5c) |
| "projects in onboarding" / "onboarding not started" / "onboarded" | `onboardingStatus` `["In-Progress"]` / `["Not-Started"]` / `["Completed"]` |
| "projects by onboarding status" | a `pie` with one slice each for Not started, In progress, Completed |
| "active projects" | `excludeClosureStates ["Suspended", "Restricted"]`; "suspended" / "restricted" projects: `closureStatus` |
| "projects ending this month / this quarter", "expiring", "renewals due" | `endDateFrom` / `endDateTo` with the month or quarter tokens; a list sorted by `endDate` `asc` |
| "my accounts", accounts by region, product, owner or ARR | not expressible: `{"error": ...}` as in section 5c |
| "incidents", "outages" (as incidents), "faults" | `resourceType: "incident"` (section 5b) |
| "open", "active", "ongoing", "unresolved" incidents | `state in ["NEW", "IN_PROGRESS", "ON_HOLD"]` |
| "new" / "on hold" / "resolved" / "closed" incidents | `state in` `["NEW"]` / `["ON_HOLD"]` / `["RESOLVED"]` / `["CLOSED"]` |
| "P1" / "critical", "P2" / "high", "P1 and P2" (incidents) | `priorities ["CRITICAL"]`, `["HIGH"]`, `["CRITICAL", "HIGH"]` |
| "SLA breached", "SLA violated" (incidents) | `slaViolated eq ["true"]`; "met SLA" is `madeSla eq ["true"]` |
| "change requests", "changes", "CRs" | `resourceType: "change_request"` (section 5b) |
| "open", "active", "in progress" change requests | `states` every state except `closed`, `canceled`, `rollback` |
| "waiting for approval", "pending approval" (change requests) | `approval eq ["requested"]`; "awaiting customer approval" is `states ["customer_approval"]` |
| "scheduled", "being implemented", "in review" (change requests) | `states ["scheduled"]` / `["implement"]` / `["review", "customer_review"]` |
| "high impact" (change requests) | `impacts ["high"]` |
| "problems", "root causes" | `resourceType: "problem"` (section 5b) |
| "open", "active" problems | `state in ["NEW", "ASSESS", "ROOT_CAUSE_ANALYSIS", "FIX_IN_PROGRESS"]` |
| "my incidents", "my changes", "my problems" | `assignedUserId in ["__current_user__"]` |
| "by state" (incidents, change requests, problems) | `pie`/`bar` with `"groupBy": {"field": "state", "maxGroups": 8}` |
| operations "by assignment group / team / service", a named team's, or another person's | not expressible: `{"error": ...}` as in section 5b |
| "time cards", "time logs", "timesheets", "logged time" | `resourceType: "time_card"` (section 5a) |
| "my time cards", "time I logged" | `time_card` with `userIds ["__current_user__"]` |
| "waiting for approval", "awaiting approval", "to approve", "submitted" (time cards) | `states ["submitted"]` |
| "pending" (time cards) | `states ["pending", "submitted"]` — not decided yet |
| "approved" (time cards) | `states ["approved", "processed"]` |
| "rejected" / "recalled" (time cards) | `states ["rejected"]` / `states ["recalled"]` |
| "time cards by state / status" | a `pie` or `bar` with one slice per state: Pending (`pending`), Submitted, Approved (approved + processed), Rejected. Leave Pending out only if the request asks to; add Recalled only if asked |
| "this month", "last month", "past 30 days" (time cards) | `startDate` (and `endDate`) with the date tokens of section 5a, on the work date |
| time cards "per user", "per engineer", "per project", a team's or a named person's, or hours | not expressible: `{"error": ...}` as in section 5a |
| "my open", "open … assigned to me", "my open cases" (open together with "my") | `state notIn ["closed"]` — "my open" means my active cases, every state except Closed (plain "open" without "my" stays the Open state) |
| other wording about a case's state | pick by meaning: a word for one stage of a case ("just raised", "reopened", "awaiting the customer") is `state in` the matching state(s) from the `state` row of section 5; a word for "everything still being worked on" ("pending", "live", "still going") is `state notIn ["closed"]` |
| "in progress" | `state in ["work_in_progress"]` |
| "waiting on us", "waiting on WSO2" | `state in ["waiting_on_wso2"]` |
| "waiting on the customer" | `state in ["awaiting_info", "solution_proposed"]` |
| "closed" / "resolved" in a period | `state in ["closed"]` plus `closedOn gte <token>` |
| "unassigned", "no assignee" | not expressible: reply with `{"error": ...}` saying unassigned filtering is not supported |
| "escalated", "escalations" | `escalation isNotEmpty` |
| "SLA breached", "SLA violations" | `slaBreached eq ["true"]` |
| "about to breach", "near SLA" | `taskSLABusinessElapsedPercent gte ["80"]` plus `slaBreached eq ["false"]` |
| "P1" / "S1" / "critical" | `severity in ["critical"]` ("catastrophic" only when asked for) |
| "high and above" | `severity in ["catastrophic", "critical", "high"]` |
| "recent", "latest" | a `list` with `sortBy createdOn desc`, `listLimit` 5 to 10 |
| "last N days/weeks" | `createdOn gte __daysAgo:N__` (weeks x 7) |
| "trend", "over time", "opened vs closed" | a `line` widget |
| "by severity / state / type" | `pie` (share) or `bar` (comparison) with `slices` |
| "top customers", "by account" | `bar` with `groupBy: {"field": "account"}` |
| "cases" with no other type named | `resourceType: "case"` |
| "service requests", "SRs" | `resourceType: "service_request"` |
| "engagements", "migrations" | `resourceType: "engagement"` (migrations add `engagementType in ["migration"]`) |
| "my team's" | a `"cre"` dashboard with `creTeam in ["__current_team__"]` |
| a team by name: "Rigel", "Rigel's", "team Rigel", "the Rigel team", "Sirius and Draco" (any case) | that team's **key** in that team's **field** from the Teams section, on a `"cs"` dashboard, in **every** widget the request is about; several teams: one `in` with all their keys; the team names start `displayName` |
| "ABT", "ABT team", "account based team", "CRE ABT" | a team whose **family is CRE ABT** in the Teams section. Teams of any other family (e.g. a CRE team such as Americas, or an SRE ABT team) are **not** ABT teams |
| "my ABT", "my ABT team" | same as "my team's" |
| a named team with "ABT": "Rigel ABT", "the Sirius ABT" | same as the team by name |
| "all ABTs", "every ABT" (no grouping) | one `creTeam in [...]` with the keys of **every** CRE ABT team |
| "by ABT", "per ABT", "for each ABT", "separated by ABT", "across ABTs" — the only grouping | one `pie` or `bar` widget, on a `"cs"` dashboard, with **one slice per CRE ABT team** (label = team name, query `creTeam in ["<key>"]`), in the order of the Teams section |
| "by ABT" **together with** another grouping, e.g. "by severity, separated by ABT" | **one widget per CRE ABT team**: each widget's `query.filters` carries `creTeam in ["<key>"]`, its `displayName` starts with the team name (e.g. "Rigel: SLA Breaches by Severity"), and its slices are the other grouping; `gridWidth` 4. Only the teams the request names, if it names some |
| a team that is not in the Teams section | not expressible: reply with `{"error": ...}` naming the team and the teams that can be used |
| "past month", "last month" (rolling), "past 30 days" | `createdOn gte __daysAgo:30__` |
| "last calendar month", "previous month" | `createdOn gte __startOfMonth:-1__` and `createdOn lte __endOfMonth:-1__` |

If two readings are plausible and the table does not settle it, pick the one above and add a
`description` on the widget saying what it counts (e.g. "Every state except Closed").

---

## 9. Check before you answer

1. Only fields, ops, values and shapes from this file. No literal dates or ids. Email addresses only in
   `createdBy in`, and only ones the request gave; "I raised" is `createdBy eq ["__current_user_email__"]`.
2. Every widget has `id`, `displayName`, `resourceType`, `shape`, `gridWidth` 1-12, `query`.
3. Widget ids are unique.
4. Every `pie`/`bar` has exactly one of `slices` / `groupBy`; every `list` has `listLimit`, and `sortBy`
   except a `problem`, `account` or `case_feedback` list, which has no `sortBy` (items 10-12).
5. `line` only on `resourceType: "case"`, `days` 1-90.
6. `severity` and `issueType` only on `case`; `engagementType` only on `engagement`.
7. `type` + `isTeamBased`: `cs` with `false`, or `cre` with `true`. `isDefault` is `false`.
8. Team filters: only keys from the Teams section, each in its own field (`creTeam` / `sreTeam`),
   op `in`, in the top-level `filters` (never an `anyOf` branch), never a team id; `__current_team__`
   only on a `cre` dashboard. A dashboard for named teams starts its `displayName` with their names.
   "ABT" means only the teams whose family is CRE ABT; a team filter may sit in a slice's query
   ("by ABT"), never in an `anyOf` branch.
9. A `time_card` widget: a flat `query` with only `states`, `userIds` (`["__current_user__"]` only),
   `startDate`, `endDate`; shape `count`, `list` (sortBy `workDate`/`updatedOn`) or `pie`/`bar` with
   flat-query `slices`; never `groupBy`, `line`, a `filters` array or a team filter.
10. An `incident` / `change_request` / `problem` widget: only the keys, fields, ops and **exact-case
    values** of section 5b; change request state in the flat `states`, never in `filters`; `groupBy` by
    `state` only; a problem list has no `sortBy`; no ids, no `anyOf`, no `line`.
11. An `account` / `project` widget: a flat `query` with only the keys of section 5c; single values as
    one string, booleans as `true` / `false`, `abtTeam` a team key; no `groupBy`; an account list has
    no `sortBy`, a project list sorts by `endDate` only.
12. A `case_feedback` widget: a flat `query` with only `rating` (a number 1-5); no `dateFrom` / `dateTo`;
    `pie`/`bar` grouped with `groupBy.bucket` (or rating `slices`), never `groupBy.field`; a list has no
    `sortBy`; no `line`.
13. The output is a single JSON object.

---

## 10. A complete example

Request: *"Show my open cases by severity as a pie, and a list of recent escalations."*

```json
{
  "displayName": "My Cases and Escalations",
  "type": "cs",
  "isDefault": false,
  "targetTeam": "",
  "isTeamBased": false,
  "widgets": [
    {
      "id": "my-open-cases-by-severity",
      "displayName": "My Open Cases by Severity",
      "description": "Assigned to me, every state except Closed.",
      "resourceType": "case",
      "shape": "pie",
      "gridWidth": 6,
      "inlineDrilldown": true,
      "query": { "filters": [
        { "field": "assignedUserId", "op": "in", "values": ["__current_user__"] },
        { "field": "state", "op": "notIn", "values": ["closed"] }
      ] },
      "slices": [
        { "label": "Catastrophic", "color": "error", "query": { "filters": [ { "field": "severity", "op": "in", "values": ["catastrophic"] } ] } },
        { "label": "Critical", "color": "warning", "query": { "filters": [ { "field": "severity", "op": "in", "values": ["critical"] } ] } },
        { "label": "High", "query": { "filters": [ { "field": "severity", "op": "in", "values": ["high"] } ] } },
        { "label": "Medium", "query": { "filters": [ { "field": "severity", "op": "in", "values": ["medium"] } ] } },
        { "label": "Low", "query": { "filters": [ { "field": "severity", "op": "in", "values": ["low"] } ] } }
      ]
    },
    {
      "id": "recent-escalations",
      "displayName": "Recent Escalations",
      "resourceType": "case",
      "shape": "list",
      "gridWidth": 6,
      "listLimit": 10,
      "sortBy": { "field": "createdOn", "order": "desc" },
      "query": { "filters": [
        { "field": "escalation", "op": "isNotEmpty" }
      ] }
    }
  ]
}
```
