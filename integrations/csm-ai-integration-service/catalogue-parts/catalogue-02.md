## 5. Filters

A filter is `{"field": "<field>", "op": "<op>", "values": ["..."]}`. All filters in `filters` are
ANDed. Only the field / op pairs in this table are accepted.

| Field | Ops | Values |
|---|---|---|
| `state` | `in`, `notIn` | `open`, `work_in_progress`, `waiting_on_wso2`, `awaiting_info`, `solution_proposed`, `reopened`, `closed` |
| `severity` | `in` | `catastrophic`, `critical`, `high`, `medium`, `low` *(resourceType `case` only)* |
| `issueType` | `in` | `error`, `partial_outage`, `performance_degradation`, `question`, `security_or_compliance`, `total_outage` *(case only)* |
| `workState` | `in` | `ongoing`, `paused` *(not announcements)* |
| `engagementType` | `in` | `migration`, `consultancy`, `new_feature_improvement`, `follow_up`, `onboarding` *(engagement only)* |
| `assignedUserId` | `in` | `["__current_user__"]` (the viewer) |
| `assignedUserId` | `isEmpty` | no `values`: unassigned, no engineer assigned |
| `createdBy` | `in` | creator email addresses, any person, **only those the request gives** (e.g. `["person.one@example.com", "person.two@example.com"]`) |
| `createdBy` | `eq` | exactly `["__current_user_email__"]` (raised by the viewer); `eq` takes no other value |
| `escalation` | `isEmpty`, `isNotEmpty` | no `values` (not escalated / escalated) |
| `escalationLevel` | `in` | `"0"` to `"5"` (as strings) |
| `slaBreached` | `eq` | `["true"]` or `["false"]` (exactly one) |
| `taskSLABusinessElapsedPercent` | `gte`, `lte` | one number as a string, e.g. `["80"]` |
| `tag` | `in`, `notIn` | tag names, **only if the request gives them** |
| `product` | `in` | exact product names, **only if the request gives them** |
| `createdOn`, `updatedOn`, `closedOn`, `resolvedOn` | `gte`, `lte` | one date (below) |
| `creTeam`, `sreTeam` | `in` | team **keys** from the Teams section (the last section), each in its team's field: e.g. `creTeam in ["rigel"]`, `creTeam in ["sirius", "draco"]`, `sreTeam in ["apollo"]` |
| `creTeam`, `sreTeam` | `in` | `["__current_team__"]` ("my team"), **only on a `"cre"` dashboard**, never together with a team key |

**Dates.** Every date filter in this file (`gte` from, `lte` to, both inclusive: `lte` covers the whole
day) takes one value, of one of two kinds:

- a **date token** for a period counted from today, so a saved dashboard moves with the date:
  `__today__`, `__daysAgo:N__` (N >= 0), `__startOfWeek:N__`, `__endOfWeek:N__` (weeks run Monday to
  Sunday), `__startOfMonth:N__`, `__endOfMonth:N__`, `__startOfQuarter:N__`, `__endOfQuarter:N__`,
  `__startOfYear:N__`, `__endOfYear:N__` (N = 0 this period, -1 the previous one, -2 the one before);
- a **plain date** `YYYY-MM-DD`, only for a period the request names by its calendar date, so it never
  moves.

| Request | `gte` | `lte` |
|---|---|---|
| "today" | `__today__` | |
| "this week" / "this month" / "this quarter" / "this year" | `__startOfWeek:0__` / `__startOfMonth:0__` / `__startOfQuarter:0__` / `__startOfYear:0__` | |
| "last week" / "last quarter" / "last year" (the previous calendar one) | `__startOfWeek:-1__` / `__startOfQuarter:-1__` / `__startOfYear:-1__` | `__endOfWeek:-1__` / `__endOfQuarter:-1__` / `__endOfYear:-1__` |
| a named month: "December 2025", "2025 December", "Dec 2025" | `2025-12-01` | `2025-12-31` (the month's last day) |
| a named quarter: "Q1 2026" | `2026-01-01` | `2026-03-31` |
| a named year: "2025", "in 2025" | `2025-01-01` | `2025-12-31` |
| a named day: "on 3 March 2026" | `2026-03-03` | `2026-03-03` |
| a range: "1 to 15 March 2026", "between 2026-03-01 and 2026-03-15" | `2026-03-01` | `2026-03-15` |
| "since 1 March 2026", "from March 2026" | `2026-03-01` | |
| "until 31 March 2026" / "before 1 March 2026" | | `2026-03-31` / `2026-02-28` (the day before) |

A month or quarter named **without a year** ("December", "in March", "Q2") is the most recent one that has
begun on today's date: this year's if it has begun, otherwise last year's (in November 2026, "December"
is December 2025 and "October" is October 2026). Today's date is given after the request
(`Today's date: YYYY-MM-DD`). Never use a plain date for a period counted from today ("this month",
"last 30 days"), and never a token for a named one ("December 2025").

```json
{ "id": "cases-dec-2025", "displayName": "Cases Created in December 2025", "resourceType": "case",
  "shape": "count", "gridWidth": 4,
  "query": { "filters": [ { "field": "createdOn", "op": "gte", "values": ["2025-12-01"] },
                          { "field": "createdOn", "op": "lte", "values": ["2025-12-31"] } ] } }
```

**OR across fields** (use sparingly): add `"anyOf": [{"filters": [...]}, {"filters": [...]}]`
beside `filters`. Each branch is ANDed inside; branches are ORed; the result is ANDed with
`filters`. **Inside an `anyOf` branch only these fields are allowed:** `state`, `severity`,
`issueType`, `workState`, `engagementType`, `assignedUserId`, `escalation`, `escalationLevel`,
`slaBreached`, `taskSLABusinessElapsedPercent`, `tag`. Dates, `createdBy`, `product`, `creTeam`
and `sreTeam` must go in the top-level `filters`, never in a branch.

Never use: ids of projects, accounts, deployments or users; `number`; `internalId`; `parentId`;
`projectId`; `accountId`; `deploymentId`; `resolutionNotes`; `accountEscalationActive`; or any
field / op not in the table.


### 5a. Time cards (`resourceType: "time_card"`)

A time card is the time one engineer logged on one case for one work day (hours split over analysing,
setting up, reproducing/debugging, providing a solution and patching). An engineer logs it and it goes
straight to **submitted**, waiting for an approver; the approver **approves** or **rejects** it; an
approved card is later **processed**. **pending** and **recalled** come from older, synced records.

A time card's `query` is a **flat object**, never a `filters` array, and takes only these keys:

| Key | Value |
|---|---|
| `states` | a non-empty list of `pending`, `submitted`, `approved`, `rejected`, `processed`, `recalled` |
| `userIds` | **only** `["__current_user__"]`: the viewer's own time cards ("my time cards") |
| `startDate` | the work date from (inclusive): one date (section 5) |
| `endDate` | the work date to (inclusive): one date (section 5) |

```json
{ "resourceType": "time_card", "shape": "count",
  "query": { "states": ["submitted"], "userIds": ["__current_user__"], "startDate": "__startOfMonth:0__" } }
```

Shapes: `count`; `list` with `sortBy` `workDate` or `updatedOn` (section 6); `pie`/`bar` with
`slices` (each slice's `query` also a flat object, e.g. one slice per state) or `groupBy` (below).

**Totals: grouping on the server.** A `pie`/`bar` time card widget may group with
`"groupBy": {"field": "<field>", "measure": "count" | "hours", "maxGroups": N}`:

| `groupBy.field` | One bar / wedge per | Notes |
|---|---|---|
| `user` | engineer who logged the cards | "per user", "per engineer", "per person", "by team member"; `bar`, `maxGroups` 5 to 12 |
| `project` | project the cards were logged on | "per project"; `bar`, `maxGroups` 5 to 12; cards with no project count under "Others" |
| `day` | work date, every day of the window in date order (days with nothing at 0) | "per day", "daily"; always `bar`, no `maxGroups`; the window rules below |
| `state` | time card state, as stored (approved and processed apart) | only for **hours** by state; card **counts** by state use the state slices of section 8 |

`measure`: `"hours"` sums the time logged on the cards ("hours", "time logged", "time spent", "effort");
`"count"` (or leaving `measure` out) counts the cards ("number of time cards", "how many"). With
`maxGroups` the top groups are kept by that measure.

**A `day` grouping needs a window**: both `startDate` and `endDate`, **two date tokens or two plain
dates** (never one of each), at most **31 days** long on every day. Use exactly one of these:

| Request | `startDate` | `endDate` |
|---|---|---|
| "last N days" per day, N = 1 to 31 (e.g. 7, 14, 30) | `__daysAgo:<N-1>__` (last 7 days: `__daysAgo:6__`) | `__today__` |
| "this week" per day | `__startOfWeek:0__` | `__today__` |
| "last week" per day | `__startOfWeek:-1__` | `__endOfWeek:-1__` |
| "this month" per day | `__startOfMonth:0__` | `__today__` |
| "last month", "previous month" per day | `__startOfMonth:-1__` | `__endOfMonth:-1__` |
| a named month or range of at most 31 days per day ("October 2026", "1 to 15 March 2026") | its first day | its last day |

Anything longer per day ("this quarter", "this year", "last 60 days", "since <date>") is not expressible
per day (section 1). A `user`, `project` or `state` grouping may use any date window (or none).

**"Last N days" on any time card widget** (count, list, slices or any grouping) is `__daysAgo:<N-1>__` to
`__today__`, exactly N work dates as in the table above ("last 30 days": `__daysAgo:29__`). This differs
from the cases' `__daysAgo:N__` (section 8) on purpose: a time card window must not depend on the grouping.

```json
{ "id": "my-hours-per-day", "displayName": "My Hours per Day (Last 14 Days)", "resourceType": "time_card",
  "shape": "bar", "gridWidth": 12,
  "query": { "userIds": ["__current_user__"], "startDate": "__daysAgo:13__", "endDate": "__today__" },
  "groupBy": { "field": "day", "measure": "hours" } }
```

```json
{ "id": "hours-per-user", "displayName": "Hours Logged per User This Month", "resourceType": "time_card",
  "shape": "bar", "gridWidth": 6,
  "query": { "startDate": "__startOfMonth:0__", "endDate": "__today__" },
  "groupBy": { "field": "user", "measure": "hours", "maxGroups": 10 } }
```

```json
{ "id": "cards-per-project", "displayName": "Approved Time Cards per Project Last Month", "resourceType": "time_card",
  "shape": "bar", "gridWidth": 6,
  "query": { "states": ["approved", "processed"], "startDate": "__startOfMonth:-1__", "endDate": "__endOfMonth:-1__" },
  "groupBy": { "field": "project", "measure": "count", "maxGroups": 10 } }
```

**Not expressible for time cards** (section 1):
- another named person's time cards (only "my" time cards, or everyone's);
- per account, per case or per approver, or a team's (Rigel's, my team's) time cards;
- one total as a single number ("my total hours this month"): a `count` counts cards, not hours;
- billable hours (billable and non-billable are not told apart);
- per day over more than 31 days (above).

### 5b. Operations: incidents, change requests, problems

These are the SRE side's records: an **incident** is an outage or a fault being restored, a **change
request** is a planned change to a deployment (it goes through approvals, is scheduled, implemented and
reviewed), a **problem** is the root cause behind one or more incidents. Each has its own query shape;
**the values are case-sensitive exactly as written below**.

**`incident`.** `query` may hold a flat `priorities` list and a `filters` list:

| Where | Field / key | Op | Values |
|---|---|---|---|
| flat | `priorities` | (list) | `CRITICAL` (P1), `HIGH` (P2), `MODERATE` (P3), `LOW` (P4), `PLANNING` (P5) |
| `filters` | `state` | `in` | `NEW`, `IN_PROGRESS`, `ON_HOLD`, `RESOLVED`, `CLOSED`, `CANCELLED` |
| `filters` | `createdOn` | `gte`, `lte` | one date (section 5) |
| `filters` | `slaViolated`, `madeSla` | `eq` | `["true"]` or `["false"]` |
| `filters` | `assignedUserId` | `in` | `["__current_user__"]` only ("my incidents") |

```json
{ "resourceType": "incident", "shape": "count",
  "query": { "priorities": ["CRITICAL", "HIGH"],
             "filters": [ { "field": "state", "op": "in", "values": ["NEW", "IN_PROGRESS", "ON_HOLD"] } ] } }
```

**`change_request`.** State is the flat, **lower-case** `states` list (never a `state` entry in `filters`):

| Where | Field / key | Op | Values |
|---|---|---|---|
| flat | `states` | (list) | `new`, `assess`, `authorize`, `customer_approval`, `scheduled`, `implement`, `review`, `customer_review`, `closed`, `canceled`, `rollback` |
| flat | `impacts` | (list) | `high`, `medium`, `low` |
| `filters` | `approval` | `eq` | exactly one of `not requested`, `requested`, `approved`, `rejected` |
| `filters` | `createdOn` | `gte`, `lte` | one date (section 5) |
| `filters` | `assignedUserId` | `in` | `["__current_user__"]` only |

**`problem`.** Only a `filters` list:

| Field | Op | Values |
|---|---|---|
| `state` | `in` | `NEW`, `ASSESS`, `ROOT_CAUSE_ANALYSIS`, `FIX_IN_PROGRESS`, `RESOLVED`, `CLOSED` |
| `assignedUserId` | `in` | `["__current_user__"]` only |

**Shapes.** `count`; `list` with `listLimit` and `sortBy`: incident `createdOn`, `updatedOn` or
`openedOn`; change request `createdOn` or `updatedOn`; **a problem list sets no `sortBy`**; `pie`/`bar`
with `slices` (each slice's `query` in the same shape) or `"groupBy": {"field": "state", "maxGroups": 8}`.

**Not expressible** for these (section 1): grouping or filtering by assignment group,
team, service or business service; a named team's or another person's records; dates other than when
created (closed or resolved dates); `line` charts; an `anyOf`.

### 5c. Customers: accounts and projects

An **account** is a customer organisation (also partners and prospects); a **project** is one of its
subscriptions (with an onboarding status, a closure status and an end date). Both take a **flat** `query`
(no `filters` list). Single values are **one string** (not a list) and booleans are JSON `true` / `false`;
values are case-sensitive exactly as written.

**`account`:**

| Key | Value |
|---|---|
| `lifeCycle` | one of `"Customer"`, `"Partner"`, `"Prospect"`, `"Lost Customer"`, `"Lost Prospect"` |
| `abtTeam` | one team **key** from the Teams section (the account's CRE or SRE team) |
| `risks` | a list (any of them): `hasNoGoLive` (not live 6 months after activation), `hasGoneLive`, `hasRecentCases`, `noSupportCases6mo` (no cases in 6 months), `hasEolProduct` (a product past end of life), `hasAbandonedMigrations`, `hasMigrationDelays`, `hasRecentEscalations` |
| `healthStatus` | one of `"at_risk"`, `"healthy"`, `"to_be_reviewed"` (the customer health status) |
| `hasProject`, `active` | `true` / `false` |

**`project`:**

| Key | Value |
|---|---|
| `onboardingStatus` | a list of `Not-Started`, `In-Progress`, `Completed`, `Not-Applicable` |
| `closureStatus` | one of `"Open"`, `"Suspended"`, `"Restricted"` |
| `excludeClosureStates` | a list of `Open`, `Suspended`, `Restricted` to leave out |
| `endDateFrom`, `endDateTo` | the subscription end date, from / to (inclusive): one date (section 5) |

```json
{ "resourceType": "account", "shape": "count", "query": { "abtTeam": "rigel", "healthStatus": "at_risk" } }
```

**Shapes.** `count`; `list` with `listLimit`: **an account list sets no `sortBy`**, a project list may
sort only by `endDate` ("ending soonest" is `asc`); `pie`/`bar` with `slices` only (each slice's `query`
flat too), e.g. one slice per risk or per onboarding status.

**Not expressible** (section 1): "my accounts" or a
named account manager's / owner's accounts; a region or sub-region; a product; ARR or revenue; a named
account's projects; grouping on the server (`groupBy`).

### 5d. Customer satisfaction (`resourceType: "case_feedback"`)

A **feedback response** is a customer's rating of a case (1 = Very Dissatisfied, 2 = Dissatisfied,
3 = Neutral, 4 = Satisfied, 5 = Very Satisfied), with an optional comment and the reasons they ticked
for that rating. "CSAT" is the average rating.

`query` is a flat object with only:

| Key | Value |
|---|---|
| `rating` | a whole **number** 1 to 5 (not a string), e.g. `"rating": 1` |

**No date filters for now** (no `dateFrom` / `dateTo`): every widget covers all feedback. A request
for a period ("this quarter", "last 30 days") is not expressible yet (section 1).

Grouping is on the server with `"groupBy": {"bucket": "<bucket>"}` on a `pie` or `bar`:

| Bucket | Shows |
|---|---|
| `month`, `week`, `day` | a bar per period of the **average rating** (the CSAT trend); use `bar` |
| `rating` | a **count** per rating level |
| `reasons_very_dissatisfied`, `reasons_dissatisfied`, `reasons_neutral`, `reasons_satisfied`, `reasons_very_satisfied` | a **count** per reason customers ticked for that rating |

```json
{ "resourceType": "case_feedback", "shape": "bar", "query": {}, "groupBy": { "bucket": "month" } }
```

Other shapes: `count` (how many responses); `list` with `listLimit` and **no `sortBy`** (newest first);
`pie`/`bar` with `slices` by `rating`. Never `groupBy.field`, `line`, or an account, team, engineer or
case filter (not expressible).

---

## 6. Sorting (lists only)

`"sortBy": {"field": "<field>", "order": "asc" | "desc"}`

Fields: `createdOn`, `updatedOn`, `severity`, `state`, `assignee`. For `time_card`: `workDate` or
`updatedOn` only ("recent time cards" means `workDate` `desc`). For operations see section 5b (a
problem list has no `sortBy`); an `account` list has no `sortBy`; a `project` list sorts by `endDate` only; a `case_feedback`
list has no `sortBy`.
"Recent" means `{"field": "createdOn", "order": "desc"}`; "recently updated" means `updatedOn`.

---

## 7. Grouping (pie and bar)

**Fixed slices**: use when the groups are a short known list (severity, state, issue type, work
state, engagement type, ABT teams). One slice per value, in a sensible order:

```json
"slices": [
  { "label": "Critical", "color": "error",
    "query": { "filters": [ { "field": "severity", "op": "in", "values": ["critical"] } ] } }
]
```

Each slice's `query.filters` is added to the widget's own `query.filters`. `color` is optional, one
of `primary`, `secondary`, `success`, `error`, `info`, `warning`; use `error` for the most severe
or breached group, `warning` next, otherwise leave it out.

**Server-side grouping**: use when the groups are open-ended:

```json
"groupBy": { "field": "account", "maxGroups": 10 }
```

`groupBy.field` must be one of `account`, `state`, `severity`, `type` (for `incident`,
`change_request` and `problem`: `state` only; `time_card`: `user`, `project`, `day` or `state` with
`measure`, section 5a; `account`, `project`: no `groupBy`; `case_feedback`: `groupBy.bucket` only,
section 5d). `measure` is for `time_card` only. Prefer `slices` for
`state` and `severity`; use `groupBy` for `account` ("top customers by ..."). For `account` you may
add `"linkResourceType": "account"` **inside `groupBy`** (never on the widget itself) so a bar opens
that account:

```json
"groupBy": { "field": "account", "maxGroups": 10, "linkResourceType": "account" }
```

`maxGroups` 5 to 12.

Never set both `slices` and `groupBy`.

---
