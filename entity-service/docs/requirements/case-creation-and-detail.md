# Cases: creation, numbering and detail — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 13.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

Related documents: [case-updates-and-states.md](case-updates-and-states.md) (changing a
case), [case-closure-and-reopen.md](case-closure-and-reopen.md) (closing, reopening,
auto-closure, feedback), [case-escalation.md](case-escalation.md) (escalation), and
[sla.md](../specs/sla.md) (the SLA clocks a new case starts).

Searching, filtering and counting cases, comments, attachments, tags and watch lists are
not covered here.

---

## 1. Purpose

Customers and CS engineers raise tickets against a customer's project. This document
covers how a ticket is created, how it is numbered, which rules a creation request must
pass, what happens right after it is created, and what a single ticket looks like when it
is read back.

## 2. Who is involved

| Who | What they do |
|---|---|
| Customer user | Raises a case or a service request in a project they are registered on. |
| CS engineer | Raises tickets on a customer's behalf, in any project. |
| Integrations and scheduled jobs | Create tickets without a signed-in person (for example a scheduled announcement). |

## 3. Kinds of ticket

The service treats five kinds of ticket as one family, often called "case-like". They share
one number series, one list and one detail view.

| Kind | What it is |
|---|---|
| Case | A support case about a problem or question on a deployed product. |
| Service request | A request raised from the catalog form (for example a configuration change). |
| Security report analysis | A customer's security report sent in for analysis. |
| Engagement | A planned piece of work (migration, consultancy, onboarding and so on). |
| Announcement | A message sent to a project's contacts. |

Incidents, problems and change requests are **not** cases. They have their own routes,
numbers and views, and a case's detail view does not serve them.

**CAS-010** The service shall accept exactly these ticket types on creation: `case`,
`service_request`, `security_report_analysis`, `engagement` and `announcement`. Any other
type, or a missing type, shall be refused.
*Check:* create with type `incident`; refused, nothing is created.

**CAS-011** The older value `default_case` shall be accepted and treated as `case`.
*Check:* create with type `default_case`; the new ticket is a case.

## 4. What a creation request must contain

**CAS-020** Every ticket shall name a project. A missing or malformed project id shall be
refused.

**CAS-021** Every ticket except an announcement shall also name a deployment and a deployed
product, both valid ids. An announcement has neither.
*Check:* create a case with no deployed product; refused. Create an announcement with none;
accepted.

**CAS-022** A **case** shall also have a subject, a description, a severity and an issue
type. Severity must be one of catastrophic, critical, high, medium or low. Issue type must
be one of error, partial outage, performance degradation, question, security or compliance,
or total outage.
*Check:* create a case with severity `urgent`; refused with a message naming severity.

**CAS-023** A **service request** shall also name a catalog, a catalog item and at least one
answer to the form. Its subject and description are optional (see CAS-060).

**CAS-024** A **security report analysis** shall also have a subject and a description.

**CAS-025** An **engagement** shall also have a subject, a description, an engagement type
(migration, consultancy, new feature or improvement, follow up, or onboarding) and a payment
type (paid or free of charge).

**CAS-026** An **announcement** shall also have a subject and a description. It may be
marked as a security announcement.

**CAS-027** A ticket may optionally name the conversation it came from and, for a case, a
related case. A malformed id shall be refused. A conversation must belong to the same
project as the new ticket, and a related case must exist and be a case, service request,
security report analysis or engagement; otherwise creation is refused and nothing is saved.
The refusal for a conversation is the same whether it does not exist or belongs to another
project, so a caller cannot probe other projects.

**CAS-028** The creator of a ticket shall always be the signed-in person. A caller cannot
choose it in the request.
*Check:* send a request body naming another creator; it is ignored.

**CAS-029** If the signed-in person has no user record, creation shall be refused.

## 5. Who may create

**CAS-030** A customer may create a case, service request, security report analysis or
engagement only in a project they belong to. For any other project the answer shall be "not
found", the same as for a project that does not exist.
*Check:* a customer creating a case in another customer's project gets "not found".

**CAS-031** CS engineers (internal users) may create tickets in any project.

**CAS-032** Without a signed-in person the service shall refuse to create a ticket, unless
the creator is supplied by the service's own code. The one such case today is the process
that publishes an approved announcement request to each project: it creates one
announcement per project in the requester's name.

**CAS-033** A request body shall be accepted up to 15 MB, large enough for a rich-text
description with inline images. Attachment files never travel in this body.

## 6. Numbering

Every ticket gets two identifiers when it is created.

**CAS-040** The **ticket number** shall be `CS` followed by a zero-padded seven-digit
sequence number, for example `CS0000123`. All five ticket kinds draw from the **same**
sequence, so a case and a service request never share a number.

**CAS-041** The **project reference** shall be the project's own key, a dash, and a
per-project counter that starts at 1, for example `ACME-12`. Each project counts on its own.

**CAS-042** A number is never reused. Two tickets created at the same moment shall get
different numbers.

**CAS-043** If the project does not exist the creation shall fail with a clear message and
nothing shall be saved.

**CAS-044** The first value of both counters is set by a one-off seeding step when the
database is switched over, so they continue after the highest number already issued. This
must be done before the service creates tickets itself. (**Confirm**, see 13.1)

## 7. Product categories a project allows

A project's type can restrict which products may be used when creating a case or a service
request.

**CAS-050** If the project's type lists the product categories allowed for **cases**, a case
shall be refused unless its deployed product belongs to one of them. The same applies to
**service requests** with the type's service request list.
*Check:* the project type allows only `software` for cases; a case against a `service`
product is refused with a message listing the allowed categories.

**CAS-051** A deployed product that has no category set shall **fail** the check once the
project type restricts the ticket's type. This is deliberate: it makes categorising a
deployed product matter.

**CAS-052** A project type that lists no categories, a project with no type, and every
other ticket type (engagement, security report, announcement) shall be unrestricted.

## 8. Service requests

**CAS-060** A service request raised from the catalog form carries only the catalog item and
the answers. If the caller sends no subject, the service shall use the answer to the form's
"Title" question (or, failing that, Short Description, Request Details, Summary or Subject),
and otherwise the catalog item's name. If the caller sends no description, the service shall
build one from the other answered questions, one "question: answer" line each, in form order.
A subject or description the caller did send is never replaced.
*Check:* create a service request with no subject; its subject is the form's title answer.

**CAS-061** A rich-text answer to the "Description" question shall be kept as formatting, not
shown as literal tags.

**CAS-062** Every new service request shall publish a "service request created" event for
the operations team's alerts. If the project's account belongs to a support team that has
automated service request handling (a configured list of teams), the request shall also be
assigned to that team and receive the standard acknowledgement comment ("Your service
request has been received successfully…"), followed by an "acknowledged" event. Failures
here are logged and shall never fail the creation.

## 9. Files and images

**CAS-070** A **security report analysis** created by a signed-in person shall include at
least one uploaded report file. The files must have been uploaded earlier by that same
person for the same project, be completely uploaded, and not already belong to another
ticket; otherwise creation is refused with a message saying so and nothing is saved. An
integration with no signed-in person is not held to the minimum.
*Check:* create a security report with no files; refused.

**CAS-071** The files are attached to the new ticket in the same step as the creation, so a
ticket never exists without the files it was created with. If a file was taken by another
creation in the meantime, the whole creation fails.

**CAS-072** Images embedded in the description are uploaded as drafts first. They shall be
checked **before** anything is created and linked to the new ticket afterwards, so a bad
request leaves nothing half created. An announcement published by a scheduled job with no
signed-in person is exempt.

## 10. Watchers named at creation

**CAS-080** A request may list people to watch the new ticket. Each must be eligible: a
registered contact of the project, or an internal user who is not a contact only if the
caller is also internal. A customer may add registered contacts and nothing else. If any
entry is not eligible the whole creation is refused, with a message that does not say which
entry failed.

**CAS-081** The list must be all emails or all user ids, not a mix.

**CAS-082** The creator shall be added as a watcher, whether or not they named themselves.
This is best effort: a failure to add them is logged and does not fail case creation (see
COL-323).

## 11. What happens right after creation

All of these happen after the ticket is saved. A failure in any of them is logged and
**never** fails or undoes the creation.

**CAS-090** The description shall also be saved as the ticket's first comment, written as the
creator, except for announcements and blank descriptions.

**CAS-091** For cases, security report analyses and service requests (not engagements or
announcements) the service shall start the ticket's SLA clocks, according to its severity and
project. See [sla.md](../specs/sla.md).

**CAS-092** The service shall publish a "case created" event so the notification service can
email the ticket's watchers. The event is not published if the ticket has no project, or if
it is a case with no severity. For an announcement the recipients are the project's contacts
with the portal-user role, or the security-contact role for a security announcement.

**CAS-093** The new ticket's state shall be **Open**, for all five kinds. A new case shows
escalation level 0 and is not flagged as escalated (see CAS-105).

**CAS-094** The reply to a successful creation shall give the ticket's id, number, project
reference, creator, creation time and state.

**CAS-095** While the old case system is still mirrored, that system creates the ticket first
and supplies the number and project reference; the service then records the same ticket
locally. If the old system refuses, nothing is created in either place. The call is made
once, with no retry, so a lost response cannot create a duplicate. (**Confirm**, see 13.2)

## 12. Reading one ticket

**CAS-100** The service shall return one ticket by id for any of the five kinds.

**CAS-101** A ticket the caller may not see shall be answered with "not found", the same as
a ticket that does not exist.
*Check:* a customer opens another project's case by id; "not found".

**CAS-102** Customers may see tickets in the projects they are registered on, and only
those. A person who is both a customer contact and internal staff is treated as a customer
for what they may see.

**CAS-103** The detail shall include: number, project reference, subject, description,
severity, issue type, state, work state, type, creator, created and updated times, project,
deployment, deployed product (and the catalogue product), account, assigned team, assigned
engineer, who acknowledged it, parent and related case, conversation, linked service
requests and change requests, tags, watch list, fix ETAs, escalation level, and the closing
fields (see [case-closure-and-reopen.md](case-closure-and-reopen.md)).

**CAS-104** A ticket with **no project** shall still be returned, with the project left
empty. The same applies to a missing deployment, deployed product, account or assigned
engineer.

**CAS-105** A ticket that has never been escalated shall show escalation level "0" and not
escalated, rather than an empty value.

**CAS-106** The closing fields shall be read from whichever of the five ticket kinds the
ticket actually is.

**CAS-107** The detail returns the same fields to everyone allowed to see the ticket,
including internal fields such as the assigned team, the watch list and the account's
support tier. Trimming for customers is done by the portals. (**Confirm**, see 13.3)

## 13. Open questions for the product owner

1. **Numbering depends on a manual seeding step.** If the counters are not seeded past the
   highest existing number before cut-over, a new ticket could repeat a number the old system
   issued. Is there a checklist item or guard for this?
2. **Two creation paths.** Creation behaves differently depending on whether the old system
   is still mirrored (the old system numbers the ticket) or not (the service numbers it).
   Is the mirrored mode still needed?
3. **Internal fields in the detail.** The detail view sends the assigned team, watch list and
   account support tier to any caller who may see the ticket. Is it intended that customers'
   portal calls get these and are trimmed later?
4. **No default product category.** A deployed product with no category blocks creation once
   the project type restricts the type. Who is responsible for categorising existing
   products?
5. **Related case on other ticket types.** A related case can be named at creation on any
   type, but is only saved on a case; for the other types it is silently ignored. Later
   updates refuse it on non-cases by name. Should creation refuse it too?

## 14. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Creation rules, aliases, category check | `internal/service/case_service.go` |
| Numbering functions and series | migration `0180_native_number_series.sql` |
| Creation insert per type, membership check | `internal/repository/case_repo.go` |
| Service request text and acknowledgement | `internal/service/case_sr_text.go`, `internal/service/sr_notice_service.go` |
| Security report files and images | `internal/service/report_drafts.go` |
| "Case created" event | `internal/service/sn_case_service.go` (publish helper) |
| Detail read | `internal/service/case_service.go`, `internal/repository/case_repo.go`, `internal/domain/entity.go` (case view) |
| Visibility and scope | `internal/service/access_service.go`, migration `100008_case_adjacent_rls.sql` |
| Routes | `POST /cases`, `GET /cases/{id}` |
| Tests | `internal/service/case_service_test.go`, `internal/repository/case_repo_test.go` |
