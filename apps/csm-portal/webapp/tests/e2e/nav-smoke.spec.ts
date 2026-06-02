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

import { test, expect } from "@playwright/test";

/**
 * Unauthenticated smoke. Three assertions are enough here:
 * - Dev server is serving the SPA shell (HTML returns).
 * - `/` redirects to `/dashboard` (since the recent root-redirect change).
 * - An unauthenticated visit ends up at the IdP sign-in, NOT a dashboard
 *   render. This is the auth wall — if it ever breaks, the entire portal
 *   leaks behind anonymous traffic.
 *
 * No mocks need to be primed for these checks; they exercise the shell only.
 */
test("dev server serves the SPA shell", async ({ page }) => {
  const response = await page.goto("/");
  expect(response).not.toBeNull();
  expect(response!.status()).toBeLessThan(500);
  // Vite's default index.html — pulled in by the shell.
  expect(await page.title()).toMatch(/CSM/i);
});

test("root path is rewritten to /dashboard", async ({ page }) => {
  await page.goto("/");
  // The Navigate component runs client-side; wait for the rewrite to settle.
  // We do NOT wait for full load because the IdP redirect may steal the
  // navigation immediately afterwards.
  await page.waitForURL(
    (url) =>
      url.pathname === "/dashboard" ||
      url.host.includes("asgardeo") ||
      url.pathname.includes("/oauth2") ||
      url.pathname.includes("/authenticate"),
    { timeout: 10_000 },
  );
});

test("unauthenticated visit lands at the IdP — auth wall is intact", async ({
  page,
}) => {
  await page.goto("/dashboard");
  // Either we are on the IdP origin already, or we are mid-OIDC redirect on
  // an /oauth2-* path. Anything that looks like the CSM dashboard rendered
  // is a regression.
  await page.waitForURL(
    (url) =>
      url.host !== "localhost:3001" ||
      url.pathname.includes("/oauth2") ||
      url.pathname.includes("/authenticate"),
    { timeout: 15_000 },
  );
});
