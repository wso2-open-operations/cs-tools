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
// Opens existing security report analyses and checks the header shows what it
// should.
//
// ✅ READ-ONLY. SRAs are opened by URL and asserted; nothing is created or
// modified, so this is safe to run repeatedly.
//
// A security report analysis reuses the case detail header, so the same locators
// apply — but it carries no severity chip, since security reports hide severity
// on the way in.
//
// What is and is not pinned:
//
// - **Subject** is pinned literally. Security report subjects are generated at
//   creation as `<deployment> - <product name> - YYYY-MM-DD`, so for an existing
//   record the string is fixed. The date in it is the SRA's *creation* date, not
//   today's — computing it would break the test the next day.
// - **WSO2 case id** and **case number** are asserted by format, since the values
//   differ per record. The id prefix is per project.
// - **State** must be one of the real case states, so a blank or placeholder
//   fails.
//
// Cloud Support has no entry: that project does not offer security reports at all
// (see the create spec), so it has none to view.
//

import { test, expect, withSession } from "../../fixtures/test";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import {
  CASE_VIEW_EXPECTATIONS,
  PROJECTS,
  SRA_VIEWS,
} from "../../config/testData";
import {
  CASE_DETAIL,
  CASE_ESCALATION,
  SECURITY_CENTER,
} from "../../utils/selectors";

withSession(test);

test.describe("View Security Report", () => {
  // Each SRA is a cold page load with several queries behind the header; the 30s
  // default is not enough.
  test.describe.configure({ timeout: 120_000 });

  for (const { projectType, sraId, subject, wso2CaseIdPattern } of SRA_VIEWS) {
    const project = PROJECTS[projectType];

    test.describe(projectType, () => {
      test("shows details for a security report analysis", async ({ page }) => {
        test.skip(
          !project.id || !sraId,
          `${projectType} needs a project id and an SRA id. ` +
            `Fill them in tests/e2e/config/testData.ts.`,
        );

        const sra = new CaseDetailPage(page);
        await sra.openSecurityReportAnalysis(project.id, sraId);

        // Case number — format only; the value differs per record.
        await expect(sra.caseNumber()).toBeVisible();
        await expect(sra.caseNumber()).toHaveText(
          CASE_VIEW_EXPECTATIONS.caseNumberPattern,
        );

        // WSO2 case id — project-specific prefix plus an integer.
        await expect(sra.wso2CaseId(wso2CaseIdPattern)).toBeVisible();

        // State — must be one of the real states rather than blank or a dash.
        const states = CASE_DETAIL.header.states;
        const visibleStates = await Promise.all(
          states.map((state) => sra.stateLabel(state).count()),
        );
        expect(
          visibleStates.reduce((total, n) => total + n, 0),
          `expected one of ${states.join(", ")} to be shown as the state`,
        ).toBeGreaterThan(0);

        // Subject — the generated title, pinned as-is.
        await expect(sra.subject()).toHaveText(subject);
      });

      test("offers only Activity, Details and Attachments, with no escalation", async ({
        page,
      }) => {
        test.skip(
          !project.id || !sraId,
          `${projectType} needs a project id and an SRA id. ` +
            `Fill them in tests/e2e/config/testData.ts.`,
        );

        const sra = new CaseDetailPage(page);
        await sra.openSecurityReportAnalysis(project.id, sraId);
        const main = page.getByTestId("app-main");
        const { shown, hidden } = SECURITY_CENTER.reportDetailTabs;

        for (const name of shown) {
          await expect(main.getByRole("tab", { name })).toBeVisible();
        }
        // Absent rather than hidden: CaseDetailsContent drops them from the
        // tab list for a security report analysis.
        for (const name of hidden) {
          await expect(main.getByRole("tab", { name })).toHaveCount(0);
        }
        await expect(main.getByRole("tab")).toHaveCount(shown.length);
        await expect(
          page.getByRole("button", { name: CASE_ESCALATION.button }),
        ).toHaveCount(0);
        await expect(
          page.getByRole("button", { name: CASE_ESCALATION.deescalateButton }),
        ).toHaveCount(0);
      });
    });
  }
});
