<!--
Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).

WSO2 LLC. licenses this file to you under the Apache License,
Version 2.0 (the "License"); you may not use this file except
in compliance with the License. You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
-->

# E2E auth — captured sessions (local)

The time-card specs run **authenticated** by loading a captured browser session,
so no login page or 2FA is ever driven locally. You capture a session once per
role into `tests/e2e/storageState/<role>.json` (git-ignored — it holds real
tokens).

Two roles:

- **`approver.json`** — an account whose `GET /users/me` `roles` include
  `csm-leads` (and/or `csm-timecard-admins`). Sees the **Approvals** tab.
- **`engineer.json`** — a plain account **without** those roles. Used for the
  negative role-gating case (must NOT see Approvals).

You only need `approver.json` to run most specs; `engineer.json` unlocks the
engineer negative test (others skip cleanly when a session file is missing).

## Option A — reuse your Chrome profile (recommended)

1. Sign in to the app in Chrome as the account you want to capture.
2. **Fully quit Chrome** (the profile is locked while it runs).
3. Run, pointing at that profile directory:

   ```bash
   ROLE=approver \
   CHROME_PROFILE_DIR="$HOME/Library/Application Support/Google/Chrome/Default" \
   pnpm exec playwright test --project=capture
   ```

   Repeat with `ROLE=engineer` (and the engineer account's profile) if needed.

The `capture` project opens the profile, confirms the Time cards page renders,
and writes `storageState/<role>.json`.

> Tip: use a dedicated Chrome profile per test account so their sessions don't
> clobber each other. Point `CHROME_PROFILE_DIR` at
> `…/Google/Chrome/Profile 1`, etc.

## Option B — manual DevTools export

If launching the profile is inconvenient: in the app tab, DevTools →
Application → Local Storage → copy the Asgardeo entries into a hand-written
`storageState.json` under `origins[].localStorage[]` (Playwright storageState
shape). Save as `tests/e2e/storageState/approver.json`.

## Notes

- **Staleness:** a captured access token expires (~1h). If the session's refresh
  token is still valid the Asgardeo SDK refreshes silently and the file keeps
  working; otherwise re-capture.
- **Secrecy:** `storageState/*.json` is git-ignored. Never commit it.
- **Config:** the app must have a working `public/config.js` pointing at the same
  tenant/backend the captured session was issued against.
