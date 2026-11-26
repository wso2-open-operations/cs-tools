# Case comments and attachments — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 10.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Prefix:** `COL` (case collaboration). This document uses `COL-001` to `COL-099`.

---

## 1. Purpose

People talk about a case in two ways: they write **comments** and they attach **files**.
This document covers both: what can be written, who can see it, what it sets off (emails,
state changes, clocks), and how files and pictures are added, limited, listed and removed.

The same comment rules apply to the other kinds of record that carry a conversation
(service requests, announcements, change requests, incidents and chat conversations),
unless a section says otherwise. Escalation work notes are covered in
[case-escalation.md](case-escalation.md).

## 2. Who is involved

| Who | What they do |
|---|---|
| Customer user | Comments on cases in their own projects; adds files and pictures there. Never sees internal notes. |
| CS engineer | Comments publicly or leaves internal work notes; edits and removes comments (subject to the portal's own rule, section 5). |
| CSM admin | Sees the real text of a removed comment; may remove an attachment someone else uploaded (section 9). |
| Watchers and stakeholders | Receive an email when a comment is added (section 6). |
| The AI assistant | Can post replies on a customer's behalf in a chat, shown under its own name. |
| Portal backends | Hold the actual files and ask this service where they belong (section 8). |

## 3. Comments: what can be written

**COL-001** The service shall support three kinds of comment: a **comment** (visible to the
customer), a **work note** (internal only) and an **activity** entry (history written by the
system, never by a person).
*Check:* try to post a comment of type "activity"; it is refused.

**COL-002** A comment shall have non-empty content. An empty comment shall be refused.

**COL-003** A comment shall be attached to one record that the caller can see. The record
types that accept comments are: cases (which also covers service requests, engagements,
security report analyses and announcements), chat conversations, change requests and
incidents. A reference to any other kind of record shall be refused.
*Check:* comment on a case the caller cannot see; the answer is "not found".

**COL-004** The author of a comment shall be the signed-in person making the request, never
a name sent in the body. The only exception is the AI assistant: a comment may be recorded
as written by "agent", which then shows under the assistant's name. Any other author value
shall be refused.
*Check:* send a comment naming someone else as author; refused.

**COL-005** A machine client that is on the service's trusted list may post a comment on
behalf of a named person. A request that carries both a signed-in person and such a name
shall be refused, and an untrusted client naming someone shall be refused.

**COL-006** Adding a comment to a case shall also record it on the case as activity: the
case's "last updated" time and "updated by" are moved to the comment's author and time.
For change requests, chat conversations and incidents this does not happen, so a comment
never overwrites who last changed them.
*Check:* comment on a case; its last-updated time moves forward.

**COL-007** When a case is created with a description, the description shall be stored as
the case's first customer-visible comment, written as the creator. Announcements are the
exception: their description is the body and is not repeated as a comment.

**COL-008** Text wrapped in a single `[code]…[/code]` pair (a leftover from older records)
shall be shown without the wrapper. The stored text is not changed.

## 4. Who can see and write comments

**COL-010** Work notes shall be internal only. A customer shall not be able to read a work
note, write one, or edit one, and the database itself enforces this, not only the portal.
*Check:* a customer asks for a case's comments; no work notes come back, even from a caller
that forgot to filter.

**COL-011** A customer shall only see and write comments on records in a project they are a
registered contact of. For an announcement, they must also be allowed to see that
announcement (section 4 of [announcements.md](announcements.md)).

**COL-012** Internal staff shall see all comments of every type on every record.

**COL-013** A comment on a change request the caller may not see shall behave as if the
change request does not exist.

## 5. Reading, editing and removing

**COL-020** A case's comments shall be returned newest first, in pages, optionally limited
to one type, with the author's name, whether the author is internal staff, and the time.

**COL-021** A case's activity feed shall merge comments, completed attachments and,
if asked, changes to the case's fields into one timeline, newest first. A caller may ask
to leave work notes out, and the total shall then leave them out as well.
*Check:* ask for the feed with work notes excluded; neither rows nor the total count them.

**COL-022** The activity feed shall only work for case-like records. For anything else, such
as a change request, it shall answer "not found".

**COL-023** An attachment shall appear in the feed once its upload is complete, with the
file name shown in place of text. Uploads in progress, failed or removed files shall not
appear.

**COL-030** An internal caller shall be able to fix (edit) a comment's text. The previous
text shall be kept in an edit history with who changed it and when, and the comment shall
show the time it was last edited.
*Check:* edit a comment twice; its history holds both earlier versions.

**COL-031** An internal caller shall be able to remove a comment. Removal is a "soft"
delete: the comment stays but is marked removed, with who removed it and when. Removing a
comment twice shall be refused as a conflict. A removed comment can no longer be edited.

**COL-032** A removed comment shall be shown as follows:
- to a customer (and to anyone coming through the customer portal, even staff), not at all
  and not counted;
- to internal staff in the general comment search, as the text `[deleted]`, except that a
  CSM admin sees the real text;
- to internal staff in a case's own comment list and activity feed, as it was written.
**Confirm** (section 10.2).

**COL-033** The service itself does not decide whether a person may edit or remove a
particular comment. The CSM portal does: the author may, and so may holders of the admin
or comment-updater role. Editing, removing, reading one comment and reading its history are
available to internal callers only.

**COL-034** Edits and removals shall not be copied to the legacy case system, because it
does not allow its notes to be changed.

## 6. What a comment sets off

**COL-040** When a comment is added to a case that belongs to a project, the service shall
publish an event so that people are emailed. The recipients are the case's watch list
(minus anyone who is no longer eligible, section 4 of
[tags-watchers-contacts.md](tags-watchers-contacts.md)) and the account's named
stakeholders.

**COL-041** For a work note, only addresses in the company's own domain shall be included.
If nobody is left, no event is sent. A case with no project, or with nobody to email,
publishes nothing.
*Check:* a work note on a case watched by a customer; the customer's address is not a
recipient.

**COL-042** The event shall say whether the comment is an internal note and whether it is a
reply from a CS engineer, so the notification service can word it and decide on alerts.

**COL-043** A CS engineer's public reply shall stop the case's first-response clock.
A work note, a customer comment or a reply from anyone else shall not. Who counts as a CS
engineer is set by configuration.

**COL-044** When a customer comments (public comment, not a work note) on a case that is
**Awaiting Info** or **Solution Proposed**, the case shall move to **Waiting on WSO2**.
Staff comments, work notes and comments in any other state shall not move it. The move
goes through the normal case update, so the usual status event, clock effects and activity
entry follow, attributed to the customer.
*Check:* a customer replies on an Awaiting Info case; it becomes Waiting on WSO2.

**COL-045** A reply relayed from GitHub shall move an **Awaiting Info** case to Waiting on
WSO2 even though no customer wrote it in the portal.

**COL-046** The person who comments shall be added to the case's watch list if they are
not already on it. This is best effort and never fails the comment.

**COL-047** Each side effect above is best effort. A failure to email, move the state,
stop a clock or add a watcher shall be logged and shall never undo or fail the comment.

**COL-048** A comment on a service request shall also publish a service-request comment
event (see [call-requests-and-service-requests.md](call-requests-and-service-requests.md)),
independent of the email event above.

## 7. Pictures inside a comment

**COL-050** A comment or a case description may show up to **5** pictures. More shall be
refused. Pictures pasted straight into the text (embedded in it) count toward the same
limits.

**COL-051** The pictures in one comment or description shall together be no larger than a
set size (default **1 MB**, configurable up to 25 MB). Because there is no separate
per-picture limit, one picture can be at most that size too.

**COL-052** A new picture added to a comment shall be a PNG, JPEG or WebP. GIF and SVG
shall be refused for new pictures (SVG can carry script). Pictures already stored in other
types are still shown.

**COL-053** A picture in a **case description** shall have been uploaded by the person
creating the case, for the right project, shall be fully uploaded, and shall not already be
used elsewhere. A description that still points at an unfinished upload shall be refused
with a message to remove and add it again.

**COL-054** A picture in a **comment** only counts, and is only attached to the comment, if
the comment's author uploaded it for that same case. Someone else's picture, or one from
another case, is left out, so a comment cannot claim it.

**COL-055** The limits shall be checked before the comment is saved. Once saved, the
pictures are linked to the comment. When a comment is edited, pictures it already had still
count toward the number but not toward the size limit, so older comments with larger
pictures can still be edited.

**COL-056** A picture that was uploaded but never used in a saved comment shall be removed
after **24 hours**. Until then it is visible only to the person who uploaded it.

**COL-057** A picture of a work note stays internal, like the note itself.

**COL-058** A picture cannot be deleted on its own; it goes with its comment.

## 8. Attaching files

Files are not stored by this service. The portal backends keep them in cloud storage, and
this service decides whether an upload is allowed, records it, and tells the backend which
storage path to use.

**COL-060** Adding a file shall be a two-step process: **start** (name, type, size) and
**confirm** (what was actually stored). Until confirmed, the file is "pending" and nobody
sees it.

**COL-061** A file may be attached to a case or other work item, to a deployment, or, as a
picture, to a comment. Anything else shall be refused.

**COL-062** At start, the service shall refuse the upload when:
- the file name is empty or longer than 255 characters;
- the size is zero or more than the limit (default **10 MB**, configurable up to 100 MB;
  for a comment picture, the picture limit in COL-051);
- the content type is not of the form type/subtype (a missing type is stored as a generic
  binary file);
- the parent record cannot be seen by the caller or does not exist.

**COL-063** A file or picture may be started **before** its case exists, against a project,
so it can be attached when the case is created. A start shall name either the parent or the
project, never both. At most **10** files can be added when creating a record.

**COL-064** No file shall be added to a **closed** case, and none shall be confirmed on a
case that was closed after the upload started.
*Check:* start an upload, close the case, confirm; refused and the file is marked failed.

**COL-065** On confirm the service shall check the reported size against the size declared
at start and against the limit, and, for a picture, that it really is a PNG, JPEG or WebP.
If any check fails, the file is marked **failed** and the backend removes what it stored.

**COL-066** Only the person who started an upload can confirm it. Confirming twice is
harmless and returns the same result.

**COL-067** A caller may start or look up at most **60** files per minute. Beyond that the
service shall answer "too many requests". The count is kept per running copy of the
service. **Confirm** (10.7).

**COL-068** When a customer's file is confirmed on a case that is **Awaiting Info** or
**Solution Proposed**, the case shall move to Waiting on WSO2, by the same rule as a
customer comment (COL-044). Only a complete file on the case itself counts, and a repeated
confirm shall not repeat the move.

## 9. Seeing, downloading and removing files

**COL-070** A file's details shall be returned only to a caller who can see its parent
record: internal staff always, customers for their own projects. An attachment of a work
note stays internal.

**COL-071** The attachment list for a case, change request, incident, conversation or
deployment shall give details only (the files are read from storage by the backends), in
pages, and shall leave out files still uploading, failed or removed.

**COL-072** A caller shall be able to ask where a file is stored, so the backend can serve
it. An unknown or invisible file shall be "not found"; looking up counts toward the
per-minute limit.

**COL-073** A picture that was removed shall answer "gone", not "not found", so the portal
can show that it was removed.

**COL-074** A file shall be removable by the person who uploaded it. Nobody else may remove
it, except the CSM portal backend acting for a portal admin.
*Check:* a second customer tries to remove someone else's file; refused.

**COL-075** A file cannot be removed from a **closed** case, and a file that is not yet
stored cannot be removed. Removing one already removed does nothing.

**COL-076** Removal marks the file **deleted** and records who did it. Deleted files stay
in the database.

**COL-077** The CSM portal (and only the CSM portal backend) may ask for the list of
**deleted** attachments of a record, so staff can see what was removed.

**COL-078** Starting, confirming and removing a file shall be written to the audit log
with who did it, the file and, for a removal, whether an admin did it on someone's behalf.

**COL-079** A scheduled clean-up, available to internal services only, shall:
- mark uploads still pending after **24 hours** as failed (up to 200 per run);
- hand over unused comment pictures older than 24 hours for the backend to delete from
  storage, and mark them deleted only once the backend confirms;
- treat a security-report draft file as usable for 24 hours, and hand it over for removal an
  hour after that.

## 10. Open questions for the product owner

1. **Edit and remove rights live in the portal.** The service trusts any internal caller to
   edit or remove any comment. Is that acceptable for other callers of this service?
2. **A removed comment's text.** The general search shows `[deleted]` to staff, but a
   case's own comment list and activity feed show staff the original text. Should they match?
3. **Work-note emails go only to company-domain addresses.** An internal person on another
   domain would not be told. Should this follow user type instead?
4. **No comment length limit** is enforced here. Is one wanted?
5. **State move on reply is not atomic.** A case closed in the instant between the check and
   the move could still be set to Waiting on WSO2.
6. **File types.** Ordinary attachments accept any declared type, with no allow-list or
   scan here. Is that covered elsewhere?
7. **The per-minute limit** is counted in each running copy of the service, so several
   copies allow more in total.
8. **A case with no project** (for example one raised from a GitHub issue) sends no comment
   email at all.

## 11. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Comment rules, author, edit, remove | `internal/service/comment_service.go`, `internal/repository/comment_repo.go` |
| Case comments, reply state move, watchers, events | `internal/service/case_service.go` (comment functions), `internal/handler/case_handler.go` |
| Activity feed, attachment list | `internal/service/case_service.go`, `internal/repository/case_repo.go` |
| Removed-comment visibility | `internal/repository/comment_visibility.go` |
| Files: start, confirm, remove, pictures | `internal/service/stored_attachment_service.go`, `stored_attachment_backend.go` |
| Upload clean-up | `internal/service/attachment_upload_cleanup_service.go` |
| Rate limit | `internal/service/share_limiter.go` |
| Routes | `POST /cases/{id}/comments`, `…/comments/search`, `…/activities/search`, `POST /comments` and `/comments/search`, `GET/PATCH/DELETE /comments/{id}`, `GET /comments/{id}/history`, `POST /attachments/search`, `POST /attachments/upload`, `…/upload/{id}/confirm`, `…/delete`, `…/locate`, `…/uploads/cleanup` |
| Row-level security | migrations `100022` (work notes), `100026` (attachments), `0131` (edit and remove) |
| Settings | `ATTACHMENT_MAX_MB`, `INLINE_IMAGES_TOTAL_MAX_MB`, the trusted machine clients, the CS engineer role |
| Portal rule for edit and remove | CSM portal backend `internal/handler/comments.go` |
| Tests | `comment_service_test.go`, `comment_repo_test.go`, `stored_attachment_service_test.go`, `attachment_upload_cleanup_service_test.go` |
