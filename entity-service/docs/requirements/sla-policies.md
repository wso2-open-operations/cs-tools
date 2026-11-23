# SLA policies — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check
are intended; they are collected in section 9.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related:** [sla-clocks.md](sla-clocks.md) (what a clock does once started),
[sla-status-and-reporting.md](sla-status-and-reporting.md), and the storage-level view in
[../specs/sla.md](../specs/sla.md).

---

## 1. Purpose

A policy is the written promise: "for cases like this, the first reply is due within so
long". The service reads policies, decides which of them apply to a case or incident, and
starts a clock for each one. This document describes how a policy is chosen and how its
rules are read. It does not describe how a clock behaves once running; see
[sla-clocks.md](sla-clocks.md).

## 2. Where policies come from

**SLA-210** The service shall use the policies held in its shared policy table. That table
is filled by the synchronisation from the legacy case system. The service has **no
operation to create, edit or retire a policy**: policies change only through the
synchronisation or a database change.
*Check:* no route in the service writes a policy.

**SLA-211** A policy shall be considered only if it is **active** (a policy with no active
flag counts as active), has a **duration longer than zero**, and has a **target** (response,
workaround or resolution).
*Check:* a policy with a zero duration never starts a clock.

**SLA-212** A policy shall carry: a name, one target, a duration, a schedule, a
*retroactive* flag, five rules (start, stop, pause, resume, cancel), and the two modes that
say how cancelling and resuming are decided ([sla-clocks.md](sla-clocks.md), SLA-041 and
SLA-042).

## 3. The facts a rule can refer to

**SLA-220** For a **case**, a rule may refer to these facts, read fresh from the case
when the rule is judged:

| Fact | Notes |
|---|---|
| State | Open, Work in progress, Waiting on WSO2, Awaiting info, Reopened, Solution proposed, Closed. |
| Priority | The case's severity: S0 Catastrophic, S1 Critical, S2 High, S3 Medium, S4 Low. A case with no severity has no priority. |
| Case type | Case, engagement, service request, security report analysis, announcement. A rule naming any other type never matches. |
| Project type | The type of the case's project, for example a subscription. |
| Product | The case's deployed product. |
| Has an account | Whether the case belongs to an account. |
| CS engineer has replied | A real public reply from a CS engineer; the copy of the description posted as the first comment does not count. |
| Workaround provided | Whether a workaround has been recorded. |
| Fix ETA shared | True when any of the best-, most-likely- or worst-case fix ETA is set. |
| Created date | When the case was created. |

**SLA-221** For an **incident**, a rule may refer to: state (New, In progress, On hold,
Resolved, Closed, Canceled), priority, category, the service it affects, and the created
date.

**SLA-222** When a change is being decided, the new value of the thing that just changed
shall be used, even if the stored copy has not caught up yet.
*Check:* moving a case to Awaiting info is judged as Awaiting info, not as its previous
state.

## 4. Which policies apply

**SLA-230** For each of the three targets the service shall list the candidate policies
and keep those whose **start rule** is true for the case right now.

**SLA-231** Every matching policy shall get its own clock, whether or not another policy
already matched for the same target. They are not ranked or merged.
*Check:* two matching resolution policies produce two resolution clocks.

**SLA-232** A policy shall be used for **cases** only if its rules refer to at least one
case-specific fact (case type, account, product, project type, CS engineer reply,
workaround provided, fix ETA, created date) **and** to none of: business service,
category, escalation severity, source table, or active escalation.

**SLA-233** A policy shall be used for **incidents** only if its rules refer to business
service or category **and** to none of the case-specific facts above or to escalation
severity, source table or active escalation. (**Confirm**, 9.1)

**SLA-234** A policy that mentions only state and priority is used for neither cases nor
incidents. (**Confirm**, 9.1)

**SLA-235** A policy that is not meant for this kind of item shall be skipped without a
warning. A policy whose start
rule mentions something the service does not understand shall never match, and a warning
shall be logged naming the unknown part, so a gap is visible and never silently counted
as a match.

**SLA-236** A policy shall start its clock only when the policy's own stop rule is not
already true ([sla-clocks.md](sla-clocks.md), SLA-031).

### Retroactive policies

**SLA-240** A policy marked retroactive starts its clock at the case's creation time, not
at the moment the clock is created ([sla-clocks.md](sla-clocks.md), SLA-032).

**SLA-241** A severity change or a case-type transfer always starts its replacement
clocks "now", even for a retroactive policy.

## 5. The rule language

Rules are short text expressions, evaluated by the service and written the same way for
every policy.

**SLA-250** The service shall understand: *equals* and *not equals*; *is one of* and
*is not one of* a list; *is empty* and *is not empty*; *on or after* a date; and the
always-true word `TRUE`.

**SLA-251** Several conditions are combined as: all must hold (`^`); a condition may be
OR'd with the one before it (`^OR`); and a rule may list alternatives, any of which is
enough (`^NQ`). An alternative is a set of conditions that all hold.
*Check:* "state is Open and priority is S1, or state is Open and case type is hosting"
matches an Open S1 case and an Open hosting case.

**SLA-252** Comparisons shall ignore letter case. A condition on a fact the service does
not know shall be false, never an error.

**SLA-253** Rules that the synchronisation stored as the legacy system's numeric codes
shall be translated into readable labels **every time they are used**, not once. The
synchronisation rewrites the stored text on every run, so a one-off rewrite would be
undone.

**SLA-254** Case rules and incident rules shall be translated separately, because the
same code means different things for each (for example the legacy code for a closed case
and an incident that is on hold are the same number).

## 6. The incident policy

**SLA-260** Today there is one incident policy, "Cloud Incident - High Priority": a
**resolution** clock of **4 hours**, **24 x 7**.
- It starts for **any service** when the incident's priority is **Critical or High** and
  its category is **Service Interruption**.
- It is **met** (stopped) when the incident is put On hold, Resolved or Closed.
- It is **cancelled** when the incident is Canceled, when its category changes away from
  Service Interruption, or when its priority drops below High.
- It is **never paused**. On hold ends the clock as met rather than pausing it.
(**Confirm**, 9.2)

**SLA-261** A change of the affected service alone revises this clock: it is cancelled
and registered again, as SLA-100 states. The service condition that used to restrict
it to two named services was removed.

**SLA-262** No response or workaround policy exists for incidents today, so incidents
carry no response or workaround clock.

## 7. Schedules and duration

**SLA-270** A policy's schedule shall be one of **24 x 7** or **WSO2 Business days**
(see [sla-clocks.md](sla-clocks.md), section 10). Any other value is counted as 24 x 7
and logged.

**SLA-271** A clock's allowed time is the policy's duration at the time the clock is
read ([sla-clocks.md](sla-clocks.md), SLA-084).

## 8. Duration list for alerts

A small read-only list of "how long is the promise for each severity and kind of clock",
used to tell people how much response time is left.

**SLA-280** The service shall offer this list to **internal callers only**.

**SLA-281** The list shall contain, for each of the five severities and each of the
three kinds of clock, the allowed time in whole seconds. The severity is the upper-case
word (CATASTROPHIC, CRITICAL, HIGH, MEDIUM, LOW), and the kind of clock is lower-case
(response, workaround, resolution). There are at most 15 entries.

**SLA-282** Each entry shall be the policy that would apply to a plain, brand-new case of
that severity: type case, state Open, a subscription project, no account, product,
workaround or fix ETA, and created now. A policy needing anything else (a particular
product, an account, a hosting case type) therefore never contributes.

**SLA-283** If more than one policy matches, the one whose start rule **names that
severity** wins over one that applies to every severity. If still tied, the lowest policy
id wins, and a warning is logged.

**SLA-284** A severity and kind of clock with no matching policy shall be left out. There
is no fallback duration.
*Check:* a severity with only a response policy returns one entry, not three.

**SLA-285** The list is computed from the live policies each time, not from a separate
copy.

## 9. Open questions for the product owner

1. **A policy that mentions only state and priority is used for neither cases nor
   incidents.** Whether a policy applies to cases is inferred from the *words* in its rules,
   not from a stored field. A policy written without any case- or incident-specific term
   would silently apply to nothing. Is there a stored "applies to" field that should be used?
2. **Cloud incident policy.** Its rule was rewritten directly on the staging database and
   was not applied to production as part of that change. Confirm the production state, and
   whether restricting it to two named services is no longer wanted.
3. **No way to edit policies in the service.** Changing a promise needs a synchronisation
   or a database change. Is an admin screen wanted?
4. **The duration list assumes a subscription project.** A promise that differs for other
   project types is not shown in alert emails.

## 10. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Choosing policies | `internal/service/sla_policy_resolver.go` |
| Candidate policies (source, active, duration) | `internal/repository/sla_engine_repo.go` (`ListCandidatePolicies`) |
| Case and incident facts | `internal/repository/sla_engine_repo.go` (`GetCaseFacts`, `GetIncidentFacts`) |
| Rule language | `internal/slacondition/condition.go` |
| Code translation | `internal/service/sn_sla_condition_translate.go`, `incident_sla_condition_translate.go` |
| Incident policy | migration `0293` |
| Duration list | `internal/service/sla_duration_policy_service.go`, `GET /sla-duration-policy` |
| Tests | `sla_policy_resolver_test.go`, `sla_duration_policy_service_test.go`, `*_sla_condition_translate_test.go`, `internal/slacondition/condition_test.go` |
