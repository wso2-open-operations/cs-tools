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
// Shared Playwright test helpers. Auth is by reusing a captured browser session
// (see auth/capture.setup.ts) — no login page is driven. Each spec picks the
// role whose session it needs and is skipped (not failed) when that session
// hasn't been captured, so the suite degrades gracefully on a fresh checkout.
//

import { test as base, expect } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

export type TimecardRole = "approver" | "engineer";

/** Absolute path to a role's captured storageState file. */
export function sessionPath(role: TimecardRole): string {
  return path.join(process.cwd(), "tests", "e2e", "storageState", `${role}.json`);
}

/**
 * Configure a test file to run authenticated as `role`. Sets the storageState
 * and, in a `beforeAll`, skips the whole file when the session file is absent
 * with a message telling you how to capture it.
 *
 * Usage at the top of a spec:
 *   withRole(test, "approver");
 */
export function withRole(t: typeof base, role: TimecardRole): void {
  t.use({ storageState: sessionPath(role) });
  t.beforeAll(() => {
    t.skip(
      !fs.existsSync(sessionPath(role)),
      `No captured session for '${role}'. Run: ROLE=${role} CHROME_PROFILE_DIR=<your Chrome profile> ` +
        `pnpm exec playwright test --project=capture`,
    );
  });
}

export const test = base;
export { expect };
