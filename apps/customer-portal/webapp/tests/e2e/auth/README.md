<!--
Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).

WSO2 LLC. licenses this file to you under the Apache License,
Version 2.0 (the "License"); you may not use this file except
in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
-->

# E2E auth — captured sessions (local)

Specs run **authenticated** by replaying a captured browser session, so no
login page or 2FA is driven locally. Capture a session once per role into
`tests/e2e/storageState/<role>.json` (git-ignored — it holds real tokens).

> **Why a full bundle, not just localStorage:** the Asgardeo React SDK keeps its
> tokens in **`sessionStorage`** (`session_data-instance_…`), with only an
> `asgardeo-session-active` flag in localStorage. Playwright's `storageState`
> restores localStorage/cookies but **not** sessionStorage, so we capture both
> stores and replay them via an init script (see `../fixtures/test.ts`).

## Capture a session (browser console)

1. Sign in to the app (`http://localhost:3000`) as the account you want.
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

4. Save it to the role's file (from `apps/customer-portal/webapp`):

   ```bash
   pbpaste > tests/e2e/storageState/admin.json
   ```

Repeat per account for `lead.json`, `portal.json`, `security.json`.

> **Local compose stack?** No capture needed — the mock identity provider signs in any email.
> Mint a session for a seeded customer (dave, erin, mira, noel) with
> `E2E_LOCAL_PERSONA=dave pnpm run test:e2e:local-auth`; see "Local stack" in
> [`../README.md`](../README.md).

## What each account needs

| Bundle | Account setup |
|---|---|
| `admin.json` | `sn_customerservice.customer_admin` in `GET /users/me` roles, and an Admin membership on the project under test |
| `lead.json` | Project membership with `isLead: true` (implies `isPortalUser`), **no** customer-admin role — isolates escalation past EL3 from admin powers |
| `portal.json` | Plain `isPortalUser: true` membership only — the baseline/negative case |
| `security.json` | `isSecurityContact: true` only — verifies exclusion from watchlist and contact pickers |

## Notes

- **Staleness:** the captured access token expires (~1h). If a run fails on
  auth, re-capture. (The bundle also carries the refresh token, so the SDK may
  refresh silently within a run.)
- **Secrecy:** `storageState/*.json` is git-ignored. Never commit it.
- **Config:** the app must have a working `public/config.js` for the same
  tenant/backend the session was issued against — the client-instance hash in
  the sessionStorage keys must match, which it does when the config is
  unchanged.
