# Saved dashboards and saved filter views — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check are
intended; they are collected in section 10.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

---

## 1. Purpose

CS engineers build their own dashboards in the CSM portal, save them, and share them with
colleagues. A shared dashboard can reach people one by one, whole teams, or anyone holding a
link. A person can also save the filters they use on the long lists (cases, incidents, change
requests, problems) under a name, so they can come back to them.

The service stores the dashboards and filter views, decides who may see and change them,
and cleans up when a dashboard's owner leaves. The portal draws the dashboards; the service
treats a dashboard's content as an opaque block of text.

## 2. Who is involved

| Who | What they do |
|---|---|
| Owner | The person who created a dashboard. Edits, deletes and shares it. |
| Recipient | An internal person a dashboard is shared with, directly, through a team, or through a link. Can view it, nothing more. |
| Team member | An internal member of a team a dashboard is shared with. |

Every action here is done as a signed-in person: the service works out who they are from
their user token, and refuses a call with no token.

## 3. My dashboards

**CUS-201** The service shall let a person list their own dashboards, by name (alphabetical,
ignoring letter case), without the dashboards' content.
*Check:* two saved dashboards come back as two entries with an id, name and times and no
definition.

**CUS-202** The service shall let a person open one of their own dashboards, with its content.
For a dashboard that is not theirs, or does not exist, the answer is the same "not found", so
nobody can probe which ids exist.

**CUS-203** The service shall let a person save a dashboard under an id they choose (a UUID):
it creates the dashboard if they have none with that id and replaces it if they do. If the id
belongs to somebody else's dashboard, the answer is "not found".

**CUS-204** A dashboard needs a name and a definition. The name is trimmed and must be 1 to
255 characters; the definition must not be empty and must be at most 65,536 bytes; neither
may contain a NUL character. The definition is stored exactly as sent and is not inspected.

**CUS-205** A person shall not have two dashboards with the same name, ignoring letter case.
A clash is refused as a conflict.

**CUS-206** A person may have at most **50** dashboards. Saving a 51st is refused; replacing
an existing one is never refused because of the limit. Two saves at the same moment cannot both
get past the limit.

**CUS-207** The service shall let a person delete their own dashboard; deleting one that is
not theirs is "not found".

**CUS-208** A malformed id is refused as a bad request rather than failing in the database.

## 4. Sharing with people

**CUS-210** The owner shall be able to see who a dashboard is shared with: each person's name
and email and when it was shared, alphabetical by name.

**CUS-211** The owner shall be able to share a dashboard with a person. The person must exist
and must be **internal staff**, and must not be the owner. Sharing with someone already shared
with changes nothing. The answer is the full list of people after the change.
*Check:* sharing with a customer user is refused; sharing with yourself is refused.

**CUS-212** The owner shall be able to stop sharing with a person. Stopping a share that does
not exist is "not found".

**CUS-213** Only the owner can see or change a dashboard's shares. For anyone else the answer
is "not found".

## 5. Sharing with teams

**CUS-220** The owner shall be able to list the teams a dashboard is shared with, alphabetical.

**CUS-221** The owner shall be able to share a dashboard with a team and to stop sharing it.
The team must exist; an unknown team is "not found". Sharing again changes nothing; stopping a
share that does not exist is "not found".

**CUS-222** Who can see a team-shared dashboard is decided **at the moment they look**, from the
team's current **internal** members. Someone who joins the team sees it; someone who leaves
stops seeing it. There is no copy to clean up.

## 6. Sharing by link

**CUS-230** The owner shall be able to turn "anyone with the link can view" on or off. Turning
it on gives the dashboard a new random link id (different from the dashboard's own id, so the
link cannot be guessed from it). Turning it on when it is already on keeps the same link.
Turning it off removes the link id.

**CUS-231** The owner shall be able to see the link's state: whether it is on, the link id, and
how many people have opened it.

**CUS-232** Turning the link **off**, and then on again, shall make every earlier copy of the
link stop working, because the link id changes.

**CUS-233** Any **internal** person who opens a live link shall be given the dashboard, and it
then appears in their shared list. A link that is unknown, switched off, or opened by someone
who is not internal staff gives the same "not found" answer, so nobody can tell whether a link
ever existed. The link id travels in the request body, not the address, so it stays out of
access logs.

**CUS-234** Turning the link off shall also take the dashboard away from everyone who had it
**only** through the link. People who have it by a direct or team share keep it.

**CUS-235** The owner opening their own link is told it is theirs; nothing is recorded.

## 7. Dashboards shared with me

**CUS-240** A person shall be able to list the dashboards shared with them, directly, through a
team they are in, or through a link they opened, alphabetical, with the owner, when it was
shared, and which kind of share gives them access.

**CUS-241** If a dashboard reaches a person in more than one way, it is listed once, and the
reason shown is: a direct share first, then a team (first by name), then the link.

**CUS-242** A person shall be able to open one shared dashboard with its content. Shared
dashboards are **view only**: there is no route that lets a recipient change or delete one.
One that is not shared with them is "not found".

**CUS-243** A person's own dashboards are never listed as shared with them.

## 8. When the owner leaves

**CUS-250** When a user is deleted, their dashboards that are shared with nobody shall be
deleted with them, since no one can see them any more.

**CUS-251** Their dashboards that **are** shared with a person or a team shall be kept, with no
owner. Nobody can then edit, delete or share them further.

**CUS-252** If such a dashboard's link was on, it shall be switched off first, and the people who
had it only through the link lose it. No link to a dashboard may keep working once nobody can
turn it off.

**CUS-253** A kept, ownerless dashboard shall be deleted automatically as soon as the last
person or team it was shared with is gone (deleted, or removed), since nobody can see it any more.

## 9. Saved filter views

**CUS-260** A person shall be able to save the filters of a list under a name, for one of four
lists: cases, incidents, change requests, or problems. Any other list is refused.

**CUS-261** The name is trimmed, required, and at most 255 characters. Saving under an existing
name (ignoring letter case) **replaces** that view.

**CUS-262** A person may have at most **50** saved views per list. A new one beyond that is
refused; replacing one is not.

**CUS-263** A person shall be able to list their saved views for a list, delete one by name, and
reorder them: by moving one up or down, or to a given position (zero or more).

**CUS-264** Every change returns the full list of views after the change. Views are private to
the person who saved them.

## 10. Open questions

1. **Dashboards are not limited to internal staff.** The dashboard routes check that the caller
   is a signed-in person with a user record, but not that they are internal. A customer who
   reaches this service could save their own dashboards. Only sharing and link-opening are
   limited to internal staff. Is that intended, or should the whole feature be internal only?
2. **A team share is not checked for people.** The team has to exist, but it may be empty or
   have no internal members; what a recipient sees is decided later from real membership.
   Confirm that sharing with such a team should simply do nothing.
3. **A saved dashboard's content is never checked.** A badly formed definition is stored and
   fails only when a portal tries to draw it.
4. **Ownerless dashboards cannot be fixed by anyone.** Once the owner is deleted, a shared
   dashboard can only disappear when its recipients do. There is no way for an admin to take it
   over or remove it.

## 11. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Own dashboards, limits, validation | `internal/service/custom_dashboard_service.go`, `internal/repository/custom_dashboard_repo.go` |
| Sharing with people | `internal/service/custom_dashboard_share_service.go`, `internal/repository/custom_dashboard_share_repo.go` |
| Sharing with teams | `internal/service/custom_dashboard_team_share_service.go`, `internal/repository/custom_dashboard_team_share_repo.go` |
| Link sharing | `internal/service/custom_dashboard_link_service.go`, `internal/repository/custom_dashboard_link_repo.go` |
| Limits and field sizes | `internal/domain/custom_dashboard.go` |
| Routes | `/users/me/dashboards` (list, get, save, delete, `/shares`, `/team-shares`, `/link`), `/users/me/shared-dashboards` (list, get, `/link`) |
| Owner-leaves clean-up | migrations `0255`, `0276`, `0277` |
| Tables | migrations `0251`, `0252`, `0253`, `0275` |
| Saved filter views | `internal/service/saved_filter_view_service.go`; routes `/users/me/saved-filter-views` |
| Tests | `custom_dashboard_*_test.go` (service, handler, and database tests) |
