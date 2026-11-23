# Users — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 12.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related:** who can see and do what is in [access-control.md](access-control.md); how a
customer becomes a project member is in
[memberships-and-onboarding.md](memberships-and-onboarding.md).

---

## 1. Purpose

Everyone who signs in to a portal, and every person that cases, escalations and rotas
refer to, has one **user** record. This document covers how users are classified, looked
up, created and updated, and how teams and groups relate to them. It describes the
database-backed user service. Where a request names a "token", that is the signed-in
person's identity token (see access-control.md).

## 2. Who is involved

| Who | What they do |
|---|---|
| Any signed-in person | Reads their own profile, changes their own time zone and phone number. |
| Portal backends | Look people up (search, by id, by list of ids) on behalf of a screen. |
| CS engineers and admins (through the CSM portal) | Create users and edit another person's profile. |

## 3. User types

**USR-010** Every user shall have a type: **internal** (WSO2 staff), **customer** (a
customer or partner contact), **system** (a service account), or **not available**.
*Check:* a new user with no roles has no usable type; granting them a customer role makes
them a customer.

**USR-011** The type shall be worked out from the person's roles, not typed in. A system
flag makes the user *system*. Otherwise the roles `admin` or `internal` make them
*internal*; any of `external`, `partner`, `customer`, `partner_admin`, `customer_admin`
make them *customer*; no matching role makes them *not available*. The type shall be
recomputed whenever the person's roles change, or when a role is renamed.
*Check:* remove a person's last internal role; their type changes without anyone editing
the user.

**USR-012** An internal role wins over a customer role if a person somehow holds both.

**USR-013** Responses shall show the customer type as "customer". A user whose type is
*not available* shows no type at all.

**USR-014** A user may be **active** or **deactivated**. A user with no recorded active
flag shall count as active; only an explicit "deactivated" deactivates.
*Check:* a user row with an empty flag appears in an "active only" search.

## 4. Reading a user

**USR-020** The service shall return one user by id: name, user name, email, phone, type,
active flag, created and updated times, the person's roles, and the teams they belong to.
An id that is not a valid identifier shall be refused; an id with no user shall be "not
found".

**USR-021** The display name shall be the user's full name when one is stored, else first
and last name joined, else the user name.

**USR-022** A phone number that is empty or only spaces shall not be returned.

**USR-023** For a user who is **not internal** and has an email, the response shall also
list that person's **project access**: one entry for each project they have been invited
to, with the project, the email used for the invitation, whether a contact record is linked,
the registration state, their project roles, and whether that membership currently grants
access to the project's cases.
*Check:* an invited-but-not-yet-registered contact lists the project but shows that it does
not grant case access; once registered it does.

**USR-024** Only a registered membership shall be shown as granting case access. Invited,
re-invited and deactivated memberships do not.

## 5. Searching users

**USR-030** The service shall search users and return a page of results with the total
count and whether more results exist.

**USR-031** Paging: default page size 10, maximum 50; a larger size shall be refused. A
negative offset is treated as 0.

**USR-032** Filters (all combine with AND; each list matches any value in it):
- free text, matched anywhere in the user name or email, ignoring case (up to 200
  characters, longer refused);
- user names, and emails, each an exact match, up to 50 values each;
- role names, up to 50 values (see USR-033);
- user ids, group ids and team ids (each must be valid identifiers);
- team names, ignoring case;
- active: "true" matches active and unset users; "false" matches only explicitly
  deactivated users.
*Check:* filtering active = false does not return a user whose flag was never set.

**USR-033** A role filter shall match both the plain name and a name carrying a prefix
before a dot, so a caller can send `timecard_approver` and still find people who hold the
prefixed form.

**USR-034** Sorting: by name, created time or updated time, ascending or descending.
Sorting by name ignores case and falls back from display name to first and last name to
user name. With no sort the newest user comes first. A sort order without a field, or an
unknown field or order, shall be refused. Ties are broken the same way on every page.

**USR-035** Each result shall include the person's roles (an empty list when none).

**USR-036** A search that matches nobody shall return an empty list, not an error.

## 6. Looking up many users by id

**USR-040** The service shall return the users for a list of ids in one call.

**USR-041** An id that is not a valid identifier shall be silently left out rather than
failing the whole call, because callers build the list from records whose "user" reference
is sometimes free text (for example an email address). Valid ids with no user are simply
absent from the answer.
*Check:* send three ids, one malformed; the call succeeds and returns the matches for the
other two.

**USR-042** An empty list returns an empty answer.

## 7. The signed-in person's own profile

**USR-050** The service shall return the caller's own profile: id, email, first and last
name, time zone, phone, type, roles and teams. The person is found by the email in their
token, ignoring case. Roles come from the user's stored roles, not from the token.

**USR-051** The caller needs a token; without one the answer is "unauthorized", and an
unreadable token is a bad request. A person with no user record gets "not found".

**USR-052** If the team lookup fails the profile shall still be returned, with an empty
team list.

**USR-053** The person is matched on the exact email in their token first. Failing that,
a single record that differs only in letter case is used (plain English letters only). If
several records differ only in case and none matches exactly, nobody is resolved. A
person is therefore never resolved to someone else's record by a case fold.

**USR-054** A person may change their own **time zone** and **phone**. At least one must
be given. Only the caller's own record can be changed this way; no id is accepted.

**USR-055** A phone number shall be trimmed, at most 32 characters, and in international
format: a plus sign, then 8 to 15 digits, the first not zero, no spaces or dashes. An empty
value clears the phone. A time zone left blank is ignored.
*Check:* `+14155552671` is accepted; `0412 345 678` is refused; an empty phone removes the
stored number.

**USR-056** The answer shall confirm the change with the person's id, who made it and when,
and the resulting time zone and phone.

## 8. Creating a user

**USR-060** The service shall create a user from an email, a first and/or last name, and
optionally roles. The new user is active. The user name is the email, lower-cased.
The call needs the caller's token, whose email is recorded as the creator.

**USR-061** The email shall be a valid address. At least one of first name and last name
is required. At most 50 roles may be given.

**USR-062** A role that makes the person internal (`admin` or `internal`) shall only be
accepted for an address on the WSO2 email domain.
*Check:* creating an internal user with a personal address is refused with a message that
names the required domain.

**USR-063** A role that makes the person a customer shall be refused for now ("creating an
external-type user is not available at this time"). Customers appear through project
invitations instead (see memberships-and-onboarding.md).

**USR-064** A role name that has not been set up in the system shall refuse the whole
request as unavailable, and nothing is created. The user and their roles are saved
together or not at all.

**USR-065** A second user with the same email shall be refused as a conflict.

**USR-066** The response shall show the created user, including the roles granted and the
type they resolved to.

**USR-067** A caller that is the CSM portal backend acting for a WSO2-domain person may
ask for the creation to be recorded as made by the system rather than by that person. Any
other caller sending the same request has the flag ignored (not refused).

## 9. Editing another person's profile

**USR-070** An internal caller shall be able to change another person's time zone and/or
phone (same rules as USR-054 to USR-056, at least one required). A caller who is not
internal shall be refused. A user id that is not valid is refused; an id with no user is
"not found".

**USR-071** The response shall name who made the change: the caller's email when it is
available, otherwise "internal".

## 10. Teams and groups

A **team** is a group of staff with roles inside it (for example lead or engineer). A
**group** is an assignable support group. A person's "groups" in their profile are their
teams.

**USR-080** The service shall list the members of a team, with each person's name, email
and role in the team. This needs a signed-in person; an unknown team is "not found".

**USR-081** The service shall search teams by name (text up to 200 characters, normal
paging up to 50), and search groups the same way. Groups can be narrowed to the support
groups an incident may be created in, or to the configured extra assignable groups (the
extra-groups filter wins if both are sent).

**USR-082** Reading a single team's details, the teams' chat webhooks and the teams' group
emails shall be for internal callers only.

**USR-083** An internal caller shall be able to set a team's chat webhook. It must be an
https address on the chat provider's own host, at most 2048 characters; an empty value
clears it. At least one field is required.

**USR-084** An internal caller shall be able to name a team's **lead**. A team has one
lead: the previous lead becomes an engineer. The person must exist. Two people assigned at
the same moment are handled one after the other, so a team never ends with two leads.
*Check:* assign B as lead of a team led by A; A is now an engineer and B the only lead.

**USR-085** A group's membership shall be readable by internal callers only.

Rotas, leave and rota access for teams are described in
[../specs/team-schedule.md](../specs/team-schedule.md).

## 11. Speed-up cache

**USR-090** When a cache is configured, a user's detail and a person's own profile shall be
served from it for up to 10 minutes (adjustable). A failure of the cache shall never fail a
request: it is treated as a miss.

**USR-091** A failed lookup (including "not found") is never cached, so a user created a
moment later is found at once.

**USR-092** Creating a user, either profile edit, and every project membership write
shall clear the affected person's cached entries.

**USR-093** Cached entries hold no email addresses in their keys (the email is hashed), and
an entry is never served for a person whose email has since changed.

## 12. Open questions for the product owner

1. **Anyone can search and read users.** User search, lookup by id, lookup by list of
   ids and a single user's detail carry no per-caller check in this service; a customer
   reaching them through a portal can find any person's name and email, and a user's
   detail shows project access for non-staff. Is access meant to be limited elsewhere
   (the portal), or should customers be restricted to their own project's people?
2. **Creating a user does not check the caller is internal.** Any valid token holder can
   call it; only the role and domain rules above apply. The CSM portal is the only
   gate. Is that acceptable?
3. **A deactivated user can still read their profile.** "My profile" does not refuse
   deactivated users (other data endpoints do refuse them, see access-control.md).
4. **Role changes made directly in the database are not noticed by the cache** until its
   time runs out (10 minutes by default). Is that delay acceptable for role changes?
5. **An unknown time zone** is rejected by the database rather than by a clear message,
   so the caller sees a generic failure. Should the service validate it?
6. **No "last name only" rule on search text:** free text matches the user name and email
   only, not the person's display name. Intended?

## 13. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Rules, validation, profile, create | `internal/service/user_service.go` |
| Queries, filters, sorting, user type map | `internal/repository/user_repo.go` |
| User type from roles | migration `0011_users_add_user_type.sql` |
| Routes | `POST /users/search`, `POST /users/by-ids`, `GET /users/{id}`, `GET/PATCH /users/me`, `POST /users`, `PATCH /users/{id}` |
| Cache | `internal/service/cached_user_service.go`, `internal/cache/user_cache.go`; setting `USER_CACHE_TTL`, Redis connection settings |
| Teams and groups | `internal/service/team_service.go`, `group_service.go`, `group_detail_service.go`; routes `/teams/...`, `/groups/...` |
| Tests | `user_service_test.go`, `cached_user_service_test.go`, `team_service` and `group_*` tests |
