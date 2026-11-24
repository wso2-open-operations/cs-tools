# Projects — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check are
intended; they are collected in section 12.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related documents:** [accounts.md](accounts.md) (the customer organisation a project
belongs to), [deployments-and-products.md](deployments-and-products.md) (what a project
runs), [../specs/sla.md](../specs/sla.md) (case SLA clocks, which are separate from the project health
indicator in section 9).

**Not covered here:** project contacts and memberships, customer health risk tracking, and
the per-project case, time card and change request counts. Each is described with the
feature it belongs to.

---

## 1. Purpose

A **project** is what a customer has bought support for: it has a key, a subscription type,
a start and end date, a closure state that decides whether the customer may still use the
portal, and onboarding information. Almost everything a customer does in the portal (cases,
deployments, change requests) happens inside one project.

The entity service stores projects, lets staff and customers read them, and lets
authorised callers change a small set of their fields.

## 2. Who can see and change what

| Who | Reading | Changing |
|---|---|---|
| Internal staff (and internal services acting for them) | Every project | Every project |
| A customer or partner user | Only projects they are a member of | Only projects they are a member of |

**PRJ-010** A person who is not internal staff shall see only the projects they belong to,
in every project read, list and count. A project outside their membership shall be reported
as "not found", never as "forbidden", so they cannot learn that it exists. **Project metadata
is the exception**: it has no membership check, so any signed-in caller can read it for any
project id they know (section 10; **Confirm**, see 12.7).
*Check:* ask for another customer's project by its id; the answer is "project not found".

**PRJ-011** Which fields a caller is *allowed* to change is decided by the portals, not by
this service. This service only checks that the caller can see the project (section 5).
**Confirm**, see 12.1.

## 3. Reading projects

**PRJ-020** The service shall return one project by id with its account, key, subscription
type, dates, closure information, onboarding information, consulting hour balances and
whether the customer verification feature is switched on.
*Check:* a project with no linked account still returns, with an empty account section.

**PRJ-021** A project's subscription type shall be derived from its project type name
(for example "Cloud Support" becomes `cloud_support`). A project with no project type has
no subscription type.

**PRJ-022** Hour balances shall be reported in hours, and the consumed query hours and
consumed onboarding hours shall always be shown as positive numbers. A balance that is not
tracked shall be reported as "not set", which is different from zero.
*Check:* a project with no query hours recorded shows no value, not 0.

**PRJ-023** The detail shall say whether the project may raise service requests. For
Cloud Support projects this follows the project's own "PDP subscription" flag; for every
other type it follows the project type's setting. **Confirm**, see 12.2.

**PRJ-024** The detail shall show the account owner's and technical owner's email, taken
from the project's account, and whether the account is a partner.

## 4. Searching projects

**PRJ-030** The service shall search projects visible to the caller, returning each
project's name, key, subscription type, dates, closure and onboarding information, account,
onboarding owner and the number of active items, with a total and a "has more" flag.
Paging defaults to 20 per page, at most 50; search text is limited to 200 characters.

**PRJ-031** Text search shall match the project name or key, ignoring letter case.

**PRJ-032** *Active items* shall count the project's cases, service requests, engagements,
security report analyses and announcements that are not closed.

**PRJ-033** The search shall support these filters, all combining (AND):
closure state (Open, Restricted or Suspended), closure states to include or exclude, an end date
range, one account, onboarding status, region, sub region, onboarding owner (by id, email
or team), five onboarding date ranges, Salesforce project ids, subscription types to
exclude, and project keys to exclude.
Any value that is not one of the allowed choices shall be refused with a message listing
the allowed choices. A date range whose start is after its end shall be refused.
*Check:* filter by closure state "Closed"; refused, because only Open, Restricted and
Suspended can be filtered. **Confirm**, see 12.3.

**PRJ-034** A project with no project type shall never be removed by the "exclude
subscription types" filter. A project with no closure state shall never be removed by the
"exclude closure states" filter.

**PRJ-035** Results shall be sortable by end date or by one of four onboarding dates
(go-live plan, go-live, onboarding expiry, production readiness completion), ascending or
descending, with projects that have no value for that date last.
With no sort chosen, projects shall be listed newest first (oldest first if ascending is
asked for).

**PRJ-036** Filtering by the account's annual recurring revenue shall be refused, because
that value is not stored.

**PRJ-037** Project keys given to the "exclude" filter shall not be empty and shall not
have leading or trailing spaces; otherwise the request is refused rather than silently
matching nothing.

## 5. Updating a project

A project can be changed with one request that carries any of the fields below. At least
one must be present.

**PRJ-040** Before anything changes, the service shall check the caller may see the
project (section 2). A caller who may not shall get "not found".

**PRJ-041** The change shall be attributed to the signed-in person. An internal service
with no person behind the call shall be attributed to that service's client id.

**PRJ-042** A project's "last updated" time and person shall change on every successful
update, including one that only changes the linked account's settings (section 7).

**PRJ-043** Two people changing the same project at once shall be handled one after the
other.

**PRJ-044** If any part of an update fails, none of it shall be saved.

## 6. Closure state

A project has three **sub-states**, each driven by a different cause, and one **overall
closure state**.

| Sub-state | Driven by |
|---|---|
| End date | Whether the subscription has passed or is about to pass its end date |
| Invoice due date | Whether invoices are overdue (the values include "previously paid" variants) |
| Compliance violation | Whether a compliance violation was recorded |

**PRJ-050** The service shall let a caller set each sub-state to any of its allowed values.
An unrecognised value shall be refused, with the allowed values named. Values may be
written in any letter case and with spaces (for example "Pending Notified").

**PRJ-051** Whenever any sub-state changes, the service shall recompute the overall closure
state from the three sub-states, in the same update:
- **Suspended** if the end-date state is Closed or Suspended, or the invoice state is any
  Suspended variant, or the compliance state is Suspended;
- otherwise **Restricted** if the end-date state is Restricted or the invoice state is any
  Restricted variant;
- otherwise **Open**.
*Check:* set the invoice state to a Restricted value on an Open project; the overall state
becomes Restricted.

**PRJ-052** The service shall let a caller set the overall closure state directly to Open,
Read Only, Closed, Restricted or Suspended (an *admin override*). If a request sets both
sub-states and the overall state, the overall state the caller named wins.
*Check:* override to Read Only; the project reads back as Read Only.

**PRJ-053** The rule in PRJ-051 never produces Read Only or Closed. Those two states can
only come from an override. **Confirm**, see 12.4.

**PRJ-054** An override shall stay until the next time any sub-state is changed, at which
point the overall state is recomputed and the override is lost. Nothing records that a
state was set by hand. **Confirm**, see 12.4.

**PRJ-055** The service shall store a free-form "suspension process state" (what the
automatic closure process did and when) exactly as sent, without checking its contents.

**PRJ-056** The service shall let a caller mark a project as "suspension process
overridden". While it is true, the automatic account closure process shall not evaluate
that project. Only an explicit true or false changes it; leaving it out changes nothing.

## 7. AI assistant settings

**PRJ-060** The service shall let a caller turn the project's AI assistant ("has agent")
and knowledge suggestions ("has knowledge references") on or off. These settings live on
the project's *account*, so they apply to every project of that account.
*Check:* turn the assistant off for one project; another project of the same account also
shows it off.

**PRJ-061** A project with no linked account cannot take these settings; the request shall
be refused as a conflict.

**PRJ-062** The same update shall be refused if the linked account disappears while the
update runs, rather than reporting success with nothing changed.

## 8. Onboarding

**PRJ-070** The service shall let a caller set the project's onboarding status to one of:
Not-Started, In-Progress, On-Hold, Completed, Cancelled, Expired, Not-Applicable. The value
may be written in any letter case, with or without hyphens, underscores or spaces.

**PRJ-071** The service shall let a caller set the onboarding owner to an **active internal
user**. An unknown id, a customer, or a deactivated person shall be refused as invalid
input. The owner cannot be cleared once set.

**PRJ-072** The service shall let a caller set five onboarding dates: start, go-live plan,
go-live, expiry and production readiness completion. Each shall be a date in
`YYYY-MM-DD` form. A date cannot be cleared once set.

**PRJ-073** No other onboarding date shall be earlier than the start date. If a request
changes the start date, every date (new or stored) is checked against it. Otherwise only the
dates in the request are checked against the stored start date, so old data that already
breaks the rule does not block an unrelated edit.
*Check:* send a go-live date one day before the stored start date; refused.

**PRJ-074** A project can be marked as "verification enabled" or not. This switches the
customer verification feature on for that project.

## 9. Project health indicator

**PRJ-080** The service shall return for each project a set of counts (time logged,
deployments, deployed products, running instances, and open items by type) and a **health
indicator** of either "All Good" or "Needs Attention".

**PRJ-081** The indicator shall be "Needs Attention" if **any** of these is true, and "All
Good" only when none is:
- the project has any open case-like item that carries a severity;
- the project has no active deployed product;
- the project has no end date, or the end date is in the past;
- the project has no live customer administrator contact.
A contact whose membership was removed does not count.
**Confirm**, see 12.5.

**PRJ-082** The "time logged" totals shall hold **minutes**, although the fields are named
as hours. **Confirm**, see 12.6.

**PRJ-083** If the running-instance count cannot be read, the project's other counts shall
still be returned, with the instance count as zero.

**PRJ-084** For a customer, a change request in the Authorize state counts as outstanding,
because it is a re-scheduled request that is back with the approval board. For staff it
does not, except on the customer portal, where it does.

## 10. Project metadata

The full contents of the metadata response are also described in
[reference-data-and-search.md](reference-data-and-search.md) (section 4). The rules below are the
ones that concern the project itself.

**PRJ-090** The service shall return, for one project, the lists of values the portals show
in their dropdowns (case states, severities, issue types, deployment types, engagement
types, resolution codes, causes and others) and the project's **feature flags**: what the
project's type allows (service requests, change requests, engagements, updates, deployments,
time logs, component analysis, usage metrics), the severities it accepts, and the product
categories for default cases and service requests.

**PRJ-091** A Cloud Support or Cloud Evaluation Support project that has the PDP
subscription flag shall always be allowed to read and raise service requests, whatever its
project type says. **Confirm**, see 12.2.

**PRJ-092** A project that does not exist shall be reported as "not found". **Confirm**,
see 12.7 for who may read this.

## 11. Counts and consumption

**PRJ-100** The service shall count projects grouped by onboarding owner, onboarding status
or region, applying the same filters and the same visibility as the project search. The
result shows the largest groups first (default 10, at most 50), and the rest and any
project with no value are summed as "others".

**PRJ-101** The product consumption settings of a project (application, status, client keys
and licence secrets) can be read and changed **only by internal callers**. The response
that carries secrets shall not be cached by anyone.

**PRJ-102** The consumption status shall be reported as a number from 1 to 5: Pending,
Application created, Subscribed, Credentials generated, Completed. A project whose
credentials are generated and which already has a primary secret key reads as 5.

**PRJ-103** Setting a consumption status of zero or below shall leave the stored status
alone. A status of 5 or above shall store Completed.

**PRJ-104** When an update moves the project to the "complete" state, or changes a secret
key of a project already in that state, the "tracking file generated" time shall be
refreshed.

**PRJ-105** A consumption change shall be attributed to the signed-in person, else the
calling service, else the subscription service by default.

## 12. Open questions for the product owner

1. **Which fields may a customer change?** This service lets any project member change any
   project field (including closure states and onboarding) if they call it directly. The
   customer portal limits it to the AI assistant settings. Should the service enforce this
   itself?
2. **Service request access differs between two reads.** The project detail gives the PDP
   flag exception only to Cloud Support. The metadata gives it to Cloud Support and Cloud
   Evaluation Support. Which is right?
3. **Read Only and Closed cannot be searched.** They can be set (PRJ-052) but the search
   filter and the exclude filter only know Open, Restricted and Suspended.
4. **An override is silent and temporary.** The next sub-state change erases it, and nothing
   shows it was ever set. Read Only and Closed are not part of the automatic rule at all.
5. **"Needs Attention" when any case is open.** A busy but healthy project reads as needing
   attention. Is that the intended meaning of the indicator?
6. **Hours that are minutes.** The time totals are named as hours but contain minutes, and
   the portals compensate.
7. **Metadata has no visibility check.** Any signed-in caller can read the metadata and
   feature flags of any project whose id they know.
8. **Consumed hours are shown as absolute values.** A negative stored value (over-use) is
   reported as positive. Is that intended?

## 13. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Read, search, filters, update | `internal/repository/project_repo.go`, `internal/service/project_service.go`, `internal/service/project_update_service.go` |
| Filter and sort validation | `internal/service/sn_project_service.go` (shared validation) |
| Visibility check | `internal/service/access_service.go` (`authorizeProject`) |
| Onboarding date rules | `internal/repository/project_onboarding_dates.go` |
| Counts by group | `internal/service/project_aggregate_service.go`, `internal/repository/project_aggregate_repo.go` |
| Health indicator and counts | `internal/service/project_stats_service.go`, `internal/repository/project_stats_repo.go` |
| Metadata and feature flags | `internal/service/project_metadata_service.go`, `internal/repository/reference_data_repo.go` |
| Consumption | `internal/handler/project_consumption.go`, `internal/repository/project_repo.go` |
| Routes | `GET /projects/{id}`, `POST /projects/search`, `POST /projects/aggregate`, `PATCH /projects/{id}`, `GET /projects/{id}/metadata`, `GET /projects/{id}/stats`, `GET`/`POST /projects/{id}/consumption/status`, `PATCH`/`POST /projects/{id}/consumption` |
| Tests | `project_service_test.go`, `project_update_service_test.go`, `project_search_filters_test.go`, `project_onboarding_*_test.go`, `project_metadata_service_test.go`, `project_stats_service_test.go`, `project_aggregate_service_test.go`, `project_consumption_test.go` |
