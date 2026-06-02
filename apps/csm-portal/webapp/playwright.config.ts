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

const STORAGE_STATE = "./tests/e2e/.auth/user.json";

export default defineConfig({
  testDir: "./tests/e2e",
  timeout: 30_000,
  fullyParallel: false, // Auth state is per-process; serial avoids HMR races.
  outputDir: "test-results",
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: "http://localhost:3001",
    trace: "retain-on-failure",
    video: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: {
    command: "npm run dev -- --port 3001",
    url: "http://localhost:3001/",
    reuseExistingServer: true,
    timeout: 60_000,
  },
  projects: [
    // Unauthenticated smoke. Verifies the auth wall and public surfaces.
    {
      name: "smoke",
      testMatch: /.*nav-smoke\.spec\.ts/,
      use: { ...devices["Desktop Chrome"] },
    },
    // Authenticated flows. Require a recorded OIDC storage state; see
    // tests/e2e/README.md for the one-time capture procedure.
    {
      name: "auth-setup",
      testMatch: /.*auth\.setup\.ts/,
      use: { ...devices["Desktop Chrome"] },
    },
    {
      name: "authenticated",
      testMatch: /(case-create|admin)\.spec\.ts/,
      dependencies: ["auth-setup"],
      use: {
        ...devices["Desktop Chrome"],
        storageState: STORAGE_STATE,
      },
    },
  ],
});
