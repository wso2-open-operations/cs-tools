<!--
Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).

WSO2 LLC. licenses this file to you under the Apache License,
Version 2.0 (the "License"); you may not use this file except
in compliance with the License. You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
-->

# E2E auth — captured sessions (local)

The specs run **authenticated** by replaying a captured browser session, so no
login page or 2FA is driven locally. You capture a session once per role into
`tests/e2e/storageState/<role>.json` (git-ignored — it holds real tokens).

> **Why a full bundle, not just localStorage:** the Asgardeo React SDK keeps its
> tokens in **`sessionStorage`** (`session_data-instance_…`), with only an
> `asgardeo-session-active` flag in localStorage. Playwright's `storageState`
> restores localStorage/cookies but **not** sessionStorage, so we capture both
> stores and replay them via an init script (see `fixtures/test.ts`).

Two roles:

- **`approver.json`** — an account whose `GET /users/me` `roles` include
  `admin` (see `TIMECARD_ADMIN_GROUP` in `timeCardConstants.ts` — temporarily
  mapped to the real `admin` role until a dedicated time-card role exists).
  Sees the **Approvals** tab.
- **`engineer.json`** — a plain account **without** the `admin` role. Unlocks
  two things: the negative role-gating case, and (paired with `approver.json`)
  real cross-user approve/reject coverage in `approvals.spec.ts`. Optional —
  those tests skip cleanly without it.

## Capture a session (browser console)

1. Sign in to the app (`http://localhost:3001`) as the account you want.
2. Open DevTools → **Console**. If Chrome shows the self-XSS warning, type
   `allow pasting` and press Enter.
3. Run — this copies a session bundle (both stores) to your clipboard:

   ```js
   copy(JSON.stringify({
     origin: location.origin,
     localStorage: Object.fromEntries(Object.entries(localStorage)),
     sessionStorage: Object.fromEntries(Object.entries(sessionStorage)),
   }, null, 2))
   ```

4. Save it to the role's file (from `apps/csm-portal/webapp`):

   ```bash
   pbpaste > tests/e2e/storageState/approver.json
   ```

   Repeat from a plain-account tab → `engineer.json` if you want the negative test.

## Notes

- **Staleness:** the captured access token expires (~1h). If the run fails on
  auth, re-capture. (The bundle also carries the refresh token, so the SDK may
  refresh silently within a run.)
- **Secrecy:** `storageState/*.json` is git-ignored. Never commit it.
- **Config:** the app must have a working `public/config.js` for the same
  tenant/backend the session was issued against (the client-instance hash in the
  sessionStorage keys must match, which it does when the config is unchanged).
