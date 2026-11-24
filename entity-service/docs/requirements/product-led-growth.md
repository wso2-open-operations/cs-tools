# Product-led growth (PLG) — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check are
intended; they are collected in section 12.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

---

## 1. Purpose

Customers can sign up for some cloud platforms on their own. The CS team follows those sign-ups:
who registered, how far each organisation has got with each platform, whether it is healthy, what
it pays, and what the team is doing about it. The PLG feature holds that data and the playbooks the
team follows.

The service **stores the data, keeps it consistent, and guards each change**. The product's own
rules (what a failed guard means to a person, who may move what) belong to the CSM portal backend,
which is the only caller. Every PLG route starts with `/plg/`.

## 2. Who is involved

| Who | What they do |
|---|---|
| CS engineer | Acknowledges registrations, owns organisations, moves them through their life cycle, follows playbooks, writes notes. |
| CSM portal backend | The only caller of these routes. It decides who may do what and turns a failed guard into a message. |
| The registration feed | A poller in the portal backend that sends sign-ups and reports failures. |

## 3. Key terms

- **Organisation**: a customer that has registered. It has a country and a CS **owner**.
- **Pairing**: one organisation together with one **platform** (product). Life cycle, health,
  subscription and trial dates belong to the pairing, not to the organisation: the same customer can
  be commercial on one platform and just registered on another.
- **Life cycle stage**: Registration, PLG CS eligible, First value achieved, Activated, Commercial,
  or **Abandoned**.
- **Health**: Healthy or At risk, set by an engineer, independent of the stage.
- **Subscription tier**: Free, Trial, Trial extended, or Pay as you go.
- **Playbook**: a template of tasks. A **run** is a copy of a playbook attached to a pairing.

## 4. Reference data and people

**CUS-701** The service shall list the platforms and the life cycle stages (with their names and
order) for the portal's menus.

**CUS-702** The service shall let the portal search the CS team. The search covers **internal staff
only**, whatever is asked for. The caller decides whether to include people who have left: an owner
picker asks for active staff only, while an attribution lookup does not, so a note by someone who
has since left still shows their name.

**CUS-703** The people search accepts ids and email addresses (each a valid id, or text with an @) and
user types from the known list; anything else is refused with the field named.

## 5. Organisations

**CUS-710** The service shall let the portal search organisations, in pages, by name, text, platform,
life cycle stage, subscription tier, owner (by id or by email, or **unowned**), country (by name, or
**unknown**), a registration date range, and by the answer given to a playbook task. Platform, stage
and tier match an organisation when **any** of its pairings matches.

**CUS-711** The filters are checked: stages and tiers must be known values; owner ids must be valid
ids; the end of a date range may not be before its start; a country name is 1 to 100 characters and
cannot be combined with "unknown country"; an owner email is at most 254 characters and cannot be
combined with "unowned"; a playbook answer filter needs a playbook task and cannot be combined with
"unanswered".

**CUS-712** A page is 25 by default and **at most 100**. A larger limit is reduced to 100, not
refused, and the answer says the limit it applied.

**CUS-713** The service shall count the organisations a search matches **by owner** or **by country**,
showing at most 50 groups (0 meaning all); more than 50 is refused.

**CUS-714** The service shall return one organisation in full, with its owner's name and email.

**CUS-715** The service shall let the portal set an organisation's owner and shall return the whole
organisation afterwards. The change is attributed to a person (section 11). The service does **not**
refuse to clear the owner; that rule belongs to the portal backend. (**Confirm**, see 12.2)

## 6. Registrations

A **registration** is a pairing that has not yet been acknowledged.

**CUS-720** The service shall let the portal search registrations (optionally only unacknowledged
ones, with the organisation filters above), count them by country or by the answer to a playbook
task, and show the registration volume per day, week or month for a window. The trend uses the same
windows and grouping as the case volume trend.

**CUS-721** **Acknowledging** a registration shall claim it and assign the organisation's CS owner,
and shall apply **only while it is still unacknowledged**: if somebody acknowledged it first, nothing
changes and the answer says no row was affected, which the portal reports as a conflict.
*Check:* acknowledge twice; the second answers "nothing changed".

## 7. The product tab (one organisation, one platform)

**CUS-730** The service shall return one pairing for display: its stage and when it entered it, its
health and who last set it, the kinds of playbook it is currently offered and the playbooks
available, its tier and trial dates, who acknowledged it and when, its attached runs with their
progress, its notes and its stage history. A platform may be named by code or id.

**CUS-731** The kinds of playbook a pairing is offered follow from its health: a **healthy** pairing is
offered **progressive** and **sustaining** playbooks; an **at-risk** pairing is offered only **recovery**
playbooks.

**CUS-732** The service shall let the portal change a pairing's stage, health, tier, and trial dates.
Any combination may be changed in one request.

**CUS-733** A change of **stage, health or tier** needs a **reason**; a blank reason is refused.

**CUS-734** A pairing moves **forward only** along Registration, PLG CS eligible, First value achieved,
Activated, Commercial. It may leave to **Abandoned** from anywhere, and Abandoned is final. A move
backwards, or out of Abandoned, is refused with a sentence saying so. The database enforces the same
rule, and wins if the two ever disagree.

**CUS-735** The caller may say which stage it **believed** the pairing was in. The change then applies
only while that is still true; if someone moved it first, nothing changes and the portal reports a
conflict. Without that statement the change is made regardless.

**CUS-736** Every change of stage shall write a **history** row with the reason, in the same step, so the
history can never disagree with the stage.

**CUS-737** Tier and dates: the tier must be known; trial dates are "YYYY-MM-DD" or empty; a trial end
date can be cleared. Only the Trial and Trial extended tiers have an end date.

**CUS-738** The service shall answer where a pairing lives (its organisation and platform) so the portal
can address it, and shall describe a run task's declared type and answers so the portal can refuse a
value it cannot hold before writing.

## 8. Playbooks and runs

### Templates

**CUS-740** The service shall list playbooks, all or for one platform, and open one.

**CUS-741** The service shall let the portal create a playbook for a platform: a name, a stage, a kind
(progressive, recovery or sustaining) and its tasks. The stage and kind must be known. A playbook
**cannot sit at Abandoned**, since nothing progresses out of it. The database refuses a playbook for a
platform and stage that has no path.

**CUS-742** A task has a type: **tick** (yes or no), **text**, **number**, **single choice** or
**checklist**. The two list types need a list of answers and nothing else may carry one. A tick is
the only type a bookend task may use. A checklist counts as answered when **any** reason is ticked.

**CUS-743** A progressive playbook carries a pairing to the **next** stage; the destination is
derived from the stage it sits at, not entered. A recovery playbook carries a pairing from at-risk
back to healthy without moving its stage. A sustaining playbook keeps a healthy pairing healthy where
it is.

**CUS-744** The service shall let the portal change a playbook's name, description, stage, kind and
active flag, replace its whole task list, and delete it. Replacing the tasks affects **new** runs only:
runs already attached are copies and are never touched.

### Runs

**CUS-750** **Attaching** a playbook to a pairing shall copy the template's **active** tasks onto a new run,
with their types. Attaching the same playbook again changes nothing. A playbook that does not exist or
is inactive is refused.

**CUS-751** **Detaching** a run shall record who did it. A run that is already **closed** cannot be
detached, so a recorded outcome is never erased.

**CUS-752** Recording a task's value is **last writer wins**, with no precondition. The value must suit
the task's type; whether the task is completed follows from the value automatically, and the time and
person who completed it are recorded and cleared with it.

## 9. Notes

**CUS-760** The service shall let the portal add a note to a pairing; the body must not be empty.

**CUS-761** Only the note's **author** may correct it, and the body must not be empty. A correction keeps
the superseded body as a revision, so the note's history can be replayed in order.

## 10. The work queue and the dashboard

**CUS-770** The service shall let the portal search the **work queue**: the pairings that need attention,
each with the reason (in progress, not started, or no playbook), filtered by owner, organisation,
playbook, stage and reason. Unknown reasons or stages, and ids that are not valid, are refused.

**CUS-771** The service shall return the dashboard figures for an optional date range: total
organisations, total registrations (pairings), new registrations, pairings needing attention, trials
ending soon, unresolved ingest failures, plus registrations by platform and by stage, the subscription
mix, and registrations over time. New registrations, needing attention and unresolved failures are
**never** limited by the date range, so a backlog cannot be hidden by narrowing it.

**CUS-772** The "pairings needing attention" figure shall come from the same view as the work queue,
so the tile and the page it links to always agree.

## 11. The registration feed and attribution

**CUS-780** The service shall accept a batch of registrations from the feed. Each registration is saved
**in its own step** (person, organisation, pairing and extra attributes together), so one bad record never
rejects the rest. The answer lists each record's outcome and counts how many were accepted and how many
failed.

**CUS-781** A registration delivered again shall do nothing: the organisation's name and its company id
are unique, so a repeat is recognised.

**CUS-782** A registration that cannot be saved shall be logged and recorded as an **ingest failure**,
with the whole record kept so it can be recovered; it is never lost.

**CUS-783** The service shall let the feed record a failure on its own (a reason is required; the event id,
type and time are optional) and the dashboard shall count the unresolved ones.

**CUS-790** Every change in this feature shall be attributed to a **person** whose id (a valid user id) is
sent with the request. A change with a missing or invalid person is refused, so nothing is written with
nobody behind it. This applies to owner changes, acknowledging, pairing changes, attaching and detaching
runs, recording task values, notes, and playbook changes.

**CUS-791** Each change shall be logged with its outcome, the caller's identity, and the person it was
done for, using ids and codes only: never names, emails or note text.

## 12. Open questions

1. **No access control in this service.** The PLG routes check neither that the caller is the portal
   backend nor that the person is internal. The person who a change is attributed to is taken from the
   request, not verified against the caller. Everything depends on the gateway and the portal backend.
   Is that intended, or should these routes trust only the portal backend, as portal banners do?
2. **An organisation can be left without an owner.** The service stores an empty owner if asked. The
   portal backend is meant to refuse it. If another caller appears, the work queue would show orphans.
3. **Page sizes are clamped silently** (to 100) here, while elsewhere in the service an oversized limit is
   refused. Callers may not notice they got fewer rows.
4. **Playbook deletion has no stated rule** about runs already using it in this service; what happens
   depends on the database.
5. **Subscription tier is entered by hand**; there is no subscription feed, so it can drift from what the
   customer actually pays.
6. **The "unknown" and "unowned" filters** cannot be combined with named countries or owners, which may
   surprise a user building a broad filter.

## 13. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Route list and wiring | `internal/server/plg_routes.go` |
| Reference data, people | `internal/service/plg_reference_service.go`, `internal/service/plg_users_service.go` |
| Organisations | `internal/service/plg_organization_service.go`, `internal/repository/plg_organization_repo.go` |
| Pairings, registrations, notes, runs | `internal/service/plg_pairing_service.go`, `internal/service/plg_pairing_writes.go`, `internal/repository/plg_orgplatform_repo.go` |
| Playbooks | `internal/service/plg_playbook_service.go`, `internal/service/plg_playbook_writes.go`, `internal/repository/plg_playbook_repo.go` |
| Registration feed | `internal/service/plg_ingest_service.go`, `internal/repository/plg_ingest_repo.go`, `internal/repository/plg_failure_repo.go` |
| Work queue, dashboard | `internal/service/plg_analytics_service.go`, `internal/repository/plg_analytics_repo.go` |
| Rules and enums | `internal/domain/plg_entity.go`, `internal/service/plg_validate.go` |
| Attribution and logging | `internal/service/plg_audit.go` |
| Tests | `plg_*_test.go` (service and repository) |
