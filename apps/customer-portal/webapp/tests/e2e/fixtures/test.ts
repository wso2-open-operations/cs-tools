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
// Shared Playwright test helpers. Auth is by replaying a captured browser
// session — no login page is driven. The Asgardeo React SDK keeps its tokens in
// **sessionStorage** (session_data-instance_… etc.), which Playwright's
// storageState does not restore, so we replay both localStorage and
// sessionStorage via an init script that runs before the app boots. A file is
// skipped (not failed) when its role's session bundle hasn't been captured.
//

import {
  test as base,
  expect,
  type Browser,
  type BrowserContext,
} from "@playwright/test";
import fs from "node:fs";
import { loginIdentity } from "../auth/credentials";
import path from "node:path";

/** The default captured session, used for login unless a spec asks for another.
 * Additional identities (for multi-account specs) are captured alongside it as
 * `storageState/<name>.json`; see tests/e2e/auth/README.md. */
export const DEFAULT_SESSION = "session";

/** A captured session: the origin's localStorage + sessionStorage snapshots.
 * `cookies` (optional) carries the IdP-domain cookies so the SDK's silent
 * token refresh can succeed mid-run when the short-lived access token expires;
 * bundles without it still replay fine for the access token's TTL. */
interface SessionBundle {
  /** Which account the session belongs to (see auth/credentials.ts). Absent in
   * bundles captured before identities existed; such a bundle is replayed with a
   * warning rather than refused, so an older capture still works. */
  identity?: string;
  /** Origin the bundle was captured from. Required in practice — it scopes the
   * storage replay so tokens are never restored into a cross-origin frame.
   * Optional here only because the on-disk JSON is untrusted input. */
  origin?: string;
  localStorage?: Record<string, string>;
  sessionStorage?: Record<string, string>;
  cookies?: Parameters<BrowserContext["addCookies"]>[0];
}

/** Absolute path to a captured session bundle. */
export function sessionPath(name: string = DEFAULT_SESSION): string {
  return path.join(process.cwd(), "tests", "e2e", "storageState", `${name}.json`);
}

export function hasSession(name: string = DEFAULT_SESSION): boolean {
  return fs.existsSync(sessionPath(name));
}

function readBundle(name: string): SessionBundle {
  return JSON.parse(fs.readFileSync(sessionPath(name), "utf8")) as SessionBundle;
}

/** Origin a captured bundle belongs to, or undefined if it has none/is absent. */
export function sessionOrigin(name: string = DEFAULT_SESSION): string | undefined {
  return hasSession(name) ? readBundle(name).origin : undefined;
}

/** Identity a captured bundle belongs to, or undefined for a pre-identity
 * bundle. */
export function sessionIdentity(
  name: string = DEFAULT_SESSION,
): string | undefined {
  return hasSession(name) ? readBundle(name).identity : undefined;
}

async function applySession(
  context: BrowserContext,
  name: string,
): Promise<void> {
  const bundle = readBundle(name);
  if (!bundle.origin) {
    // Without an origin we cannot tell the portal's documents apart from the
    // IdP's, and the init script below would have to either skip everything or
    // write tokens into whatever frame loads first. Fail loudly here instead of
    // silently booting signed-out. The capture snippet in auth/README.md always
    // records `origin`; a bundle missing it predates that and must be recaptured.
    throw new Error(
      `Session bundle '${name}.json' has no "origin". Recapture it — see tests/e2e/auth/README.md.`,
    );
  }
  if (bundle.cookies?.length) {
    // Best-effort: lets the SDK's hidden-iframe silent refresh reach the IdP
    // with an existing session when the access token expires during a long run.
    // A malformed cookies array must not abort the beforeEach — degrade
    // gracefully, same as the storage replay below.
    try {
      await context.addCookies(bundle.cookies);
    } catch {
      // Cookies not applicable to this context; the silent refresh path just
      // won't have an IdP session to lean on, which is safe to ignore here.
    }
  }
  await context.addInitScript((b: SessionBundle) => {
    // This runs in *every* document in the context, including cross-origin
    // frames — notably the SDK's hidden IdP iframe used for silent token
    // refresh. Restore only into the origin the bundle was captured from, so
    // the portal's tokens are never written into another origin's storage.
    // (A stale bundle captured against a different origin than the one under
    // test therefore restores nothing and the app boots signed-out; recapture
    // against the target environment — see auth/README.md.)
    if (!b.origin || window.location.origin !== b.origin) return;
    try {
      for (const [k, v] of Object.entries(b.localStorage ?? {})) {
        window.localStorage.setItem(k, v);
      }
      for (const [k, v] of Object.entries(b.sessionStorage ?? {})) {
        window.sessionStorage.setItem(k, v);
      }
    } catch {
      // Storage not accessible on this document yet; the next navigation
      // re-runs this init script, so it's safe to ignore.
    }
  }, bundle);
}

/**
 * Opens a second, independent authenticated browser context — for specs that
 * need two identities in the same test (e.g. an admin edits another user's
 * roles). Pass the name of another captured bundle. Caller must close the
 * returned context.
 */
export async function openContextAs(
  browser: Browser,
  name: string,
): Promise<BrowserContext> {
  const context = await browser.newContext();
  await applySession(context, name);
  return context;
}

/**
 * Configure a test file to run authenticated. Replays the captured
 * localStorage + sessionStorage from `storageState/session.json` before each
 * page loads, so the Asgardeo SDK finds its session and boots signed-in.
 * Defaults to the shared `session` bundle; pass a name to use another identity.
 *
 * It skips from `beforeEach`, so each test that uses it is reported
 * individually as skipped (rather than the file being skipped as a unit) when
 * the bundle is missing, or when the run targets an origin the bundle was not
 * captured against — in that case the storage replay is scoped out and the app
 * would silently boot signed-out, which reads as a mass assertion failure
 * instead of a setup problem.
 *
 * Usage at the top of a spec:
 *   withSession(test);
 */
export function withSession(t: typeof base, name: string = DEFAULT_SESSION): void {
  t.beforeEach(async ({ context, baseURL }) => {
    t.skip(
      !hasSession(name),
      `No captured session '${name}'. See tests/e2e/auth/README.md to create ` +
        `tests/e2e/storageState/${name}.json.`,
    );
    const captured = sessionOrigin(name);
    const target = baseURL ? new URL(baseURL).origin : undefined;
    t.skip(
      !!captured && !!target && captured !== target,
      `Session '${name}' was captured against ${captured} but this run targets ` +
        `${target}. Re-run with E2E_BASE_URL=${captured} E2E_NO_WEBSERVER=1, or ` +
        `recapture against ${target}.`,
    );
    // Same shape of check as the origin above, and for the same class of bug:
    // replaying a bundle captured as another account runs the suite as the wrong
    // user. Only enforced when the bundle records an identity — an older capture
    // has none, and refusing those would break the documented fallback to a
    // hand-captured session.
    const capturedIdentity = sessionIdentity(name);
    const wantedIdentity = loginIdentity();
    t.skip(
      !!capturedIdentity && capturedIdentity !== wantedIdentity,
      `Session '${name}' was captured as "${capturedIdentity}" but this run ` +
        `wants "${wantedIdentity}". Delete ` +
        `tests/e2e/storageState/${name}.json to have the auth setup mint a new ` +
        `one, or set E2E_LOGIN_IDENTITY=${capturedIdentity}.`,
    );

    await applySession(context, name);
  });
}

//
// Everything under tests/e2e imports `test`, `expect` and the Playwright types
// from this module rather than from "@playwright/test" directly. Today `test` is
// just `base`, so the two are equivalent — but the moment withSession becomes a
// proper `base.extend()` fixture, a direct import would silently bypass it and
// run without the session replay. Re-exporting here keeps that refactor safe.
//
export const test = base;
export { expect };
export type { Download, Locator, Page, Response } from "@playwright/test";
