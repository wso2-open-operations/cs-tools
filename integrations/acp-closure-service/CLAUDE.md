# ACP Closure Service

Go port of Phase 1 of the Account Closure Process (ACP) — subscription
end-date closure only. Ports the decision/action logic from
`docs/legacy-servicenow-reference/ACPMainProcess.js` and
`ACPActionModules.js` (repo root); invoice/compliance logic (Phase 2) in
those files is reference-only and out of scope here.

## Shape: run-to-completion CLI, not a server

Every other Go component in this repo (`entity-service`,
`apps/csm-portal/backend`, `integrations/csm-integration-service`) is an
HTTP server. This one isn't: `cmd/acp-closure/main.go` performs one full
sweep and exits. A Choreo Task component's cron owns the schedule — there is
no in-process ticker, no long-running state, no health endpoint.

## Calls csm-integration-service, not entity-service directly

`internal/entity` is an HTTP client for `csm-integration-service`
(`integrations/csm-integration-service`), not entity-service. This was a
deliberate choice, not an oversight: `csm-integration-service` was built
with this automation specifically in mind (its `UpdateProject` client method
carries a comment to that effect), and the API team pointed us at its
`openapi.yaml` directly when asked how to call the API. Do not add a direct
entity-service client here without revisiting that decision.

`internal/entity.Client` deliberately does not implement the
`x-user-id-token` pass-through that `csm-integration-service`'s own entity
client has — this component is a headless batch job with no end-user
session, ever, so that code path would be permanently dead here.

## Package layout and the pure/I-O split

- `internal/closure` — pure decision logic. `Decide(now, endDate,
  lastNoticeWindow) Decision` reports what's due (which notice window, and
  whether suspend applies) given the confirmed 90/60/30/15/7/0 cascading
  thresholds. No I/O, no sequencing, no recipient logic. `Decision.ShouldSuspend`
  has no idempotency signal of its own — it fires on every day-0 evaluation
  regardless of `lastNoticeWindow`, because the real suspend-idempotency
  signal (`closureStatus`) isn't a parameter this function receives at all;
  callers must check it themselves (`sweep.suspend` does).
- `internal/recipients` — pure customer-contact and Account-Manager-email
  resolution. `ResolveCustomerContact` implements the three-tier fallback
  (business-contact-role Project Contact → account-level Primary Contact →
  signal to nudge the Account Manager instead). `AccountManagerEmail`
  extracts an email from an already-fetched `PersonRef`, treating "no AM
  assigned" and "AM assigned but no email" both as legitimate absence
  (`""`), not errors — many real accounts have incomplete role assignments.
  `Contact` (`{Name, Email}`) is the resolved-recipient shape shared with
  `notify.Recipients`.
- `internal/suspensionstate` — translates between
  `suspensionProcessState`'s real wire shape (see below) and
  `closure.NoticeWindow`. `WithSubscriptionEndDateState` only ever touches
  the `based_on_subscription_end_date` key; `based_on_due_invoices` and
  `based_on_compliance` (Phase 2, legacy-owned) must survive every write
  byte-for-byte untouched — this is covered by a dedicated regression test
  using a realistic multi-section payload, not a trivial empty case.
- `internal/notify` — `Notice`/`Recipients` shape, `LoggingNotifier`, and
  `EmailNotifier` (real sending — see "Real email sending" below).
- `internal/emailservice` — HTTP client for WSO2's internal email
  notification service, satisfying `EmailNotifier`'s `Sender` interface.
- `internal/sweep` — orchestration. `Run` paginates `/projects/search` (or,
  when `TEST_PROJECT_ID` is set, fetches exactly one project via
  `GetProject` and skips pagination entirely); `processProject` evaluates
  and acts on a single project.

Each pure package has an I/O counterpart living in `sweep` (e.g.
`resolveAccountContacts` does the `GetAccount` call and DTO parsing, then
hands parsed data to `recipients.AccountManagerEmail`). Keep new decision
logic in the pure packages and I/O in `sweep` — this split is what makes the
decision logic cheaply testable without mocks.

## Cross-cascade ordering and the in-memory "already closed" gate

`processProject` (`sweep.go`) evaluates both closure reasons —
`buildSubscriptionCascade` and `buildInvoiceCascade` — *before* acting on
either, collecting a `cascadeDecision{decision, act}` for each one that
fires, then sorts them by `decision.DaysRemaining` ascending (most overdue
first) and executes them one at a time. This mirrors legacy's own two-phase
shape exactly: `ACPMainProcess.js`'s `calculateProjectSuspension` computes a
`Decision` for every reason first, sorts `sorted_by_days_left` ascending,
and only then does `actionHandler` act on them in that order.

This isn't just about which email goes out first — it's what makes the
"already closed" gate correct at all. Legacy's `checkForOpenProject` reads
the project's single shared `u_wso2_closure_state` field before every
`actionSendEmailNotification` call, so when the more-urgent reason suspends
a project partway through a run, the next (less-urgent) reason sees that
and skips its own notification (`IGNORED`) — legacy only ever sends one
cascade's notice per run, even when multiple reasons fire at once.

`processProject` reproduces that with an **in-memory** `alreadyClosed bool`
that starts from `proj.ClosureState` (closed from a *previous* run) and
flips to `true` the instant a cascade's own `decision.ShouldSuspend` is
true (closed *by this run*) — never a live API re-fetch between cascades.
This went through two wrong versions before landing here, both caught via
real live tests against the dedicated test project, worth recording so the
reasoning isn't re-litigated:

1. **Fixed order, stale snapshot.** The very first version ran
   subscription-then-invoice unconditionally, gated only by
   `proj.ClosureState` fetched once at the top of the run — a snapshot that
   could never reflect a same-run suspend from the other cascade. A live
   test where a project qualified for both reasons at once sent two
   separate "Project Suspension Notice" emails (same subject, genuinely
   different bodies) plus two byte-for-byte-identical no-business-contact
   nudge emails — nothing legacy would ever produce.
2. **Dynamic order, live re-fetch.** The next version fixed the ordering
   (sort by `DaysRemaining`, matching legacy) and added a `GetProject`
   re-fetch of `closureState` right before the second cascade acts, trying
   to reproduce legacy's live DB read. This is *wrong for this backend*: a
   second live test showed the re-fetch, issued mere seconds after the
   first cascade's `suspend()` PATCH, still read the pre-suspend value —
   the same read-after-write staleness this backend has shown
   repeatedly throughout this project (see "Known discrepancies" and prior
   handoffs — never trust a PATCH's own success response, or even one GET
   right after it, as proof a write is live). Both cascades notified anyway,
   identical symptom to version 1.

The in-memory `alreadyClosed` tracking sidesteps the staleness problem
entirely rather than racing it: `decision.ShouldSuspend` is computed
locally in this same process, not read back from an API that might not
have caught up yet, so there is nothing to be stale. If you're ever tempted
to add a live re-check back here for extra safety, don't — this has been
tried twice and failed both times for the same underlying reason.

Suspend itself is **not** affected by any of this: `suspend`/`suspendInvoice`
guard on their own per-dimension field (`EndDateClosureState`/
`InvoiceDueDateClosureState`), fetched once from `proj` — these are
genuinely independent per-reason dimensions, so one cascade's suspend can
never be mistaken for the other's. Only the shared, rolled-up notify gate
needed the same-run tracking.

## isPartner does not gate the invoice cascade on its own

`buildInvoiceCascade` (`invoice_orchestrate.go`) does **not** check
`isPartner` before fetching invoice/account data, and does not disable the
cascade on `isPartner` alone. Legacy's `calculateEventTypeFromDate`
(`ACPMainProcess.js`) checks `hasPrimaryPartner` **first, unconditionally**
— a project whose account has *both* `isPartner=true` and
`hasPrimaryPartner=true` still fires, via the grace-period (`parterLed`)
path — and only disables the cascade via `isPartner` once `hasPrimaryPartner`
is confirmed false. Legacy also fetches due invoices unconditionally
regardless of `isPartner` at all (`ACPInvoiceUtils.fetchDueInvoicesByProject`
takes no `isPartner` parameter). The gate actually applied is
`isPartner && !hasPrimaryPartner`, checked only after both facts are known —
not `isPartner` alone, checked first.

A prior version of this function got this wrong: it gated on `isPartner`
alone as an early return, before `hasPrimaryPartner` was ever fetched. That
silently disabled the `isPartner=true`/`hasPrimaryPartner=true` combination
entirely — a real behavioral gap versus legacy, not a documented
simplification, caught by `/code-review`'s Spec axis rather than by any live
test (this combination didn't show up in the projects tested against). If
you're tempted to reintroduce an early `isPartner` check for efficiency
(skip fetching invoice data for obviously-disabled accounts), don't — there
is no way to know the cascade is actually disabled without `hasPrimaryPartner`,
which requires the same `GetAccount` call regardless.

## Only Closed Won opportunities' invoices count

`eligibleOpportunity` (`invoice_resolve.go`) applies legacy's full check
from `ACPInvoiceUtils.fetchDueInvoicesByProject`: `stage` must equal
`"50 - Closed Won"` exactly, and the text EULA field must be non-null and
not `"Customer contract"`. A null or any other stage is ineligible, as in
legacy's equality check.

The stage half was missing until the Opportunity API started returning
`stage` (added by Sajith; confirmed present on both `GET /opportunities/{id}`
and `/opportunities/search`). Before that, an unpaid invoice on a
not-yet-won opportunity (e.g. `"45 - Proposal"`) could drive invoice
notices and suspension, which legacy never did. The shared test
opportunity ("ACP Partner Opportunity") is Closed Won in staging, so the
test projects' invoice cascade is unaffected.

## Invoice-side searches must paginate

`fetchAllProjectOpportunityLinks`/`fetchAllInvoicesForOpportunity`
(`invoice_resolve.go`) page through `/project-opportunity-links/search` and
`/invoices/search` exactly like `Run` already does for `/projects/search` —
`pagination.limit`/`offset` on the request, looping until `hasMore` is false
or a page comes back empty. `resolveDueInvoice` originally sent neither
search with a `pagination` field at all and never checked the response's
`hasMore` (CodeRabbit, PR #1933) — a project or opportunity with more rows
than a single page would silently have the rest ignored, up to and including
a genuinely more-overdue eligible invoice sitting on a page 2 that was never
fetched. Both response schemas carry `total`/`limit`/`offset`/`hasMore`
identically to `ProjectSearchResponse`, so there was no API-shape reason for
the gap — just an oversight when this file was first written.

## An invoice-cascade build failure doesn't block the subscription cascade

`processProject`'s two build steps are treated differently on error
(CodeRabbit, PR #1933). A `buildSubscriptionCascade` error still aborts the
whole call immediately — its only failure mode is a corrupt
`suspensionProcessState` (pure parsing, no I/O), which is genuinely unsafe
to decide *anything* from, including whether the invoice cascade's own
section of that same JSON blob can be trusted. A `buildInvoiceCascade`
error — far more likely to be a transient upstream failure
(`SearchProjectOpportunityLinks`/`SearchInvoices`/`GetOpportunity`) or one
malformed ServiceNow-synced row (bad date, bad EULA-version string) rather
than a genuinely corrupt project — is logged and does **not** stop the
already-built subscription cascade from executing, since it doesn't depend
on invoice data at all. The invoice error is still returned once the
execution loop finishes (deferred, not swallowed), so `Run` still counts the
project as failed — but only after the subscription cascade got its fair
shot at a real, time-sensitive day-0 suspend that has nothing to do with
invoices. Before this fix, a single bad invoice row could silently block a
project's subscription-based suspend on every sweep until the invoice-side
issue was fixed.

## Dry-run is an injection choice, not a branch

`DRY_RUN` never appears as an `if` inside `processProject` or `Run`. Both
have exactly two side-effecting dependencies — `projectUpdater` (writes) and
`notifier` (sends) — expressed as small interfaces. `main.go` decides which
concrete implementation to inject for `projectUpdater` based on `DRY_RUN`:
`sweep.DryRunProjectUpdater` (silently no-ops, never calls `UpdateProject`
— see the notice-content-redesign section below for why it deliberately
doesn't log) vs. the real `*entity.Client`. Reads (`SearchProjects`, `GetAccount`,
`SearchProjectContacts`, `SearchAccountContacts`) are never dry-run-gated —
fetching and deciding has no write effect to protect against.

`notifier` is chosen the same way — injection, not a branch inside
`processProject`/`Run` — but on a **separate** flag, `IS_EMAIL_SEND_ENABLED`,
not `DRY_RUN`. This is deliberate: writing real project state and sending a
real email to a real person are different risks, and conflating them into
one flag would make it impossible to (for example) safely test the write
path against a real project while emails stay log-only, or vice versa. See
"Real email sending" below for `notify.EmailNotifier`, the real
implementation `main.go` injects when `IS_EMAIL_SEND_ENABLED=true`.

If you add a new side-effecting call, give it the same treatment: define a
minimal interface, inject the real implementation and a logging one, and
never branch on a config flag inside the orchestration logic itself. Give
it its own flag rather than reusing `DRY_RUN`/`IS_EMAIL_SEND_ENABLED` unless
the risk it protects against is genuinely the same as one of theirs.

## TEST_PROJECT_ID scoping

`Run`'s `projectID` parameter, when non-empty, makes the broad
`SearchProjects` pagination loop structurally unreachable for that
invocation — it returns after evaluating the one fetched project, before the
loop's `offset := 0` line. This is what backs safe testing against a single
dedicated project without risk of touching every open project in an
environment.

## No runs on weekends

No emails may go out on Saturday or Sunday (business requirement). `main`
checks `isWeekend(time.Now())` right after the startup log line and exits 0
before building any client — no reads, no writes, no sends. "Weekend" is
judged in UTC+05:30 (`operationsZone`), not Choreo's UTC clock; a fixed
offset rather than a named zone, so the container needs no tzdata. This
applies to every run, including `TEST_PROJECT_ID`-scoped ones — there's no
override flag.

The whole run is skipped, deliberately, rather than running and only
holding back the sends:

- **Skipping the run loses nothing.** Notice windows and suspension are
  threshold-based (`daysRemaining <= window`, see `closure.Decide`), so
  Monday's run sends whatever came due over the weekend. The windows are at
  least 7 days apart, so a two-day gap can never skip past a whole window.
  The trade-off, accepted explicitly: a day 0 that lands on a weekend
  suspends on Monday, so the customer gets up to two extra days.
- **Holding back only the sends would break the cascade.** A Saturday
  day-0 run would still suspend the project, and Monday's run would then
  seed `alreadyClosed` from the now-closed project and skip its suspension
  notice for good (see "Cross-cascade ordering" above). Suspending on
  schedule while emailing later would need the notify/suspend coupling
  reworked. Don't attempt it without revisiting this.

The Choreo cron should also be set to weekdays only (`30 9 * * 1-5`, i.e.
09:30 Mon–Fri — Choreo evaluates this component's cron in UTC+05:30, which
is why the original daily `30 9 */1 * *` fired at 09:30 local, not UTC) so
weekend invocations don't happen at all. This
guard is defence in depth for a manual trigger or a mis-edited cron.

## EXCLUDED_PROJECT_IDS — deliberate exclusion, not a bug workaround

`Run`'s `excludedProjectIDs` parameter (backed by the `EXCLUDED_PROJECT_IDS`
env var, comma-separated, parsed by `main.go`'s `parseExcludedProjectIDs`) is
a set of project IDs the sweep skips entirely — not fetched in detail, not
evaluated, not counted as a failure, just logged and counted in
`Result.ProjectsExcluded`. This applies uniformly to both the broad sweep and
the `TEST_PROJECT_ID`-scoped path: if the scoped `projectID` is itself
excluded, `GetProject` is never called at all.

This came out of a real production incident (a project returning `500` on
both `GET` and `PATCH` — a genuine data problem on the entity-service side,
confirmed by the fact that even a bare read failed, not just the write) and
a design discussion with Sajith Ekanayake about how to handle it. The
resulting agreement, worth preserving verbatim since it's easy to
misapply this mechanism otherwise:

- **This is for deliberate, verified business exclusions only** — a project
  someone has actually decided should never go through ACP, for a real
  business reason. It is explicitly **not** a workaround for data bugs like
  the incident that prompted it. A project excluded here produces zero log
  signal about whatever might actually be wrong with it — the opposite of
  what you want when something is broken and needs fixing.
- **Expected to be empty almost all the time in production.** It's fine —
  expected, even — for this to hold real entries in dev/staging (e.g.
  keeping a known-broken test project out of the way while iterating).
- The real incident that prompted this discussion was **not** resolved by
  adding the project to this list — it needed (and still needs, as of this
  writing) an actual data-level fix from whoever owns `entity-service`. See
  the "known discrepancies" pattern elsewhere in this file for the general
  practice of escalating rather than silently working around upstream
  problems.
- **Visibility was the sticking point in the design discussion**: the
  concern was "if we skip a project entirely, how do we know something's
  still wrong with it, or that it's since been fixed?" The answer landed on:
  every excluded project ID is logged (`"project excluded from evaluation"`)
  each time the sweep would otherwise have touched it, and the full
  configured list is logged once at startup (`"excludedProjectIDs"` on the
  `"acp-closure-service starting"` line) — so the exclusion itself stays
  visible in the logs even though the project's own data never gets
  evaluated. This does *not* answer "is the underlying issue still there" —
  that still requires someone to actually go check, same as before.

## Notice audience matrix and content (redesigned per Chamara's direct request)

Confirmed audience rule, unchanged: 90/60/30-day windows are internal-only;
15/7/0-day windows are both internal and customer (`needsCustomerAudience`
in `sweep.go`). Notice *shape* went through two redesigns superseding the
original `Kind` internal/customer/am_nudge model — the second one, current
as of this writing, is a materially different design from the first (a
single consolidated `Notice` per window), because it turned out the
internal and customer copies have genuinely different subject/body content,
not just different recipients. Recorded here in detail so the reasoning
isn't lost or re-litigated:

- **Internal and customer notices are always two separate `Send` calls**
  for a customer-audience window (15/7/0) — never one `Notice` bundling
  both. The internal notice fires unconditionally for every window
  (90/60/30/15/7/0); a second notice (customer, or the no-business-contact
  nudge) fires only for 15/7/0.
- **`Subject`** has no single template — four distinct ones
  (`internalNoticeSubject`/`customerNoticeSubject` in `sweep.go`), confirmed
  against multiple real examples from Chamara:
  - Internal, day-count (90/60/30/15/7): `"[ACP] {N} Days Reminder of
    Project for {ProjectName} of {AccountName}"`. The `[ACP]` prefix marks
    "this is the internal-audience copy" and applies to **every** window,
    including 15/7 — not just 90/60/30. (An earlier version of this logic
    had that backwards; confirmed wrong directly against real examples
    where a 15-day internal subject still carried `[ACP]`.)
  - Internal, day-0: `"[ACP] Project Suspension Notice of {ProjectName} of
    {AccountName}"` — no "days remaining" left to report once suspended.
  - Customer, 15/7: `"Upcoming Project Suspension Notice - {ProjectName}"`
    — never `[ACP]`-prefixed, never names the account.
  - Customer, day-0: `"Project Suspension Notice - {ProjectName}"` — past
    tense, no "Upcoming".
  - No-business-contact (see below): `"[Urgent] [ACP] No Business Contacts
    Specified for Project {ProjectName}"`.
  `ProjectName` itself often already contains the word "Subscription"
  (e.g. `"TICKETNETWORK - Subscription"`), which is why a literal internal
  subject can visually resemble "...Subscription of TicketNetwork" without
  "Subscription of" being separate template wording.
- **Every notice has a real `Body` now** — not just the no-business-contact
  one. Internal bodies (`internalNoticeBody`) open with a greeting that
  **always names the Account Manager** (`"Dear {AccountManagerName}"`),
  regardless of which of the three internal recipients is actually reading
  their own copy — confirmed explicitly, not personalized per recipient —
  and list `Project Name`/`Project Key`/`Account Owner`/`Start Date`/`End
  Date` in `2006-01-02` date format. Customer bodies (`customerNoticeBody`)
  have **no greeting at all** and use `01/02/2006` (US-style) dates embedded
  in prose instead. Day-0 bodies (both internal and customer) use distinct
  past-tense/"already suspended" wording instead of the day-count
  reminder's future-tense "needs renewal" wording — see the four body
  template constants in `sweep.go` for the exact confirmed text.
- **The no-business-contact case** (three-tier customer-contact fallback
  lands on `NeedsAMNudge`: no business contact, no primary contact) sends a
  **second, separate** `Notice` alongside the internal notice — not instead
  of it, and with no suppression logic collapsing the two. Recipients are
  **all three** internal recipients (Account Owner, Renewal Manager,
  Technical Owner) — confirmed explicitly; an earlier version sent this to
  the Account Owner alone. Sending both notices (internal + nudge) is a
  deliberate simplification the user confirmed rather than inventing a
  suppression rule for this shape — revisit if it proves too noisy in
  practice. (The original design's `shouldSuppressInternalNotice`, which
  collapsed a same-recipient internal+nudge pair into one send, no longer
  applies — there's no shared-recipient collision to worry about now that
  internal and nudge always target the same three internal recipients by
  design.)
- **`DryRunProjectUpdater` intentionally logs nothing** (`dryrun.go`) — per
  explicit user direction, the only log line that should exist for a dry
  run is `notify.LoggingNotifier`'s `"notice"` line (the actual email
  content: subject, body, recipients). A separate `"dry-run: would update
  project"` line describing the raw PATCH body used to exist here and was
  removed deliberately — it's noise once every window produces a real
  notice log, and stays noise once real email sending (Sajith's team, still
  pending) replaces `LoggingNotifier` as the thing this component
  ultimately integrates with. Don't re-add logging to this type without
  confirming that direction has changed.
- **That `"notice"` line masks personal contact details** (`maskEmail`,
  `maskName` in `notify.go`). Every email address keeps only its first
  character and domain (`p********@wso2.com`), and the customer's name keeps
  only initials. Staff names stay readable, since they're the point of the
  dry-run review and already appear in the internal body. The
  customer-facing body names no one, so log-only mode writes no customer
  personal data at all. Real staging logs from before email sending was
  enabled showed full addresses and names in this line; that was flagged in
  the threat model's privacy review and closed by this masking. Don't log a
  raw address here again. `EmailNotifier` already logs only recipient
  counts.

## Project Name links to Salesforce (internal notices only)

Confirmed via a real reference email
(`local-docs/actual_0_days_invoice_email.html`): every internal notice's
"Project Name" field value is a hyperlink to
`https://wso2.my.salesforce.com/{sfId}` — Salesforce's generic
record-redirect URL, which resolves to the record regardless of object
type. This was missing entirely from the initial port (the field just
rendered as plain bold text) until caught against the real reference.

- `project.SfID` (`types.go`, tagged `json:"sfId"`) carries the project's
  Salesforce ID from the wire, confirmed present on `GetProject`.
- `notify.Notice.ProjectSfID` carries it from `sweep.baseNotice` through to
  `EmailNotifier.Send`.
- `notify.projectNameFieldRowHTML` (`email_notifier.go`) is the one field
  row that ever gets linked — every other field (Project Key, Invoice Id,
  Opportunity, Due Date, ...) always stays `fieldRowHTML`'s plain bolded
  text, matching the real reference (only Project Name links there).
  Falls back to `fieldRowHTML`'s plain rendering when `ProjectSfID` is
  empty — a project genuinely without a Salesforce ID on file.
- **Customer-facing notices never get this link** — confirmed absent from
  the real customer-facing reference email (customers have no Salesforce
  access). Structurally guaranteed here too: customer notices render via
  `renderEmailHTML`/`plainTextToHTML`, which never touches `fieldRowHTML`
  or `projectNameFieldRowHTML` at all — there's no shared code path that
  could accidentally leak the link onto a customer copy.

**Project `sfId` in the broad sweep:** `/projects/search` items now carry
an `sfId` key, but as of 2026-09-26 it is `null` for every project in
staging, even where `GET /projects/{id}` returns a real value. So the
Project Name link only appears in `TEST_PROJECT_ID`-scoped runs until the
API populates it. No change is needed here when it does: `project.SfID`
already reads the field from both endpoints.

**"Open in Salesforce" (invoice notices, internal only):** the real
reference email also has a separate "Open in Salesforce" link inside the
invoice box, pointing at the *invoice's own* Salesforce record (an `a0I…`
ID, not the project's `a0d…`). `invoiceDTO.SfID` (`sfId`, confirmed on both
`GET /invoices/{id}` and `/invoices/search`) flows through
`resolvedInvoice` → `dueInvoice` → `notifyForWindow`'s `invoiceSfID`, which
sets `Notice.InvoiceSfID` on the **internal notice only**.
`openInSalesforceLinkHTML` renders it at the start of the invoice box's
right half (the box is split into two equal halves, as in the reference),
using table cells rather than the reference's `display:flex`, which email
clients don't all support. There's no link when the invoice has no `sfId`, and
never on customer or nudge notices. The reference's small external-link
icon is deliberately left out, as for the Project Name link.

## suspensionProcessState's real shape

Free-form JSON written by an existing, live ServiceNow suspension flow —
**not** something this component's design invented. Confirmed via a real
write against the dedicated test project
(`e3e87599-1bc7-6650-182c-0dc5604bcb68`):

```json
{
  "based_on_subscription_end_date": {"event_type": "30_days_notice", "actionSendEmailNotification": "SUCCESSFUL"},
  "based_on_due_invoices": {"event_type": "7_days_notice", "actionSendEmailNotification": "SUCCESSFUL", "actionServicePortalAnnouncement": "SUCCESSFUL"},
  "based_on_compliance": {"event_type": "open"}
}
```

This matches legacy's exact structure (`event_type` + per-action
`SUCCESSFUL`/`FAILED`/`IGNORED` results, three top-level dimensions). Phase 1
only ever reads/writes `based_on_subscription_end_date` — the other two
dimensions belong to Phase 2 / legacy and must never be touched.

## Known discrepancies between documented/coded behavior and live behavior

Confirmed via direct Postman testing against staging — each of these is a
case where reading a sibling service's source or docs would have given the
wrong answer:

- **Page size.** entity-service's own `maxLimit` constant
  (`entity-service/internal/service/user_service.go`) states `100`. The
  real, live maximum for `/projects/search` is **50** — `limit: 51` returns
  a 400. `internal/sweep/run.go`'s `pageSize` is set to `50` with this
  documented inline. If entity-service's constant is ever corrected, verify
  live behavior again before changing this — don't just copy the new
  constant.
- **PATCH /projects/{id} under M2M-only auth.** `csm-integration-service`'s
  own `CLAUDE.md` states this endpoint "currently receives a mapped 401 from
  `mapUpstreamError`, unconditionally" under M2M-only auth. Confirmed via
  direct, repeated testing (including after a fresh merge, to rule out
  staleness) that this is not true in practice: the endpoint accepts
  M2M-only writes successfully, including real writes to
  `suspensionProcessState`. Flagging discrepancies like this rather than
  silently trusting either source is a deliberate practice on this
  component — verify against real behavior before code changes that depend
  on an assumption from documentation or source reading alone.
- **`account` on `/projects/search` items.** For a period during this
  component's development, entity-service's `ProjectView` type had no
  account reference at all on search results (only the single-project
  detail endpoint carried one). That gap was closed
  (`domain.ProjectView.Account`) partway through this component's build.
  `internal/sweep/types.go`'s `project.Account` has always expected the
  nested `{id, name}` shape; only the doc comment needed correcting once the
  broader `SearchProjects` gap closed.
- **Project key field name.** `csm-integration-service`'s own `openapi.yaml`
  documents this field as `projectKey` on the `Project` schema. The real,
  live `GetProject` response actually names it `key` (confirmed directly by
  the user via Postman against the dedicated test project — the response
  had `"key": "APPSUB"`, no `projectKey` field at all). `internal/sweep/
  types.go`'s `project.ProjectKey` was tagged `json:"projectKey"` for a
  while as a result — silently, always empty on every real response, since
  the tag never matched anything on the wire. Caught only because the
  notice-content redesign started actually reading and logging the value;
  before that, nothing exercised it. Now tagged `json:"key"`, confirmed
  against the real response. If this ever gets "corrected" back to
  `projectKey` by an openapi.yaml update, verify live behavior again before
  copying it — don't just trust the spec.

## Open dependencies

- **Business-contact role string** (`internal/recipients`'s
  `businessContactRole` constant, marked `PLACEHOLDER`) — exact
  ServiceNow-side literal still unconfirmed with the API team. Broad-sweep
  testing against real data shows this role is rarely configured in
  practice regardless — most real resolutions land on `primary_contact` or
  `am_nudge`, not `business_contact`.

## Both HTTP clients share one set of transport guards

`internal/entity` and `internal/emailservice` both build their OAuth2
client through `internal/httpsec`, and must keep doing so:

- `httpsec.RequireHTTPS` makes `NewClient` refuse a non-https `TokenURL` or
  `BaseURL`, and one with no host (a bare `https://` parses cleanly and
  would otherwise only fail on the first request; CodeRabbit, PR #2008).
  Loopback is exempt, since `httptest` servers bind there. The
  token request carries the real client secret, and every API call carries
  the bearer token and real customer data.
- `httpsec.RefuseRedirects` goes on **both** the token client (the one in
  `tokenCtx`, which POSTs the secret) **and** the API client (where
  `oauth2.Transport` would re-attach the bearer token to a followed
  redirect). Guarding only the API client leaves the secret exposed; this
  exact mistake happened once in the email client.

These checks were first added to the email client alone (CodeRabbit, PR
#1657). The entity client went without them until they were moved into the
shared package, which is why they live in one place now. If you add a third
HTTP client, build it the same way. `TestTokenFetchRejectsRedirects` in each
client package was confirmed to fail with the token-client guard removed.

## Real email sending

`notify.EmailNotifier` calls WSO2's internal email notification service
(owned by Rashmika's team) via `internal/emailservice.Client`, replacing
`LoggingNotifier` when `IS_EMAIL_SEND_ENABLED=true`. Confirmed directly with
Rashmika, this superseded an earlier plan (referenced in older commit
history) to publish an event/message onto a queue instead — "I don't think
you need to publish an event to send an email for this use case, you could
use our email service directly." No queue exists or is needed; this is a
plain authenticated HTTP call.

- **Wire contract confirmed against real code, not a spec document**:
  `integrations/csm-notification-service/internal/notifications/email.go`
  on the `dev-app-csm-portal` branch of this same repo is Rashmika's own
  client for this service — `POST /send-email`, OAuth2 client-credentials
  auth, `{to, cc, from, subject, template}` request body. `emailservice`'s
  `sendEmailRequest` mirrors this exactly, including typing `Template` as
  `[]byte` (not `string`) specifically so `encoding/json` base64-encodes it
  the same way the real service expects — sending it as a plain string
  would not match what the server decodes. `bcc`/`replyTo`/`attachments`
  exist on the real API but have no ACP use case, so they're left out of
  this component's client entirely rather than plumbed through unused.
- **No OAuth2 scope is required** for this token endpoint — confirmed
  explicitly with Rashmika, unlike `csm-integration-service`'s
  `CSM_INTEGRATION_SCOPES`. Don't add a scopes config value here without
  re-confirming that's changed.
- **`FromAddress` is fixed at config level**, not a per-`Notice` value —
  `no-reply@wso2.com`.
- **`EmailNotifier` maps `Recipients` onto to/cc**: when `Customer` is
  present, the customer is the primary `to` and the three internal people
  are `cc`'d; otherwise (internal-only notices, and the no-business-contact
  notice) all populated internal recipients go in `to`. This is a design
  decision made in this codebase, not something Rashmika's API dictates —
  reconsider if it turns out wrong in practice.
- **`StandingCC` (`STANDING_CC_RECIPIENTS`) cc's a fixed address list on
  every notice**, uniformly — internal, customer-facing, and the
  no-business-contact nudge alike, subscription and invoice cascades alike,
  added in `Send` right after `recipientsToToCC` and before filtering. This
  was a real gap in the initial port, caught late: every real legacy
  reference email this project has (both internal and customer-facing) cc's
  `customer-lifecycle-notification@wso2.com` and `billing@wso2.com`, and
  this component never sent to either until this field existed. Deliberately
  env-configurable rather than a hardcoded constant like `wso2LogoURL` —
  these are real production distribution lists, and staging/dev must leave
  this empty for the same reason `EMAIL_SERVICE_ALLOW_NON_WSO2_RECIPIENTS`
  defaults false: real people/teams must not receive test traffic. Entries
  still pass through `filterRecipients` like any other recipient — this is
  additive cc, not a bypass of the WSO2-only staging safeguard.
- **The WSO2-only staging safeguard is a hard requirement from Rashmika's
  team**, not a suggestion: "make sure emails aren't being sent in staging
  environment for any non-wso2 emails." `EMAIL_SERVICE_ALLOW_NON_WSO2_RECIPIENTS`
  defaults to `false`, filtering any recipient not ending in `@wso2.com`
  before every send. If filtering leaves zero `to` recipients, `Send` skips
  cleanly (logs, returns `nil`) rather than forcing a call the real API
  would reject anyway (it requires at least one `to`) — a project with
  nobody left to notify after filtering is treated the same as any other
  legitimate-absence case already established throughout this codebase,
  not an error.
- **Notice bodies are plain text; the real API expects HTML** (its own
  doc comment calls `SendEmail`'s content "an HTML email", and the Go
  parameter is named `htmlBody`). `notify.plainTextToHTML` escapes special
  characters first (so a project/account name containing `&`, `<`, etc.
  can never break the resulting markup), then converts every newline to
  `<br>`. Every notice — internal and customer-facing alike — additionally
  gets wrapped in `emailHTMLTemplate` (`renderEmailHTML`), unconditionally:
  a real branded shell (WSO2 logo, orange accent border, footer
  disclaimer), confirmed against real received examples of both kinds
  (e.g. a real internal day-0 notice, "Dear Nisha Farook..."). An earlier
  version of this code kept the internal notice on the bare
  `plainTextToHTML` fragment alone (based on a different, less complete
  reference) — that was wrong, corrected once a fuller real example
  surfaced it. Sending that bare, unwrapped fragment (no real block-level
  container) was also the likely cause of a real symptom seen in a live
  test: the trailing "WSO2 Team" signature line visually missing from the
  received email. The logo is a hosted URL (`wso2LogoURL`, WSO2's own
  public CDN), not an embedded `data:` URI — confirmed via a real send
  that Gmail blocks inline `data:` images in received mail.
- **`notifier.Send` reports delivery per call, not per notifier.** The
  interface is `Send(ctx, notice) (delivered bool, err error)` — no
  separate `Delivers()` method. `LoggingNotifier.Send` always returns
  `(false, nil)`; `EmailNotifier.Send` returns `(true, nil)` only when the
  notice actually reached the real API, and `(false, nil)` when every
  recipient got filtered out (e.g. the WSO2-only staging safeguard leaving
  zero `to` addresses) — not an error, but not delivered either. This
  replaced an earlier, blanket per-notifier `Delivers()` signal that had a
  real bug (per CodeRabbit): a customer notice silently filtered out
  in staging was still recorded as `"SUCCESSFUL"` in
  `suspensionProcessState`, since the blanket signal only reflected "is
  this notifier type capable of real delivery," not "did this specific
  notice actually go out." `sweep.notifyForWindow` now ANDs the delivered
  result across every `Send` call it makes for a window (internal +
  customer, or internal + nudge) before handing that combined result to
  `recordNoticeSent` — a window is only recorded `"SUCCESSFUL"` if every
  notice sent for it actually delivered.

## Testing conventions

- Hand-rolled mocks (function-field structs, e.g. `mockEntityReader`,
  `mockProjectUpdater`, `mockNotifier` in `internal/sweep/helpers_test.go`),
  matching `csm-integration-service`'s own test convention — no mocking
  library.
- Prefer real, previously-confirmed response shapes as test fixtures over
  synthetic/trivial ones where the exact shape matters (e.g.
  `TestProject_ParsesNestedAccountFromRealGetProjectResponse`,
  `TestWithSubscriptionEndDateState_PreservesOtherSectionsByteForByte` use
  the literal JSON confirmed via Postman against the dedicated test
  project/account) — this catches shape mismatches that a hand-written
  trivial fixture would silently paper over.
- TDD throughout: red before green, one seam at a time. Seams under test:
  `closure.Decide`, `recipients.ResolveCustomerContact` /
  `AccountManagerEmail`, `suspensionstate.LastNoticeWindow` /
  `WithSubscriptionEndDateState`, `sweep.processProject`, `sweep.Run`, the
  pure subject/body builders (`internalNoticeSubject`,
  `customerNoticeSubject`, `internalNoticeBody`, `customerNoticeBody` in
  `sweep.go`) tested directly rather than only through `processProject`,
  `emailservice.Client.SendEmail` (against a real `httptest.Server`, same
  pattern as `entity.Client`), and `notify.EmailNotifier.Send` (recipient
  mapping, the WSO2-only filter, HTML conversion — via a hand-rolled
  `mockEmailSender`, no real HTTP involved at that layer).
  `main.go` and the two logging/no-op implementations
  (`notify.LoggingNotifier`, `sweep.DryRunProjectUpdater`) are deliberately
  untested, matching this repo's convention that wiring-only code and
  behaviorless placeholders don't need dedicated tests.
