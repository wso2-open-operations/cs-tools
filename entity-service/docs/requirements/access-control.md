# Access control — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 10.

**Audience:** product owners, CS leads, QA, security reviewers, and engineers new to the
service.

**Related:** [users.md](users.md) (user types and records),
[memberships-and-onboarding.md](memberships-and-onboarding.md) (how a customer gains
access to a project).

---

## 1. Purpose

The entity service holds data for every customer. Each request must therefore be tied to a
known caller, and the data returned must be limited to what that caller may see. This
document describes how the caller is identified, which kinds of caller exist, what each can
see, and where each "admin only" decision is made.

## 2. Kinds of caller

| Caller | How it arrives | What it can see |
|---|---|---|
| WSO2 staff (internal user) | Signed-in person, through a portal | Everything |
| Customer or partner contact | Signed-in person, through the customer portal | Only the projects they are a registered member of |
| CSM portal backend | The CSM portal's own service identity plus the signed-in person | Everything, if the person is on the WSO2 email domain; otherwise refused |
| Customer portal backend | The customer portal's service identity plus the signed-in person | Decided by the person, exactly as above |
| Machine caller (a listed internal service) | Its own service identity, no person needed | Everything |

## 3. Identifying the caller

Two credentials can arrive with a request:
- the **person's identity token**, issued by the identity provider; and
- the calling application's own **service assertion**, carrying its client id.

**USR-100** The service shall verify a person's identity token on every request that
carries one: its signature against the provider's published keys, the issuer, the expiry
(with a small clock tolerance, 30 seconds by default), and that its audience is one of the
accepted application ids. The token must also carry an email.
*Check:* a token that is expired, for another audience, or has no email is refused.

**USR-101** A request carrying a token that fails any of these checks shall be refused as
"unauthorized" with a plain message ("invalid or expired token"), whichever route it is
for. The reason is logged, not shown.

**USR-102** Only signing algorithms based on asymmetric keys shall be accepted, so a
token can never be "verified" by treating a public key as a shared secret.

**USR-103** The person's token may carry **portal roles**; the service shall read them
whether the identity provider sends one role or a list. A roles claim in any other shape
is treated as no roles, never as a failure.

**USR-104** A service assertion shall be read for its client id (taken from the client id
claim, else the authorized-party claim). A request whose assertion carries neither is
refused. The assertion is decoded but its signature is not re-checked here; the gateway in
front of the service is what authenticates the application. (**Confirm**, see 10.1)

**USR-105** Token validation is always on and cannot be switched off. The service shall
refuse to start unless the identity provider's address, its published-keys address and at
least one accepted audience are configured, and unless it can load at least one signing key.

**USR-106** A request with no token and no recognised service assertion has no identity.
Any route that needs one shall answer "unauthorized".

## 4. Deciding what a caller may see

The service turns the caller into a **scope**, in this order:

1. If the caller's client id is the **customer portal backend**: the scope is always
   decided by the signed-in person (step 4). A person is required; without one the answer
   is "unauthorized". This is never overridden by the same client id being listed
   anywhere else.
2. Otherwise, if it is the **CSM portal backend**: the person's email must end in the
   configured WSO2 portal domain (compared ignoring case, and matching the part after
   the "@", so a lookalike domain fails). Then the caller sees everything and is treated
   as internal. Otherwise the request is refused as forbidden.
3. Otherwise, if it is a **listed machine caller**: the caller sees everything,
   whatever person token it also carries (a person token is then used only to name who
   acted).
4. Otherwise the person's identity token alone decides (section 5).
5. With no person and no recognised service identity, the answer is "unauthorized".

**USR-110** The service shall follow the order above.
*Check:* a machine caller sending a customer's token still sees everything; the customer
portal backend sending a staff token sees what that staff person sees.

**USR-111** A request that reaches a scoped endpoint without a verified identity shall be
refused as unavailable rather than guessed at.

**USR-112** The scope shall be worked out once per request and reused by everything that
request does.

**USR-113** A background job or system action runs with an explicit "system" scope that a
caller can never request. A request's own scope is never quietly upgraded to system
because it passes through a background step.

**USR-114** Startup shall refuse these configurations: the CSM portal client id set
without its domain (or the reverse); the CSM portal and customer portal sharing one client
id. A CSM portal client id that is also in the machine caller list shall log a warning;
the CSM portal rules still apply to it.

## 5. What a signed-in person may see

The person is found by the email in their token among the user records.

**USR-120** Where several user records share one email, only **active** records count; a
record with no active flag counts as active.

**USR-121** A person whose active records are all internal shall see everything, and be
treated as staff.

**USR-122** A person with a **customer** record shall see only the projects they are a
registered member of (see USR-130). If the same email also has an active internal record,
the narrower customer view applies to what they can list, but they are still recognised as
staff where a decision is about being staff rather than about which data is visible.
*Check:* an email that is both a staff and a customer record lists only the customer's
projects, yet is still allowed staff-only actions such as filling in resolution details.

**USR-123** A person with any other mix (system or not-available records, or only
deactivated records) shall be refused as forbidden ("no access for this user"). A person
with no user record at all shall be refused the same way.

**USR-124** The customer portal backend marks the request as coming from the customer
portal. The data a staff person sees is unchanged; the effect is that numbers on the
customer portal's cards use the customer grouping, so a card and the list under it agree.

## 6. Which projects a customer sees

**USR-130** A customer's projects shall be those where they are a project contact whose
membership is **registered**. Invited, re-invited and deactivated memberships give no
access.
*Check:* a newly invited person sees nothing until they register; a deactivated contact
loses access.

**USR-131** A project that is inactive shall not count, nor a project whose account has
been deleted.

**USR-132** Email comparison for membership shall ignore letter case.

**USR-133** Cases, and everything under a project, are limited by that project list.
There is no separate list of cases per person.

**USR-134** Looking up one project, or one case, outside the caller's scope shall answer
"not found", exactly as for a record that does not exist, never "forbidden", so a customer
cannot learn that another customer's record exists. The scope is checked before the
record is looked up.

**USR-135** A project id in a request may be written in upper or lower case.

## 7. Who may use which route

**USR-140** **Internal-only** routes shall answer "forbidden" unless the caller can see
everything (staff, the CSM portal backend with a WSO2 person, or a listed machine
caller). Examples: account search and details, project consumption updates, and the routes
that return a project's usage secrets. An identity that cannot be resolved is answered as
"unauthorized" or "unavailable", never downgraded to "forbidden".

**USR-141** **Project-member** routes (project contacts search and detail) shall run only
for someone who may act on that project: staff for any project, a customer only for a
project they are registered on. Anyone else is told the project was not found; a badly
formed id is a bad request.

**USR-142** **Service-only** routes (for example the team schedule's holiday reminder
bookkeeping) shall refuse any request that carries a person's token, even a staff
person's.

**USR-143** Where a service writes a record on a caller's behalf that the caller must
not read or write directly (such as the internal work note added when a case is escalated
by a customer), it shall write as the system and shall not return that record to the
caller.

## 8. Row-level protection in the database

**USR-150** The resolved scope shall be passed to the database with each statement, so
the database itself refuses rows the caller may not see. This is a second line of defence
under the checks above.

**USR-151** Tables holding customer-facing records are protected this way, among them
cases and their comments and attachments, escalations, time cards, deployments and
deployed products, change requests, incidents, service requests, announcements, work item
activity, tags and watchers.

**USR-152** If no identity reaches the database layer, protected tables return no rows
rather than all rows, and a protected query attempted without an identity fails with an
error. A new route that forgets to carry an identity therefore fails closed.

**USR-153** The user table, roles, teams and team membership are **not** protected this
way (see 10.2).

## 9. Where each "admin only" decision is made

An "admin" feature is gated in one of three ways. They are different on purpose; this
table says which feature uses which.

| Feature | Who may use it | Decided by |
|---|---|---|
| Portal banners (change) | CSM admin | The caller being the CSM portal backend; the portal decides who is an admin |
| Who may de-escalate (list, add, remove) | CSM admin | The same: caller must be the CSM portal backend |
| SLA recalculation and the SLA admin view | CSM admin | The same |
| Skills catalogue (create, change) | CSM admin | A role in the person's token that matches the configured admin role list |
| Everyone's skills (search) | CSM admin, or a CRE or SRE team lead | The same role list, or being such a lead |
| Rota access, schedule administration, rota reminder settings, case alert settings | CSM admin, or the stored rota admin / lead roles for their own family or team | The same role list, plus stored roles; see [../specs/team-schedule.md](../specs/team-schedule.md) |
| Updating another person's profile; assigning a team lead | Any internal caller | Internal-only check; any finer rule is the CSM portal's |

**USR-160** For the first group (banners, who may de-escalate, SLA recalculation) the
service shall accept the request only when the caller is the CSM portal backend, with a
signed-in person, and shall **not** check an admin role list again. Any other caller,
including one holding an admin role in its own token, is refused.
*Check:* the same admin token sent directly to the service is refused; sent through the
CSM portal it is accepted.

**USR-161** For the second group (skills and rota features) the service shall check the
person's own token roles against the configured admin role list. That list must be set to
the same value here as in the CSM portal backend. If it is empty, nobody counts as an
admin for these features.

**USR-162** In every case an admin check finer than "is internal" belongs to the CSM
portal, which decides who may reach the route at all. The service re-checks only where the
table says so.

## 10. Open questions for the product owner

1. **The service assertion is trusted at face value.** The client id is read without
   re-verifying its signature, on the basis that the gateway has already authenticated the
   application. Anything able to reach the service directly could claim to be the CSM
   portal. Is network-level protection of the service enough?
2. **Users, roles and teams are readable by any caller** that reaches the service; the
   user table has no row-level protection. (See users.md.) Should a customer be
   restricted to the people of their own project?
3. **Two ways to decide "admin".** Some features trust the CSM portal's own check;
   others re-check a role list that must be configured identically in both services. A
   mismatch has already locked out a real admin once. Should the remaining features move
   to the first model?
4. **A person arriving through the CSM portal is not looked up in the user table.** A
   valid token on the WSO2 domain is fully trusted even if that person has been
   deactivated or has no user record. (The de-escalation feature adds its own
   deactivation check for this reason.) Should deactivation apply everywhere?
5. **WSO2-domain test, not user type.** The CSM portal path trusts the email domain; the
   person path trusts the user's type. A staff member with an address on another domain is
   treated differently by the two paths.
6. **Mixed identities** (the same email as staff and customer) get the customer's narrower
   list. Is silently narrowing, rather than flagging the duplicate, intended?

## 11. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Token checks, roles, client id | `internal/auth/validator.go`, `internal/auth/middleware.go` |
| Caller scope, project checks | `internal/service/access_service.go`, `internal/repository/access_repo.go` |
| Internal-only helper | `internal/service/require_internal.go`; route wrappers `internal/server/internal_only.go`, `project_member_only.go` |
| Identity passed to the database | `internal/server/identity_middleware.go`, `internal/repository/scoped.go` |
| Row-level policies | migrations carrying `ROW LEVEL SECURITY` (cases, comments, attachments, escalations, time cards, deployments, change requests, incidents and others) |
| Client-id based admin gates | `portal_banner_service.go`, `case_escalation_access_service.go`, `sla_status_service.go`, `case_sla_recalc_access_service.go` |
| Role-list based admin gates | `skill_service.go`, `rota_access_service.go`, `schedule_service.go`, `rota_reminder_settings_service.go`, `case_alert_settings_service.go` |
| Settings | `AUTH_ISSUER`, `AUTH_JWKS_URL`, `AUTH_USER_TOKEN_AUDIENCES`, `AUTH_CLOCK_SKEW`, `AUTH_ADMIN_ROLES`, `M2M_CLIENT_IDS`, `CSM_PORTAL_BACKEND_CLIENT_ID`, `CSM_PORTAL_USER_DOMAIN`, `CUSTOMER_PORTAL_BACKEND_CLIENT_ID` |
| Tests | `access_service_test.go`, `internal/auth/*_test.go`, `internal_only_test.go`, `project_member_only_test.go` |
