# Case search and counts — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended;
they are collected in section 10.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

Related: [dashboard-reports.md](dashboard-reports.md) (volume trend, resolution-time
report, the unified search box, project list counts) and [time-cards.md](time-cards.md).

---

## 1. Purpose

CS engineers and customers need to find cases, filter them in many ways, and see counts
of them for lists and dashboard widgets. This document covers searching cases, counting
them, and grouping them into buckets for charts.

In this service the word **case** in a search means any of five kinds of work item that
behave like cases: ordinary cases, service requests, security report analyses,
announcements and engagements. Change requests and incidents are never returned by a
case search; they have their own searches.

## 2. Who is involved

| Who | What they can do |
|---|---|
| CS engineer (internal user) | Search every case in the system. |
| Customer user | Search only the cases of projects they are a registered contact of. |
| Internal service (machine caller) | Search every case. |
| CSM portal | Searches as the signed-in CS engineer; sees everything that person may see. |
| Customer portal | Searches as the signed-in customer; sees only that customer's projects. |

## 3. Who sees which cases

**CSR-010** An internal user shall see every case. A customer shall see only the cases of
projects they are a registered contact of. This is decided in the database itself, so a
request that names a project the customer does not belong to returns nothing, never
another customer's data.
*Check:* a customer searching by another project's id gets an empty result.

**CSR-011** A caller that cannot be identified, a deactivated user, or a user type that is
neither internal nor customer shall be refused rather than given a default view.

**CSR-012** A customer who is registered on no project shall receive an empty result
without the database being asked.

**CSR-013** Announcements that a caller is not entitled to see shall not appear in
search results, in counts or in buckets, for any caller.
*Check:* an announcement aimed at other customers is absent from a customer's search and
does not raise their count.

**CSR-014** Estimated fix times (best case, most likely, worst case) are internal. They
shall be removed from every case returned to a customer.
*Check:* the same case read by an internal user and a customer; only the first shows fix
times.

**CSR-015** If a person is both an internal user and an external customer on file, the
service shall treat them as a customer for what they can list ("less access, never
more"). **Confirm** (see 10.4).

## 4. Asking for a page of cases

**CSR-020** A search shall return one page of cases, plus the total number of matches,
the page size and the position of the page.

**CSR-021** The page size defaults to 20 and shall never exceed 50. A larger request is
refused with a message naming the limit. A negative start position is treated as zero.
*Check:* ask for 51 cases; refused. Ask for none stated; 20 returned.

**CSR-022** Results shall be sorted by creation time, newest first, unless the caller
asks otherwise. The caller may sort by created time, updated time, severity, state or
assignee name, ascending or descending. Any other field or direction is refused.
Cases with no value for the sort field come last. Ties are broken in a fixed order, so
paging returns each case once only while the set of matching cases does not change. Cases
added, removed or re-sorted between page requests can shift page positions and repeat or
skip a case.

**CSR-023** Free-text search shall match part of the case's title, its case number or its
internal case id, ignoring letter case. The text may be at most 200 characters.
Characters that would otherwise act as wildcards are treated as plain text.
*Check:* searching for `50%` finds cases containing "50%", not every case.

**CSR-024** Every returned case shall carry: number, internal id, type, title,
description, severity, issue type, state, engagement type, work state, escalation level,
created and updated times and creator, project, deployment, deployed product, product,
assignee, parent case, related case, account (name and support tier), the account's CRE
and SRE teams, and resolved and closed times.

## 5. Filtering

Filters are given as a list of conditions, each with a field, an operation and values.
All conditions must hold together (see 6 for "either-or").

**CSR-030** The service shall accept only known fields and operations. An unknown field or
operation is refused naming it.

**CSR-031** Each field accepts only the operations that make sense for it; any other pairing
is refused. A condition that needs values but has none is refused.
*Check:* `severity` with `notIn` is refused.

**CSR-032** Values shall be checked before the search runs: identifiers must be valid
ids, and case type, state, severity, issue type, engagement type and work state must be
known values. A bad value is refused, never silently ignored.

**CSR-033** A condition the service cannot apply shall be refused, never dropped,
because dropping an exclusion would widen the result.

### 5.1 What can be filtered

| Field | Operations | Meaning |
|---|---|---|
| type | in | Case, service request, security report analysis, announcement, engagement. The older name `default_case` is accepted as an alias. |
| state | in, notIn | Open, work in progress, waiting on WSO2, awaiting info, reopened, solution proposed, closed. |
| severity | in | Catastrophic, critical, high, medium, low. |
| issueType | in | Error, partial outage, performance degradation, question, security or compliance, total outage. |
| engagementType | in | Migration, consultancy, new feature or improvement, follow-up, onboarding. |
| workState | in | Ongoing, paused. |
| tag | in, notIn | Cases carrying (or not carrying) any of the free-text tags. |
| projectId | in, notIn | By project id. |
| deploymentId | in | By deployment id. |
| accountId | in, notIn | By customer account id. |
| assignedUserId | in, isEmpty | Assigned to one of these people, or not assigned to anyone. |
| createdBy | in, eq | Created by one of these emails, or (eq with a special placeholder) created by the signed-in person. |
| createdOn, updatedOn, closedOn, resolvedOn | gte, lte | A date range. |
| number | eq | The exact case number. |
| internalId | eq | The exact internal case id. |
| parentId | eq | The child cases of one case. |
| product | in, notIn | By the deployed product's catalogue name. |
| productVersion | in | By product version, only together with exactly one product. |
| region | in, notIn | By the account's region, ignoring letter case and surrounding spaces. |
| projectOnboardingStatus | in, notIn | The project's onboarding status: cancelled, completed, expired, in-progress, not-applicable, not-started, on-hold. |
| projectType | in | By project type name. |
| creTeam, sreTeam | in | The account's CRE or SRE team. |
| escalationLevel | in | Escalation level 0 to 5. |
| escalation | isEmpty, isNotEmpty | Whether the case itself is currently escalated. |
| slaBreached | eq | Whether the case has a currently breached SLA. |
| taskSLABusinessElapsedPercent | gte, lte | Share of an SLA's business time used. |
| stateEntered | in | Cases that moved into one of these states at some point. |
| stateEnteredOn | gte, lte | When that move was recorded; needs stateEntered. |

**CSR-040** Dates shall be accepted as a full timestamp, a plain date, or one of the
relative placeholders: today, N days ago, and the start or end of a week, month, quarter
or year, N periods from now. Weeks run Monday to Sunday and all dates are UTC. A plain
date used as an upper bound includes the whole of that day.
*Check:* `closedOn lte 2026-03-31` includes a case closed at 23:00 on 31 March.

**CSR-041** A placeholder that is malformed, has a non-integer or missing offset, or an
offset beyond about four million shall be refused. `daysAgo` does not accept a negative
offset.

**CSR-042** For every date range, an end before its start shall be refused (created,
updated, closed and state-entered ranges).

**CSR-043** The "created by me" condition shall need a signed-in person; without one it
is refused. A literal creator is supplied with the `in` form instead.

**CSR-044** A product version filter without a product filter naming exactly one product
shall be refused, because a version number alone is shared by several products.

**CSR-045** Exclusion filters shall keep cases that have no value at all. A case with no
deployed product is kept by "product not in", a case with no account or region by "region
not in", a case with no project by "project not in", and a case whose project has no
onboarding status by "onboarding status not in".
*Check:* excluding one product still returns cases that have no product.

**CSR-046** The onboarding status filter shall accept the names case-insensitively and
ignoring dashes, underscores and spaces. An unknown status is refused naming the allowed
ones, because for "not in" ignoring it would widen the result.

**CSR-047** The state-entered filter shall match a case that entered the state at any time
in its history, whatever state it is in now, and a case that entered it several times
shall appear once. The state is matched on its display name (for example "waiting on
wso2"), ignoring letter case, as the history records it. **Confirm** (see 10.7).

**CSR-048** The "accountEscalationActive" condition and the "resolution notes empty"
condition shall be refused on this data source.

**CSR-049** SLA filters (percent used and breached) shall be applied, for internal callers,
to the live SLA data. When the deployment is set to answer SLA questions from the legacy
case system, an internal caller's SLA-filtered search is answered there instead;
customers keep the standard rules.

## 6. Either-or conditions

**CSR-060** A search may carry several groups of conditions ("any of"). Within a group
all conditions must hold; the group matches if any group matches; and the result must
also satisfy the ordinary conditions.

**CSR-061** A group with no conditions shall be refused, because it would match
everything.

**CSR-062** Inside a group only these fields are allowed: type, state (in), severity,
engagement type, issue type, work state, project, deployment, assignee, escalation level
and tags. Any other field is refused naming the group.
*Check:* a date range inside a group is refused.

## 7. Special request options

**CSR-070** *Count only.* A caller may ask for the count without the cases. The service
shall then skip the page entirely and return the total with an empty list.

**CSR-071** *Skip total.* A caller may ask the service not to count. The total is then
reported as not computed (-1). It is for callers that never show a total.

**CSR-072** Count only and skip total together shall be refused, since nothing would be left
to compute.

**CSR-073** *Leave out description.* A caller may ask for the description to be left out of
every row, for lists that never show it. The description comes back empty, and the
database does not read it. Leaving it out does not change which fields the free-text search
matches (title, case number and internal case id only, see CSR-023). Any other field name is
refused.

**CSR-074** *Group by a fixed list.* A caller may ask for counts per value of one field
instead of cases. The groupable fields are state, severity, type, engagement type, issue
type and work state. Every possible value shall be listed, with zero for empty ones, in a
fixed order. Any other field is refused.

## 8. Counting per bucket (dashboards)

**CSR-080** The service shall count cases per value of a chosen field, with the same
filters as a search. The fields are account, state, severity, type, product, product
version, project and region. A missing or unknown field is refused.

**CSR-081** Buckets shall be ordered largest first, with ties broken by key. Cases with no
value for the chosen field are in no bucket.

**CSR-082** The caller may cap the number of buckets; the default is 10. The buckets beyond
the cap shall be added together into one "others" count, and the total shall equal the
shown buckets plus "others". The total counts only cases that have a value for the chosen
field.
*Check:* ten buckets of one case each plus "others" of three gives a total of thirteen.

**CSR-083** Every bucket has a key (what to filter on) and a label (what to show): accounts
show their name, projects their name or else their key, and regions the spelling most of
their cases use.

**CSR-084** Grouping by product version needs a product filter naming exactly one product.

**CSR-085** A bucket's count shall always equal the total of the same search with that
bucket's filter applied, because both use the same rules.

## 9. See also

- Searching escalations across cases: [case-escalation.md](case-escalation.md).
- Case feedback has its own search and counts and is not described here.

## 10. Open questions for the product owner

1. **Search on a case's description is only in the unified search box.** The case search
   itself matches title, number and internal id. Is that intended?
2. **A plain 200-character limit on free text** counts characters, not words. Fine?
3. **A type filter silently includes engagements and announcements** when nothing is
   specified. Should some screens default to ordinary cases?
4. **Dual identity:** a person who is both internal and external is treated as a
   customer for listing (CSR-015). Is that the desired outcome for staff who are also
   customer contacts?
5. **"Resolution notes empty" is refused on this data source** while it existed on the
   previous one. Is the dashboard still expected to offer it?
6. **Whether the search count agrees with a list:** counts for a project's tiles use their
   own rules (see [dashboard-reports.md](dashboard-reports.md), section 5).
7. **State-entered matches the display name only.** History rows written with the raw
   internal spelling of a state (for example `solution_proposed`) are not found by this
   filter, although the resolution-time report counts both spellings. Should the filter
   accept both? A case whose history was never recorded is also invisible to it.

## 11. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Filter parsing and validation | `internal/service/case_filters.go`, `internal/service/sla_filters.go` |
| Search, groupBy, aggregate, options | `internal/service/case_service.go` |
| Filters to queries, sorting, counts, buckets | `internal/repository/case_repo.go`, `internal/repository/case_field_predicates.go` |
| Who sees which cases | `internal/service/access_service.go`; row-level security migrations on work items |
| Request and response shapes | `internal/domain/entity.go` |
| Routes | `POST /cases/search`, `POST /cases/aggregate` |
| Tests | `case_service_test.go`, `case_filters_test.go`, `case_repo_test.go` |
