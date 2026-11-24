# Call requests and service requests — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 8.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Prefix:** `COL` (case collaboration). This document uses `COL-100` to `COL-199`.

---

## 1. Purpose

Two ways to get help beyond a case's comment thread:

- A **call request** asks for a live call about a case: the customer (or a CS engineer)
  proposes times, and the call is then scheduled, held and written up.
- A **service request** is a standard, form-based request chosen from a **catalog**
  (for example "give me access to X"), handled by the SRE team.

This document covers call requests, the service-request catalog the forms come from, and
the notices the service publishes about service requests. How a service request itself is
created, updated and numbered is part of the general case requirements.

## 2. Who is involved

| Who | What they do |
|---|---|
| Customer user | Asks for a call on a case in their project; accepts, rejects or cancels calls. |
| CS engineer | Asks the customer for a call; schedules, reschedules, rejects and writes up calls. |
| SRE team | Receives a chat card when a service request arrives and when a customer comments on it. |
| Notification service | Turns the published events into chat cards. |

## 3. Call requests: asking for a call

**COL-100** A call request shall be raised against one case-like record (a case, service
request, engagement and so on). A call on anything else shall be refused as "case not
found".

**COL-101** A call request shall need a **case**, a **reason**, at least one **preferred
time**, and a **duration** in minutes that is more than zero. Anything missing shall be
refused with a clear message.
*Check:* raise a request with no preferred times; refused.

**COL-102** The new call shall be in state **Pending on WSO2** (a customer asked, WSO2
needs to answer). A CS engineer asking on the customer's behalf can instead create it as
**Pending on Customer**. Any other starting state shall be refused.

**COL-103** The request shall be recorded as raised by the signed-in person, with a
call number from its own number series. The preferred times are kept as given and returned
in UTC.

**COL-104** A person shall only raise a call on a record they can see. A customer can only
do so for a project they are a registered contact of; for an announcement they must also
be allowed to see that announcement.
*Check:* a customer raises a call on another project's case; "case not found".

**COL-105** This service does not check how soon the preferred times are, or that they fall
in working hours. Any such rule (for example a minimum lead time by case severity) is applied
by the portals before the request is sent. **Confirm** (8.1).

## 4. Call requests: the life of a call

A call is always in one of eight states:

| State | Meaning |
|---|---|
| Pending on Customer | WSO2 proposed; waiting for the customer |
| Pending on WSO2 | The customer asked; waiting for WSO2 |
| Scheduled | A time is agreed |
| Rejected by customer | The customer turned down WSO2's proposal |
| Rejected by WSO2 | WSO2 turned down the request |
| Canceled | Called off by either side |
| Notes pending | The call took place; the write-up is owed |
| Concluded | The call is written up and closed |

**COL-110** A call shall be updated in one step that names its new state and any of: new
preferred times, duration, meeting time, assignee, notes, plan, attendees, action items,
actual call length and a reason for cancelling or rejecting. Fields not named are left
alone.

**COL-111** A state that is not one of the eight shall be refused.

**COL-112** Moving a call to **Scheduled** shall need a meeting time and a duration. The
meeting time shall be a valid date and time.

**COL-113** If new preferred times are sent, there shall be at least one. A duration or
actual call length shall be more than zero.

**COL-114** An **assignee** shall be named by email and shall be a person the system
knows; an unknown person shall be refused as "not found".

**COL-115** A reason for rejecting or cancelling shall be kept with the call and shown
back as its **cancellation reason**.

**COL-116** A call may be written up (concluded) in two ways:
- **With notes**: any caller who can see the call may conclude it with notes. Blank notes
  count as no notes.
- **Without notes** ("mark as completed"): only internal staff may do it, and only while the
  call is **Scheduled** or **Notes pending**. Anyone else is refused; from any other state
  the answer is a conflict that names the state the call is in. Notes already on the call
  are never erased by this.
*Check:* a customer marks a scheduled call as completed; refused. A CS engineer does it on
a Notes-pending call; it becomes Concluded and its notes stay.

**COL-117** Other state changes are **not** checked against a state diagram. Any caller who
can see the call can set any of the eight states (for example straight from Pending to
Concluded with notes). **Confirm** (8.2).

**COL-118** A call that does not exist, or that the caller cannot see, shall be "not
found". If the case is named too, a call that belongs to a different case shall also be
"not found".

**COL-119** Each update shall record who made it and when.

## 5. Call requests: finding them

**COL-120** For one case, the service shall return its call requests in pages, optionally
only in certain states.

**COL-121** Across all cases (the dashboards' view), the service shall return call requests
filtered by any of: the state of the call, the state of its case, cases not in given states,
the case's assignee, and the case's team. It shall sort by created, updated or meeting time,
either direction (default: most recently updated first), and page the result.

**COL-122** "Assigned to" in that view means the person who owns the **case**; a call
handed to someone else also matches that someone, so a call reaches the engineer who agreed
to attend even when they do not own the case.
*Check:* filter by a case owner; pending calls on their cases appear, though none has an
assignee of its own yet.

**COL-123** A case's team in that view is its account's CRE team.

**COL-124** A call shall show its number, case (id, title, number), reason, cancellation
reason, preferred times, duration, meeting time and link, state, assignee, notes, plan,
attendees, action items, actual call length, and created and updated times.

**COL-125** Call requests shall not send emails or events of their own.
**Confirm** (8.3).

**COL-126** While the legacy case system is mirrored, a new call shall be created there
first and fail if that fails; an update shall be copied there best effort and never fail
the update here.

## 6. The service-request catalog

**COL-130** The service shall list the **catalogs** (groups of request types) that have at
least one request type available for a given **deployed product**, each with its request
types, in pages. A deployed product that does not exist shall be "not found".

**COL-131** A request type is available for a deployed product when a routing rule allows
it, where a rule can name a product unit and a product classification, and leaving either
blank means "any". A rule's unit must match the product's unit, but only when both are
known; a rule's classification must match the deployed product's own. A deployed product
with no classification matches only rules that do not ask for one.
*Check:* a deployed product with an unknown classification does not get every
classification's request types.

**COL-132** Only active catalogs shall be listed, sorted by name, and request types within
a catalog shall be sorted by name.

**COL-133** For a request type, the service shall return the **questions** of its form in
their order: the question text, kind of answer, the key it is saved under, whether it is
mandatory, active, read-only or hidden, a default, a maximum length, a related table for
look-ups, a validation rule, and the choices for a multiple-choice question. A request type
that does not belong to the stated catalog shall be "not found".

**COL-134** The catalog and its questions shall be read-only here: they are loaded from the
source system by a separate synchronisation, never edited through this service.

**COL-135** When the service request is created, a catalog, a request type and at least
one answer shall be required.
*Check:* create a service request with no answers; refused.

**COL-136** The deployed product's category must be allowed for service requests in the
project's type, where the project type lists allowed categories. When it lists none, any
category is allowed.

**COL-137** A question marked mandatory is **not** enforced when a service request is
created; whoever builds the form must enforce it. **Confirm** (8.4).

## 7. Service request notices

These notices work only when the service runs with a team-events destination configured.

**COL-140** When a service request is created, the service shall publish an **SR created**
event for every new one, carrying its number, title, description, state, project and who
created it. A service request created from a GitHub issue gets the same treatment.

**COL-141** If the service request's account has an SRE team that this deployment has
listed for automation, the service shall first **assign** the request to that team, so
the event already shows the team.

**COL-142** For such an automated request, the service shall then post the standard
acknowledgement as a customer-visible comment ("Your service request has been received
successfully and has entered our review process…"), set its state to **Open**, and
publish an **SR acknowledged** event. A request whose team is not listed is left alone.
The acknowledgement is written directly and does not trigger a comment email.

**COL-143** Every comment or work note on a service request shall publish an **SR comment
added** event, carrying the author, the text, its type and the request's tags, so that the
notification service can alert the SRE team when a customer comments.
*Check:* a customer comments on a service request; one SR comment event is published.

**COL-144** The SR events shall be published whatever other emails the comment triggers.
They shall not be published when no author is known, or for any type of comment other than
comment and work note.

**COL-145** Failing to read or publish a notice shall be logged and shall never fail the
service request or comment.

**COL-146** While the legacy case system creates a service request first, it runs its own
notice flow, so this service shall not publish a created or acknowledged notice for it. A
service request raised from a GitHub issue is the exception: this service publishes the
notices for it in every mode.

## 8. Open questions for the product owner

1. **Preferred-time rules** (lead time, working hours) are not checked here, so another
   caller of this service could book any time. Who owns the rule?
2. **No call state diagram.** A customer could set a call to Scheduled or Concluded
   themselves. Is it intended that only "mark as completed" is staff-only?
3. **No emails for call requests.** Nothing tells the other side that a call was asked for,
   scheduled or rejected. Is that handled by the portals?
4. **Mandatory form questions** are not enforced at create.
5. **Automation is per SRE team**, set by configuration. A request for a team not on the
   list is never acknowledged or assigned.

## 9. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Call request rules and states | `internal/service/call_request_service.go`, `internal/repository/call_request_repo.go` |
| Call request routes | `POST /call-requests`, `POST /call-requests/search`, `POST /call-requests/search-all`, `PATCH /call-requests/{id}` |
| Catalog | `internal/service/catalog_service.go`, `internal/repository/catalog_repo.go`; `POST /catalogs/search`, `GET /catalogs/{catalogId}/items/{catalogItemId}/variables` |
| Service request validation | `internal/service/case_service.go` (create checks) |
| Service request notices | `internal/service/sr_notice_service.go`, `internal/repository/sr_notice_repo.go`, `internal/events/service_request.go` |
| Row-level security | migration `100004` (call requests) |
| Settings | `SRE_EVENT_HUB_TOPIC`, `SR_ALERT_SRE_TEAM_IDS` |
| Tests | `call_request_service_test.go`, `call_request_repo_test.go`, `catalog_service_test.go`, `sr_notice_service_test.go` |
