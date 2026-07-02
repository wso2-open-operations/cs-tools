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

import { defineConfig, devices } from "@playwright/test";

// Base URL of the locally-served app. Override with E2E_BASE_URL to point at a
// deployed environment (the `webServer` block below is skipped in that case by
// setting E2E_NO_WEBSERVER=1).
const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:3001";

// The `capture` project (auth/capture.setup.ts) reuses your real browser session
// to produce tests/e2e/storageState/<role>.json — it must not run as part of the
// normal suite. The `chromium` project runs the actual specs and ignores it.
const CAPTURE_MATCH = /auth\/capture\.setup\.ts/;

export default defineConfig({
  testDir: "./tests/e2e",
  timeout: 30_000,
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  outputDir: "test-results",
  reporter: [["html", { open: "never" }], ["list"]],
  use: {
    baseURL: BASE_URL,
    trace: "retain-on-failure",
    video: "retain-on-failure",
  },
  // Boot the local dev server for the run (reused if already running). Skipped
  // when targeting a remote E2E_BASE_URL.
  webServer: process.env.E2E_NO_WEBSERVER
    ? undefined
    : {
        command: "pnpm run dev",
        url: BASE_URL,
        reuseExistingServer: true,
        timeout: 120_000,
      },
  projects: [
    {
      // On-demand: `ROLE=approver CHROME_PROFILE_DIR=… pnpm exec playwright test --project=capture`
      name: "capture",
      testMatch: CAPTURE_MATCH,
      use: { ...devices["Desktop Chrome"] },
    },
    {
      name: "chromium",
      testIgnore: CAPTURE_MATCH,
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
