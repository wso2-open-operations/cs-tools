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
// The header's "Customer Portal" link, as the Subscription portal user
// (E2E_SUB_PORTAL_*, customer-portal-sub-portal-stg@wso2support.com).
//
// Unlike the admin — who lands on the portal home with its Projects and Cases
// tables (dashboard.spec.ts) — this user belongs to a single project, so the
// link redirects straight to that project's dashboard.
//
// Lives in its own file because dashboard.spec.ts replays the admin session
// before every test; this one signs in as a different user.
//
// ✅ READ-ONLY.
//
import { test, expect } from "../../fixtures/test";
import { hasRoleCredentials } from "../../auth/credentials";
import { signInAsRole } from "../../auth/signInAsRole";
import { DashboardPage } from "../../pages/DashboardPage";
import { PROJECTS, ProjectType } from "../../config/testData";
import { DASHBOARD, PORTAL_HOME } from "../../utils/selectors";
import { projectPathPattern } from "../../utils/ids";

test.use({ trace: "off", video: "off" });

const PROJECT_KEY = "SUB" as const;
const project = PROJECTS[ProjectType.SUBSCRIPTION];

test.describe("Dashboard — portal user", () => {
  test.describe.configure({ timeout: 300_000 });

  test("the Customer Portal link opens the project dashboard", async ({
    page,
    baseURL,
  }) => {
    test.skip(
      !hasRoleCredentials(PROJECT_KEY, "PORTAL"),
      `No credentials for ${PROJECT_KEY}/PORTAL — set E2E_${PROJECT_KEY}_PORTAL_* ` +
        "in webapp/.env.e2e.local.",
    );
    test.skip(!project.id, "Subscription needs a project id.");

    await signInAsRole(page, PROJECT_KEY, "PORTAL", new URL(baseURL!).origin);

    const header = page.locator("header").first();
    const link = header.getByText(PORTAL_HOME.headerLink, { exact: true });
    await expect(link).toBeVisible({ timeout: 60_000 });
    await link.click();

    // Redirected to the one project's dashboard, not the portal home.
    await expect(page).toHaveURL(projectPathPattern(project.id, "dashboard"), {
      timeout: 60_000,
    });
    await expect(
      page.getByRole("heading", { name: PORTAL_HOME.projectsHeading }),
    ).toHaveCount(0);

    // The top four stat cards.
    const dashboard = new DashboardPage(page);
    for (const card of DASHBOARD.statCards) {
      await expect(dashboard.statCard(card.label), card.label).toBeVisible({
        timeout: 60_000,
      });
    }
    expect(DASHBOARD.statCards).toHaveLength(4);

    // The Outstanding Support Cases section.
    const main = page.getByTestId("app-main");
    await expect(
      main.getByText(DASHBOARD.sections.outstandingCases, { exact: true }).first(),
    ).toBeVisible({ timeout: 60_000 });
    await expect(
      main.getByText(DASHBOARD.casesTable.subtitle, { exact: true }),
    ).toBeVisible();

    console.log(`Portal user: Customer Portal → ${page.url()}`);
  });
});
