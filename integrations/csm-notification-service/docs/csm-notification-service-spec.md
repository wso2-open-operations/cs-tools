# CSM Notification Service — Specification

## 1. Purpose

This document describes what the CSM Notification Service does today: every
notification and alert it is responsible for, what triggers each one, who
receives it, and how it is delivered. It also sets out the data-handling
principle the service should follow going forward, and lists a few related
areas that are not yet covered but are natural candidates for the same
treatment.

## 2. How it fits together

Several platform services — the core entity service, the CSM portal backend,
the customer portal backend, and the change-flow service — each produce a
record of something that happened: a case was created, a comment was added,
an incident was reported, a change request moved forward, and so on. These
records are delivered to the CSM Notification Service through a shared message
stream, organized into a small number of topics by subject area (case-related
activity, incident and service-request activity, project onboarding, and
change-request activity).

The CSM Notification Service listens to that stream and is the only place that
turns a raw event into something a person actually sees — an email, a chat
card, a phone call, or a public status update. It does not expose a general
inbound API of its own; its only job is to consume events and deliver
notifications outward. One partial exception: the SLA-tracking logic inside
the service already reaches back into the core entity service on its own
schedule to double-check its records, which is useful context for how a
similar reach-back could work for other notifications in future (see section
4).

Delivery happens over four channels:

- **Email**, for anything that needs a durable, read-later record — most case
  and change-request activity.
- **Chat**, for anything that benefits from being seen quickly by a team —
  case creation and status changes, SLA warnings, incident alerts.
- **Voice calls**, for paging an on-call person or escalation chain when an
  incident needs immediate human attention.
- **A public status page update**, for outage communications meant for an
  external audience rather than a specific recipient list.

A failed delivery gets one retry pass; if that also fails, the record is set
aside rather than endlessly retried.

### 2.1 How chat notifications are routed today

Chat is not a single destination. Every chat alert is posted into one
specific chat space, and there are several genuinely different routing
mechanisms in play depending on which kind of alert it is — not one shared
rule with minor variations. This section documents each one precisely,
including the exact fallback order, because getting this wrong has real
operational consequences (a misconfigured or ambiguous entry doesn't error
loudly — it just quietly stops an alert from reaching anyone).

#### 2.1.1 The underlying mechanism: a list of named spaces

Most of the routing below (everything except incident alerts — see 2.1.4)
works against one shared, named list of chat spaces: a space is given a
name (an "audience" — a general monitoring space is one such name, and each
team that has its own space is another), and an alert names which one it
wants.

- Matching a requested name against the configured list is **case-sensitive**
  and only trims surrounding whitespace — two names that differ only in
  capitalization are treated as two unrelated, separate audiences.
- **A real, live gotcha worth knowing**: if the same audience name is
  configured twice (even accidentally, e.g. two config entries that both
  resolve to the same trimmed name), the service does not pick either one —
  it silently treats that whole audience as having **no space configured at
  all**, forever, with no startup error and no indication anywhere except a
  recurring "alert was not posted" log line whenever something tries to
  reach it. This is not a "last one wins" or "first one wins" situation —
  a genuine duplicate quietly disables the audience.
- When an alert asks for an audience with no configured space (whether
  because none was ever set up, or because of the duplicate-name trap
  above), the alert for that audience is simply dropped: logged as a
  warning, not retried, not redirected anywhere else, and not treated as a
  failure of the overall notification.
- Where an alert resolves to more than one audience at once (see 2.1.2), each
  one is attempted independently — one audience having no space configured
  does not stop the others from receiving their copy, and the overall
  outcome is only reported as failed if every one of them failed to post.

#### 2.1.2 Case-level alerts (fixed routing — no team awareness at all)

A case being created, a case being acknowledged, and a case's severity
changing all post to **one fixed, general monitoring audience**, every
time, with absolutely no team lookup involved — not even as a fallback,
because there is nothing to fall back from. This applies regardless of the
case's product or team. A case of the "security report analysis" type gets
its own distinct card layout, but it still goes to the same fixed
monitoring audience as every other case-created alert.

#### 2.1.3 Team-aware alerts: SLA warnings/breaches and customer-frustration alerts

Exactly two kinds of alert look at the case's own team before choosing
where to post: an SLA warning/breach alert, and a customer-frustration
alert. Both resolve their audience (or audiences — see below, this can
genuinely be more than one) through the same shared decision, applied in
this exact order:

1. **An evaluation-account case is a hard override, checked first, and
   nothing else below is evaluated for it at all.** If the case belongs to
   an evaluation-subscription project, the alert goes to a single, dedicated
   "Evaluation" audience and the decision stops there — team, onboarding
   status, time of day, and day of week are all irrelevant for an
   evaluation-account case.
2. **Otherwise, team-or-monitoring-space is decided next, and it is a strict
   either/or — exactly one of these two happens:**
   - If the case has a known team **and** that team has a configured chat
     space, the alert goes to that team's own space.
   - Otherwise (no team recorded, or a team recorded but with no space
     configured for it) — the alert goes to the general monitoring
     audience instead.
3. **An onboarding check runs next, independently of step 2, and adds a
   second audience on top rather than replacing the one from step 2.** If
   the case's project is still in its onboarding period (recorded as either
   "in progress" or "not yet started"), the alert is *also* sent to a
   dedicated onboarding audience — in addition to whatever step 2 produced,
   not instead of it.
4. **A late-night coverage window is checked next, again additive.** The
   current time, converted to Indian Standard Time, is checked against a
   fixed overnight window: **9:00 PM through 5:59 AM IST** (the window wraps
   past midnight — it is not a same-day range). If the alert fires inside
   that window, it is *also* sent to a dedicated overnight/"Americas
   coverage" audience, on top of everything already added.
5. **A weekend check runs last, also additive.** The weekend window is
   defined as **6:00 AM Saturday IST through 5:59 AM Monday IST** — that is,
   from Saturday morning onward, the whole of Sunday, and up to (but not
   including) 6 AM Monday. If the alert falls inside that window, the
   general monitoring audience is added once more — this has no visible
   effect if step 2 already chose the monitoring audience (it's the same
   name, so it doesn't duplicate), but if step 2 resolved to a real team's
   own space, the weekend check adds the monitoring audience as a genuine
   **second**, separate destination alongside that team's space.

Because steps 3–5 are additive rather than exclusive, a single alert can
legitimately resolve to **up to four distinct audiences at once** (for
example: a team's own space, plus onboarding, plus the overnight window,
plus — if it also happens to be a weekend — the monitoring space as well),
and **every one of those resolved audiences gets its own, independent
attempt to post the card** — there is no "first match wins" behavior
anywhere in this decision. A failure on one of them is logged on its own
and does not stop the others.

Two things are worth flagging about how complete this decision actually is
in practice for each alert type:

- The **frustration alert** has everything it needs to make this decision
  in full — the comment's own case data carries its team, its evaluation
  flag, and its onboarding status.
- The **SLA breach alert** does not: nothing published to this service any
  longer carries a case's evaluation-account flag or onboarding status by
  the time an SLA tracker needs them, so an SLA alert's version of this
  decision always treats the case as not an evaluation account and as
  having no onboarding status — meaning, in practice, an SLA breach on an
  evaluation or newly-onboarded account's case is only ever distinguishable
  from an ordinary one by its team-or-monitoring-space outcome (steps 1 and
  3 above never actually apply to an SLA alert today). This is a known,
  accepted gap, not a bug — it would need a publisher to start sending
  those two facts on the events this tracker already consumes.

#### 2.1.4 One more override just for frustration alerts: migration-type comments

Before the team-aware decision above even runs, a frustration alert checks
one more thing first: if the comment landed on a case that is specifically
an engagement of the "migration" kind, the entire decision in 2.1.3 above is
skipped, and the alert is sent to one single, dedicated "Migration" audience
instead — never a team space, never the monitoring space, never the
onboarding/overnight/weekend additions. This check runs ahead of
everything else in 2.1.3, so a migration-engagement comment never reaches
the evaluation-account check either, even if the case otherwise is on an
evaluation account.

#### 2.1.5 Incident alerts: a separate mechanism, with its own namespace

Incident-created and incident-assigned alerts are **not** routed through the
named-space list in 2.1.1 at all — they use a completely separate mapping
from the incident's own assignment group straight to a chat space.

- This mapping's own matching rule is different from every other audience
  lookup in this service: an assignment-group name is matched
  **case-insensitively** (lower-cased and trimmed), and the resulting
  audience key always carries a distinct prefix of its own — so an
  assignment group's incident space can never collide with (or
  accidentally match) an ordinary team name used elsewhere for case/SLA/SR
  alerts, even if the two happen to share the same text.
- **There is no monitoring-space fallback here at all.** An assignment group
  with no mapped space simply gets no chat alert for its incidents — not
  even the general monitoring audience other alert types fall back to.
- A single assignment group can be configured to fan its incidents out to
  one or more *other* groups' own mapped spaces instead of (or in addition
  to) its own — useful for an unassigned/default group whose incidents more
  than one team wants visibility into. This fan-out lookup is also
  case-insensitive on the source group name.
- When an incident's alert resolves to more than one space this way, posting
  is treated as **one logical delivery**: if at least one of the spaces
  receives the card, the whole thing counts as delivered and is not
  retried, even if another of the spaces failed (that individual failure is
  still logged). Only a case where *every* resolved space failed is
  retried as a whole.
- Every reply related to one incident (the creation card, an assignment
  reply) is threaded under the same running conversation per space, so a
  team watching its own space sees one continuous thread for the life of
  that incident.

#### 2.1.6 Service-request alerts: team-only, no fallback

A service request's created/acknowledged/customer-comment alerts all go
straight to the request's own team's space, matched case-sensitively
against the same named-space list case/SLA alerts use (not the
case-insensitive incident mapping in 2.1.5). There is no fallback of any
kind: a request with no team recorded, or a team with no configured space,
simply gets no chat alert — nothing is sent to the monitoring space, and
nothing is retried.

#### 2.1.7 The phone-call escalation chains

The call-escalation chains — the customer-facing one, the infrastructure
one, and the specialist hand-off chain that any of them can hand off to —
need somewhere to post a running record as they climb through rungs of
people to call, and they resolve their own chat destination through a
three-tier order of precedence, checked in exactly this sequence for every
posted update (each attempt, and the eventual outcome):

1. **Highest precedence: if the incident already has a mapped alert space
   (the same group-to-space mapping from 2.1.5), every update from the
   chain — each call attempt's card, the final "nobody answered" summary,
   and a specialist hand-off notice — is posted as a reply under that same
   original alert, in that same space (or spaces, if the original alert
   itself fanned out to more than one), rather than to any of the chain's
   own configured spaces at all.** This takes priority over everything
   below it and overrides a team's own dedicated escalation space entirely.
   The one exception is a chain that has been deliberately set to a
   dry-run/logging-only mode for testing — that mode never posts anywhere,
   regardless of this rule.
2. **Next, if the incident has no mapped alert space of its own:** the
   incident's team gets checked for a dedicated escalation-chain space of
   its own. If the team has one configured, that is used. But if the
   incident genuinely belongs to a real, recognized team that simply
   doesn't have a dedicated escalation space configured for it, the chain
   does **not** fall through to the "teamless" list below — it falls
   straight through to the chain's own shared default space (tier 3). The
   "teamless" fan-out only applies to an incident that has **no** recognized
   team at all (an unassigned/default group, a monitoring alert with no
   team, or a customer case escalated internally) — and in that case, it can
   genuinely post to more than one space at once, if more than one is
   configured to receive teamless alerts.
3. **Lowest precedence, the final fallback: the chain's own single, shared
   default space**, used whenever neither of the above applied.

A few more routing details specific to the escalation chains:

- **The closing "nobody answered" summary and the specialist hand-off
  notice both follow the identical three-tier rule above — including tier
  1's override.** Each has its own separate on/off setting per team, but
  that setting is only actually consulted when tier 1 doesn't apply (i.e.
  the incident has no mapped alert space of its own); when tier 1 does
  apply, the summary/notice is posted regardless of what the on/off setting
  says.
- **A separate, simpler on/off switch** governs the short "stopped"
  confirmation reply posted when a chain is cancelled or answered (defaults
  to on). Unlike the two above, this one does not have the tier-1 override
  baked in the same way, and it only ever posts at all if calling was
  actually enabled for the chain and at least one call had genuinely been
  placed before the chain stopped.
- **A specific team can be configured to use a more restricted delivery
  channel than the chain as a whole** — for example, the chain overall
  dials and posts to chat, but one particular team is set to receive chat
  updates only, or is set to a logging-only mode. This override can only
  take channels *away* from what the chain's own setting allows; it can
  never add a channel the chain itself doesn't use (a team cannot be
  granted phone calls if the chain as a whole is chat-only).

#### 2.1.8 Where this configuration actually lives, and what happens if it's wrong

The full picture of spaces, team mappings, and on/off toggles described
above starts from a static configuration file, but a portal settings screen
lets an administrator update a team's space, or flip one of the on/off
settings, directly — without editing or redeploying that file. This runs in
one of three modes:

- **Off** — the saved settings are never read at all; the static file alone
  governs everything.
- **Shadow (a cautious validation mode)** — the saved settings are read and
  compared against the file, and what they *would* change is logged, but
  the service still runs entirely on the static file. This exists so a
  change can be reviewed in the logs before it's trusted to take effect.
- **Live** — the saved settings are actually read and applied on top of the
  file at startup. Only what was explicitly saved replaces its file
  equivalent; anything never touched in the settings screen keeps whatever
  the file already says for it.

A few failure-handling details worth knowing, since they decide what a
deployment actually runs on when something goes wrong:

- If reading the saved settings fails when the service is meant to apply
  them live, it does **not** fall back to a generic built-in default. It
  first tries the last settings it successfully loaded and cached from a
  previous successful start — except that cached copy deliberately never
  includes the actual chat space addresses (only team/routing/timing
  values), so a fallback like this always uses the **file's own** chat
  spaces even while reusing cached routing values. If there is no usable
  cached copy at all, it falls all the way back to the static file,
  unchanged.
- If applying the saved settings would produce an invalid overall
  configuration, the whole attempt is discarded and the static file is used
  exactly as it is — never a partially-applied mix, and never the built-in
  default configuration.
- In short: the settings screen is the practical, day-to-day source of
  truth for where a given team's alerts currently go, but the static file
  is always the guaranteed fallback the service can never be left without.

### 2.2 How email recipients are determined

Who actually receives a case email is not a single list handed over as-is —
it is assembled from a few different sources, combined differently
depending on the kind of notification, and in some cases re-checked again
right before sending. This section documents that precisely, because the
rules differ in ways that matter (who's included, who's deliberately left
out, and when a stale recipient gets quietly dropped).

#### 2.2.1 The two sources behind an ordinary case's recipient list

For a normal case — created, commented on, reassigned, or changing status
or severity — the recipient list going out on that email is the union of
two separate sources, combined fresh every time, not stored as one combined
list anywhere:

1. **The case's own explicit watch list.** This is real, persisted state:
   whoever was deliberately added as a watcher, plus — as a convenience —
   whoever has ever posted a comment on the case is automatically added to
   it too, without disturbing anyone already on the list.
2. **The account's own default stakeholders**, resolved fresh from the
   account's current configuration every single time an email is about to
   go out — never stored as part of the case itself. These are four
   specific roles recorded against the case's account: its technical owner,
   its secondary technical owner, its account manager, and its renewal
   account manager. **The customer success manager is deliberately excluded
   from this set and does not receive ordinary case emails** — this is a
   conscious choice, not an oversight (the customer success manager is
   still shown as a locked, always-present entry on the case's own watcher
   list inside the portal, but that is a display-only convenience, not an
   email recipient).

Because the second source is resolved live rather than stored, reassigning
an account's technical owner (for example) takes effect immediately, on the
very next email sent for any of that account's cases — no case needs to be
touched individually for the change to apply.

#### 2.2.2 A watcher can be dropped right before sending, and a work note narrows further

Before most of these emails go out — a comment being added, a status
change, a severity change, or a reassignment, but deliberately **not** the
original case-creation email, since a freshly-added watcher can't have gone
stale within the same request that added them — every already-recorded
watcher on the explicit watch list is checked again for whether they still
belong:

- An internal/WSO2 staff watcher is always considered current, with no
  further check.
- An external/customer watcher is kept only if they are still an active,
  registered contact of the case's own project. Someone who has since left
  the project, or whose invitation was never completed, is quietly left off
  that one email — logged, but not actually removed from the stored watch
  list itself, so they'd be included again if their standing on the project
  changes back.
- If this check itself fails for some transient reason (rather than
  cleanly answering yes or no), the watcher is **kept**, not dropped — the
  service treats losing a real recipient to a glitch as the worse outcome
  compared to one extra, possibly-unwanted email.

Separately, when a comment is marked as an internal note (not meant to be
seen by the customer), the combined recipient list from 2.2.1 is narrowed
down to WSO2 addresses only, as an extra step layered on top of everything
above — so a customer who happens to be sitting on a case's watch list
never receives an internal note, regardless of how they got onto that list
in the first place.

#### 2.2.3 Announcements, and why a security announcement is narrower

An announcement is created the same way an ordinary case is — it rides the
exact same creation email described above — but its recipient list is
computed completely differently from 2.2.1:

- An **ordinary** announcement's audience is every one of the target
  project's own registered portal users.
- A **security** announcement (a distinct flag set at creation, not a
  different case type) narrows that audience down to only the project
  contacts specifically designated as that project's security contacts —
  nobody else on the project receives it.
- If a project genuinely has nobody holding the targeted role at all (no
  portal users on an ordinary announcement, or no designated security
  contact on a security one), the service does not give up and send
  nothing — it falls back to the same watch-list-plus-default-stakeholders
  combination an ordinary case would use (2.2.1), so the announcement still
  reaches someone connected to that project rather than silently reaching
  no one.
- Like every other non-case-type notification, an announcement is
  email-only — it never generates a chat alert, regardless of severity or
  whether it's a security announcement.

**This is separate from the internal review workflow an announcement goes
through before it is ever created per project** (draft → pending review →
approved → published). That review workflow itself sends no notification of
any kind — it exists purely to track the approval decision and to record
that publishing happened, and it does not decide, or keep a record of,
exactly which projects ended up receiving a case. The actual per-project
case creation — and therefore each project's own announcement email,
audience rules included — happens as a separate, later action once a
request has been approved.

#### 2.2.4 Case escalation: a cumulative, level-by-level recipient list

An escalation's recipient list is not a single lookup for whichever level
was just reached — it accumulates, level by level, so reaching a higher
level still includes everyone who would have been notified at every lower
level on the way up:

- **The first level** adds: the account's own team lead(s); a fixed list of
  people covering the Americas region for this level, included
  unconditionally regardless of the case's own time zone; the account's own
  manager; and its technical owner.
- **The second level** adds: a fixed Americas-region list for that tier,
  plus exactly one product-specific contact, chosen by what kind of product
  the case's deployment belongs to — a dedicated address for a
  service-category product, a separate dedicated address specifically for
  the Identity Server product, and a general default address for everything
  else.
- **The third level** adds: everyone holding the organization's "CRE head"
  role, plus the account's own customer success manager — **the one place
  the customer success manager is actually included in an email audience**,
  unlike every other case notification in this document, where they are
  deliberately excluded (2.2.1).
- **The fourth and fifth levels** each add everyone holding a dedicated,
  named escalation role for that exact level.
- **Whatever this cumulative list resolves to, it is always narrowed down
  to WSO2 addresses only before the email is sent** — an escalation can
  never reach a customer contact, even if one of the per-account fields it
  draws from (an account manager, a technical owner, the customer success
  manager) happens to point at someone outside WSO2.
- **De-escalating a case still notifies the full set of people who were
  originally notified on the way up to that level** — there is no separate,
  smaller "de-escalation" audience; moving back down simply reuses the same
  cumulative list the escalation itself built going up.
- An escalation step that resolves to no recipients at all simply sends no
  email, rather than failing or erroring.

#### 2.2.5 Delivery: one email per portal, not one email to everyone

Because a case's combined recipient list can genuinely mix customer
contacts with internal staff, the service does not send a single email
(or a single email with everyone included as a copy) to the whole list —
it works out which portal each individual recipient belongs to, and sends
one completely separate email per portal, each with the link appropriate to
that portal. A case whose audience spans both sides produces two distinct
emails, not one shared one.

- **Classification is role-based first.** A recipient's own recorded
  platform role decides which portal's email they end up in. Only when a
  recipient's role is unknown, or matches neither the customer-side nor the
  staff-side role sets — including when the recipient isn't a known
  platform user at all — does the service fall back to their email
  address's domain: a WSO2 address is treated as staff/internal, anything
  else is treated as a customer. Every time this domain fallback is used,
  it is logged as a warning, since it means the proper, role-based answer
  wasn't available.
- **A small, fixed list of default internal addresses is copied onto every
  case email's internal/staff copy only** — never the customer-facing copy,
  under any circumstance, including the service's own safe-testing mode
  (where that copy list is deliberately left off entirely, so a test run
  can never also land in a real, shared internal inbox).
- **An internal note gets one more, independent safety check on top of all
  of the above.** Whatever the per-portal split above produces, only the
  internal/staff copy is actually sent for an internal note — any
  customer-facing copy the split would otherwise have produced is discarded
  outright before anything goes out. This is a deliberate second line of
  defense, applied regardless of how the recipient list was already
  narrowed upstream (2.2.2) — specifically so an internal note can never
  reach a customer even if something earlier in the chain let a customer
  address through.

## 3. Notification catalog

This section lists every notification the service currently sends, grouped
by subject area.

### 3.1 Case activity

The case-created/acknowledged/severity-changed chat entries below always use
the fixed monitoring audience with no team lookup at all (section 2.1.2).
The frustration alert is the one case-activity notification that is
team-aware (section 2.1.3), with its own migration-case override
(section 2.1.4). "The case's recipient list" in the table below means the
combined, re-checked list built the way section 2.2.1–2.2.2 describes,
unless noted otherwise.

| Notification | What triggers it | Channel(s) | Who receives it |
|---|---|---|---|
| Case created | A new case is opened | Email, and chat unless the case is low severity or one of a few case types that are email-only | The case's recipient list; chat goes to a fixed monitoring space |
| Announcement created | An announcement case is created for a project | Email only, never chat | Every registered portal user on the project — or, for a security announcement, only that project's designated security contacts (section 2.2.3) |
| Comment added | A comment is posted on a case | Email, with a different layout for an internal-only note | The case's recipient list, narrowed to WSO2 addresses only for an internal note (section 2.2.2) |
| Status changed | A case's status changes | Email | The case's recipient list |
| Case assigned | A case gets a new assignee | Email | The case's recipient list |
| Case acknowledged | A case is claimed for the first time | Chat only, as a reply under the creation alert | A fixed monitoring space |
| Severity changed | A case's severity changes | Email and chat | The case's recipient list, and a fixed monitoring space |
| Workaround recorded | A workaround is marked as provided | None — this is an internal tracking signal only | — |
| Case escalated | A case moves to a higher escalation level | Email | A cumulative, level-by-level internal staff list, always narrowed to WSO2 addresses (section 2.2.4) |

A side reaction worth noting: a customer's comment is also checked by a
separate detection step for signs of frustration; if it crosses a
threshold, a chat alert is sent — to the case's own team space if one
exists, otherwise the general monitoring space, with the evaluation-account,
onboarding, overnight, and weekend additions described in section 2.1.3, and
the migration-case override in section 2.1.4 — without including the
customer's name or contact details on the alert itself.

### 3.2 Service-level agreement alerts

An SLA-tracking component inside the service watches each case's own
timeline and sends a chat alert when a case crosses a warning or breach
threshold (50%, 75%, and fully breached). These alerts follow the same
team-aware decision as the frustration alert above (section 2.1.3) — team
space first, general monitoring space otherwise, plus the onboarding/
overnight/weekend additions — except that an SLA alert is never able to
resolve the evaluation-account or onboarding-status checks in practice,
since the information those two checks need isn't available to this part
of the service (also noted in section 2.1.3).

**What starts a clock, and for how long, is decided entirely by the core
entity service's own SLA policy table — not a fixed table of durations per
severity.** Each case can have a Response, Workaround and/or Resolution
clock, and which of those actually start, their exact duration, and the
conditions that pause, resume, cancel or complete them, come from whichever
real policy definition matches the case's own type, severity and project at
the moment it's evaluated. A case can match more than one policy for the
same clock type at once (confirmed real: a case can legitimately run two
separate Resolution clocks side by side), in which case this service alerts
on each independently. Most clocks run on ordinary continuous time, but a
clock can instead run on a Monday-to-Friday business-day calendar (skipping
weekends entirely) when its policy says so — this changes how "50%
elapsed"/"breached" is calculated and when the alert actually fires, but
does not change anything about who receives it or where.

Pause and resume are driven by the same per-policy conditions, read live
from the core entity service rather than inferred from a fixed list of case
states here — a case moving to a particular status pauses a clock only if
that clock's own policy says that status should pause it. If the service
can't briefly reach the core entity service to check whether a clock is
really due (a transient outage), it retries a few times rather than sending
based on stale information; if it still can't confirm after retrying, the
alert is deferred to the next successful check rather than sent without
being able to verify it hasn't already been resolved — an outage delays an
alert, it never risks sending a duplicate or a false one.

One known limitation: the alert is meant to name the team's own lead, but
that information is not currently available to this part of the service, so
the alert only shows the team name.

### 3.3 Incidents

Incident alerts and the escalation chain use the assignment-group-mapped
spaces and thread-under-the-original-alert behavior described in sections
2.1.5 and 2.1.7 — a completely separate mechanism from the fixed monitoring
space or the team-first lookup the case alerts above use, with its own,
case-insensitive matching rule and no monitoring-space fallback at all.

| Notification | What triggers it | Channel(s) | Who receives it |
|---|---|---|---|
| Incident reported | A new incident is logged | A phone call reading out the incident summary, plus a chat card | The on-call number; the owning team's chat space |
| Incident assigned | An incident gets an assignee | Chat only, as a reply under the reported alert | The same thread |
| Incident acknowledged / priority raised / commented on / escalated to a specialist | Drives an escalation call sequence, not a single alert | A calling sequence through the relevant rota and leadership roles, with optional parallel chat postings | Whoever's turn it is in the escalation sequence |
| Test call | An administrator requests a one-off test call | A phone call | The specific person chosen for the test |

The escalation call sequence climbs through a defined order — rota members,
then team leads, then wider leadership — until someone acknowledges, assigns,
or responds publicly; any of those stops the sequence.

### 3.4 Change requests

| Notification | What triggers it | Channel | Who receives it |
|---|---|---|---|
| Approval requested | A change request enters an approval stage | Email | The relevant internal approval group, or the project's customer contacts, depending on whose approval is needed |
| Plan date notice | A planned implementation date is set or changed | Email | Same as above |

### 3.5 Service requests

These have no monitoring-space fallback — a service request's team must have
a configured space, matched case-sensitively like a case/SLA audience (not
the incident mechanism's case-insensitive matching), or its alerts simply
don't reach chat (see section 2.1.6).

| Notification | What triggers it | Channel | Who receives it |
|---|---|---|---|
| Service request created | A new service request is logged | Chat | The request's own team chat space |
| Service request acknowledged | The request is acknowledged | Chat | The same space |
| Customer comment on a service request | A customer adds a qualifying comment | Chat | The same space |

### 3.6 Outage communications

| Notification | What triggers it | Channel | Who receives it |
|---|---|---|---|
| Outage notice | A scheduled communication point is reached | Email | The outage's recipient list |
| Outage follow-up | A later communication point is reached | Email | The outage's recipient list |
| Status page update | A status change is due to be reflected publicly | A public status page post | The general public, not a specific recipient |

### 3.7 Project contact onboarding

| Notification | What triggers it | Channel | Who receives it |
|---|---|---|---|
| Contact invited | A new project membership is created | Account setup, then a welcome or reminder email depending on whether the person already has an account | The invited contact |
| Contact registered | The invited person signs in for the first time | A welcome email | The same contact |

## 4. Information handling

Every notification above is built from the underlying record of what
happened. As a general principle going forward, what travels on the shared
message stream between services should be limited to what is needed to route
and identify a notification — things like a case or incident reference
number, its type, its priority or severity, a status, and a timestamp. Names,
contact details, free-text content such as comments or descriptions, and
account or customer names are a different category: they are needed only at
the point a notification is actually rendered and sent, not by every service
that might read the shared stream.

The SLA-alerting component's existing habit of reaching back into the core
entity service for a handful of supporting facts, on its own schedule rather
than trusting the stream to carry everything, is a reasonable model for how
the rest of the catalog above could evolve: keep the shared stream to routing
information, and have the notification service fetch the richer details it
needs — a name, a comment body, an account name — only when it is about to
build and send that one notification. This both narrows how far sensitive
details travel and gives the notification service one place to apply
whatever access and retention rules apply to that data.

One practical exception worth keeping in mind: a few alerts exist purely to
dial a specific person on request (the test-call case in section 3.3). That
kind of alert is inherently about one named, chosen individual, so it does
not fit the same "route by reference, resolve details later" shape as the
rest of the catalog — it is better treated as a direct, one-off action
between an administrator and the notification service rather than a general
broadcast.

## 5. Gaps and future candidates

A few areas have real, tracked lifecycles in the wider platform but no
notification of any kind today. These are natural candidates to bring into
the same catalog once there is a need:

- **Timecards** — submission, approval, and rejection.
- **Customer-requested calls** — request, scheduling, and conclusion.

None of these currently generate any alert; a customer feedback rating is
similarly tracked but not currently alerted on, since it is primarily a
reporting metric rather than something that needs an immediate notification.

## 6. Open considerations

- The SLA alert's missing team-lead detail (section 3.2) is a concrete,
  near-term fix once that information can be resolved.
- Rolling out any change to what travels on the shared stream needs careful
  sequencing across every service that publishes to it and the notification
  service that reads it, so that older and newer formats can coexist during
  a transition rather than one side breaking the other.
- As the catalog grows (section 5), each new notification should be
  reviewed against the same information-handling principle in section 4
  before it ships, rather than after.
