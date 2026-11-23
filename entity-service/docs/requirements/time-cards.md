# Time cards — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended;
they are collected in section 10.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

Related: [case-search.md](case-search.md) and [dashboard-reports.md](dashboard-reports.md).
Project-level time card figures (a project's time card stats and report) are served by
the project dashboard routes and are not described here.

---

## 1. Purpose

CS engineers record the time they spend on a case as **time cards**. A card says who
worked, on which case, on what day, how many minutes on each kind of work, and whether
it is billable. A card is then approved or rejected by a lead. Dashboards count cards and
add up their minutes.

## 2. Who is involved

| Who | What they do |
|---|---|
| CS engineer (submitter) | Creates, edits and deletes their own cards while they are submitted. |
| Approver | A person named on the card, or an admin. Approves or rejects a submitted card. |
| Customer user | Can see the cards on cases of their own projects (read only). |
| Internal service | Needs a signed-in person to read or change cards; a machine caller alone is refused. |

## 3. Card states

A card is in one of six states: **pending, submitted, approved, rejected, processed,
recalled.**

**CSR-200** A new card shall start as **submitted**.

**CSR-201** Only two changes of state are possible through this service: **submitted to
approved** and **submitted to rejected**. A card that is no longer submitted cannot be
approved or rejected again.

**CSR-202** The states pending, processed and recalled can be searched and counted but are
never set by this service; they come from elsewhere.

## 4. Creating a card

Creating, editing and deleting are **internal only**: a customer is refused.

**CSR-210** The submitter shall be the signed-in person, never a name taken from the
request. A request without a signed-in person is refused.

**CSR-211** A card shall name a case, a work date (as YYYY-MM-DD), and the minutes spent.
The minutes are recorded under five headings: **analyzing, setting up, reproducing or
debugging, providing a solution, patching.**

**CSR-212** Minutes shall not be negative, and each heading may carry at most 480.
The five together shall not exceed **480 minutes (8 hours) per card**. The limit is per
card, not per day: a person may log several cards on the same day, each judged on its own.
*Check:* a card with 300 + 200 minutes is refused with a message about the 8-hour limit;
two cards of 300 minutes on one day are both accepted.

**CSR-213** A card may only be logged against a case-like item (an ordinary case, service
request, security report analysis, announcement or engagement). A change request or
incident id is refused as "case not found".

**CSR-214** If the card names a project, it must be the case's own project, otherwise it
is refused. If no project is named, the card has none (it is not filled in from the case).
This applies to the native data source. When the legacy case system is the data source, a
create request without a project is refused, and it must also name at least one approver.

**CSR-215** A case the caller is not allowed to see shall be reported as "case not found",
never as forbidden, so its existence is not revealed.

**CSR-216** The card may carry a billable flag, an **issue complexity** and a free-text work
log comment. The issue complexity values shown to people are **N/A, Low, Medium, High**.
A card with no complexity is allowed.

**CSR-217** A card may list **approvers**. The submitter shall not be one of them: "you
cannot be your own time card approver". Approver ids that do not exist are refused.

**CSR-218** All ids on the request shall be valid ids, otherwise the request is refused.

**CSR-219** A successful create shall return the new card with a confirmation message.

## 5. Editing a card

**CSR-220** Only the submitter may edit a card, and only while it is submitted. Any other
attempt is refused as a conflict ("not editable: it may not exist, may not belong to you,
or is no longer in the submitted state") without saying which.

**CSR-221** The editable parts are the date, billable flag, issue complexity, work log
comment, the five minute headings and the list of approvers. A date shall still be
YYYY-MM-DD.

**CSR-222** Editing minutes shall re-check the **8-hour limit against the whole card**: the
new values are combined with the ones not changed, and the total shall not exceed 480.
*Check:* a card of 400 minutes edited to add 100 more is refused.

**CSR-223** Replacing the approvers shall replace the whole list. The submitter still may
not be on it.

**CSR-224** Two edits at the same moment shall be handled one after the other, so the limit
cannot be passed by editing at the same time.

## 6. Approving or rejecting

**CSR-230** A card may be approved or rejected by a person named as an approver on that card,
or by an **admin**. The submitter can never decide their own card, even if they are an
admin or are named as an approver.
*Check:* the submitter approves their own card; refused.

**CSR-231** The card must be **submitted**. Approving or rejecting a card in any other
state is refused as a conflict.

**CSR-232** Rejecting needs a **comment** (the lead's reason). Approving does not.

**CSR-233** Approving shall record who approved it. A rejection's comment is shown on the
card as its rejection reason.

**CSR-234** If a request carries a state and also field changes, the native data source
validates the ids it was given, makes the state change and ignores the field changes. When
the legacy case system is the data source, such a request is refused. **Confirm** (see 10.2).

**CSR-235** Only "approved" and "rejected" are accepted as a state to move to.

**CSR-236** After an approval or rejection the service shall refresh the project's
allocated, consumed and remaining hours, and those of the other projects that share an
opportunity with it, because the hours pool is shared. If this fails the decision still
stands, the problem is logged, and the regular hourly refresh corrects the figures. The
refresh is skipped when the card has no project, or when no refresh is configured, so an
immediate refresh is not guaranteed for every decision.

## 7. Deleting a card

**CSR-240** Only the submitter may delete a card, and only while it is submitted. Anything
else is refused as a conflict, without saying whether the card exists.

**CSR-241** A deleted card is removed completely.

## 8. Searching cards

Searching is open to any signed-in person but only returns cards they may see.

**CSR-250** A search shall need a signed-in person; a request without one is refused.

**CSR-251** An internal user shall see all cards. A customer shall see only the cards on
cases of projects they are a registered contact of.

**CSR-252** The search shall accept these filters, all of which must hold together:
project ids, a case, a submitter, any of several submitters, teams, an eligible approver,
who approved, a work-date range and states.

**CSR-253** The **team** filter shall match cards submitted by the **current** members of
any of the teams, even if the person was in another team when they logged the card. A
filter may name at most 100 teams. This filter exists only on the native data source: when
the legacy case system is the data source, a search that names any team is refused.

**CSR-254** The **approver** filter shall find the cards the person is eligible to decide,
and shall always leave out the person's own cards.

**CSR-255** Ids shall be valid, dates YYYY-MM-DD, and states known values; otherwise the
request is refused. A date range uses the **work date**, both ends included.

**CSR-256** Results shall be sorted by last update, newest first, unless the caller chooses
to sort by work date or by last update, ascending or descending. Any other field or
direction is refused. Ties are broken in a fixed order.

**CSR-257** The page size defaults to 20 and shall never exceed 50.

**CSR-258** Each returned card shall carry: id, total minutes, work date, billable flag,
the five minute headings, issue complexity, work log comment, rejection reason, state,
approvers, the submitter, who approved it, the project and the case (number and title).
Minutes that were never filled in shall read as zero. The state is shown in lower case and
the issue complexity as N/A, Low, Medium or High.

**CSR-259** An older field, "created on", is kept and carries the same value as the work
date until callers stop using it. It is not the time the record was made.

## 9. Counting and adding up cards

**CSR-260** The service shall group the cards a search would find and give the **count and
the total minutes** per group. The groupings are **state, user, project, day** and
**team**. This is available only on the native data source: when the legacy case system is
the data source, every grouping is refused.

**CSR-261** The groups shall be ranked by count (the default) or by minutes, the top ten
shown unless the caller chooses another number, and all the rest added together into
"others". The totals shall equal the shown groups plus "others". A negative number of groups
is refused.
*Check:* 14 projects, top 10 shown; "others" holds the other 4 projects' cards and minutes.

**CSR-262** A card with **no project** has no project group; it is counted in "others", so
every grouping except *by team* gives the same totals (see CSR-264 for why team totals can
differ).

**CSR-263** *By day* needs both a start and an end date, the end not before the start and the
window no longer than **31 days**. It returns one group per day, in date order, **including
days with no cards at zero**. Ranking and the number of groups do not apply.

**CSR-264** *By team* needs the teams to be named. It returns one group per named team that
exists, labelled with the team's name, a team with no cards at zero. A team id that is valid
but names no team gets no group. A card counts once in **every
named team its engineer is currently in**, so the groups can add up to more than the
number of cards. Without a chosen number of groups every named team is shown.

**CSR-265** *By user* labels a group with the person's name, or else their email, or else
their user name. *By state* shows states in lower case.

**CSR-266** A request with no grouping, an unknown grouping or an unknown measure is
refused.

**CSR-267** The service shall also give, per **case**, a summary of the cards matching the
same filters: the case, its project, the total minutes and count, and separately the
billable and non-billable minutes and counts. Cases are ordered by the case's last
update, newest first, and paged like other lists. The count of results is the number of
cases, not cards.

## 10. Open questions for the product owner

1. **Customers can read time cards.** A customer sees the cards, with the engineer's name,
   minutes, work log comment and rejection reason, on cases of their projects. Is that
   intended, or are time cards internal?
2. **State and edits in one request.** On the native data source the state change wins and
   the edits are silently dropped; the legacy case system refuses such a request. Should the
   native data source refuse it too? (CSR-234)
3. **Admins can approve any card** (except their own), not only named approvers. Is this
   "approve by exception" still wanted?
4. **Unknown issue complexity.** A value other than N/A, Low, Medium or High is not checked
   by the service. It fails inside the database with an unhelpful error rather than a clear
   message. Should it be refused up front?
5. **The 8-hour limit is per card.** A person can log many cards in a day. Is there a daily
   limit intended?
6. **Processed and recalled** are never set here. Who sets them, and should this service
   explain them?
7. **Who may be an approver** is not checked: any existing user (other than the submitter)
   can be named, including a customer user. Should approvers be limited to internal
   staff?

## 11. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Validation, approval rules, side effects | `internal/service/time_card_service.go` |
| Create, edit, approve, delete, search, counts | `internal/repository/time_card_repo.go` |
| Request and response shapes, limits | `internal/domain/entity.go` |
| Who sees which cards | row-level security migration for time cards |
| Internal-only gate | `internal/server/internal_only.go` |
| Routes | `POST /time-cards/search`, `POST /time-cards/aggregate`, `POST /cases/time-cards/search`, `POST /time-cards`, `PATCH /time-cards/{id}`, `DELETE /time-cards/{id}` |
| Tests | `time_card_service_test.go`, `time_card_repo_test.go` |
