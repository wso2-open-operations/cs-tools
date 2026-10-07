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
// Mints a session bundle for one of the customers the LOCAL compose seed
// registers, with no human in the loop.
//
// Run via `pnpm run test:e2e:local-auth` (playwright.local-auth.config.ts — a
// separate config on purpose: the main one would otherwise discover this file
// and mint a new session on every regression run as a side effect).
//
// How it authenticates: it drives the REAL sign-in, as a person would. Visiting
// the app signed out makes AuthGuard redirect to the identity provider, which in
// the local stack is the mock OIDC provider (scripts/csm-compose/mock-oidc) — a
// plain form with an Email and a Groups box that signs in ANY email with no
// credential check. The Asgardeo SDK builds the PKCE challenge and redirect URI
// itself, exactly as for a real user; nothing is hand-constructed or
// reverse-engineered.
//
// ⚠️ GROUPS MUST BE EMPTY for a customer. The box is pre-filled with
// `cs_engineer`, a CSM *staff* group (the CSM portal's engineer tier). The mock
// provider copies that box into the token's `groups`/`roles` claims verbatim, so
// leaving it filled signs the customer in carrying a staff role. What a customer
// may see and do comes from their user record instead (role `customer`, a
// registered PORTAL_USER contact of a project) — the same record entity-service
// scopes by — so the box is cleared and asserted empty before submitting.
//
// What gets written: the same bundle shape as a hand capture (origin +
// localStorage + sessionStorage + cookies), to storageState/local-<persona>.json,
// replayed unchanged by fixtures/test.ts. The mock provider's tokens last ONE
// HOUR and it issues no refresh token: re-run this to refresh (it overwrites).
//
// Which origin the bundle is for is whatever E2E_BASE_URL says (default
// http://localhost:3000). A bundle only restores into the origin it was minted
// at, so a stack published on other ports (e.g. 13000) needs E2E_BASE_URL set
// both here and when running the specs.
//

import { test as setup, expect } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import {
  LOCAL_PERSONAS,
  isLocalPersona,
  localSessionName,
} from "./localSessions";

/** Persona by name (`dave`, `erin`, `mira`, `noel`), default dave. */
const PERSONA = (process.env.E2E_LOCAL_PERSONA ?? "dave").trim().toLowerCase();

setup("mint a local customer session through the real sign-in", async ({
  page,
  context,
  baseURL,
}) => {
  if (!isLocalPersona(PERSONA)) {
    throw new Error(
      `E2E_LOCAL_PERSONA="${PERSONA}" is not a seeded customer. ` +
        `Use one of: ${Object.keys(LOCAL_PERSONAS).join(", ")}.`,
    );
  }
  const { email } = LOCAL_PERSONAS[PERSONA];
  const origin = new URL(baseURL!).origin;

  // Visiting any route while signed out is enough: AuthGuard's ProtectedRoute
  // starts the redirect to the identity provider itself.
  await page.goto("/");
  await page.waitForURL(/\/oauth2\/authorize(\?|$)/, { timeout: 30_000 });

  await page.getByLabel("Email", { exact: true }).fill(email);

  // EMPTY, not the pre-filled staff group — see the header.
  const groups = page.getByLabel(/^Groups/);
  await groups.fill("");
  await expect(
    groups,
    "a customer must sign in with NO groups; the box is pre-filled with a staff group",
  ).toHaveValue("");

  // The customer backend's GET /users/me answers right after the token exchange.
  // A failed or incomplete sign-in never fires it, so this is also the failure
  // signal — and its body proves WHO signed in.
  const [me] = await Promise.all([
    page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname.endsWith("/users/me") &&
        r.request().method() === "GET",
      { timeout: 60_000 },
    ),
    page.getByRole("button", { name: "Sign in" }).click(),
  ]);
  expect(
    me.ok(),
    `GET /users/me answered ${me.status()} after signing in as ${email}. The stack may be ` +
      "unseeded or the user unknown to entity-service (it must be a seeded persona).",
  ).toBeTruthy();
  const details = (await me.json()) as { email?: string; roles?: string[] };
  expect(
    (details.email ?? "").toLowerCase(),
    "signed in as the wrong account",
  ).toBe(email.toLowerCase());
  // A customer, and ONLY a customer: no staff role leaked in through the Groups box.
  expect(details.roles ?? [], `${email} must hold only the customer role`).toEqual([
    "customer",
  ]);

  // Back on the app's own origin and past the project list / a project page, so the
  // snapshot below holds a booted app's tokens rather than the redirect in flight.
  await expect(page).toHaveURL(
    new RegExp(`^${origin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}`),
  );
  await expect(
    page
      .getByRole("heading", { name: /^Projects \(\d+\)/ })
      .or(page.getByRole("button", { name: "Get Help", exact: true }))
      .first(),
    "signed in, but neither the project list nor a project page rendered",
  ).toBeVisible({ timeout: 60_000 });

  // localStorage via storageState(); sessionStorage (where the SDK keeps its
  // tokens) needs a direct read, which Playwright's own API does not offer.
  const storageState = await context.storageState();
  const localStorage: Record<string, string> = {};
  for (const { name, value } of storageState.origins.find((o) => o.origin === origin)
    ?.localStorage ?? []) {
    localStorage[name] = value;
  }
  const sessionStorage = await page.evaluate(() => ({ ...window.sessionStorage }));
  expect(
    Object.keys(sessionStorage).length,
    "signed in but sessionStorage is empty — the SDK keeps its tokens there, so a " +
      "bundle without it replays as signed-out",
  ).toBeGreaterThan(0);
  const cookies = await context.cookies();

  // No `identity`: that field belongs to the staging accounts (admin / portal) and
  // withSession would refuse a bundle naming one this run is not signing in as.
  const bundle = {
    origin,
    persona: PERSONA,
    email,
    localStorage,
    sessionStorage,
    ...(cookies.length ? { cookies } : {}),
  };
  const out = path.join(
    process.cwd(),
    "tests",
    "e2e",
    "storageState",
    `${localSessionName(PERSONA)}.json`,
  );
  fs.mkdirSync(path.dirname(out), { recursive: true });
  fs.writeFileSync(out, `${JSON.stringify(bundle, null, 2)}\n`);

  console.log(
    `[local-session] wrote ${path.relative(process.cwd(), out)} for ${email} at ${origin} ` +
      `(${Object.keys(sessionStorage).length} sessionStorage keys; valid ~1 hour)`,
  );
});
