# csm-portal e2e tests

Playwright suite for the CSM portal. Mocks are on (`window.config.CSM_PORTAL_USE_MOCKS = true`), so no backend is required for any test.

## Layout

| File | Purpose | Auth |
|---|---|---|
| `nav-smoke.spec.ts` | Verifies the dev server is serving, root redirects to `/dashboard`, and that the unauthenticated visit lands at the IdP sign-in. Covers the auth wall. | None |
| `auth.setup.ts` | Loads a pre-recorded `storageState` so the authenticated suite skips the IdP dance. Errors with a clear message if missing. | n/a |
| `case-create.spec.ts` | Drives `/cases/new` end-to-end: project → environment → product cascade, severity, subject, WYSIWYG description, submit. | Required |
| `admin.spec.ts` | Hits each admin tab (Users → Subscription closure) and asserts the heading and a representative row render. | Required |

## Running

```bash
cd apps/csm-portal/webapp
# Smoke only — no auth needed.
npx playwright test --project=smoke

# Authenticated suite (needs storage state — see below).
npx playwright test --project=authenticated
```

The Playwright config starts `npm run dev -- --port 3001` automatically (`webServer.reuseExistingServer: true`, so if you already have it running we attach instead of double-launching).

## Recording the auth storage state (one-time)

The portal sign-in goes through Asgardeo OIDC. Driving that flow from Playwright every run is slow and brittle. Instead, capture a `storageState.json` once and reuse it.

```bash
mkdir -p tests/e2e/.auth
# Opens a real browser; you sign in interactively, then close it.
npx playwright codegen \
  --output=/dev/null \
  --save-storage=tests/e2e/.auth/user.json \
  http://localhost:3001/
```

Add `tests/e2e/.auth/user.json` to `.gitignore` — it contains a real session token.

If your session expires (Asgardeo defaults ~1h), re-run the command above. The authenticated suite reads this file via `playwright.config.ts` (`projects[name=authenticated].use.storageState`).

## Why mocks must be on

Tests assert against seeded mock rows (e.g. project names, severity options, admin tabs). If `CSM_PORTAL_USE_MOCKS` is flipped off in `public/config.js`, the authenticated suite will hit the real BFF and the assertions will drift.

The smoke test does not depend on data shape and will still pass either way.
