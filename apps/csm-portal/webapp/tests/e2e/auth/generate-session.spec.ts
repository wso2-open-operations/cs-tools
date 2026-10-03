// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

//
// Mints a real, authenticated Playwright session bundle with NO human in the
// loop, replacing the manual "sign in by hand, copy from DevTools console"
// capture step documented in this folder's README for a local run. Run via
// `pnpm run test:e2e:auth` (see package.json) — e.g. in a pre-push hook or
// CI step, right before the specs in ../specs that need it.
//
// Requires the local docker-compose stack already running and reachable
// (webapp, mock-oidc — see apps/csm-portal/README.md) — this spec never
// starts either one itself, see playwright.auth.config.ts's own doc comment
// for why that config (not the main playwright.config.ts) is the only way
// this file ever runs.
//
// How it authenticates: this drives the REAL app sign-in flow end to end,
// exactly as a human would in a browser — it never hand-constructs the OAuth
// URL or talks to mock-oidc's endpoints directly. Visiting any route while
// signed out is enough to trigger the app's own redirect to the IdP
// (AuthGuard's SignInRedirect, src/layouts/AuthGuard.tsx — there is no
// separate "sign in" button to click first), so the Asgardeo SDK is the one
// building the PKCE challenge/state/redirect_uri, exactly as it does for a
// real user. That redirect lands on mock-oidc's own plain HTML sign-in form
// (scripts/csm-compose/mock-oidc/main.go's handleAuthorizeForm — "a mock
// OIDC provider ... signs tokens for any requested username with no real
// credential check"), which this test fills in (Email, Groups) and submits
// like a human would. Submitting redirects back to the app with a real
// authorization code; the app exchanges it for tokens and boots signed-in.
//
// What gets captured, and why both stores: fixtures/test.ts's own doc
// comment explains that the Asgardeo React SDK keeps its tokens in
// **sessionStorage**, which Playwright's `context.storageState()` does not
// restore — so, like the manual capture process documented in README.md,
// this captures both localStorage (via storageState()) and sessionStorage
// (via a page.evaluate reading window.sessionStorage directly) into one
// `SessionBundle` object, matching fixtures/test.ts's own `SessionBundle`
// shape exactly so the existing `withRole`/`applySession`/`openContextAs`
// machinery there works against a generated file completely unchanged.
//
// Re-run freely: mock-oidc's tokens carry a 1-hour TTL (see that service's
// own signJWT calls), so re-running this (fresh browser context, fresh
// sign-in, fresh file write) is the intended way to refresh a stale bundle —
// there is no persisted state here to clean up first.
//

import fs from "node:fs";
import path from "node:path";
import { test, expect, type BrowserContext } from "@playwright/test";

/** Mirrors fixtures/test.ts's own (private) SessionBundle shape exactly —
 * duplicated here rather than imported so this file has zero dependency on
 * that module's internals, just the on-disk JSON contract between them. */
interface SessionBundle {
  origin?: string;
  localStorage?: Record<string, string>;
  sessionStorage?: Record<string, string>;
  cookies?: Awaited<ReturnType<BrowserContext["cookies"]>>;
}

// Parameterized by env vars (not CLI args — simplest to pass through
// `pnpm run test:e2e:auth` and a pre-push hook/CI step alike without needing
// to thread `--` past both pnpm and the playwright CLI).
const EMAIL = process.env.E2E_AUTH_EMAIL ?? "jane.doe@example.com";
const GROUPS = process.env.E2E_AUTH_GROUPS ?? "cs_engineer";
// The filename this writes to (tests/e2e/storageState/<role>.json) — not
// necessarily one of fixtures/test.ts's existing TimecardRole values
// ("approver" | "engineer"). Generating a bundle for a new role name doesn't
// by itself let a spec call `withRole(test, "<newRole>")` — that type is a
// closed union in fixtures/test.ts and still needs widening there once a
// real spec needs it. This script only owns producing the file on disk.
// "crApprover" (not "approver"/"engineer"): those two are captured by hand
// against a real staging backend (see fixtures/test.ts's own doc comment),
// and a plain, no-args run of this script would otherwise silently
// overwrite one of them with a token that only works against the local
// stack — breaking every staging-targeted spec using that role.
// "crApprover" is this script's own, local-stack-only role, so it's the
// only safe no-args default.
const ROLE = process.env.E2E_AUTH_ROLE ?? "crApprover";

test("mint a session bundle for a test identity via the real sign-in flow", async ({
  page,
  context,
  baseURL,
}) => {
  const origin = baseURL ?? "http://localhost:3001";

  // Visiting any route while signed out is enough — AuthGuard's own
  // SignInRedirect starts the redirect itself (see this file's top comment).
  await page.goto("/");

  // Lands on mock-oidc's own sign-in form once the SDK's redirect completes.
  await page.waitForURL(/\/oauth2\/authorize(\?|$)/, { timeout: 20_000 });

  await page.getByLabel("Email", { exact: true }).fill(EMAIL);
  await page.getByLabel(/^Groups/).fill(GROUPS);

  // Submitting redirects back to the app's own redirect_uri with a real
  // authorization code; the app exchanges it for tokens and then
  // CurrentUserProvider bootstraps via GET /users/me — the same signal
  // fixtures/test.ts's own approverSearchQuery helper waits on for a
  // similar purpose. A failed/incomplete sign-in never fires this response
  // at all, so the 30s timeout below is also the real failure signal.
  const [meResponse] = await Promise.all([
    page.waitForResponse((r) => r.url().includes("/users/me"), { timeout: 30_000 }),
    page.getByRole("button", { name: "Sign in" }).click(),
  ]);
  expect(
    meResponse.ok(),
    `GET /users/me did not succeed after sign-in (status ${meResponse.status()})`,
  ).toBeTruthy();

  // Belt-and-suspenders on top of the /users/me signal above: confirm the
  // browser actually landed back on the app's own origin, not stranded on
  // the IdP or bounced into another sign-in redirect.
  const escapedOrigin = origin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  await expect(page).toHaveURL(new RegExp(`^${escapedOrigin}`));

  // context.storageState() captures localStorage (+ cookies) for every
  // origin the context has visited; sessionStorage needs a direct read (see
  // this file's top comment) since Playwright's own API doesn't expose it.
  const storageState = await context.storageState();
  const originState = storageState.origins.find((o) => o.origin === origin);
  const localStorage: Record<string, string> = {};
  for (const { name, value } of originState?.localStorage ?? []) {
    localStorage[name] = value;
  }

  const sessionStorage = await page.evaluate(() => ({ ...window.sessionStorage }));
  const cookies = await context.cookies();

  const bundle: SessionBundle = {
    origin,
    localStorage,
    sessionStorage,
    ...(cookies.length ? { cookies } : {}),
  };

  const outPath = path.join(process.cwd(), "tests", "e2e", "storageState", `${ROLE}.json`);
  fs.mkdirSync(path.dirname(outPath), { recursive: true });
  fs.writeFileSync(outPath, JSON.stringify(bundle, null, 2));

  // eslint-disable-next-line no-console -- deliberate CLI/CI feedback, not app logging.
  console.log(
    `[generate-session] wrote ${outPath} for ${EMAIL} (groups: ${GROUPS})`,
  );
});
