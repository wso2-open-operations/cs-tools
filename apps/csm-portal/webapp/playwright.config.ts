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

export default defineConfig({
  testDir: "./tests/e2e",
  timeout: 30_000,
  // Specs run against a real staging backend (no mocking), all under the same
  // captured account -- concurrent workers cause real network contention and
  // flaky timeouts (observed directly: a state-filter assertion failed under
  // 4 workers, passed cleanly serial). One worker trades speed for determinism.
  fullyParallel: false,
  workers: 1,
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
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
