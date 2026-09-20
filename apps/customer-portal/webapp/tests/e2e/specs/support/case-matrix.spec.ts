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
// Creates one case per project type per severity — S1 to S4 across
// Subscription, Managed Cloud Subscription and Cloud Support.
//
// ⚠️ NOT idempotent, and this is the most expensive suite in the repo to run.
// Every execution raises a case for each offered combination — up to twelve —
// and cases have no delete endpoint, so nothing here or elsewhere can remove
// them. Retries add more. Treat a full run as a permanent write to the target
// environment.
//
// It previously searched each project's case list for the deterministic subject
// (`<prefix> <severity code>`, from CASE_MATRIX) and created only when nothing
// matched. That made repeat runs free, but it also meant the create path stopped
// being exercised as soon as the row existed: on a populated environment every
// test passed without submitting anything. Creating unconditionally is the
// deliberate trade — coverage of the write path in exchange for records per run.
//
// The subjects are still deterministic, so the cases this leaves behind remain
// identifiable and grouped in the target environment. They are no longer a key,
// though: repeated runs now produce many cases sharing one subject.
//
// Severity availability is per project: it comes from `acceptedSeverityValues`,
// so a project that does not offer a severity skips that combination rather than
// failing.
//

import { test, expect, withSession } from "../../fixtures/test";
import { CaseCreatePage } from "../../pages/CaseCreatePage";
import {
  CASE_MATRIX,
  CASE_MATRIX_SEVERITIES,
  IssueType,
  PROJECTS,
  ProjectType,
  SEVERITY_CODES,
} from "../../config/testData";
import { expectSuccess, skipWhenUnconfigured } from "../../utils/caseFlows";
import { CREATE_CASE } from "../../utils/selectors";
import {
  permanentWriteSkipReason,
  permanentWritesAllowed,
} from "../../utils/permanentWrites";

withSession(test);

test.describe("Case Matrix", () => {
  // Each test may load a list, search it, then run the whole create-case flow;
  // the 30s default is nowhere near enough.
  test.describe.configure({ timeout: 180_000 });

  for (const projectType of Object.values(ProjectType)) {
    const project = PROJECTS[projectType];
    const naming = CASE_MATRIX[projectType];

    test.describe(projectType, () => {
      for (const severity of CASE_MATRIX_SEVERITIES) {
        const code = SEVERITY_CODES[severity];
        const subject = `${naming.titlePrefix} ${code}`;

        test(`create ${code} case`, async ({ page }) => {
          skipWhenUnconfigured(project);
          test.skip(
            !permanentWritesAllowed(),
            permanentWriteSkipReason(
              "a support case (up to 12 across the full matrix)",
            ),
          );

          console.log(`${projectType} ${code}: creating "${subject}"`);

          const form = new CaseCreatePage(page);
          await form.openViaGetHelp(project.id);

          if (project.autoSelectsDeployment) {
            await expect(form.deploymentSelect()).toBeHidden();
          } else {
            await form.selectDeployment(project.deployment);
          }
          await form.selectProductVersion(project.productVersion);

          // Skip rather than fail when the project does not offer this severity:
          // the available set is project data, not a defect.
          await form.severitySelect().click();
          const option = page.getByRole("option", {
            name: severity,
            exact: true,
          });
          const offered = (await option.count()) > 0;
          if (!offered) {
            await page.keyboard.press("Escape");
          }
          test.skip(
            !offered,
            `${projectType} does not offer ${severity} — its acceptedSeverityValues exclude it.`,
          );
          await option.click();

          await form.fillTitle(subject);
          await form.fillDescription(`${naming.descriptionPrefix} ${code}`);
          await form.selectIssueType(IssueType.QUESTION);

          await expect(form.submitButton()).toBeEnabled();

          const [response] = await Promise.all([
            page.waitForResponse(
              (r) =>
                new URL(r.url()).pathname.endsWith("/cases") &&
                r.request().method() === "POST",
            ),
            form.submit(),
          ]);

          // Status asserted here rather than in the predicate, so a rejected
          // create reports the server's message instead of timing out.
          await expectSuccess(response, "create case");

          const created = (await response.json()) as {
            id?: string;
            number?: string;
          };
          expect(created.id, "backend returned no case id").toBeTruthy();

          await expect(
            page.getByText(CREATE_CASE.successMessage),
          ).toBeVisible();

          console.log(
            `${projectType} ${code}: created ${created.number ?? created.id}`,
          );
        });
      }
    });
  }
});
