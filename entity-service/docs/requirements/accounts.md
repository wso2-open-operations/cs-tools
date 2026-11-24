# Accounts — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check are
intended; they are collected in section 8.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related documents:** [projects.md](projects.md) (an account owns one or more projects).

**Not covered here:** how accounts arrive from the sales system (the Salesforce ingest),
and the customer health *risk tracking* actions (opening a risk, marking an account
healthy). Both are described with the features they belong to.

---

## 1. Purpose

An **account** is a customer organisation (or a partner). It records who looks after the
customer inside WSO2 (the account manager, the technical owner, the renewal account
manager, the customer success manager), which support teams cover it, where it is, whether
it is a partner, and the AI assistant settings that apply to all its projects. A project
belongs to at most one account.

Accounts are an **internal staff** concept. Customers see their account only through the
projects they belong to.

## 2. Who can use accounts

**PRJ-110** Every account route (read one, search, update teams, health detail, contacts)
shall be available to **internal callers only**: WSO2 staff and internal services acting
for them. Any other caller shall be refused.
*Check:* call the account search with a customer's token; refused.

## 3. Reading and searching accounts

**PRJ-120** The service shall return one account by id with its name, number,
classification, pod, region, country, city, document drive location, the four named
people (technical owner, account manager, renewal account manager, customer success
manager), the CRE and SRE teams, activation and deactivation dates, the AI assistant
settings, partner information, life cycle, and when it was created and last updated.
An unknown id shall be "not found"; an id that is not a UUID shall be refused.

**PRJ-121** An account shall be flagged as a **partner** when its classification is
"Partner". It shall also say whether it has a primary partner.

**PRJ-122** The service shall search accounts with paging (20 per page by default, at most
50, search text at most 200 characters), returning a total and a "has more" flag.

**PRJ-123** Account search shall hide accounts that were deleted in the sales system. They
remain readable by id, so projects and cases that point to them keep working.
*Check:* an account deleted at source does not appear in the search but still opens by id.

**PRJ-124** Text search shall match the account name, its Salesforce id or its account
number (for example to find an account from its number), ignoring letter case.

**PRJ-125** The search shall support these filters, all combining (AND):

| Filter | Meaning |
|---|---|
| Active | Active means "has no deactivation date"; inactive means it has one |
| Pod | The account's pod, exact match |
| Owner email | The person is the technical owner, the account manager **or** the renewal account manager (any one of the three), ignoring letter case |
| Classification | The account's classification, exact match |
| Life cycle | For example Customer, Partner, Prospect, exact match. A different dimension from classification |
| Has a project | The account has at least one project |
| Sub region | The account's sub region is any of the listed values |
| Product | The account has an active deployed product with this product name, on any of its projects' deployments, ignoring letter case |
| Team | The name of the account's CRE or SRE team, ignoring letter case |

## 4. Account health

An account can show a **health summary**: seven flags and an overall status. Searching by
risk or by health status, or asking for the summary, includes it in the result.

**PRJ-130** The summary shall contain these flags, each worked out as follows:

| Flag | True when |
|---|---|
| Recent cases | The account had a case, service request, engagement or security report analysis created in the last 180 days. Announcements do not count |
| No support cases in 6 months | The opposite of "recent cases" |
| Recent escalations | An escalation was recorded on any of the account's items in the last 90 days |
| Migration delays | An engagement of type migration is not closed and was created 60 days ago or earlier |
| Abandoned migrations | The same, but created 180 days ago or earlier. Every abandoned migration is also delayed |
| End-of-life product | An active deployed product, on any of the account's deployments (active or not), runs a product version whose support end date has passed |
| Go-live status | See PRJ-131 |

**PRJ-131** The **go-live status** of a project shall be:
- *Has gone live* if the project has a go-live date, or the account was activated before
  1 January 2024;
- otherwise *pending* while the account has no activation date, or the date is in the
  future, or fewer than 6 whole months have passed since activation;
- otherwise *failure* ("hasn't gone live in 6 months") once 6 or more whole months have
  passed.
Months are counted as days divided by 30.4375, rounded down. For the account as a whole the
status is "gone live" unless some project has not gone live, in which case it follows the
activation-date result above.
*Check:* an account activated in the past 2 weeks with no go-live shows "Recently
activated (less than a month)".

**PRJ-132** While pending, the status shall be worded to say how long: "N/A (No Start
Date)", "N/A (Activation Date in Future)", "Recently Activated (Less than a month)",
"Hasn't Gone Live Since 1 month", or "Hasn't Gone Live Since N months".

**PRJ-133** The **overall health status** shall be "at risk" if any of the account's
projects is at risk; otherwise "healthy" if every project is healthy; otherwise "to be
reviewed". An account with no project health records shall read as "to be reviewed" in the
summary but shall match none of the three values when searching by health status.

**PRJ-134** The search shall let staff filter by one or more **risks** (an account matching
any of them is kept): no go-live, has gone live, recent cases, no support cases in 6
months, end-of-life product, abandoned migrations, migration delays, recent escalations.
An unknown risk name shall match nothing; blank entries are ignored.

**PRJ-135** The service shall return an account's **health detail**: for each project of
the account, newest first, the go-live status and the items behind each flag.
- *Recent cases*: the cases created in the last 180 days, grouped by priority (a case with
  no severity is "Unspecified"), newest first.
- *Migrations*: the delayed and abandoned migration engagements.
- *Escalations*: every escalation on the project's items, newest first.
- *Deployments*: the project's active deployments, newest first.
- *End-of-life software*: one entry per product version that is past its support end date,
  listing the distinct deployments running it.
An account that does not exist shall be "not found".
**Confirm**, see 8.2.

## 5. Changing an account's support teams

**PRJ-140** The service shall let staff set an account's **CRE team** and/or **SRE team**.
At least one shall be given, and each shall be the id of an existing team. An unknown team
shall be refused with a message that names the problem. An unknown account shall be "not
found".

**PRJ-141** A team already set cannot be cleared; a request cannot say "no team". Sending
only one of the two leaves the other as it was.

**PRJ-142** The account's "last updated" time shall change. The person who made the change
is **not** recorded. **Confirm**, see 8.1.

The CRE team's leads are the people told first when a case on the account is escalated
(see [case-escalation.md](case-escalation.md)).

## 6. Account contacts

The same list is also described from the contacts side in
[tags-watchers-contacts.md](tags-watchers-contacts.md) (section 5, COL-340).

**PRJ-150** The service shall list an account's contacts with paging, newest first, each
with a name, an email and whether they are the primary contact.

**PRJ-151** A contact shall be shown with the name and email of the matching user (matched
on user name, ignoring letter case). A contact with no matching user shall show its user
name in both fields.

**PRJ-152** Text search shall match the contact's user name, email or name.

**PRJ-153** The list shall include contacts that are no longer active. **Confirm**, see 8.3.

## 7. AI assistant settings

The AI assistant ("has agent") and knowledge suggestions ("has knowledge references")
switches are stored on the account and shown on every read. They are changed through the
project update (see [projects.md](projects.md), section 7), and so affect every project of
the account.

## 8. Open questions for the product owner

1. **Who changed the teams?** Changing an account's teams records the time but not the
   person, unlike most other changes.
2. **Health detail is not checked against the person.** It is staff-only, which is the only
   protection. Is that enough for what it reveals about each customer?
3. **Inactive contacts are listed.** The list shows contacts regardless of whether they are
   still active, and does not show which are inactive. Should it?
4. **Active means "no deactivation date".** An account with a future deactivation date
   counts as inactive today. Is that intended?
5. **Go-live cut-off date.** Accounts activated before 1 January 2024 always count as gone
   live. The date is fixed in the code. Who owns changing it?

## 9. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Read, search, update teams | `internal/service/account_service.go`, `internal/repository/account_repo.go` |
| Health summary and risks | `internal/repository/account_health_search.go` |
| Health detail | `internal/repository/account_health_detail.go` |
| Contacts | `internal/service/account_contact_service.go`, `internal/repository/account_contact_repo.go` |
| Internal-only check | `internal/server/internal_only.go`, `internal/server/routes.go` |
| Routes | `GET /accounts/{id}`, `POST /accounts/search`, `PATCH /accounts/{id}`, `GET /accounts/{id}/health-detail`, `POST /accounts/{id}/contacts/search` |
| Tests | `account_service_test.go`, `account_health_summary_test.go`, `account_health_search_test.go`, `account_health_detail_test.go`, `account_repo_lifecycle_integration_test.go` |
