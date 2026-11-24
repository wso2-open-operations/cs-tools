# Skills — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 9.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related:** [access-control.md](access-control.md) (how "CSM admin" is decided for these
routes), [users.md](users.md).

---

## 1. Purpose

Skills let the CS organisation record what each engineer is good at, so that managers can
find the right person for a case or a rota. There is a **catalogue** of skills kept by
administrators, and each engineer keeps a **profile** of the skills they hold, each with a
level. Skills exist only in the customer database.

## 2. Who is involved

| Who | What they do |
|---|---|
| Any internal staff member | Reads the catalogue; keeps their own skills profile and its history. |
| CRE or SRE team lead | Also searches everyone's skills. |
| CSM admin | Also creates and changes catalogue skills, and searches everyone's skills. |

Customers and service credentials cannot use any skills route.

## 3. Concepts

- A **skill** has a name, an optional description and search keywords, any number of
  **categories**, an active flag, and a **level scale**.
- A **level scale** is an ordered set of levels (each with a name, a numeric value, a
  description and a colour) that a skill is rated on. Different skills may use different
  scales.
- A person's **profile** is the list of skills they hold, each at one level. A skill may be
  one the person added themselves, or **inherited** from a group they belong to; inherited
  skills are shown but cannot be changed here.
- A **history** records every change to a person's profile: what was added, re-levelled or
  removed, from which level to which, by whom and when.

## 4. Access

**USR-400** Every skills route shall be for internal staff only. A customer, or a request
with no person (a service credential), shall be refused as forbidden.

**USR-401** Reading the catalogue, the level scales and the categories shall be allowed for
any internal staff member.

**USR-402** A person's own profile and history routes shall need a person's token and an
**active internal user record** for that person; otherwise they are refused ("no active
internal user record for the caller").

**USR-403** Changing the catalogue (create, change) shall be for a **CSM admin** only: a
person whose token carries one of the configured admin roles (see access-control.md).

**USR-404** Searching everyone's skills shall be for a CSM admin **or** the lead (including
the Americas team lead) of a CRE or SRE team who is active internal staff. Anyone else is
refused with a message naming those two groups.

## 5. The catalogue

**USR-410** The service shall list the level scales (each with its levels) and the skill
categories.

**USR-411** The service shall search skills with these optional filters, combined with
AND: free text, categories, a level scale, and "include inactive". Inactive skills are left
out unless asked for. Category and scale ids must be valid identifiers; at most 200 ids may
be given in a list.

**USR-412** Paging for skill search: default 50, maximum 200; a larger size is refused.

**USR-413** The service shall return one skill by id. An id that is not valid is a bad
request; an unknown skill is "not found".

**USR-414** An administrator shall be able to **create** a skill. Required: a name
(trimmed, at most 255 characters) and a level scale that exists. Optional: description (at
most 4000 characters), keywords (at most 500), categories (all must exist; duplicates
are folded) and an active flag. The skill is created active unless told otherwise.

**USR-415** Skill names shall be unique. Creating or renaming a skill to a name another
skill already has is refused as a conflict ("another skill already has this name").

**USR-416** An administrator shall be able to **change** a skill: name, description,
keywords, active flag, level scale and categories. At least one field must be given, and
the same limits as creation apply.

**USR-417** A skill's level scale shall not be changed while people hold levels of that
skill. The change is refused as a conflict.
*Check:* once any engineer has rated themselves on a skill, its scale is fixed.

**USR-418** Making a skill inactive shall stop it being added to profiles. It stays on the
profiles of people who already hold it.

**USR-419** There is no delete: a skill that is no longer wanted is made inactive.

## 6. A person's own skills profile

**USR-420** The service shall return the caller's profile: the skills they hold, each with
its level, whether the skill is still active, whether it is inherited, and when it last
changed.

**USR-421** A person shall be able to **add** a skill to their profile by giving the skill
and a level. The level is required and must be a valid level. The skill must exist, be
active, and have a level scale, and the level must belong to that scale.

**USR-422** Adding a skill the person already holds shall be refused ("change its level
instead"). Adding a skill they hold through a group shall be refused as inherited.

**USR-423** A person shall be able to **change the level** of a skill they hold. The level
must belong to the skill's scale. A skill they do not hold is "not found" ("not on your
profile"); an inherited skill cannot be changed.

**USR-424** A person shall be able to **remove** a skill they added themselves. A skill not
on their profile is "not found"; an inherited skill cannot be removed here.

**USR-425** If a skill's scale was changed or the skill removed while the request was being
handled, the person shall be told to choose a level of its current scale, or that the skill
was not found, rather than getting a generic failure.

**USR-426** A person's skills are only ever changed through their own profile routes: no
route changes another person's skills.

**USR-427** Skills added or changed shall be attributed to the person (their email) and
timed.

## 7. History

**USR-430** Every add, level change and removal on a person's profile shall be recorded in
their history with the skill, the level before (none when added), the level after (none when
removed), who did it and when. The record is written in the same step as the change.

**USR-431** A person shall be able to read their own history, optionally for one skill, with
paging: default 20, maximum 100; a larger size is refused.

## 8. Everyone's skills (managers)

**USR-440** A manager (USR-404) shall be able to search across people. Filters: free text,
people, skills, levels, a minimum level value (not negative), and team keys. Lists hold at
most 200 values each. Each result is one person-and-skill pair with the person's name and
email, their teams, the skill, the level, whether inherited, and when it changed.

**USR-441** Team keys are compared ignoring case and surrounding spaces; blank ones are
ignored.

**USR-442** Paging as for the catalogue (default 50, maximum 200).

## 9. Open questions for the product owner

1. **The admin check is a role in the person's token.** It relies on the admin role list
   being set identically here and in the CSM portal; if it is not, a real admin gets a
   refusal. Other admin features trust the portal's own check instead
   (see access-control.md). Should skills move to that model?
2. **Anyone on staff reads every skill and level scale** but only managers see who holds
   what. Is a person's profile meant to be private from peers?
3. **No way to remove an inactive or unwanted skill from other people's profiles.** A skill
   made inactive stays on the profiles that hold it, and only its holders can remove it.
4. **Inherited skills are read-only here** and come from data loaded from elsewhere. There
   is no route that creates or changes inheritance, so it cannot be corrected from this
   service.
5. **A person with no active internal record (for example a newly joined engineer not yet
   set up) cannot keep a profile**, and is told only that no record exists.

## 10. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Rules and validation | `internal/service/skill_service.go` |
| Queries, history, manager check | `internal/repository/skill_repo.go` |
| Routes | `GET /skills/levels`, `GET /skills/categories`, `POST /skills/search`, `GET /skills/{id}`, `POST /skills`, `PATCH /skills/{id}`, `GET/POST /skills/me`, `PUT/DELETE /skills/me/{skillId}`, `GET /skills/me/history`, `POST /skills/users/search` |
| Tables | migration `0079_skill_tables.sql` |
| Settings | `AUTH_ADMIN_ROLES` (the admin role list) |
| Tests | `skill_service_test.go`, `skill_repo_test.go` |
