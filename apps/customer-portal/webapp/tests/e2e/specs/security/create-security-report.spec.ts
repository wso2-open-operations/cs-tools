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
// Raises a security report from the "Get Help" dropdown, on each project type.
//
// A security report is a case raised at /support/security-report/create. The
// same CreateCasePage backs it, with `isSecurityReport` derived from the path —
// so Issue Type and Severity are hidden, an attachment is mandatory, and the
// Title is generated from the deployment, product and today's date rather than
// entered.
//
// Coverage differs by project type, verified live:
//
// - Subscription and Managed Cloud Subscription offer the option and raise a
//   report. Both show a Deployment select, with Product gated on it ("Select
//   deployment first").
// - Cloud Support does NOT offer it — the Get Help dropdown has no Security
//   Report item, because the option is gated on the project's SRA write access.
//   Its test asserts that absence, which passes today and would fail if the
//   option appeared. A skip would silently pass either way.
//
// The `hasSecurityReport` flag on each fixture decides which branch a project
// takes, so the expectation is declared in config rather than discovered at
// runtime.
//
// ⚠️ NOT idempotent. This test raises a report unconditionally, so every run —
// including every retry and every scheduled execution — leaves one more
// permanent, customer-visible record. Reports are cases, and cases have no
// delete endpoint, so nothing here or elsewhere can clean them up.
//
// It previously looked for an existing report by description and returned early
// when it found one. That guard made repeat runs free, but it also meant the
// creation path stopped being exercised the moment the first report existed: on
// a project that already had one the test passed without submitting anything.
// Creating unconditionally is the deliberate trade — coverage of the write path
// in exchange for a record per run.
//
// The descriptions in SECURITY_REPORT_INPUT are still stable per project, so the
// reports this leaves behind remain identifiable in the target environment.
//

import { test, expect, withSession } from "../../fixtures/test";
import { SecurityReportCreatePage } from "../../pages/SecurityReportCreatePage";
import {
  PROJECTS,
  ProjectType,
  SECURITY_REPORT_ATTACHMENT,
  SECURITY_REPORT_INPUT,
} from "../../config/testData";
import { expectSuccess, skipWhenUnconfigured } from "../../utils/caseFlows";
import { idPattern, projectPathPattern } from "../../utils/ids";
import {
  permanentWriteSkipReason,
  permanentWritesAllowed,
} from "../../utils/permanentWrites";

withSession(test);

test.describe("Security Report", () => {
  // Spans a dashboard load, a menu navigation, backend-populated dropdowns and a
  // file upload; the 30s default is not enough.
  test.describe.configure({ timeout: 180_000 });

  for (const projectType of Object.values(ProjectType)) {
    const project = PROJECTS[projectType];
    const input = SECURITY_REPORT_INPUT[projectType];

    test.describe(projectType, () => {
      if (!project.hasSecurityReport) {
        // Security reports are gated on the project's SRA write access, which
        // this project does not have. Asserting the option is absent is the
        // correct expectation here — and it still fails if the option ever
        // appears, which a skip would not.
        test("does not offer security reports", async ({ page }) => {
          skipWhenUnconfigured(project);

          const form = new SecurityReportCreatePage(page);
          await page.goto(`/projects/${project.id}/dashboard`);
          await expect(
            page.getByRole("button", { name: "Get Help", exact: true }),
          ).toBeVisible({ timeout: 60_000 });

          await form.openGetHelpMenu();
          await expect(form.securityReportMenuItem()).toHaveCount(0);
        });
        return;
      }

      test("create security report", async ({ page }) => {
        skipWhenUnconfigured(project);
        test.skip(
          !permanentWritesAllowed(),
          permanentWriteSkipReason("a security report (a case)"),
        );

        const form = new SecurityReportCreatePage(page);
        await form.openViaGetHelpMenu(project.id);

        // Issue Type and Severity belong to the ordinary case form only.
        // Asserting they are absent is the check that this route really is in
        // security-report mode, rather than the case form at another URL.
        await expect(form.issueTypeSelect()).toBeHidden();
        await expect(form.severitySelect()).toBeHidden();

        if (project.autoSelectsDeployment) {
          // Locked to primary production, so the field is not rendered at all.
          await expect(form.deploymentSelect()).toBeHidden();
        } else {
          await form.selectDeployment(project.deployment);
        }
        await form.selectProductVersion(project.productVersion);

        // The title is not typed: choosing a deployment and product generates it
        // as "<deployment> - <product name> - YYYY-MM-DD", overwriting anything
        // entered. Asserting it means the report is submitted with a title we
        // have actually verified rather than an unchecked one.
        await expect(form.titleInput()).toHaveValue(
          SecurityReportCreatePage.expectedTitlePattern(
            project.autoSelectsDeployment ? null : project.deployment,
            project.productName,
          ),
        );

        await form.fillDescription(input.description);
        await form.attachSecurityReport(SECURITY_REPORT_ATTACHMENT);

        await expect(form.submitButton()).toBeEnabled();

        // Capture the created report from the response so the assertions below
        // prove the backend accepted it, not just that the UI moved on. Security
        // reports post to /cases like every other case type.
        const [createResponse] = await Promise.all([
          page.waitForResponse(
            (r) =>
              new URL(r.url()).pathname.endsWith("/cases") &&
              r.request().method() === "POST",
          ),
          form.submit(),
        ]);

        // Status asserted here rather than in the predicate, so a rejected
        // create reports the server's message instead of timing out.
        await expectSuccess(createResponse, "create security report");

        const created = (await createResponse.json()) as {
          id?: string;
          number?: string;
        };
        expect(created.id, "backend returned no case id").toBeTruthy();

        // Both ids go through idPattern: the portal renders them in their
        // UUID-hyphenated spelling while the fixture and the API use the plain
        // 32-hex form. This assertion never ran while the existence guard
        // short-circuited the test, so it had not yet hit that mismatch.
        await expect(page).toHaveURL(
          projectPathPattern(project.id, `.*/${idPattern(created.id!)}`),
        );

        console.log(
          `Created security report (${projectType}): ` +
            `${created.number ?? created.id}`,
        );
      });
    });
  }
});
