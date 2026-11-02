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
// Edge cases and field gating on the create-case form, across all three project
// types.
//
// ✅ Unlike the happy-path suite, NOTHING HERE CREATES A RECORD. Every scenario
// is either read-only or is rejected by `handleSubmit` before a request is made
// (see CreateCasePage.tsx — it returns early on an empty title, an over-long
// title, and an empty description). That makes this suite safe to run
// repeatedly, which the creation suite is not.
//
// Each test asserts the form stayed put afterwards, so a validation rule that
// silently stopped working would surface as an unexpected navigation rather
// than as a passing test plus a stray case in staging.
//

import { test, expect, withSession } from "../../fixtures/test";
import { CaseCreatePage } from "../../pages/CaseCreatePage";
import { CASE_INPUT, PROJECTS, ProjectType } from "../../config/testData";
import { skipWhenUnconfigured } from "../../utils/caseFlows";
import { CREATE_CASE } from "../../utils/selectors";

withSession(test);

/**
 * Whether the form opened with a deployment already chosen. The Deployment
 * select is located by its placeholder, so it matches nothing once a value is
 * set; the caller has already waited for the form to load.
 *
 * @param form - The open create-case form.
 * @returns True when no deployment needs choosing.
 */
async function deploymentPreselected(form: CaseCreatePage): Promise<boolean> {
  const preselected = (await form.deploymentSelect().count()) === 0;
  if (preselected) console.log("Deployment arrived pre-selected; no gate to check.");
  return preselected;
}

test.describe("Create Case — validation", () => {
  // Each test loads the form, which waits on project details, features and
  // filters; the 30s default is not enough for a cold navigation.
  test.describe.configure({ timeout: 120_000 });

  for (const projectType of Object.values(ProjectType)) {
    const project = PROJECTS[projectType];
    const caseInput = CASE_INPUT[projectType];

    test.describe(projectType, () => {
      test("rejects submit when the title is empty", async ({ page }) => {
        skipWhenUnconfigured(project);
        const form = new CaseCreatePage(page);
        await form.openViaGetHelp(project.id);
        await form.fillBasicInformation(project.deployment, project.productVersion);
        await form.fillDescription(caseInput.description);
        await form.selectIssueType(caseInput.issueType);
        await form.selectSeverity(caseInput.severity);

        await form.attemptSubmit();

        await expect(form.errorAlert()).toContainText(
          CREATE_CASE.validationErrors.missingTitle,
        );
        // Still on the form: no case was created.
        await expect(page).toHaveURL(/create-case/);
      });

      test("rejects submit when the description is empty", async ({ page }) => {
        skipWhenUnconfigured(project);
        const form = new CaseCreatePage(page);
        await form.openViaGetHelp(project.id);
        await form.fillBasicInformation(project.deployment, project.productVersion);
        await form.fillTitle(caseInput.title);
        await form.selectIssueType(caseInput.issueType);
        await form.selectSeverity(caseInput.severity);

        await form.attemptSubmit();

        await expect(form.errorAlert()).toContainText(
          CREATE_CASE.validationErrors.missingDescription,
        );
        await expect(page).toHaveURL(/create-case/);
      });

      test("rejects a title longer than 160 characters", async ({ page }) => {
        skipWhenUnconfigured(project);
        const form = new CaseCreatePage(page);
        await form.openViaGetHelp(project.id);
        await form.fillBasicInformation(project.deployment, project.productVersion);

        const overLimit = "x".repeat(CREATE_CASE.titleMaxLength + 1);
        await form.fillTitle(overLimit);
        await form.fillDescription(caseInput.description);
        await form.selectIssueType(caseInput.issueType);
        await form.selectSeverity(caseInput.severity);

        // The field flags it, and handleSubmit refuses to send — note it does so
        // silently, with no error banner, which is why the URL is the assertion
        // that the submit was actually blocked.
        await expect(form.titleLengthError()).toBeVisible();
        await expect(form.titleCounter()).toHaveText(
          `${CREATE_CASE.titleMaxLength + 1}/${CREATE_CASE.titleMaxLength}`,
        );

        await form.attemptSubmit();
        await expect(page).toHaveURL(/create-case/);
      });

      test("accepts a title of exactly 160 characters", async ({ page }) => {
        skipWhenUnconfigured(project);
        const form = new CaseCreatePage(page);
        await form.openViaGetHelp(project.id);
        await form.fillTitle("x".repeat(CREATE_CASE.titleMaxLength));

        // Boundary: 160 is allowed, 161 is not. Deliberately not submitted —
        // that would create a record; the field-level state is the assertion.
        await expect(form.titleLengthError()).toBeHidden();
        await expect(form.titleCounter()).toHaveText(
          `${CREATE_CASE.titleMaxLength}/${CREATE_CASE.titleMaxLength}`,
        );
      });

      test("offers the severity this project is configured for", async ({
        page,
      }) => {
        skipWhenUnconfigured(project);
        const form = new CaseCreatePage(page);
        await form.openViaGetHelp(project.id);
        await form.fillBasicInformation(project.deployment, project.productVersion);

        // Severity options come from the project's acceptedSeverityValues, so
        // the set differs per project. Asserting the configured one is present
        // is the check that holds for every type.
        await form.severitySelect().click();
        await expect(
          page.getByRole("option", { name: caseInput.severity, exact: true }),
        ).toBeVisible();
      });

      if (project.autoSelectsDeployment) {
        test("hides the deployment field and still allows submit", async ({
          page,
        }) => {
          skipWhenUnconfigured(project);
          const form = new CaseCreatePage(page);
          await form.openViaGetHelp(project.id);

          // Cloud Support locks the deployment to primary production, so the
          // field is absent and the submit button is not gated on choosing one.
          await expect(form.deploymentSelect()).toBeHidden();
          await expect(form.submitButton()).toBeEnabled();
        });
      } else {
        test("gates product selection on choosing a deployment", async ({
          page,
        }) => {
          skipWhenUnconfigured(project);
          const form = new CaseCreatePage(page);
          await form.openViaGetHelp(project.id);

          // Staging's Managed Cloud form now opens with its deployment
          // ("Production") already chosen (live 2026-10-27), so there is no gate
          // to observe — the product field must then be open straight away.
          if (await deploymentPreselected(form)) {
            await expect(form.productVersionSelect()).toBeEnabled();
            await expect(form.productGateMessage()).toHaveCount(0);
            return;
          }

          // Product options are fetched per deployment, so the field reads
          // "Select deployment first" and stays disabled until one is picked.
          await expect(form.productVersionSelect()).toBeDisabled();
          await form.selectDeployment(project.deployment);

          // The gate having lifted, rather than the placeholder becoming
          // enabled: a deployment with a single product auto-selects it, so the
          // placeholder is replaced by the product's name and never becomes an
          // "enabled placeholder" at all.
          await expect(
            form.productGateMessage(),
            "choosing a deployment should unlock the product field",
          ).toHaveCount(0, { timeout: 60_000 });
        });

        test("keeps submit disabled until a deployment is chosen", async ({
          page,
        }) => {
          skipWhenUnconfigured(project);
          const form = new CaseCreatePage(page);
          await form.openViaGetHelp(project.id);

          // A pre-selected deployment (see the gate test above) leaves nothing
          // to wait for, so submit must already be available.
          if (await deploymentPreselected(form)) {
            await expect(form.submitButton()).toBeEnabled();
            return;
          }

          await expect(form.submitButton()).toBeDisabled();
          await form.selectDeployment(project.deployment);
          await expect(form.submitButton()).toBeEnabled();
        });
      }
    });
  }
});
