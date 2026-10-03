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

import { defineConfig } from "@playwright/test";

// A separate, minimal Playwright config — deliberately NOT a project inside
// the main playwright.config.ts — for tests/e2e/auth/generate-session.spec.ts.
// Two reasons it has to be separate:
//
// 1. The main config's `webServer` block boots `pnpm run dev` when
//    E2E_NO_WEBSERVER isn't set. Session generation is meant to run against
//    the already-running docker-compose stack (webapp + mock-oidc), never a
//    freshly-booted dev server — this config has no `webServer` block at all,
//    so it always targets whatever's already listening at BASE_URL.
// 2. If generate-session.spec.ts were discovered by the main config (it lives
//    under tests/e2e, which that config globs recursively), a plain
//    `pnpm run test:e2e` would mint a brand new session on every regression
//    run as a side effect. The main config's own testIgnore excludes
//    tests/e2e/auth/** for exactly this reason; this config is the only way
//    that file is ever run.
const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:3001";

export default defineConfig({
  testDir: "./tests/e2e/auth",
  testMatch: "generate-session.spec.ts",
  timeout: 60_000,
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: 0,
  outputDir: "test-results/auth",
  reporter: [["list"]],
  use: {
    baseURL: BASE_URL,
    trace: "off",
    video: "off",
  },
  projects: [{ name: "chromium" }],
});
