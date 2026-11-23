# Tags, watch list, contacts and chat links — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 7.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Prefix:** `COL` (case collaboration). This document uses `COL-300` to `COL-399`.

---

## 1. Purpose

Four small features that connect a case to people and context:

- **Tags** are short labels on a case that staff use to group and find cases.
- The **watch list** says who is kept informed about a case.
- **Contacts** are the customer people attached to an account or a project.
- A case can be **linked to the chat conversation** it was created from.

## 2. Who is involved

| Who | What they do |
|---|---|
| CS engineer | Tags cases, manages watchers, looks up contacts. |
| Customer user | Can tag and watch cases in their own projects; sees their own project's contacts. |
| Account stakeholders | Named on the account (owners, managers); always shown as watchers. |
| Customer contact | A person invited to a project; can be registered, invited or deactivated. |

## 3. Tags

**COL-300** A signed-in person shall be able to attach a tag to a case by giving its label.
The label is trimmed and shall not be empty or longer than 255 characters.

**COL-301** Tags are shared by name. If a tag with that label already exists (ignoring
upper and lower case) it is reused, otherwise it is created with the spelling given.
*Check:* add "Patch" to one case and "patch" to another; both use the same tag.

**COL-302** Adding a tag a case already has shall succeed and change nothing, returning the
existing tag.

**COL-303** A tag added to a case that does not exist, or that the caller cannot see, shall
be "not found". For a change request the caller may not see, the answer is the same.

**COL-304** A person shall be able to remove a tag from a case. Removing a tag the case
does not have shall be "not found". Removing it never deletes the tag itself, only its use
on that case.

**COL-305** Whoever can see a case may add and remove its tags, internal staff always and
customers for their own projects.

**COL-306** The service shall let a signed-in person search tags: tags whose name contains
the text (ignoring case), newest first, up to a limit (default 20, at most 50). The search
text is at most 200 characters. Both a POST form and a quick GET form (`q`, `limit`) are
offered.

**COL-307** A case tagged **patch** while its severity is **Low (S4)** shall have its time
cards marked **non-billable**, because a patch is covered by support even on a best-effort
case. At any other severity, adding the tag changes nothing then; the rule is applied the
next time the case's severity moves into Low. A failure here never stops the tag being added.

**COL-308** A tag change on a case shall also be copied to the legacy case system, best
effort, while it is mirrored. A failure shall be recorded and shall never fail the change.

**COL-309** The **Security Announcement** tag marks an announcement as a security one (see
[announcements.md](announcements.md)), and a service request's tags travel with its
comment events (see
[call-requests-and-service-requests.md](call-requests-and-service-requests.md)).

## 4. The watch list

A case's watch list is the set of people who are emailed when something happens to it.

**COL-320** The watch list a case returns shall combine two kinds of people:
- **watchers** added to the case by a person or by following it;
- the account's **named stakeholders**: technical owner, secondary technical owner, account
  manager, renewal account manager and customer success manager. These are always shown, are
  marked **locked**, and cannot be removed from the case. A person who is both appears once,
  as locked. They change only when the account's own stakeholders change.

**COL-321** A customer who was a watcher shall only be shown while they are still an active
(not deactivated) contact of the case's project. Internal staff are always shown.

**COL-322** When the case's project is in active onboarding, its **onboarding owner** also
receives the case emails, but is not shown in the list.

**COL-323** The person who **creates** a case shall be added to its watch list, and so shall
a person who **comments** on it, if they are not on it already. Both are best effort and
never fail the request.

**COL-324** A case may be created with a watch list. Its entries shall be all emails or
all user ids, never mixed, and every entry shall be allowed:
- a **registered contact** of the case's project is always allowed;
- an **internal user** who is not a contact is allowed only when the person creating the case
  is internal too, so a customer can add registered contacts and nobody else;
- a request with no signed-in person shall be refused.
A refused entry fails the whole request with a message that does not say which person it
was.
*Check:* a customer adds an internal engineer on a new case; refused.

**COL-325** The watch list of an existing case shall be changed by sending the **whole
desired list**; it replaces the old one. An empty list clears every watcher. Stakeholders
(COL-320) are not part of it.

**COL-326** Only people **newly added** by a change are checked: a registered contact or
internal staff. Someone already on the list stays valid even if they have since stopped
being a contact; removing them is the caller's choice. An id that does not exist shall be
refused.

**COL-327** A watch-list change shall not be combined with any other change to the case in
the same request. It records who changed it and when on the case.

**COL-328** When emails are sent about a case, the recipients are its watchers who are
still eligible plus the account's stakeholders. A watcher is eligible if they are a
**registered** contact of the project or internal staff; anyone else, for example someone who
has left the project, is left out quietly.
*Check:* a customer watcher is deactivated; the next case email does not go to them.

**COL-329** A watch-list change shall also be copied to the legacy case system, best
effort, while it is mirrored.

**COL-330** Who can see or change a case's watch list follows who can see the case. For an
announcement, a person who cannot see the announcement cannot see or change its watchers.

## 5. Contacts

Account contacts are also covered in [accounts.md](accounts.md) (section 6, PRJ-150 to PRJ-153).

**COL-340** The service shall list the **contacts of an account**: the people linked to an
account, with name, email and whether they are its primary contact. It can be searched by
text (name or email, at most 200 characters), newest first, in pages. This is for internal
callers only.

**COL-341** The service shall list the **contacts of a project**, searchable the same way,
newest first, in pages, and return one contact by user id. Internal staff may do this for any
project; a customer only for their own project, otherwise "project not found".

**COL-342** A contact in a project's list shall show: name, email, **registration state**,
their roles on this project, their account-level roles, whether a user account is linked,
whether the contact is a system integration user, whether notifications are on, and, where
the link to the sales system is enabled, whether the change has reached it.

**COL-343** The account-level roles shown shall be only these five: external, customer,
partner, customer admin and partner admin, so an internal role never leaks into a customer
list.

**COL-344** A contact shall be marked as **granting case access** only when a user account
is linked, its address matches the address the contact was invited under, and the contact
is **registered**. This lets a screen show whether a person can really open the project's
cases.

**COL-345** Deactivated contacts shall not be listed. (A deactivated person can be invited
again through the membership process.)

**COL-346** Notifications are always shown as **on**: there is no setting stored for it.
**Confirm** (7.4).

**COL-347** A signed-in person is required to look up contacts, except for an internal
machine caller.

Inviting, changing roles of and deactivating contacts belong to project membership and are
not covered here.

## 6. Chat conversations and the link to a case

A **conversation** is a chat a customer started in the portal (for example with the AI
assistant). A case can be created from one.

**COL-360** A person shall be able to start a conversation in a project with an opening
message (required). Its subject is the first 100 characters of that message, its state is
**Active**, and it is recorded as started by the signed-in person. A customer may start one
in a project they belong to.

**COL-361** A conversation's state may be changed to **Active**, **Resolved**, **Converted**,
**Abandoned** or **Closed**. **Open** cannot be set. The change records who made it and when.
A person who is not a member of the conversation's project gets "not found", not a silent
success.

**COL-362** Conversations shall be searchable by project, by last-updated date range (the
end must not be before the start), and by "started by me" (which needs a signed-in
person), in pages, and one conversation can be fetched by id.

**COL-363** A case may be created **from** a conversation by naming it. The name shall be a
valid id, and the conversation shall exist **in the same project** as the new case. A
conversation that does not exist and one in another project shall get the **same** message,
so nobody can use it to find other projects' conversations.

**COL-364** The link shall be stored, and the case shall return the conversation's id and
subject, so a screen can show the chat history with the case. This applies to cases created
from now on; older cases have no stored link.

**COL-365** A case may also name a **related case** it was created from, which shall also be
a valid id and shall be shown on the new case.

## 7. Open questions for the product owner

1. **Tag names are shared across all projects.** Tag search is not limited to a project, so
   a customer searching tags could see labels staff used on other customers' cases.
2. **A duplicate tag can appear** if two people add the same new label at the same moment.
   Harmless but cosmetic.
3. **Customers can change tags and watchers** on their own project's cases. Is that
   intended for tags?
4. **Contact notifications** are always shown on, since no preference is stored.
5. **List and email use different bars.** A customer watcher is shown on the list while their
   contact is not deactivated, but is emailed only while it is **registered**, so someone
   invited but not yet registered is listed and not emailed.
6. **The onboarding owner** gets emails but is not shown as a watcher, so staff cannot see
   why they are copied.

## 8. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Tags | `internal/service/case_service.go` (tag functions), `internal/repository/case_repo.go` |
| Watch list | `internal/service/case_service.go` (watch list functions, create checks), `internal/repository/case_repo.go` |
| Account contacts | `internal/service/account_contact_service.go`, `internal/repository/account_contact_repo.go` |
| Project contacts | `internal/service/project_contact_service.go`, `internal/repository/project_contact_repo.go` |
| Conversations | `internal/service/conversation_service.go`, `internal/repository/conversation_repo.go` |
| Routes | `POST /cases/{id}/tags`, `DELETE /cases/{id}/tags/{tagId}`, `POST` and `GET /tags/search`, `PATCH /cases/{id}` (watch list), `POST /accounts/{id}/contacts/search`, `POST /projects/{id}/contacts/search`, `GET /projects/{id}/contacts/{contactId}`, `POST /conversations`, `POST /conversations/search`, `GET`/`PATCH /conversations/{id}` |
| Row-level security | migrations `100008` (tags, watchers), `100016` (watchers on announcements), `100020` (conversations) |
| Tables | `tag` and `work_item_tag` (0026), `work_item_watcher` (0042), `conversation` (0057) |
| Tests | `case_service_change_request_tags_test.go`, `case_watchlist_create_test.go`, `project_contact_account_roles_test.go`, `conversation_service_test.go` |
