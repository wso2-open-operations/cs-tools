# Announcements — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 9.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Prefix:** `COL` (case collaboration). This document uses `COL-200` to `COL-299`.

---

## 1. Purpose

An **announcement** is a notice WSO2 sends to many customers at once: a service notice, an
end-of-life warning or a security bulletin. Each customer project that is in the audience
gets its own announcement case, which its users read in the portal and can reply to.

Sending one safely takes several steps (write, test, approve, publish, follow up), so the
service keeps an **announcement request** that carries the draft through those steps and
records what was delivered to whom. This document covers that workflow, who can see a
published announcement, scheduled publishing, and the registry staff use to look back.

## 2. Who is involved

| Who | What they do |
|---|---|
| Announcement creator (a CS engineer with the announcement permission, or an admin) | Writes, tests, submits, schedules and publishes a request; posts follow-ups. |
| Approver | Approves a submitted request (the decision itself is taken outside this system). |
| Customer user | Reads the announcements of their own project and may reply. |
| Security contact | A customer contact who also receives security announcements (section 4). |
| Scheduled job | Publishes approved requests when their scheduled time arrives. |

## 3. Announcement requests

A request is always in one of four states: **draft**, **pending approval**, **approved**,
**published**. There are two kinds: a **customer** announcement and an **end-of-life**
announcement.

**COL-200** A request shall be created as a **draft** with a kind (customer or end-of-life),
the person creating it, and optionally a subject, description, a security flag and an
audience definition. A kind other than those two, or no creator, shall be refused.

**COL-201** A draft may be edited. Only the fields sent change; the rest stay.

**COL-202** Before a draft can be submitted, a **dry run** shall be recorded: the test
announcement case that was sent, and who ran it and when. A dry run can only be recorded
while the request is a draft.
*Check:* submit a draft that never had a dry run; refused.

**COL-203** **Submitting** shall move a draft to pending approval. It needs the list of
projects the audience resolved to (not empty) and the person submitting, and records both.
A request that is not a draft, or has no dry run, shall be refused as a conflict. The
service shall also set a **due date** one month after submission. The due date is shown
only; nothing acts on it.

**COL-204** If a request that is **pending approval** is edited, it shall go back to
**draft**, and its resolved audience, dry run and submission details shall be cleared, so
the whole check is done again.
*Check:* edit a pending request; it is a draft with no dry run.

**COL-205** **Approving** shall move a pending request to approved and record who approved
it and when. Only a request pending approval can be approved. The service does not check
the approver's role, and does not require them to be a different person from the creator:
the approval decision is taken outside the system. **Confirm** (9.1).

**COL-206** An **approved** request may still have its subject, description or security
flag edited, but its **audience cannot be changed**: the approved list of projects is
frozen. Trying shall be refused.

**COL-207** A **published** request cannot be edited.

**COL-208** Requests shall be searchable by state (one or several), by creator, and, for the
scheduled job, by "approved and due now". A single state cannot be combined with several
states, and "due now" cannot be combined with either. Results are newest first, in pages.

## 4. Who can see a published announcement

**COL-210** Each announcement case shall be visible according to these rules, enforced in
the database:

| Reader | Sees |
|---|---|
| Internal staff | general and security announcements |
| Security contact of the project | general and security announcements |
| Ordinary portal user or lead user of the project | general announcements, and security announcements **only when the project has no security contact at all** |
| Anyone else | nothing |

**COL-211** A business contact alone, with no portal or security role, shall see nothing.
A lead user has the same access as an ordinary portal user. As for any case, the reader
must also be a registered contact of the project.

**COL-212** An announcement counts as a **security** announcement if its type says so or it
carries the **Security Announcement** tag, so older announcements marked only by the tag
stay protected.
**Confirm** (9.2: very old ones carry neither signal and are treated as general).

**COL-213** The comments, watchers and call requests of an announcement shall follow its
visibility: if a person cannot see the announcement, they cannot see or add any of them.
*Check:* a portal user in a project with a security contact opens a security announcement
by its address; it is "not found".

**COL-214** An announcement has no deployment or product. Its subject and description are
required, and its description is the announcement body (it is not repeated as a first
comment).

## 5. Publishing

**COL-220** **Publishing** shall record the case that was sent to each project, and move an
**approved** request to **published** with who published it and when. It needs at least one
case. Only the request's **creator** may publish it. A request that is not approved is
refused as a conflict.

**COL-221** A request can be **scheduled**: an approved request may be given a future time
(and, by sending none, have its schedule cleared). Only the creator may schedule it, only
when it is approved, and the time must be in the future.

**COL-222** When a scheduled time arrives, a scheduled job shall publish the request on the
creator's behalf, exactly as if the creator had pressed publish. This is available to
internal callers only.

**COL-223** Automatic publishing shall refuse, with a conflict, a request that:
- is not approved, or whose scheduled time has not arrived;
- has no resolved audience;
- is a security announcement but has no recorded creator email to attach the security tag as.

**COL-224** Automatic publishing shall create **one announcement case per project** in the
resolved audience, using the request's subject and description, as the creator. For a
security announcement it shall add the **Security Announcement** tag to each case.

**COL-225** Only one run at a time shall publish a given request: a run claims it, and a
claim older than **13 minutes** is treated as abandoned so another run can take over. The
claim is released when the run ends.

**COL-226** Projects shall be worked on a few at a time (up to **5** at once).

**COL-227** Publishing shall be safe to repeat. Projects already delivered are skipped on a
retry, and a project whose case was created but whose security tag failed has only the tag
retried, so no project ever gets a second case.
*Check:* one project fails; the next run creates only that one.

**COL-228** If any project fails, the run shall report a conflict saying how many projects
failed and that it will retry; the request stays approved. Only when every project has been
delivered shall the request become published.

**COL-229** Description pictures are not checked for an announcement created by automatic
publishing, since there is no person to own a draft picture. They are left as written.

## 6. Delivery records

**COL-230** The service shall keep, for each project of a request, whether delivery
**succeeded**, **failed**, or created the case but **failed the security tag**, with the case
(if any) and an error message.

**COL-231** Deliveries can be recorded only while the request is approved, and only by its
creator. Each must name a project that is part of the request's resolved audience.

**COL-232** A **succeeded** or **tag failed** delivery shall name the case. A **failed**
delivery shall not name one, since it never produced a real case. Recording again for the
same project replaces the earlier record.

**COL-233** Anyone who can see the request may list its deliveries.

## 7. Follow-up updates

**COL-240** After a request is published, its creator may post **updates** (a short text)
to it. Anyone who can see the request may list them. An update needs content, a request
that is published, and the creator as the person posting.

## 8. The announcement registry

The registry is the staff view of every announcement sent.

**COL-250** The registry shall be available to internal callers only.

**COL-251** The registry shall always be limited to announcements. A filter on case type
sent by the caller is replaced; "any of" filters are not supported.

**COL-252** The registry shall offer two shapes:
- **All matching cases**, newest updated first, up to **10,000**; more than that shall be
  refused with a message to narrow the search;
- **Grouped rows**, in pages: one row for each published request (a "batch" listing its
  projects and cases, and whether it is a security announcement) plus one row for each
  announcement case that no published request owns. The total counts rows, not cases.

**COL-253** Filters shall mean the same as in the ordinary case search.

## 9. Open questions for the product owner

1. **No separation of duties.** The service accepts any approver, including the creator.
   The decision is taken by email outside the system. Should the system record or enforce
   it?
2. **Old security announcements** (from the previous system) that carry neither the type nor
   the tag are treated as general announcements.
3. **Announcement request routes are not limited to internal callers** in this service
   (only automatic publishing is). They rely on the CSM portal's permissions and on
   creator-only checks. Is that enough?
4. **The "creator" is an opaque account id** supplied by the caller, not checked against the
   signed-in person here.
5. **The due date** one month after submission is informational only.
6. **A request in a project with a security contact** hides security announcements from
   ordinary portal users; one without a contact shows them. Is this the wanted fallback?

## 10. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Request workflow, publishing, delivery | `internal/service/announcement_request_service.go`, `internal/repository/announcement_request_repo.go` |
| Registry | `internal/service/announcement_registry_service.go`, `internal/repository/announcement_registry_repo.go` |
| Routes | `POST /announcement-requests`, `GET /announcement-requests/{id}`, `…/search`, `PATCH …/{id}`, `…/{id}/dry-run`, `/submit`, `/approve`, `/schedule`, `/auto-publish`, `/publish`, `/updates`, `/deliveries`; `POST /announcements/registry/cases`, `…/registry/rows` |
| Visibility | migrations `100001`, `100010`, `100016`, `100019` |
| Case creation for announcements | `internal/service/case_service.go` (create checks) |
| Scheduled publishing | the scheduled-tasks service, "publish scheduled announcements" |
| Portal permissions | CSM portal backend `internal/handler/announcement_requests.go`, `announcement_registry.go` |
| Tests | `announcement_request_service_test.go`, `announcement_registry_service_test.go`, `announcement_leak_guard_test.go` |
