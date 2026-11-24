# Dashboard figures and the search box — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended;
they are collected in section 7.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

Related: [case-search.md](case-search.md) (case search and bucket counts) and
[time-cards.md](time-cards.md) (time card counts and sums).

---

## 1. Purpose

Four figures and views that screens are built on:

1. **Case volume trend:** how many cases were opened and closed per day, week or month.
2. **Resolution-time report:** how long cases took to reach a proposed solution, by severity.
3. **The search box:** one search that finds both projects and cases.
4. **Project list counts:** three numbers beside each project in the project list.

## 2. Who is involved

| Who | What they can use |
|---|---|
| CS engineer (internal user) | All four. |
| Customer user | The search box and the project list counts, scoped to their own projects. |
| Internal service | All four where the route allows a service. |

The volume trend and the resolution-time report are **internal only**. A customer who
calls them is refused.

## 3. Case volume trend

A line chart of "Opened" and "Closed" cases.

**CSR-100** The service shall return exactly two series, **Opened** (counted by the day the
case was created) and **Closed** (counted by the day the case was closed), one point per
day over the window, oldest first.

**CSR-101** Days are UTC calendar days. A day with no cases shall still appear, with a
count of zero, so the line is continuous. Days still in the future count zero.

**CSR-102** The caller may choose the window in one of two ways, never both:
- **Trailing window:** the last N days up to and including today. N defaults to 30 and
  shall be between 1 and 90.
- **Fixed window:** a start and an end date (plain dates or relative placeholders such as
  "start of last month"), with the end not before the start.
A request that mixes N with dates, or gives only one date, shall be refused.
*Check:* days 120 is refused; days 30 together with a start date is refused.

**CSR-103** A fixed window may group points into **day**, **week** or **month** buckets
(day is the default). A bucket without a start and end date shall be refused. Limits:
a day bucket covers at most 90 days, a week bucket at most 54 weeks, a month bucket at
most 24 months. Weeks run Monday to Sunday. Any other bucket name is refused.

**CSR-104** For a week or month bucket each point shall carry its first and last day, both
clipped to the requested window, so the dates show exactly the days the count covers.
The response echoes the resolved start, end and bucket of a fixed window.

**CSR-105** The trend shall accept the same filters as a case search
([case-search.md](case-search.md), section 5), so a chart can be limited to a team,
product, severity and so on. The usual rules apply, including that only the five
case-like types are counted when no type is given.

**CSR-106** Opened and Closed shall be counted for the same set of cases, so a case that was
both opened and closed in the window appears in both.

**CSR-107** Only internal users may use the trend. Everyone else is refused.

**CSR-108** Counts shall be remembered per day and per set of filters, to keep dashboards
fast. A day that has ended is remembered for about 120 days; today is remembered only until
UTC midnight, so no day is ever shown from before it finished. The remembered figures are
shared by every internal caller, which is why only internal callers are allowed.
*Check:* a figure computed at 10:00 for today is served again at 11:00, and recomputed on the
first request after UTC midnight. **Confirm** (see 7.1).

**CSR-109** Recomputing missing days shall cost one database query however many days are
missing.

## 4. Resolution-time report

How long, on average, it took to bring cases to a proposed solution, by severity.

**CSR-110** The report shall be limited to internal callers.

**CSR-111** The caller shall name exactly one window: **months** (1, 3, 6 or 12) or
**days** (7 only). Naming both, neither, or any other value is refused with a message
listing the allowed ones.

**CSR-112** The window ends now and starts the stated number of months or days earlier
(UTC). A case is in the window when its last entry into Solution Proposed falls in it
(the start included, the end excluded).

**CSR-113** For each case, the time to resolution is the time from the case being created
to the **last** time it entered Solution Proposed. The last entry is used, not the first,
because a rejected proposal sends the case back and a later proposal is the one that
stuck.
*Check:* a case proposed, rejected, then proposed again 3 days later is measured to the
second proposal.

**CSR-114** Only a case that is resolved **now** shall be counted: its current state must be
Solution Proposed or Closed. A case whose proposal was rejected and is back in progress
is not counted until it is proposed again or closed.

**CSR-115** Only ordinary cases with a severity are counted. Service requests,
engagements, announcements and other types are not.

**CSR-116** Results shall be given in these bands, in this order: **S1–S4**, **S1–S3**,
**S0**, **S1**, **S2**, **S3**, **S4**. S0 (catastrophic) is its own band and is never
folded into S1–S4 or S1–S3. Each band reports its case count and its average resolution
time in hours.

**CSR-117** A combined band shall add the cases and the total time of its severities
first and divide once, so a severity with many cases weighs more than one with few. A band
with no cases reports zero.
*Check:* S1 with 1 case of 10 h and S2 with 9 cases of 20 h average to 19 h, not 15 h.

**CSR-118** The caller may restrict the report to one or more CRE teams (the team that owns
the customer account). Blank values are ignored; every other value must be a valid id.
With none given, every team is included.

**CSR-119** The report shall count every case regardless of which projects the caller
belongs to, so the figures are the same for every internal caller.

**CSR-120** The response shall also carry when it was generated, the window used and the
teams applied.

## 5. The search box (projects and cases)

One search that finds both the projects and the cases a person may see.

**CSR-130** The search shall return projects and cases the caller may see: a customer sees
only projects they are a registered contact of and the cases in them; an internal user
sees everything. A caller with no resolvable access is refused, never given an unscoped
result. A customer registered on no project gets empty lists.

**CSR-131** A project matches when the text is part of its name or its key. A case matches
when the text is part of its number, its title, its internal id or its **description**.
Letter case is ignored, and surrounding spaces in the text are trimmed. A text of only
spaces means no text (everything). The text may be at most 200 characters.

**CSR-132** Cases searched here are the same five case-like kinds as in the case search.
Announcements the caller is not entitled to see are left out.

**CSR-133** The caller may ask for projects only, cases only, or both (the default). Any
other table name is refused.

**CSR-134** Projects and cases are sorted separately. By default projects sort by name
(A to Z) and cases by last update, newest first. A caller-chosen field (name, created
time, updated time) applies to both; its default direction is A to Z for name and newest
first for the dates, unless the caller states a direction. Any other field or direction is
refused.

**CSR-135** Projects and cases have their own paging: 20 per page by default, never more
than 50.

**CSR-136** The response shall give the text searched, the list of projects and of cases
(never missing, an empty list when there are none), and the total of each.

**CSR-137** Each project in the result shall carry its name, key, description, type,
account, start and end dates, closure state, a flag saying whether it has a PDP
subscription, and the three counts in section 6.

**CSR-138** Each case in the result shall carry its number, internal id, title,
description, creation and update times, creator, project, type, state, severity,
assigned engineer and account.

## 6. Project list counts

Three numbers shown beside each project in the project list.

**CSR-140** For every project on the returned page the service shall give an **outstanding**
count, an **action required** count and an **active chats** count. They are worked out for
the projects on the page only, not the whole list.

**CSR-141** The counts shall be the same numbers the project's own dashboard shows, from the
same groupings of states, so a project's row and its dashboard tiles agree. This is the
rule "cards count what their lists show".

**CSR-142** **Outstanding** counts ordinary cases, service requests, engagements and
security report analyses that are not closed, plus change requests that are in motion
(see 6.1). Announcements are not counted. An item that has no state of its own is not
counted.

**CSR-143** **Action required** counts the items waiting on the customer: cases in Awaiting
Info or Solution Proposed, and change requests in Customer Approval or Customer Review.

**CSR-144** **Active chats** counts conversations that are open or active.

**CSR-145** A customer's counts shall include only what they may see. In particular a
customer sees a change request only if it was **designated** to them, or if it is an older
change request that stays visible without designation (the exception in CSR-152, see 6.1), and
the counts apply that same rule, so the count matches the list the customer sees.

**CSR-146** If working out the counts fails, the project list shall still be returned with
the three counts at zero and the problem logged, because the list is the way into a project.
The one exception is a request that has run out of time or was cancelled: that is returned
as a failure rather than shown as zeros.

**CSR-147** The counts shall be worked out in small groups of projects at a time, so a
customer with many projects does not make the list slow.

### 6.1 Change request rules used by the counts

**CSR-150** Change requests in **Customer Approval, Scheduled, Implement, Rollback, Review
and Customer Review** are outstanding. Closed and cancelled ones are in no group. New and
Assess are not outstanding.

**CSR-151** **Authorize** is outstanding for a customer, and for any user seen through the
customer portal, but not for internal users elsewhere. This keeps a card and the list
behind it in agreement, because the customer portal's list shows Authorize to everyone.

**CSR-152** A customer sees a change request only if (a) they are a registered contact of its
current project, and (b) it was designated to them, meaning their approval or review was
asked of them at some point; or (c) it is an older change request created before a
configured cut-over date, which stays visible in the states customers saw before this rule,
that is, everything after Authorize. With no cut-over date set every change request counts
as older. Internal users see all.

## 7. Open questions for the product owner

1. **Volume trend is not scoped by caller.** Its cache is shared by all internal
   callers, so the endpoint is closed to anyone else. Is it ever meant for customers?
   (CSR-108)
2. **Description is searched in the search box but not in the case search.** Is the
   difference intended? (CSR-131)
3. **Resolution time is measured to "Solution Proposed", not to closure.** Is that what
   the business means by resolution time?
4. **Project counts ignore a user's role.** The dashboard narrows by what the user's role
   may see; the list counts do not, so a user without access to a type still has those
   items in the totals.
5. **Announcements are left out of the project list counts**, matching the dashboard.
   Confirm they should stay out.
6. **Resolution time covers ordinary cases only.** Service requests and engagements are
   not measured. Is that intended?
7. **Cut-over date for change requests.** With the date unset, every change request is
   treated as older. Is that the intended production setting?

## 8. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Volume trend | `internal/service/case_service.go`, `internal/repository/case_repo.go` |
| Resolution-time report | `internal/service/mttr_report_service.go`, `internal/repository/mttr_report_repo.go`, `internal/handler/mttr_report_handler.go` |
| Search box | `internal/service/global_service.go`, `internal/repository/global_search_repo.go` |
| Project list counts | `internal/repository/global_search_repo.go`, `internal/service/project_stats_service.go`, `internal/service/project_case_stats_service.go` |
| Change request visibility | `internal/repository/change_request_visibility.go` |
| Internal-only gate | `internal/server/internal_only.go`, `internal/service/require_internal.go` |
| Routes | `POST /cases/volume-trend`, `GET /reports/mttr`, `POST /search` |
| Settings | `CR_STRICT_VISIBILITY_FROM` |
| Tests | `mttr_report_*_test.go`, `global_search_*_test.go`, `case_service_test.go` |
