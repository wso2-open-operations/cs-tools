# Salesforce ingest — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 15.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

Related: [event-publishing.md](event-publishing.md) (the invitation and welcome events this
ingest publishes), [data-sources-and-scheduled-tasks.md](data-sources-and-scheduled-tasks.md),
[platform-and-operations.md](platform-and-operations.md) (who may call, and the switches).

---

## 1. Purpose

Salesforce is where customers, contacts, projects, subscriptions and partner links are
created. The portals need that data in the CSM database. The Salesforce ingest keeps the
CSM copy in step: whenever a record changes in Salesforce, the service is told, reads the
current record, and updates its own tables. It is also how a person invited from
Salesforce (or from a portal) becomes a portal user.

Records covered: accounts, contacts, project memberships, projects, subscriptions
(opportunities, their line items and invoices), subscription-to-project links, and partner
links. Section 13 covers the daily reopening of suspended projects, which this ingest
triggers. Allocation events, a separate feed, are covered in section 14.

## 2. Who is involved

| Who | What they do |
|---|---|
| Salesforce | The source of truth. Raises a change notice when a record is created, updated, deleted or restored. |
| The event subscriber | Receives those notices and forwards each one to this service. |
| The sales-record service | The only way this service reads Salesforce. It is asked for the current record. |
| This service | Applies the change to the CSM database, keeps a record of what it has applied, and retries what could not be applied yet. |
| The portals and scheduled tasks | After a portal write or a reconcile, replay a notice to this service so it re-reads Salesforce. They never write these tables themselves. |
| The notification service | Sends the invitation and welcome emails this ingest asks for (section 7). |

## 3. Receiving a change notice

**INT-010** The service shall accept a change notice made of three things: the kind of
change, the kind of record, and the record's Salesforce id. On success it shall answer
"no content", so the sender only needs a success status.
*Check:* send a valid notice for a known account; the answer is a success with no body.

**INT-011** A notice with a missing or blank kind of change, kind of record, or record id
shall be refused as a bad request.

**INT-012** For a record kind the service handles, the kind of change shall be exactly one of
CREATED, UPDATED, DELETED or RESTORED, in capitals. "UNDEFINED" shall be refused as not
supported, and any other value refused as a bad request listing the allowed values. The
kind of change is not checked for record kinds that are ignored (INT-014).
*Check:* send an account notice with a made-up change kind; it is refused with a message
listing the allowed values.

**INT-013** The kind of record shall be recognised without regard to letter case. A
project membership may be named with or without the Salesforce suffix, and linked
opportunities and invoices likewise.

**INT-014** A notice for a kind of record the service does not handle, or for a kind whose
switch is off (section 4), shall be accepted and ignored with a success answer. A refusal
would make the sender retry the same notice forever.
*Check:* send a notice for an unrelated Salesforce object; success, nothing written.

**INT-015** The notice carries only identifiers. The service shall read the current record
from Salesforce for every created, updated or restored notice and shall not trust any
content in the notice itself.

**INT-016** If the sales-record service returns nothing for a record that was just created
or updated (the notice can arrive before Salesforce has finished saving), the service shall
answer "temporarily unavailable" so the sender tries again, rather than treat it as "no
such record".

**INT-017** The endpoint shall exist only when PostgreSQL holds the data and the
connection to the sales-record service is fully configured. Otherwise it is not available.
(**Confirm**, see 15.1: who may call it is not checked inside the service.)

## 4. Switches

Each kind of record has its own switch. A switch turned off makes that kind accepted and
ignored (INT-014). Every switch is **on unless it is explicitly set to the word `false`**,
so a deployment that sets nothing has them all on (**Confirm**, see 15.2).

| Switch | Turns on |
|---|---|
| Account ingest | Accounts (section 5). |
| Membership ingest | Project memberships and contacts (section 6). Also needed for the retry job (section 12). |
| Opportunity ingest | Subscriptions, their line items, invoices and subscription-to-project links (section 10). |
| Project ingest | Updating existing projects (section 9). |
| Project insert | Letting the project ingest create projects the CSM does not have. |
| Partner ingest | Partner links (section 11). |

**INT-020** The service shall treat each switch independently.

**INT-021** A record kind shall be processed only when its switch is on **and** its own
prerequisites are present. In particular a contact or membership needs the membership
switch, and project creation needs both the project switch and the project insert switch.

**INT-022** Turning a switch off shall only stop new changes being applied. Data already
written stays as it is.

## 5. Accounts

**INT-030** For a created, updated or restored account notice the service shall read the
customer from Salesforce and write the account in the CSM database, creating it if it does
not exist and updating it if it does. The account is found by its Salesforce id.

**INT-031** The service shall write only the fields Salesforce owns: name, industry,
regions and pod, phone, life cycle, industry classification, billing address, owner,
technical owner, customer success manager, secondary technical owner, renewal manager,
activation, deactivation and lost dates and reason, vertical, and the primary partner.
A value cleared in Salesforce is cleared in the CSM.

**INT-032** The service shall never overwrite the fields the CSM owns: the account's teams,
support tier and time zone, suspension process state, the AI settings, and the drive
location. An existing account's number is kept; an account with no number takes it from
Salesforce.

**INT-033** People named on the account (owner, technical owner, customer success manager,
secondary technical owner, renewal manager) shall be matched to CSM users by email. For the
owner and technical owner an email with no matching user is stored as empty and a warning
is logged. For the other three an email with no matching user keeps the value already stored
(an email that is cleared in Salesforce still clears it).

**INT-034** If person auto-creation is configured, an unmatched email shall first be looked
up in the internal directory and, if found, a CSM user shall be created for that person with
the internal role, and given a visible marker so staff can tell it was created
automatically. Otherwise nothing is created. (**Confirm**, see 15.5.)

**INT-035** A value that does not fit its field (too long) or a date that cannot be read
shall be stored as empty with a warning. One bad field shall never fail the whole notice.
A phone number that is too long keeps the stored phone.

**INT-036** A customer with no name shall be acknowledged without retry, logged, and
recorded as failed in the ingest record (section 12).

**INT-037** A deleted account shall be marked deleted (a date is set), never removed, and
that is recorded. Search results shall exclude deleted accounts; reading one directly, and
the links from projects, cases and global search, shall still find it. Any later created,
updated or restored notice for the account shall clear the mark.
*Check:* delete an account in Salesforce; it disappears from account search but its cases
still show the account name. Restore it; it returns.

**INT-038** The service shall keep one account row per Salesforce id even if the data holds
more than one with the same id: writes update exactly one, chosen the same way every time.
Deletes apply to every copy.

**INT-039** An internal-only action shall let a CS engineer or an operator re-read one
account from Salesforce on demand and write it, ignoring the "already applied" check
(section 12). It exists to backfill a field added after the account was last saved. It does
not refresh partner links. It is available only when the account switch is on, and answers
"not available to external callers" otherwise.

## 6. Contacts and project memberships

A *membership* is one contact's place on one project. A *contact* is a person on a
customer account.

**INT-040** For a created, updated or restored membership notice the service shall read the
membership and its contact from Salesforce and, in one all-or-nothing step, write: the
project's link to the account, the user, the contact on the account, the project membership
and its state, the user's global roles, the project groups, and the derived admin role.

**INT-041** Each row shall be found by its natural key first, and given its Salesforce id
on the way, so replaying the same notice any number of times creates no duplicates and old
rows that pre-date the ingest are given their ids:
- the project by its subscription key, then by Salesforce id; not found is a "not found";
- the account by the contact's customer id, falling back to the project's account for a
  non-partner contact;
- the user by Salesforce id, then by email. More than one user with that email is a conflict,
  never a guess. A new user is created active. An existing user's name, email and
  integration flag are refreshed, never their username.

**INT-042** A membership's state shall be one of invited, registered, re-invited or
deactivated. "Re_invited" and "reinvited" are accepted as re-invited. Any other state shall
be refused.

**INT-043** Global roles shall follow the **contact's account** classification: every
contact is "external"; a contact on a Partner-classified account is also a "partner"; every
other contact is a "customer". The customer/partner pair is kept exclusive, so
re-classifying an account flips the role. When the classification is unknown nothing is
revoked. A contact flagged as an integration user gets no global roles. A role that does not
exist in the role list is a temporary failure naming it.

**INT-044** Project groups shall be set from the membership's Salesforce roles:

| Salesforce roles | Project group |
|---|---|
| Portal user and Security contact | Full Access |
| Portal user only | General Access |
| Security contact only | Security Only |
| Lead (in addition) | Lead User Group |
| Admin (in addition) | Admin |
| Business contact (in addition) | Business Contact Group (two spaces in the stored name) |

Salesforce role labels with no CSM group (business owner, promoter, detractor, technical
owner, champion, technical detractor) shall be ignored, logged once per membership, and
shall never fail the ingest. Unknown labels are logged as warnings. The set of groups is
replaced each time. A missing group in the list is a temporary failure.

**INT-045** After the groups are written, the service shall work out the user's
account-level admin role: admin when any of their live memberships carries the Admin
project role or the contact is flagged as a CS admin. The role is partner admin for a
Partner account and customer admin otherwise; the other is always revoked.

**INT-046** A deleted membership shall be set to deactivated, never removed, and the user's
admin role shall be worked out again in the same step. An unknown membership is a no-op that
still succeeds.

**INT-047** For a created, updated or restored **contact** notice the service shall read the
contact once and write the user, the contact on the account, the contact's global roles and
admin role, and record it. It shall then apply the membership steps above to each of the
contact's memberships using the contact it already read, attempting every membership and
returning the first error.

**INT-048** A contact whose account is not yet in the CSM shall cause the account to be
ingested first when the account switch is on. When it is off, the notice fails temporarily
and is retried later (section 12).

**INT-049** When a contact moves to another account, their contact rows on the old account
shall be set inactive, unless they still hold a live membership on a project of that
account (a partner contact legitimately keeps one). A contact with no email, or no account,
shall write nothing for the contact but still go on to its memberships.

**INT-050** A deleted contact shall set the contact and the user inactive and record it as
deleted. Nothing is hard-deleted. Their roles are left alone, because Salesforce deletes the
memberships with their own notices.

**INT-051** An integration user (a contact flagged as such) shall still be created as a
system user with a contact row, but with no global roles and no admin decision.

**INT-052** After any contact or membership write, the service shall drop the affected
user's cached profile so the portals see the change at once.

## 7. Invitation and welcome emails from the ingest

**INT-053** After a membership is written, the service shall ask for an **invitation email**
when the membership is invited or re-invited **and this notice actually moved it into that
state**: the membership row was created by this notice, or its stored state before was
different. The email is requested through an event (see event-publishing.md).
*Check:* invite someone in Salesforce; one invitation is requested. Re-invite someone who
was deactivated; one is requested. A repeat of the same notice requests nothing.

**INT-054** A notice that merely repeats a state the portal has already written (the echo of
the portal's own write) shall update the row silently and request nothing, so one
invitation never produces two emails.

**INT-055** A membership that Salesforce saves as registered shall still get an invitation
email when it is new (created in the last 24 hours) or comes back from deactivated, but not
on a restore. This covers a contact who has already signed in.

**INT-056** A **welcome email** shall be requested only when an existing membership moves
from invited or re-invited to registered. Echoes, replays and rows created already
registered stay silent.

**INT-057** If the event cannot be published, the database write still stands, the failure
is recorded, and the notice succeeds. A deployment with no event publishing configured
writes the rows and sends nothing.

## 8. Onboarding steps

An *onboarding step* records how far one membership's onboarding got: identity created,
database written, email sent, registered, welcome email sent.

**INT-060** Each membership shall have at most one record per step (identity, database,
email, registration, welcome email), with a status of succeeded, failed or skipped, a count
of how many times it was rewritten, and the Salesforce version it was based on. The
"database" step is written in the same all-or-nothing step as the membership itself.

**INT-061** On failure the ingest shall record the database step as failed with a short
reason, then return the original error.

The step API (who may write, read and search steps, and the rules for each field) is
described in [memberships-and-onboarding.md](memberships-and-onboarding.md), section 14.
This document covers only what the ingest itself records.

## 9. Projects

**INT-070** For a created, updated or restored project notice the service shall read the
project from Salesforce, make sure its account exists, work out its project type from the
type's exact name (an unknown name stores no type with a warning; types are never created),
and update the project found by its Salesforce id, else by its subscription key (the id is
then stamped on the row).

**INT-071** The project ingest shall write only these fields: Salesforce id, key, name,
account (kept when Salesforce names none), start and end dates, description, project type,
compliance violation date, and go-live date. It shall never touch closure states, hour
counters, onboarding fields, credentials, number or active flag.

**INT-072** The ingest always **updates** projects the CSM already has. A project the CSM lacks
is created, active, when project insert is on, which it is unless set to `false` (section 4).
When project insert is off, such a project is a "not found" recorded as failed and retried later
(section 12).

**INT-073** When the data source is dual-write (see data-sources-and-scheduled-tasks.md),
a project the ingest must create shall take the id the legacy system already uses for it, so
the two agree. If the legacy system does not have it yet, the notice fails temporarily and
is retried. The lookup matches the key exactly (case-sensitive). Two different ids for one
key, or a derived id already used by an unrelated row, is a conflict that nothing guesses
through and that needs a person.

**INT-074** A project with no subscription key shall be refused, recorded as failed with the
reason, and acknowledged without retry, because no retry can fix it.

**INT-075** A deleted project shall be set inactive on every row with its Salesforce id,
never removed. A restore, or any later update after a recorded delete, shall set it active
again.

**INT-076** A membership or linked-subscription notice that finds its project missing shall
ask for the project to be ingested first when both project switches are on, and retry once.
When project insert is off, it keeps its original failure for the retry job.

## 10. Subscriptions, invoices and links

These share the opportunity switch.

**INT-080** For a created, updated or restored subscription (opportunity) notice the service
shall read it, make sure its account exists, and write the subscription, its line items and
the "already applied" record in one all-or-nothing step.

**INT-081** The subscription shall store: name, account, stage, won flag, close date, and
EULA version. The close date is Salesforce's **support account end date roll-up**, not
Salesforce's own close date. The EULA version is kept as a number (first decimal found;
3.4 when empty; none when there is no decimal), and both EULA fields are left alone when
Salesforce sends none. Type, owner, engagement code and the query-hour state are never
written. Values too long for their field are cut and logged.

**INT-082** The embedded line items shall replace the subscription's stored line items as a
set: matching ones updated, new ones added, those not mentioned removed. Development support
hours and engagement code on a line item are never written.

**INT-083** A deleted subscription shall be removed, together with its line items and
project links, and its invoices shall lose their link to it. A subscription never ingested
is acknowledged and logged.

**INT-084** Linked subscriptions (a subscription's link to a project), invoices and
stand-alone line items shall each be read from Salesforce on their own notice. Each makes
sure its parent exists first (re-ingesting a missing subscription at its current version),
then writes under a lock so two notices cannot create the same row twice. A deleted one is
removed. Notices for things never ingested are acknowledged.

**INT-085** An invoice shall store every Salesforce data field. Its original due date is the
original due date, else the due date. An invoice with no subscription id is stored with no
parent and a warning. Auto-created invoices are included. Invoices embedded inside a
subscription notice shall be ignored, because they lack fields the stand-alone invoice has.

**INT-086** A linked subscription without a project id or subscription id shall be recorded
as failed and acknowledged.

**INT-087** The service does not receive a last-modified time for subscriptions, so for them
the "already applied" check (section 12) is skipped and the idempotent write runs on every
notice. (**Confirm**, see 15.3.)

## 11. Partner links

**INT-090** The service shall keep, for each customer account, the list of its partners: a
forward link "partner is partner of customer" and its mirror "customer is customer of
partner". When partner ingest is on, the list is refreshed from Salesforce.

**INT-091** The refresh shall read the customer's partners from Salesforce. A reply that
does not say who the partners are (as opposed to saying there are none) shall be a temporary
failure, so an older sales-record build can never wipe stored links.

**INT-092** The refresh shall make sure the customer and every partner exist as accounts
first (ingesting them when the account switch is on), then in one all-or-nothing step add
missing links in both directions and remove the partner links of partners who left the set.
Links with any other label are never touched.

**INT-093** The refresh shall run: after every account created, updated or restored notice
(even one the "already applied" check skipped, because a partner change does not move the
account's last-modified time; a failure fails the notice so it is delivered again); after a
membership notice whose contact's account differs from the project's account (best effort,
never failing the membership); on demand through an internal-only action; and from the retry
job.

**INT-094** The sweep that would catch a partner added in Salesforce with no related save is
not built. (**Confirm**, see 15.4.)

## 12. The "already applied" record, duplicate check and retry job

**INT-100** For every kind of record except memberships the service shall keep one record per
(kind, Salesforce id), noting whether the last write succeeded or failed, the Salesforce
version it was based on, when, and how many times in a row it has failed. Memberships keep the
same information in their database onboarding step.

**INT-101** Salesforce sends several update notices per save and the portals replay the notice
after their own writes, so the same version arrives more than once. The service shall skip a
notice when the record shows the same or a newer version already written successfully. A failed
record, a record last written by a delete, and a notice with no readable last-modified time
shall never be skipped (in the last case a warning is logged and the idempotent write runs).

**INT-102** The record shall only move forward: a write recording an older version shall not
replace a newer one, except that a delete always replaces it.

**INT-103** A failed attempt shall be recorded. When the failure may clear on its own, the
original error shall be returned so the sender delivers the notice again. A failure that no retry
can fix, such as a project with no subscription key (INT-074), shall be recorded as failed and
acknowledged without a retry.

**INT-104** A **delayed-retry job** shall run in the background when the membership switch is
on and its interval is above zero (default five minutes; zero or an invalid value turns it
off). Each run it shall pick up records that failed only because a parent record (an
account or project) was not yet in the CSM, and run them again as an update. The sender
re-delivers a failed notice five times within seconds, but a new project can take up to five
minutes to reach the CSM, so this job covers the gap.

**INT-105** It shall retry at most 100 records per run, give each 30 seconds, and give up on
a record after 12 failed re-runs (about an hour at the default). Redeliveries and new notices
never count towards the 12. A later successful ingest of the missing parent resets the count
for records that were waiting on it.

**INT-106** It shall retry memberships, contacts, subscriptions, linked subscriptions,
invoices, line items (when the opportunity switch is on), projects (when the project switch is
on) and partner refreshes (when the partner switch is on). Accounts have no parent to wait
for, so they are not retried.

## 13. Reopening suspended projects after renewal or payment

When a customer renews or pays, a project that was suspended by the account closure process
should reopen without a person doing it. This is off unless it is explicitly turned on, and
runs as a **dry run** (it logs what it would do and changes nothing) until the dry-run
setting is turned off. Turning it live needs the matching daily check in the scheduled tasks
turned on at the same time; the service logs a warning at start as a reminder.

**INT-110** After a project or invoice change notice (created, updated or restored, never
deleted), and once the ingest itself succeeded, the service shall check whether that project
should be reopened. It reads Salesforce itself, whatever the project and subscription switches
say.

**INT-111** The service shall reopen only a project that the closure process suspended (a
record of that suspension exists). A project suspended by hand, or whose overall closure state
was set by hand, shall be left alone and counted.

**INT-112** The decision shall follow the closure process's own rules exactly, read from
Salesforce at that moment: the end-date track reopens when the project's end date is set and
not yet past; the invoice track reopens when no counted invoice is past its suspend date. A
date that cannot be read keeps the track suspended.

**INT-113** A reopen shall, in one all-or-nothing step: lock the project, re-check, set the
track to open, record it as open, recompute the overall state, set the project Open in
Salesforce (within 20 seconds; a refusal rolls it all back) when the overall state is now
open, and queue the "back to open" notice. A failure fails the Salesforce notice (live mode)
so it is delivered again; in a dry run a failure is only logged.

**INT-114** A **daily check** shall go through every project still recorded as suspended by
the closure process and apply the same rules, page by page (default page 10, at most 50;
larger is clamped). It catches missed or failed events and projects set back to Open by hand.
It shall be callable only by an internal service credential, never by a person's token.

**INT-115** A project a person has set back to Open shall be finished by the daily check: its
record cleared, Salesforce set Open, and the notice queued only when the suspension was
recent (a week or less, or dated); older or undated suspensions are finished silently so the
first live run does not email for every project ever reopened by hand.

**INT-116** The "back to open" notice shall go to the account owner, renewal account manager
and technical owner (each once), keeping only well-formed addresses in the configured internal
domains (default the company's own). It is queued with the reopen and published by a
background job every five seconds, retried with growing delay (30 seconds doubling, at most an
hour) and set aside after 20 attempts. Delivery is at least once.

**INT-117** The project's "suspension process overridden" flag, when set, is stored and
readable, but the reopen check does not read it yet. (**Confirm**, see 15.6.)

## 14. Related feeds

Allocation events (customer engagements and resource allocations) arrive on their own
endpoint and are described in [engagement-allocations.md](engagement-allocations.md). They
follow the same approach as this ingest: accept the event, apply it once in one
all-or-nothing step, and answer success with a reason when the event cannot be applied, so
the sender never retries something no retry can fix.

## 15. Open questions for the product owner

1. **Who may call the change-notice endpoint.** The endpoint does not check the caller
   inside the service. It relies on the gateway and the sender. The onboarding-step,
   account-refresh, partner-refresh and daily-check actions do check; the notice endpoint and
   the event-failure and scheduled-run records do not. Is that intended?
2. **Switches are on by default.** The Go comments, the example settings file and the API
   description all say the migration switches default to *off*; the code and the README
   confirm they are *on unless set to false*, **including project insert**. A new deployment
   that sets nothing writes accounts, memberships and projects from Salesforce straight away.
   Which default is wanted, and should the other documents be corrected?
3. **Subscriptions have no duplicate check.** Until the sales-record service sends a
   last-modified time, every subscription notice rewrites the data.
4. **Partner links miss silent changes.** A partner added in Salesforce with no related save
   is not seen until some other change touches the account.
5. **Auto-created people.** When person auto-creation is on, staff found in the directory are
   created as internal users with a visible marker on the last name. Is the marker wanted in
   the portals?
6. **Closure-process overridden flag is stored but not honoured** by the reopen check.
7. **Dry run first.** Going live with project reopening without the matching daily check
   leaves failed reopens and by-hand reopens unfinished; only a warning at start reminds the
   operator.

## 16. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Notice entry point, dispatch, accounts | `internal/handler/salesforce_event_handler.go`, `internal/service/salesforce_event_service.go`, `internal/repository/account_repo.go` |
| Memberships, contacts, roles, groups | `internal/service/salesforce_membership_ingest.go`, `salesforce_membership_mapping.go`, `salesforce_contact_ingest.go`, `internal/repository/project_membership_repo.go`, `salesforce_contact_repo.go` |
| Onboarding steps (ingest side) | `internal/service/salesforce_membership_ingest.go`, `internal/repository/onboarding_step_repo.go` |
| Projects | `internal/service/salesforce_project_ingest.go`, `internal/repository/salesforce_project_repo.go` |
| Subscriptions, invoices, links | `internal/service/salesforce_opportunity_ingest.go`, `salesforce_invoice_ingest.go`, `salesforce_linked_opportunity_ingest.go`, `salesforce_opportunity_line_item_ingest.go`, `salesforce_opportunity_ensure.go` |
| Partner links | `internal/service/salesforce_partner_ingest.go`, `internal/repository/account_partner_write_repo.go` |
| Duplicate check, record, retry job | `internal/service/salesforce_ingest_guard.go`, `salesforce_ingest_retry_worker.go`, `internal/repository/salesforce_ingest_state_repo.go` |
| Project reopen | `internal/service/acp_reopen.go`, `acp_reopen_rules.go`, `acp_reactivation_drainer.go`, `internal/repository/acp_reopen_repo.go` |
| Routes | `POST /salesforce/events`, `POST /salesforce/accounts/{sfId}/refresh`, `POST /salesforce/accounts/{sfId}/refresh-partners`, `POST /acp/reopen-check` |
| Migrations | the ingest-record table (`0170`), retry counter (`0174`), onboarding steps (`0118`), account deleted mark (`0171`), primary partner (`0291`), reopen override (`0281`) |
| Settings | `CSM_MIGRATION_SALESFORCE_*`, `SALESFORCE_INGEST_RETRY_INTERVAL`, `SALES_ENTITY_*`, `CSM_ACP_REOPEN_*`, `ACP_*` |
| Tests | `salesforce_*_test.go`, `acp_reopen*_test.go`, `account_repo_salesforce_test.go`, `testdata/acp_rules_parity.json` |
