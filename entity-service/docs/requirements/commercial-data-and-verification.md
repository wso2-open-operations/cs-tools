# Commercial data and pending verification — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check are
intended; they are collected in section 6.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

---

## 1. Purpose

Two unrelated features that portal screens depend on:

- **Commercial data**: the opportunities, invoices and project links that sales records hold, shown
  to CS engineers beside an account or project. The service only reads them; they arrive from the
  sales system through the ingest described in the integration requirements.
- **Pending verification**: a list of closed cases that a customer has yet to confirm are really
  resolved. A case goes onto the list when it is closed (automatically, or by hand), and comes off
  when someone verifies it.

## 2. Who is involved

| Who | What they do |
|---|---|
| CS engineer | Reads commercial data; adds closed cases to the verification list; verifies them. |
| Customer user | Adds a closed case to the list by hand, and verifies cases, in projects they belong to and that have the feature on. |
| Internal services | Add a case to the list automatically (as auto-closed) when it is closed while waiting on the customer. |

## 3. Commercial data (read only)

**CUS-901** The service shall let an **internal** caller search **opportunities**, optionally for one
account, in pages. Each opportunity shows its name, account, stage, the licence agreement version it was
sold under (also as a decimal), and its sales-system id.

**CUS-902** The service shall let an internal caller open one opportunity by id.

**CUS-903** The service shall let an internal caller search **invoices**, optionally for one opportunity,
in pages. Each invoice shows its name, amount, invoice date, due date, original due date, paid date,
classification, opportunity and sales-system id. Dates and the amount are shown as received.

**CUS-904** The service shall let an internal caller open one invoice by id.

**CUS-905** The service shall let an internal caller search the **links between projects and opportunities**,
by project, by opportunity, or both, in pages. This is how a project is tied to what was sold.

**CUS-906** A caller who is not internal, a customer included, is refused for all of the above. An id given
as a filter or in the address must be a valid UUID, or the request is refused as a bad request.

**CUS-907** Pages follow the service-wide rules (a limit above the maximum is refused), and every search
answer carries the total, the limit, the offset and whether there are more.

**CUS-908** Opportunities, invoices and links cannot be created, changed or deleted through the service.

## 4. Pending verification

A **pending verification** entry ties one **closed case** to a verification step. It has a reason it was
added (**auto-closed** or **manual**), and, for an auto-closed one, the status the case was in before
it closed (**Solution Proposed** or **Awaiting Info**).

### Adding

**CUS-910** The service shall let a person add a case to the list. Only **cases** can be added; any other
kind of record is refused.

**CUS-911** The case must be **closed**. Adding an open case is refused.

**CUS-912** The reason must be auto-closed or manual. For **auto-closed** the earlier status is required;
for **manual** it must **not** be given; and it must be Solution Proposed or Awaiting Info when given.

**CUS-913** Only an **internal** caller may add an **auto-closed** entry, because it claims the system
closed the case. A customer can add only **manual** entries.
*Check:* a customer adding an auto-closed entry is refused.

**CUS-914** A case that cannot be seen by the caller is treated as not existing, so nobody learns of it.

**CUS-915** Adding is refused when the case's **project has the feature switched off**, even for a
project member. The same applies to searching and verifying.

**CUS-916** A case may have only **one unverified** entry at a time. Adding it again is refused as a
conflict. Once verified, it can be added again later, and each round is kept.

**CUS-917** The person who added the entry is the **signed-in caller**. It cannot be set in the request, so
nobody can add an entry in another person's name.

### Searching

**CUS-920** The service shall let a person list the entries of **one project** (the project id is required and must be valid), in pages, with:
the entries, the total, the number added automatically and by hand, a count per record type, and whether
there are more.

**CUS-921** By default the list shows only **unverified** entries. It can show verified and unverified
together (a case's full history), or **verified only**, which shows **one entry per case**: its most recent
round, so a case verified, re-added and verified again is not listed twice.

**CUS-922** The list can be narrowed to one reason, to one case, and by text matching the case's number,
subject or description (ignoring letter case, wildcard characters taken literally; at most 200 characters).
The record-type filter accepts only **case**; any other type is refused.

**CUS-923** The counts shall reflect every active filter except the record-type filter itself.

### Verifying

**CUS-930** The service shall let a person mark an entry **verified**, recording who and when. An entry that
does not exist or is already verified is "not found, or already verified". The answer confirms the case has
been removed from the person's list.

**CUS-931** The verifier is the signed-in caller; it cannot be set in the request.

### Who can see what

**CUS-935** A person shall see and change only entries for cases in projects they belong to. Internal callers
see all. Both are enforced in the database as well as in the service.

## 5. Features available only with the legacy case system

**CUS-940** Case **tasks** (listing a case's tasks, creating one, searching tasks, opening one, changing one)
are not available unless the service is running on the legacy data source. On the service's own database
(including the mode where both are written) every task request is refused with a "service unavailable"
answer saying tasks are only supported for the legacy data source, instead of a "not found".

## 6. Open questions

1. **Any customer can verify any entry in a project they belong to,** not only the one who closed or
   raised the case. Is that intended?
2. **The record-type filter is left in the request but cannot change the result.** The list is always
   cases only. Callers that still send other types now get an error.
3. **Commercial data is internal only but not scoped further.** Any internal person can read every
   account's opportunities and invoices. Is a finer rule needed, for example by account team?
4. **Tasks are unavailable on the service's own database.** Is a replacement planned, or should the
   feature be retired?
5. **An automatic entry is attributed to whoever calls the service** (an internal service or person), as
   for a manual one. Confirm that is what the verification screens should show.

## 7. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Commercial data reads | `internal/service/sf_read_service.go`, `internal/repository/sf_invoice_repo.go`, `sf_opportunity_repo.go`, `sf_opportunity_link_repo.go` |
| Commercial data routes | `POST /opportunities/search`, `GET /opportunities/{id}`, `POST /invoices/search`, `GET /invoices/{id}`, `POST /project-opportunity-links/search` |
| Pending verification | `internal/service/pending_verification_service.go`, `internal/repository/pending_verification_repo.go` |
| Pending verification routes | `POST /pending-verifications`, `POST /pending-verifications/search`, `PATCH /pending-verifications/{id}` |
| Tables and access | migrations `0176`, `0178`, `0191`, `100025` |
| Tasks | `internal/service/task_service.go` |
| Tests | `pending_verification_service_test.go` |
