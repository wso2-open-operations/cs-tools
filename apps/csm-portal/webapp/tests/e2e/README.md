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
  specs needing two identities; `currentUserSearchQuery(page)` returns a query
  guaranteed to match the signed-in user in the approver picker (a generic
  single-letter query can match only an empty-email service account in this
  small staging tenant — confirmed live).
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
