# Customer engagements and allocations — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check are
intended; they are collected in section 6.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related documents:** [../specs/team-schedule.md](../specs/team-schedule.md) (the schedule that also
shows allocated engineers), [accounts.md](accounts.md).

---

## 1. Purpose

A **customer engagement** is a piece of planned work for a customer, such as consulting or
an urgent "firefighting" effort. WSO2's resource planning system decides which engineer is
**allocated** to an engagement for which dates. This service:

1. takes allocation events from that planning system and keeps engagements and allocations
   up to date (section 3);
2. lets internal staff look at allocations and lets an allocated engineer post a weekly
   update (section 4).

Customers never see any of it.

## 2. Who can use it

**PRJ-510** Receiving allocation events shall be possible **only for allow-listed internal
client credentials**. Any other caller, including a signed-in person, shall be refused as
unauthorised.
*Check:* post an event with a user token; refused.

**PRJ-511** Looking at allocations and their updates shall be possible for **internal staff
only**. Posting an update is further limited to the allocated engineer (4.3).

## 3. Receiving allocation events

An **allocation event** says: this engineer is allocated, for these dates, to this
engagement, with this clearance status. The planning system sends one each time an
allocation is added or changed.

### 3.1 What the service checks first

**PRJ-520** The event shall carry an allocation id, the engineer's email and an allocation
type greater than zero. Otherwise the event is refused as invalid so the sender does not
retry it.

**PRJ-521** Start and end dates, when present, shall be dates (`YYYY-MM-DD`; any time after
the date is ignored). Text values that are too long for their field shall be refused with
the field named. Fields the service does not know are ignored.

**PRJ-522** The request shall be one JSON object of a limited size.

**PRJ-523** The clearance status shall be converted to the allocation's state:

| Clearance status | State |
|---|---|
| Tentative | Tentative |
| Ready for Clearance | Ready for clearance |
| Confirmed | Confirmed |
| Clearance In Progress | Clearance in progress |
| Allocation Cancelled | Allocation cancelled |
| Accepted by Consultant | Accepted by consultant |
| Rejected - Other | Rejected (other) |
| Rejected by Consultant | Rejected by consultant |
| Confirmed - Visa Pending | Confirmed, visa pending |
| Rejected - Visa Issues | Rejected, visa issues |

A status that is not in the list shall not set a state.

### 3.2 Finding or creating the engagement

**PRJ-530** An event with no engagement details shall be skipped with the reason "no
engagement details". A skipped event is **not an error**: the sender receives success so it
does not retry.

**PRJ-531** There are two kinds of allocation. An allocation of type **76 or 83** is a
**firefighting** allocation; every other type is a **line item** allocation.

**PRJ-532** A line item allocation shall be attached to the engagement that has the event's
product (line item) id. If there is no product id, or no such engagement, the event shall be
skipped ("no engagement for line item"). The service shall **not** create an engagement for
it.

**PRJ-533** A firefighting allocation shall be attached to the engagement with the event's
engagement id. If none exists, the service shall **create one**, if the service has been
configured to allow creating firefighting engagements; otherwise the event shall be skipped
("firefighting engagement type id not configured"). An event with no engagement id shall be
skipped ("no engagement id").

**PRJ-534** To create a firefighting engagement the service shall find the customer's
account: first by the customer code (matched to the account's Salesforce id), otherwise by
the customer **name**, preferring accounts that are still live. If no account matches, the
event shall be skipped ("account not found"); if several match, it shall be skipped
("ambiguous account name").

**PRJ-535** The created engagement shall be of the type **firefighting**, named
"*customer* - *allocation type*" (just the allocation type when no customer name was sent;
at most 200 characters), flagged paid when the engagement type name is "Paid", with a
delivery mode of **offsite** or **onsite** worked out from the engagement nature (anything
else leaves it empty), and with the allocation's dates as the planned start and end.

### 3.3 Recording the allocation

**PRJ-540** If the allocation id is already known, its dates, times, time zone and state
shall be updated, and the result shall be "updated".

**PRJ-541** Otherwise the service shall find the engineer by email (or user name). If there
is no such user, the event shall be skipped ("user not found"). **An engagement created
earlier in the same event is kept.**

**PRJ-542** Otherwise a new allocation shall be created and the result shall be "created".

**PRJ-543** The whole event shall be handled as one unit: if any step fails, nothing from
that event is saved.

**PRJ-544** The reply shall say what happened ("created", "updated" or "skipped" with a
reason), and the engagement id, whether the engagement was created, and the allocation id
when there are any.

## 4. Looking at allocations

### 4.1 Searching and reading

**PRJ-550** Internal staff shall be able to search allocations with paging (20 per page, at
most 50) and see each one's dates, times, state, utilisation, the engineer, the customer
account and the engagement (its reference, name, type, delivery mode, state, business unit and
whether it is paid).

**PRJ-551** Filters: free text, account name, account id, engineer id, engagement id, one or
more states, and a date range ("from" and "to"). Ids shall be valid UUIDs, text at most 200
characters, states one of the ten in PRJ-523, and "to" shall not be before "from".

**PRJ-552** Results shall be sortable by start date, end date, state, allocation id, account
name, engineer name or utilisation, ascending or descending. Any other field shall be
refused.

**PRJ-553** One allocation shall be returned by its id; an unknown id shall be "not found".

### 4.2 Updates on an allocation

**PRJ-560** Internal staff shall be able to list the updates posted on an allocation, with
paging. An unknown allocation shall be "not found", not an empty list.

### 4.3 Posting an update

**PRJ-570** The engineer an allocation is for shall be able to post an update on it. It
needs a subject (at most 512 characters) and content (at most 8,000 characters); blank text
shall be refused.

**PRJ-571** Only the **allocated engineer** may post. Anyone else, or a call with no
signed-in person, shall be refused with "only the engineer allocated can post an update on
this allocation". The person is identified by the email of the signed-in user, matched to the
platform user; the identity provider's own user id is never compared with a platform id.

**PRJ-572** A posted update shall be **published** and **internal**, of **weekly** frequency
and for the **individual**, and shall belong to the week it was posted in, counted from the
Monday (UTC).

## 5. Related schedule views

The team schedule has its own, separate list of engagement allocations and a way to dismiss
one; see [../specs/team-schedule.md](../specs/team-schedule.md).

## 6. Open questions for the product owner

1. **Allocation types 76 and 83 are fixed in the code.** They decide what counts as
   firefighting. Who owns the list if the planning system adds a type?
2. **Name-based account matching.** A firefighting engagement for a customer with no code is
   matched on the account **name**, which the planning system may spell differently. Such
   events are skipped; who sees the skip?
3. **An engagement can be created and the allocation still skipped** (engineer not found).
   The engagement stays, with no allocation. Is that intended?
4. **No way to remove an allocation.** A cancelled allocation is kept with its state
   changed, and no route deletes one. Intended?
5. **Updates cannot be edited or removed**, and only the allocated engineer can add one.
   If the engineer changes, the previous engineer's updates stay with them.

## 7. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Event handling | `internal/service/customer_engagement_allocation_service.go`, `internal/handler/customer_engagement_allocation_handler.go`, `internal/domain/customer_engagement_allocation.go` |
| Allocation list and updates | `internal/service/engagement_allocation_view_service.go`, `internal/repository/engagement_allocation_view_repo.go` |
| Settings | `AUTH_INTERNAL_CLIENT_IDS` (who may send events), `CUSTOMER_ENGAGEMENT_FIREFIGHTING_TYPE_ID` (switches on creating firefighting engagements) |
| Routes | `POST /customer-engagements/allocation-events`, `POST /customer-engagements/allocations/search`, `GET /customer-engagements/allocations/{id}`, `GET`/`POST /customer-engagements/allocations/{id}/updates` |
| Tests | `customer_engagement_allocation_service_test.go`, `customer_engagement_allocation_handler_test.go`, `engagement_allocation_view_service_test.go`, `engagement_allocation_view_handler_test.go` |
