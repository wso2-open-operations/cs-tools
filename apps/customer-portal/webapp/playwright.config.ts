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
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

// This package is ESM ("type": "module"), so __dirname does not exist — derive
// the config's own directory so the env files resolve regardless of cwd.
const CONFIG_DIR = path.dirname(fileURLToPath(import.meta.url));

// Load E2E env files (see .env.e2e for the documented variables). Uses node's
// built-in loader rather than a dotenv dependency, and it never overwrites a
// variable already present in the environment — so precedence is:
//   real env / CLI  >  .env.e2e.local (personal, git-ignored)  >  .env.e2e
// Load order matters: the first file to define a key wins, so the personal
// override file is read first.
for (const file of [".env.e2e.local", ".env.e2e"]) {
  const envPath = path.join(CONFIG_DIR, file);
  if (fs.existsSync(envPath)) {
    process.loadEnvFile(envPath);
  }
}

// Base URL of the locally-served app (vite dev server, see vite.config.ts).
// Override with E2E_BASE_URL to point at a deployed environment, and set
// E2E_NO_WEBSERVER=1 so the `webServer` block below is skipped in that case.
const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:3000";

export default defineConfig({
  testDir: "./tests/e2e",
  timeout: 30_000,
  // Specs are expected to run against a real (staging) backend under a single
  // captured account, where concurrent workers cause real network contention
  // and flaky timeouts. Serial by default; revisit if specs ever get isolated
  // per-account fixtures.
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
        // Locally, reuse an already-running dev server. In CI, always boot a
        // fresh one — reusing a stray/stale process there could mask a real
        // failure to start, or run against unexpected leftover state.
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
      },
  projects: [
    {
      // Signs in and writes tests/e2e/storageState/session.json, which every
      // spec then replays. Runs first because the test project depends on it,
      // so a run always starts from a freshly minted session rather than a
      // hand-captured one that may already have expired.
      //
      // Skips itself when no credentials are configured, leaving whatever
      // bundle is on disk in place.
      name: "auth",
      testMatch: /auth\.setup\.ts$/,
      use: {
        ...devices["Desktop Chrome"],
        // A password and a TOTP code are typed here. Traces and video capture
        // keystrokes and DOM, so this project records neither — the artefacts
        // would otherwise contain the credential in plain text.
        trace: "off",
        video: "off",
      },
    },
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
      dependencies: ["auth"],
      // The setup project owns this file; excluding it here keeps the sign-in
      // from also running as an ordinary test.
      testIgnore: /auth\.setup\.ts$/,
    },
  ],
});
