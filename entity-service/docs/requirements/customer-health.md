# Customer health: project risks and action items — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check are
intended; they are collected in section 9.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

---

## 1. Purpose

CS engineers keep track of how each customer project is doing. A project can be marked **at risk**,
with the reason written down, and the team then records **action items** to bring it back. When the
risk has been dealt with it is **closed**, and someone later confirms the project is **healthy**.
The account's overall health is worked out from its projects.

This is internal, CS-only data. Customers never see it. The CSM portal's Customer Health dashboard
draws it.

## 2. Who is involved

| Who | What they do |
|---|---|
| CS engineer | Opens and closes risks, confirms health, records action items and comments. |
| Action item assignee | The person named on an action item. |

**CUS-801** Every route in this feature shall be available to **internal** callers only. A caller who
is not internal, a customer included, is refused. The data has no row-level protection of its own, so
this check is the only barrier.

**CUS-802** Changes that record who made them (opening and closing a risk, confirming health, creating
an item, changing its status, commenting) need the signed-in person's identity from their token. Without
it the request is refused, and the person's email is recorded.

## 3. Health of a project

A project has one health status: **To be reviewed**, **Healthy** or **At risk**. A project has **no
status at all** until tracking is started for it.

**CUS-810** The service shall let a person start health tracking for some of an account's projects.
Each given project gets the status **To be reviewed**. A project that already has a status is left
alone. A project that does not exist, or does not belong to that account, is refused, and at least one
project is required.

**CUS-811** The service shall let a person list an account's projects with their status, and, for each
project that is **at risk**, its open risk.

**CUS-812** An account's **overall** health shall be: **at risk** if any of its projects is at risk;
otherwise **healthy** if every tracked project is healthy; otherwise **to be reviewed**. An account with
no tracked projects reads as to be reviewed.
*Check:* one healthy project and one to-be-reviewed project make the account "to be reviewed"; add an
at-risk one and it becomes "at risk".

**CUS-813** The service shall give the overall health of one account, of many accounts in one request, and
the ids of every account whose overall health is a chosen value. An account that has never had tracking
started appears in none of the three lists. A chosen value other than the three is refused.

**CUS-814** The filter by health and the status shown for an account shall agree, because both come
from the same calculation over the projects' statuses. The one exception is an account that has never had
tracking started: its status reads as **to be reviewed**, but it appears in none of the three lists (CUS-813),
so filtering by "to be reviewed" does not return it.

## 4. Risks

A **risk** is a written statement that a project is in trouble. It is **open** until it is closed.

**CUS-820** The service shall let a person open a risk on a project. A comment is required and the project
must exist. The project's status becomes **At risk** and who set it and when are recorded.

**CUS-821** A project may have **only one open risk** at a time. Opening a second is refused as a conflict
naming the open one. Two people opening a risk at the same moment cannot both succeed.

**CUS-822** The service shall let a person close a risk. A comment is required. The risk must be open; a
risk that is not is refused. The risk's closing comment, person and time are recorded.

**CUS-823** A risk **cannot be closed through the close-risk action while any of its action items is still
open or in progress**. The refusal says how many are. (Confirming a project healthy, CUS-825, closes a risk
without this check.) Items that are resolved or cancelled do not block it.
*Check:* close a risk with one open item; refused with "1 action item(s) are still open".

**CUS-824** Closing a risk shall set the project's status back to **To be reviewed** (not Healthy) and clear
who reviewed it, so someone must confirm it again.

**CUS-825** The service shall let a person confirm a project **healthy**. This shall set its status to
**Healthy** with the reviewer and time, close any risk still open on it (with the comment given, or
"Reviewed and confirmed healthy" if none) **even when that risk still has open or in-progress action items**,
which are left as they are, and leave a closed entry in its risk history saying it was
marked healthy. The project must exist, but need not have been tracked before.

**CUS-826** The service shall let a person revert a project to **To be reviewed**. A project with no status
is "not found".

**CUS-827** The service shall let a person read a project's risk history: every risk with its action items.

**CUS-828** Opening a risk, closing one, and confirming health shall each be saved in one step, so a risk and
a project's status never disagree.

## 5. Action items

An action item is a follow-up on a risk, with a **priority** (high, medium or low) and a **status**:
**Open**, **In progress**, **Resolved** or **Cancelled**.

**CUS-840** The service shall let a person add an action item to an **open** risk. A title and a priority are
required; the description, assignee's email and due date are optional. The due date, when given, is a plain
date ("YYYY-MM-DD"). The item starts as **Open**. Adding one to a risk that is not open is refused as a
conflict.

**CUS-841** The service shall let a person change an item's status. **Resolved** and **Cancelled** need a
resolution comment and record who resolved it and when. Moving an item back to **Open** or **In progress**
is allowed only while its risk is still open.

**CUS-842** The service shall let a person edit an item's title, description, priority, assignee and due
date, but **only while it is Open or In progress**. A finished item cannot be edited.

**CUS-843** The service shall list the items of one risk, optionally by status, and the items of one account
(optionally for one project and one status), each with its comment count. The account list is newest
first and covers only items on **open** risks.

**CUS-844** The status filter may be written in any letter case; an unknown status is refused.

## 6. Comments on action items

**CUS-850** The service shall let a person comment on an action item; the comment must not be empty. An unknown
item is "not found". Comments are recorded with the commenter and time.

**CUS-851** The service shall list an item's comments.

## 7. Consistency under concurrent use

**CUS-860** Changes that decide whether a project "already has an open risk", or whether a risk can still be
closed, shall be handled one after the other for the same project or risk, so no project ends with two open
risks, and no item is added to a risk that has just closed.

## 8. What this covers

How each account's health is shown on the account page (the account health detail and the account search by
health) belongs to the accounts requirements.

## 9. Open questions

1. **Confirming health always adds a closed entry to the risk history,** even if the project was already
   healthy and had no risk. Repeated confirmations will fill the history with "Marked as healthy" entries.
2. **Reverting health and editing an action item record nobody.** Only the other changes store the person's
   email.
3. **Closing a risk resets to "to be reviewed", but confirming healthy does not require a prior review.** A
   project can go straight to healthy from any state. Is that the intended review flow?
4. **An account with no tracked projects reads as "to be reviewed" in its overall health,** yet appears in
   none of the by-health lists. The two views disagree for such accounts.
5. **Confirming health closes a risk that still has open action items.** Closing through the close-risk
   action refuses in that case; confirming health does not, and the items stay open on the closed risk. Should
   both paths follow the same rule?
6. **The account action-item list hides items on closed risks,** so a resolved piece of work on a closed
   risk can only be found through the project's risk history.

## 10. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Rules, validation | `internal/service/customer_health_service.go` |
| Risks, status, items, comments, locking | `internal/repository/customer_health_repo.go` |
| Routes | `POST /projects/{id}/risk`, `PUT /risks/{id}/close`, `POST /projects/{id}/mark-healthy`, `POST /projects/{id}/revert-health`, `GET /projects/{id}/risk-history`, `/accounts/{id}/project-health-statuses`, `/accounts/{id}/health-summary`, `/accounts/health-summaries/search`, `/accounts/by-health-status/search`, `/accounts/{id}/init-health-tracking`, `/risks/{id}/action-items`, `/action-items/{id}`, `/action-items/{id}/status`, `/action-items/{id}/comments`, `/accounts/{id}/action-items` |
| Access check | `internalOnly` on every route in `internal/server/routes.go` |
| Tables | migrations `0219`, `0222` |
| Tests | `customer_health_repo_integration_test.go` |
