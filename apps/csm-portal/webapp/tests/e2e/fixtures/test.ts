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

import { test as base, expect, type Browser, type BrowserContext, type Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

export type TimecardRole = "approver" | "engineer";

/** A captured session: the origin's localStorage + sessionStorage snapshots. */
interface SessionBundle {
  origin?: string;
  localStorage?: Record<string, string>;
  sessionStorage?: Record<string, string>;
}

/** Absolute path to a role's captured session bundle. */
export function sessionPath(role: TimecardRole): string {
  return path.join(process.cwd(), "tests", "e2e", "storageState", `${role}.json`);
}

export function hasSession(role: TimecardRole): boolean {
  return fs.existsSync(sessionPath(role));
}

function readBundle(role: TimecardRole): SessionBundle {
  return JSON.parse(fs.readFileSync(sessionPath(role), "utf8")) as SessionBundle;
}

async function applySession(context: BrowserContext, role: TimecardRole): Promise<void> {
  const bundle = readBundle(role);
  await context.addInitScript((b: SessionBundle) => {
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
 * Opens a second, independent browser context authenticated as `role` — for
 * specs that need two identities in the same test (e.g. one account creates
 * a card, a different account decides it). Caller must close the returned
 * context. Requires that role's session to have been captured.
 */
export async function openContextAs(
  browser: Browser,
  role: TimecardRole,
): Promise<BrowserContext> {
  const context = await browser.newContext();
  await applySession(context, role);
  return context;
}

/**
 * Configure a test file to run authenticated as `role`. Replays the captured
 * localStorage + sessionStorage before each page loads (so the Asgardeo SDK
 * finds its session and boots signed-in). Skips the whole file when the bundle
 * is absent, with a message pointing at the capture steps.
 *
 * Usage at the top of a spec:
 *   withRole(test, "approver");
 */
export function withRole(t: typeof base, role: TimecardRole): void {
  t.beforeEach(async ({ context }) => {
    t.skip(
      !hasSession(role),
      `No captured session for '${role}'. See tests/e2e/auth/README.md to create ` +
        `tests/e2e/storageState/${role}.json.`,
    );
    await applySession(context, role);
  });
}

/**
 * A search string for the approver picker that's virtually guaranteed to
 * surface a real, email-having, *other* account — the signed-in user's own
 * email domain. A generic single-letter query can match only empty-email
 * service accounts in a small staging tenant (confirmed), so the query still
 * needs to be domain-shaped, not just any string. Deliberately not the
 * signed-in user's own address: `LogTimeCardDialog` excludes the signed-in
 * user from approver candidates (self-approval prevention), so a query that
 * only the current user's own account could match would always come back
 * empty.
 */
export async function approverSearchQuery(page: Page): Promise<string> {
  const [meResp] = await Promise.all([
    page.waitForResponse((r) => r.url().includes("/users/me")),
    page.goto("/dashboard"),
  ]);
  const me = (await meResp.json()) as { email?: string };
  return me.email?.split("@")[1] ?? "a";
}

export const test = base;
export { expect };
