<!--
Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).

WSO2 LLC. licenses this file to you under the Apache License,
Version 2.0 (the "License"); you may not use this file except
in compliance with the License. You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
-->

# Time Cards E2E (Playwright, local)

End-to-end coverage for the time-cards feature, run locally against
`pnpm run dev` (:3001) and authenticated by a **captured browser session** — no
login page or 2FA is driven. See [`auth/README.md`](./auth/README.md) to capture
a session.

The feature is wired to the **real** `csm-portal-backend` (`POST /time-cards`,
`POST /time-cards/search`, `PATCH /time-cards/{id}`) — there is no mock data
and no delete endpoint, so specs that create a card leave a **permanent
record in staging**. Every card an E2E spec creates is tagged in its
work-log comment via `E2E_TAG` (`utils/selectors.ts`) so it stays identifiable.

## ⚠️ `POST /time-cards/search` requires a project scope

**`filters.projectIds` must be non-empty or the search always returns
`total: 0`** — confirmed live: an unscoped search (or one scoped only by
state/date) returns nothing even for cards created moments earlier with a
real `201` response, while the exact same search scoped by `projectIds`
returns real data. `openapi.yaml` documents `projectIds` as optional, but in
practice it's required. This was originally (and incorrectly) diagnosed as an
unfixable server-side defect; it's actually a frontend bug — the FE just
wasn't always sending a project scope.

**Fix, now in place:** every search call defaults to a real project scope
instead of omitting it — `useCaseTimeCards` scopes to the case's own project,
and `useMyTimeSheets` / `useApprovalQueue` default to every project the
signed-in user can see (via `useProjectOptions()`) when no explicit project
filter is picked. See `src/features/csm-timecards/api/{useTimeCards,useTimeSheets}.ts`.

## ⚠️ `POST /time-cards/search` 500s when `states` + a large `projectIds` array are combined

Found while verifying the fix above: sending both `filters.states` and a large
`filters.projectIds` array (confirmed at ~88 project ids, this tenant's full
visible set) returns `500 {"message":"Failed to search time cards."}`. Smaller
`projectIds` arrays (confirmed at 2 and 10) combined with `states` return
`200` normally, as does the full ~88-project array *without* `states`. This is
a genuine backend defect, not something the FE can fully avoid while staying
correct — worth reporting to the backend team.

**Workaround in place:** `searchTimeCards()` never sends `filters.states` on
the wire; it fetches by `projectIds`/date only and filters by state
client-side over the returned page. This sidesteps the 500 entirely (`useApprovalQueue`
always needs a "submitted" filter and now defaults to every visible project,
so without this workaround the Approvals queue would 500 outright for any
user with enough visible projects).

## ⚠️ `PATCH /time-cards/{id}` 403s unless you're the card's assigned approver

Confirmed live: approving/rejecting a card you just created (assigned
yourself as approver) succeeds (`200`); approving a *different*, real,
pre-existing card from another engineer in the queue returns `403
{"message":"Access to the requested resource is forbidden!"}`. The Approvals
queue UI doesn't know which specific cards you're the assigned approver for —
it shows every submitted card from other users and offers Approve/Reject on
all of them, so clicking Approve/Reject on a card assigned to someone else
will 403. This looks identical to "approve/reject doesn't work" from the UI.

**Fix in place:** `CsmTimeCardsPage.tsx` and `CaseTimeCardsPanel.tsx` now wire
an `onError` handler on the decide mutation (previously only had `onSuccess`
— a failed decide was silently swallowed with zero feedback) that surfaces
the backend's own message via the app's `useErrorBanner()`. This makes the
403 visible instead of silent, but doesn't change who's actually allowed to
decide a card — that's a real backend authorization rule, not fixable from
the FE. The `approvals.spec.ts` cross-user test creates its card with the
*approver* session's own identity as the assigned approver (not the
engineer's), specifically to avoid hitting this 403 itself.

## Run

```bash
# one-time: capture an approver session (browser console) — see auth/README.md,
# then save the copied bundle:
pbpaste > tests/e2e/storageState/approver.json

# run the suite (boots dev server automatically)
pnpm run test:e2e                          # all timecard specs
node_modules/.bin/playwright test --ui     # author/debug interactively
node_modules/.bin/playwright show-report   # open the last HTML report
```

Specs skip themselves (not fail) when the session they need is missing.

> Note: `pnpm exec playwright …` fails in this repo ("packages field missing");
> call the binary directly via `node_modules/.bin/playwright …`.

## Layout

- `auth/README.md` — how to capture a session bundle (localStorage + sessionStorage).
- `fixtures/test.ts` — `withRole(test, role)` replays a session + skips if absent;
  `openContextAs(browser, role)` opens a **second** authenticated context for
  specs needing two identities; `approverSearchQuery(page)` returns a query
  (the signed-in user's email domain) guaranteed to surface a real, *other*
  candidate in the approver picker — the picker excludes the signed-in user
  (self-approval prevention), so a generic single-letter query can match only
  an empty-email service account in this small staging tenant, and the
  signed-in user's own address would always come back empty (confirmed live).
- `pages/TimeCardsPage.ts` — page object for `/time-cards`.
- `pages/LogTimeDialog.ts` — fills and submits the real "Log time" form.
- `utils/selectors.ts` — `E2E_TAG` / `e2eWorkLogComment()` for tagging created data.
- `specs/timecards/` — the specs.

## Coverage

| Spec | Session(s) | What it covers |
|---|---|---|
| `role-gating` | approver + engineer | Approvals tab visible to approver, hidden from engineer, URL-force fallback (no data dependency) |
| `case-integration` | approver | opens a case's Time tracking tab, submits the Log time form for real, and asserts the panel's entry count reflects the new card |
| `my-sheets` | approver | page/tab/filters render; a newly logged card appears grouped in My time sheets; state/work-item filters narrow to it |
| `approvals` | approver (+ **engineer**, optional) | queue tab/filters render; cross-user approve clears a card from the queue — **skipped** unless `storageState/engineer.json` is also captured (a second identity is required to create a card the approver didn't author) |

Runtime data (cases, projects, approver search results) comes from staging as-is;
specs pick "whatever's first" rather than asserting on specific records, and
skip gracefully when there's nothing to act on (e.g. no open case).

## Known gaps

- **Cross-user approve test needs a second captured session** — see Coverage
  above; capture `storageState/engineer.json` (any non-approver account) to
  enable it.
- **No delete endpoint**: cards created by these specs (and by manual runs)
  accumulate in staging. Look for the `[E2E]` tag in `workLogComment` if a
  cleanup pass is ever needed.
- **Fractional hours round to whole numbers**: the backend's hour fields are
  integers (confirmed: `0.5` is rejected with 400, `1` succeeds) even though
  the Log Time form logs quarter-hour increments — `usePostTimeCard` rounds
  each bucket right before sending. A user splitting a small amount across
  several activities may see some round down to 0h; the form shows a notice.
- **Real login + TOTP + CI**: deferred; see the plan. This phase is local only.

## Team Schedule: SME L1-L3 layers (real local stack)

`specs/team-schedule/sme-tiers.spec.ts` drives the real stack: the roster's cell
picker offers L1/L2/L3 for an SME team's Day and Night windows, and rostering an
L1, an L2 and an L3 on one SME night clears exactly that night's three gaps on the
Case Paging tab's SME chain (the same day's Day window keeps its gaps). It signs in
through the local mock identity provider as the seeded `sme.rota.admin@example.com`
and WRITES to Postgres (three fake people, `qa.sme.*@example.com`, tagged
`qa-sme-e2e`, and their turns), removed before and after, so it is skipped unless
the stack is named:

```bash
E2E_NO_WEBSERVER=1 E2E_POSTGRES_CONTAINER=csm-platform-postgres-1 \
  npx playwright test tests/e2e/specs/team-schedule/sme-tiers.spec.ts
```

## Standard changes: propose, CAB approval, raise from a template (real local stack)

`specs/operations/standard-changes.spec.ts` drives the real stack through the UI.

Prerequisite: the standard change seed. The first test checks the seed's own templates and proposals (by the names the
script gives them), so seed the stack once before running the spec (it is idempotent; `--reset` rebuilds it from scratch and, on a stack
started under another compose project name, needs `COMPOSE_PROJECT_NAME=<name>`, e.g. `csmcr`):

```bash
scripts/csm-compose/seed-standard-changes.sh    # from the repository root (entity-service :8081, mock OIDC :9100)
```

What it checks:

- **The page heading** (what users complained about): on Operations > Change requests > Standard change catalogue
  (`?view=catalogue`) and > Standard change proposals (`?view=proposals`) the heading is exactly "Standard changes" with
  that view's own line, and neither says anything about service requests, incidents, problems or runbooks. Every other
  Operations tab (the change request list included) keeps "Operations" and its Operations-wide line.
- **The seed is listed**: the catalogue shows the seed's five templates with their state and version (the suspended
  "Restart an API Manager node", "Scale a Kubernetes deployment's replicas" at v2); the suspended template's page shows the
  reason it was suspended and who did it; the proposals sub-view lists the seed's rejected, in-CAB-review and draft
  proposals and its approved revision, each in its state.
- **Old links and the sidebar**: `/operations?tab=standard_changes`, `/operations?tab=standard_changes&view=proposals`,
  `/operations/standard-changes` and `/operations/standard-changes?view=proposals` each land on the right sub-view, and
  the sidebar's "Change requests" is the one highlighted entry there, on a template page and on a proposal page.
- **The whole flow**: `bob` proposes a standard change from a closed change request ("Propose as standard change"),
  templates it (name, category, a text and a select parameter, `{{placeholders}}` in the subject and the implementation,
  backout and test plans, typed and through "Insert {{…}}"), and the live preview renders every placeholder with the
  sample values; he saves and submits it to the CAB; `carol` (CAB) approves it; the template is in the catalogue as active
  version 1; `alice` raises a change request from it on the create form (type Standard, the template, the change
  specifics, the read-only preview). The change request holds the template's content rendered with her values (subject,
  implementation / backout / test plans, impact, category, rollback duration on the page; risk, which the page does not
  show, from the API and Postgres), shows the "Raised from standard change template" banner, and its lifecycle bar shows
  Assess and Authorize as not taken. In Edit every field the template specifies (subject, type, impact, category, rollback
  duration, the plans, the customer boxes, plan visibility) is disabled while priority, assignment and affected services
  stay editable. She schedules it ("Schedule" is Request Approval), implements, reviews and rolls it back; the template then
  reads "Review needed", linking to that change request.

Every persona signs in through the local mock identity provider (no password).

It WRITES: a change request raised and walked to Closed through the CSM backend's API (the evidence), the proposal, the
template and the change request raised from it, all named with `QA-STD-E2E`. Before and after, it deletes from Postgres,
in one transaction, the proposals, change requests and templates carrying that tag (and every change request raised from
a tagged template) together with the `event_outbox` rows of exactly those records (that table has no foreign key, so they
would otherwise be left pointing at nothing), and nothing else. So it is skipped unless the stack is named:

```bash
E2E_NO_WEBSERVER=1 E2E_POSTGRES_CONTAINER=csmcr-postgres-1 \
E2E_CSM_BFF_URL=http://localhost:8082 E2E_OIDC_URL=http://localhost:9100 E2E_ENTITY_SERVICE_URL=http://localhost:8081 \
  npx playwright test tests/e2e/specs/operations/standard-changes.spec.ts
```

Set `E2E_SHOT_DIR=<dir>` to also save a picture of each step: the region the step is about (the app's main area scrolls
inside a 100dvh shell, so a full-page picture would only show its top). The webapp must be a build that has the Change
requests sub-views (rebuild the compose service `csm-portal-webapp` after changing them).
