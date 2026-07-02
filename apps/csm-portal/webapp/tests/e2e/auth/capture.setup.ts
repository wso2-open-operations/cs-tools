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
// On-demand capture of an already-authenticated browser session into a
// storageState file that the specs reuse — so the suite never drives the
// Asgardeo login page or 2FA locally. Runs only via the `capture` project:
//
//   ROLE=approver CHROME_PROFILE_DIR="$HOME/Library/Application Support/Google/Chrome/Default" \
//     pnpm exec playwright test --project=capture
//
// Requirements: you are already signed in to the app in that Chrome profile,
// and Chrome is fully closed (the profile is locked while Chrome runs).
// See ./README.md for the manual DevTools alternative.
//

import { test, chromium, expect } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

test("capture authenticated session", async () => {
  test.slow(); // launching a real profile + first paint can be slow

  const role = process.env.ROLE ?? "approver";
  const profileDir = process.env.CHROME_PROFILE_DIR;
  const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:3001";

  expect(
    profileDir,
    "Set CHROME_PROFILE_DIR to your logged-in Chrome profile directory.",
  ).toBeTruthy();

  const outDir = path.join(process.cwd(), "tests", "e2e", "storageState");
  fs.mkdirSync(outDir, { recursive: true });
  const outPath = path.join(outDir, `${role}.json`);

  // Reuse the real profile so we inherit its existing Asgardeo session.
  const ctx = await chromium.launchPersistentContext(profileDir!, {
    channel: "chrome",
    headless: false,
  });
  try {
    const page = ctx.pages()[0] ?? (await ctx.newPage());
    await page.goto(`${baseURL}/time-cards`);
    // Confirm we're authenticated and the app rendered (not the IdP page).
    await expect(
      page.getByRole("heading", { name: "Time cards" }),
    ).toBeVisible({ timeout: 60_000 });

    await ctx.storageState({ path: outPath });
    console.log(`\nSaved ${role} session → ${outPath}\n`);
  } finally {
    await ctx.close();
  }
});
