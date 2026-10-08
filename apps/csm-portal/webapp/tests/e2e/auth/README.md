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

## Capture a session automatically (no human step)

`generate-session.spec.ts` drives the exact same sign-in flow a human would in
a browser — it navigates the webapp, lets `AuthGuard` redirect it to the IdP,
fills in mock-oidc's own plain "Email"/"Groups" sign-in form, and submits —
then captures the resulting localStorage + sessionStorage into the same
bundle shape described above. No login page is hand-constructed and no token
is reverse-engineered; the Asgardeo SDK itself builds the PKCE
challenge/state/redirect_uri, same as it does for a real user. This is what
makes session capture safe to run unattended (a pre-push hook, a CI step)
instead of a one-time manual capture.

Requires the local docker-compose stack already running and reachable — the
webapp and mock-oidc (`docker ps`; see `apps/csm-portal/README.md`) — and
targets whichever port is actually published (`http://localhost:3001` by
default; override with `E2E_BASE_URL`). It deliberately has its own
`playwright.auth.config.ts` (not a project inside the main
`playwright.config.ts`) with no `webServer` block, so it never tries to boot
`pnpm run dev` itself — it only ever points at a stack you already started.

```bash
# Defaults (jane.doe@example.com / cs_engineer / crApprover.json) — jane.doe
# is a user entity-service's local seed data already recognizes; GET
# /users/me resolves a caller's roles/groups/teams from entity-service's own
# stored record for that email, NOT from the ID token's groups claim, so an
# email with no matching entity-service user 404s here regardless of what's
# typed into mock-oidc's Groups field. Pick (or ask the entity-service owner
# to seed) an email that already exists there.
pnpm run test:e2e:auth

# A specific identity/role
E2E_AUTH_EMAIL=jane.doe@example.com E2E_AUTH_GROUPS=cs_engineer E2E_AUTH_ROLE=crApprover \
  pnpm run test:e2e:auth
```

Env vars (all optional):

- `E2E_AUTH_EMAIL` — the account to sign in as (default `jane.doe@example.com`,
  mock-oidc's own form default).
- `E2E_AUTH_GROUPS` — comma-separated groups sent to mock-oidc (default
  `cs_engineer`). Mostly relevant to mock-oidc's own ID token claims; the
  portal's actual role/team/group gating comes from entity-service's stored
  record for the email above, not from this value.
- `E2E_AUTH_ROLE` — the output filename, `tests/e2e/storageState/<role>.json`
  (default `crApprover` — the one role this script itself owns end to end;
  never `"approver"`/`"engineer"`, both captured by hand against a real
  staging backend, which a plain local-stack run of this script would
  otherwise silently overwrite with a token that only works locally). Not
  restricted to the roles `fixtures/test.ts`'s `withRole` currently knows
  about (`"approver" | "engineer" | "crApprover" | "crInternalApprover"`) — generating a new name
  here is fine, but a spec can't call `withRole(test, "<newRole>")` until
  that union type is widened to include it.

Re-run any time to mint a fresh bundle — mock-oidc's tokens carry a 1-hour
TTL (see that service's own `signJWT` calls), so there's no "stale bundle"
state to clean up first; a fresh run always overwrites the file.

### Sessions for the Change Request seed personas

The Change Request lifecycle spec (`specs/operations/change-request-lifecycle.spec.ts`)
signs in as the seed's *internal* personas (`scripts/csm-compose/seed-entity-service.sql`; the
table is under "Local seed personas" in `entity-service/CLAUDE.md`), one captured
session per role. Mint both against the running local stack (webapp on
`http://localhost:3001`, mock-oidc, entity-service + BFF up and seeded) from
`apps/csm-portal/webapp`:

| Role (`storageState/<role>.json`) | `E2E_AUTH_EMAIL` | Who |
|---|---|---|
| `crApprover` | `jane.doe@example.com` | internal; the requester persona (in no approval group) |
| `crInternalApprover` | `alice.perera@example.com` | internal; peer / CAB approver (Bob Fernando and Carol Silva hold the same seats) |

```bash
mint() { # mint <role> <email local part>
  E2E_AUTH_EMAIL="$2@example.com" E2E_AUTH_ROLE="$1" E2E_AUTH_GROUPS=cs_engineer E2E_NO_WEBSERVER=1 \
    node_modules/.bin/playwright test --config=playwright.auth.config.ts
}
mint crApprover jane.doe
mint crInternalApprover alice.perera
```

(Add `E2E_BASE_URL=http://localhost:<port>` when the webapp is not on `:3001`.)

Each email must exist in the entity-service seed or `GET /users/me` 404s and the mint
fails. Tokens last an hour: re-run the two `mint` lines if the spec starts failing on
auth. A test whose role has no session is *skipped*, not failed.

**There is no session for the seed's customers** (`dave.mendis`, `erin.jayawardena`,
`mira.santos`, `noel.prasad`): customers do not sign in to the CSM portal. They answer a
change request's Customer Approval / Customer Review in the **customer portal**
(`apps/customer-portal`), so a customer session belongs to that portal's own test setup,
not here. A CSM spec that needs the customer's answer — to assert what the Approvals tab then
*shows* (the deciding contact's row Approved / Rejected, the others' Cancelled, the change
Scheduled / Closed / Canceled / Rollback) — applies the answer server-side and then reloads the
CSM page as an internal user:

- on the real stack, `utils/customerPortalDecision.ts` signs the contact in at the local
  mock-oidc (no browser) and sends the decision to entity-service with that contact's own ID
  token, which is what the customer portal's backend forwards (`E2E_OIDC_URL`,
  `E2E_ENTITY_SERVICE_URL` and `E2E_CUSTOMER_PORTAL_URL` override its `http://localhost:9100`
  / `:8081` / `:3000` defaults);
- against the fake API, it is `api.customerDecides(contact, decision)`.

A customer's PROPOSED TIME (a new start; the change then waits in Customer Approval for WSO2's answer) is applied the same
way: `proposeAsCustomer(crId, email, { plannedStartOn })` in `utils/customerPortalDecision.ts` on the real stack
(`PATCH /change-requests/{id}` to entity-service with the contact's own ID token, as the customer portal's backend sends it),
`api.customerProposes(contact, startOn)` against the fake API. `api.seedProposal(...)` seeds the shapes that must NOT read as a
proposal ("a customer's proposed time (mocked backend)" in the lifecycle spec).

Staff never record a customer's approval or review, so there is nothing for a spec to drive on the CSM side
either: no "Bypass customer approval" / "Bypass customer review" entry exists in the "Change state" menu, enabled or
disabled, and the specs assert it is absent (with the customer asked and with nobody asked). The seeded-fixture
describe only LOOKS at CHG-FIXED-007 / -008 (Dave and Erin still requested) and sends the manual PATCH the backend
refuses (`state "scheduled" cannot be set manually from customer_approval: the customer's approval can only be given by
the customer in the Customer Portal; ...`), so nothing changes.

**The real-stack lock tests** ("the customer requirements lock (real stack)" in the same spec) RAISE change
requests through the CSM portal's backend as the seed's staff (`utils/realStackApi.ts`: jane raises, alice and bob
approve; the mock identity provider signs any email in) and drive the Edit dialog as jane. They write, so they need the
stack under test named explicitly (the backend, the identity provider and entity-service: all three) and SKIP without it -- there is no default, the stock `:8082` could be somebody's running stack:

One of them ("Request Approval is refused when a customer box is ticked and nobody on the project can be asked") needs a project whose
contacts cannot be asked: it takes the first active project for which the backend itself reports no `customerContacts` (the generated
projects hold no registered portal-user contact; the seeded Example Corp and Other Corp do) and skips when every project has one.

| Variable | Meaning |
|---|---|
| `E2E_CSM_BFF_URL` | The CSM portal's backend as the host reaches it (isolated stack: `http://localhost:18082`) |
| `E2E_POSTGRES_CONTAINER` | Its Postgres (isolated stack: `csmenv-postgres-1`); the tests delete what they raised (`E2E lock: ...`) before and after |
| `E2E_OIDC_URL` | The mock identity provider (isolated stack: `http://localhost:19100`) |
| `E2E_ENTITY_SERVICE_URL`, `E2E_CUSTOMER_PORTAL_URL` | Where a customer's answer is applied and the customer webapp's origin (isolated stack: `http://localhost:18081`, `http://localhost:13000`) |
| `E2E_SHOT_DIR` | Optional: save pictures of the key screens there |

```bash
E2E_BASE_URL=http://localhost:13001 E2E_NO_WEBSERVER=1 E2E_POSTGRES_CONTAINER=csmenv-postgres-1 \
E2E_CSM_BFF_URL=http://localhost:18082 E2E_OIDC_URL=http://localhost:19100 \
E2E_ENTITY_SERVICE_URL=http://localhost:18081 E2E_CUSTOMER_PORTAL_URL=http://localhost:13000 \
  node_modules/.bin/playwright test tests/e2e/specs/operations/change-request-lifecycle.spec.ts --project=chromium
```

The seeded-fixture describes of the spec also reset the fixtures first, by piping
`seed-entity-service.sql` into the compose Postgres with `docker exec -i`
(`E2E_POSTGRES_CONTAINER`, default `csm-platform-postgres-1`; for the `csmcr` project use
`csmcr-postgres-1`) — the seed is self-healing, so that puts the `CHG-FIXED-*` rows back
to their starting state.

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
