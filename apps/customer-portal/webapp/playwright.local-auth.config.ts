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

// A separate, minimal Playwright config — deliberately NOT a project inside
// playwright.config.ts — for tests/e2e/auth/local-session.setup.ts, which mints a
// session for one of the customers the LOCAL docker-compose seed registers.
// (The CSM portal's webapp has the same split: playwright.auth.config.ts.)
//
// Two reasons it has to be separate:
//
// 1. It has no `webServer` block. The main config boots `pnpm run dev` when
//    E2E_NO_WEBSERVER is unset; minting is meant to run against the stack that is
//    ALREADY up (webapp + mock OIDC provider), never a freshly booted dev server.
// 2. It reads NO env files. The main config loads .env.e2e / .env.e2e.local, which
//    configure the sign-in of a deployed environment (credentials, TOTP seeds);
//    nothing there applies to the local mock provider, so the only input here is
//    the real environment (E2E_BASE_URL, E2E_LOCAL_PERSONA).
//
// The base URL is the customer webapp the local stack publishes (:3000 for the
// stock docker-compose.yml). A bundle only restores into the origin it was minted
// at, so mint and run the specs against the same E2E_BASE_URL.
const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:3000";

export default defineConfig({
  testDir: "./tests/e2e/auth",
  testMatch: /local-session\.setup\.ts$/,
  timeout: 90_000,
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: 0,
  outputDir: "test-results/local-auth",
  reporter: [["list"]],
  use: {
    baseURL: BASE_URL,
    // The page is the mock provider's no-credential form, but keep the habit of the
    // other auth config: record nothing from a sign-in.
    trace: "off",
    video: "off",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
