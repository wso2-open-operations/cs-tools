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
- `fixtures/test.ts` — `withRole(test, role)` replays the session + skips if absent.
- `pages/TimeCardsPage.ts` — page object for `/time-cards`.
- `utils/selectors.ts` — stable anchors for the seeded store's demo data.
- `specs/timecards/` — the specs.

## Coverage

| Spec | Session | What it covers |
|---|---|---|
| `role-gating` | approver + engineer | Approvals tab visible to approver, hidden from engineer, URL-force fallback |
| `my-sheets` | approver | seeded cards render; submit; edit dialog; delete; state filter |
| `approvals` | approver | queue lists others' submitted sheets; accept; reject + comment; delegate dialog |
| `case-integration` | approver | case "Time tracking" tab renders; open the Log time dialog |

## Test data

The UI is FE-first: `features/csm-timecards/api/timeCardStore.ts` seeds demo data
per signed-in user and **resets on full page load**, so each test starts clean.
Card ids are random and dates are relative to "now" — specs key off stable case
numbers (`CS0353001`, `CS0352584`, …) and states, never ids/dates
(`utils/selectors.ts`).

## Known gaps (deliberate, for later)

- **Notifications banner** (rejected/recalled alerts): no deterministic in-app
  path to put the signed-in user's own card into `rejected`/`recalled` (the
  approval queue excludes your own sheets), so it isn't covered yet.
- **Reports**: the current `/time-cards` page renders no reports section.
- **Real backend persistence**: data is the in-memory store; add a `@live`
  subset if/when the FE adopts the PR-#990 backend endpoints.
- **Real login + TOTP + CI**: deferred; see the plan. This phase is local only.
