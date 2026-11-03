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
// Deployment license download: Project Details → Deployments, expand a
// deployment, and use the Download License button at the foot of the card.
//
// Read-only — the license is fetched with a POST but nothing is written.
//

import { readFile } from "node:fs/promises";
import { test, expect, withSession } from "../../fixtures/test";
import { ProjectDeploymentsPage } from "../../pages/ProjectDeploymentsPage";
import { PROJECTS, ProjectType } from "../../config/testData";
import { skipWhenUnconfigured } from "../../utils/caseFlows";
import { CASE_DETAIL, DEPLOYMENT_LICENSE } from "../../utils/selectors";

withSession(test);

test.describe("Deployment License", () => {
  test.setTimeout(120_000);

  test("downloads the license file from an expanded deployment", async ({
    page,
  }) => {
    const project = PROJECTS[ProjectType.SUBSCRIPTION];
    skipWhenUnconfigured(project);

    const deployments = new ProjectDeploymentsPage(page);
    await deployments.openDeploymentsTab(project.id);

    // The first deployment card's summary — the accordion header carries
    // aria-expanded, and its contents (the footer included) are unmounted
    // until it is opened.
    const summary = page
      .getByTestId(CASE_DETAIL.mainTestId)
      .locator("[aria-expanded]")
      .first();
    await expect(summary).toBeVisible({ timeout: 30_000 });
    await summary.click();
    await expect(summary).toHaveAttribute("aria-expanded", "true");

    const button = page.getByRole("button", {
      name: DEPLOYMENT_LICENSE.downloadButton,
    });
    await expect(button).toHaveCount(1);
    await button.scrollIntoViewIfNeeded();

    const [response, download] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          DEPLOYMENT_LICENSE.requestPathPattern.test(new URL(r.url()).pathname),
        { timeout: 60_000 },
      ),
      page.waitForEvent("download", { timeout: 60_000 }),
      button.click(),
    ]);
    expect(response.ok(), `license POST returned ${response.status()}`).toBe(
      true,
    );

    expect(download.suggestedFilename()).toMatch(
      DEPLOYMENT_LICENSE.filenamePattern,
    );
    const content = await readFile(await download.path(), "utf8");
    const license = JSON.parse(content) as unknown;
    expect(license && typeof license === "object").toBe(true);
    expect(Object.keys(license as object).length).toBeGreaterThan(0);
  });
});
