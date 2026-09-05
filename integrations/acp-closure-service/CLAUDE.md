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
`processProject`/`Run` — but on a **separate** flag, `SEND_REAL_EMAILS`, not
`DRY_RUN`. This is deliberate: writing real project state and sending a
real email to a real person are different risks, and conflating them into
one flag would make it impossible to (for example) safely test the write
path against a real project while emails stay log-only, or vice versa. See
"Real email sending" below for `notify.EmailNotifier`, the real
implementation `main.go` injects when `SEND_REAL_EMAILS=true`.

If you add a new side-effecting call, give it the same treatment: define a
minimal interface, inject the real implementation and a logging one, and
never branch on a config flag inside the orchestration logic itself. Give
it its own flag rather than reusing `DRY_RUN`/`SEND_REAL_EMAILS` unless the
risk it protects against is genuinely the same as one of theirs.

## TEST_PROJECT_ID scoping

`Run`'s `projectID` parameter, when non-empty, makes the broad
`SearchProjects` pagination loop structurally unreachable for that
invocation — it returns after evaluating the one fetched project, before the
loop's `offset := 0` line. This is what backs safe testing against a single
dedicated project without risk of touching every open project in an
environment.

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
- **`internal/entity.Client` doesn't validate its configured URLs use
  `https`** — the same gap `internal/emailservice.Client.NewClient` was
  given a fix for (CodeRabbit, PR #1657; see `requireHTTPS` there).
  Deliberately not fixed here in the same PR — scoped out to keep that PR
  focused on the email code it was actually about. `entity.Client` carries
  the same category of risk (its `ClientSecret` flows through the same
  kind of token request) and should get the equivalent check in its own
  follow-up.

## Real email sending

`notify.EmailNotifier` calls WSO2's internal email notification service
(owned by Rashmika's team) via `internal/emailservice.Client`, replacing
`LoggingNotifier` when `SEND_REAL_EMAILS=true`. Confirmed directly with
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
  confirmed via a real received email to be `no-reply@wso2.com`.
- **`EmailNotifier` maps `Recipients` onto to/cc**: when `Customer` is
  present, the customer is the primary `to` and the three internal people
  are `cc`'d; otherwise (internal-only notices, and the no-business-contact
  notice) all populated internal recipients go in `to`. This is a design
  decision made in this codebase, not something Rashmika's API dictates —
  reconsider if it turns out wrong in practice.
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
  `<br>`. The internal notice stops there — plain text, no further
  wrapping, per its own confirmed reference design (real screenshots
  Chamara shared). The customer-facing notice additionally gets wrapped in
  `emailHTMLTemplate` (`renderEmailHTML`) — a real branded shell (WSO2
  logo, orange accent border, footer disclaimer), also confirmed against
  real received examples — only when `notice.Recipients.Customer != nil`.
  The logo is a hosted URL (`wso2LogoURL`, WSO2's own public CDN), not an
  embedded `data:` URI — confirmed via a real send that Gmail blocks
  inline `data:` images in received mail.
- **`notifier.Send` reports delivery per call, not per notifier.** The
  interface is `Send(ctx, notice) (delivered bool, err error)` — no
  separate `Delivers()` method. `LoggingNotifier.Send` always returns
  `(false, nil)`; `EmailNotifier.Send` returns `(true, nil)` only when the
  notice actually reached the real API, and `(false, nil)` when every
  recipient got filtered out (e.g. the WSO2-only staging safeguard leaving
  zero `to` addresses) — not an error, but not delivered either. This
  replaced an earlier, blanket per-notifier `Delivers()` signal that had a
  real bug (CodeRabbit, PR #1657): a customer notice silently filtered out
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
