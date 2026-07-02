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

import { test as base, expect } from "@playwright/test";
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
    const p = sessionPath(role);
    t.skip(
      !fs.existsSync(p),
      `No captured session for '${role}'. See tests/e2e/auth/README.md to create ` +
        `tests/e2e/storageState/${role}.json.`,
    );
    const bundle = JSON.parse(fs.readFileSync(p, "utf8")) as SessionBundle;
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
  });
}

export const test = base;
export { expect };
