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
//
// Security Center — the Outstanding and Resolved (Last 30d) stat cards.
//
// ✅ READ-ONLY. Cards are clicked and reports opened; nothing is submitted.
//
// Runs on the projects with Security Report Analysis (the cards render only
// when hasSecurityReportAnalysis is on), i.e. those in SRA_VIEWS.
//
import { test, expect, withSession } from "../../fixtures/test";
import { CasesListPage } from "../../pages/CasesListPage";
import { SecurityCenterPage } from "../../pages/SecurityCenterPage";
import { PROJECTS, SRA_VIEWS } from "../../config/testData";
import { SECURITY_CENTER } from "../../utils/selectors";
import { RECORD_ID_PATTERN, idPattern } from "../../utils/ids";

withSession(test);

const LOAD_TIMEOUT_MS = 60_000;

test.describe("Security Center stat cards", () => {
  test.describe.configure({ timeout: 180_000 });

  for (const { projectType } of SRA_VIEWS) {
    const project = PROJECTS[projectType];

    test.describe(projectType, () => {
      test("shows the Outstanding and Resolved security report cards", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        await new SecurityCenterPage(page).openViaSideNav(project.id);
        const main = page.getByTestId("app-main");
        for (const card of SECURITY_CENTER.statCards) {
          const button = main.getByRole("button", { name: card.label });
          await expect(button).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
          // The count replaces the skeleton once the stats query answers.
          await expect(button).toHaveText(/\d/, { timeout: LOAD_TIMEOUT_MS });
        }
      });

      for (const card of SECURITY_CENTER.statCards) {
        test(`the ${card.label} card drills down to its reports`, async ({
          page,
        }) => {
          test.skip(!project.id, `${projectType} needs a project id.`);

          const security = new SecurityCenterPage(page);
          await security.openViaSideNav(project.id);
          const main = page.getByTestId("app-main");
          const cardButton = main.getByRole("button", { name: card.label });
          await expect(cardButton).toHaveText(/\d/, {
            timeout: LOAD_TIMEOUT_MS,
          });
          const path = new URL(page.url()).pathname;

          await cardButton.click();

          // The filter is applied in place: same path (the page adds its own
          // ?tab= after load, so the query is not compared), cards and tabs
          // swapped for a Back button and the card's heading.
          expect(new URL(page.url()).pathname).toBe(path);
          await expect(
            main.getByText(card.title, { exact: true }),
          ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
          await expect(main.getByText(card.subtitle, { exact: true })).toBeVisible();
          await expect(
            main.getByRole("button", { name: SECURITY_CENTER.statBackButton }),
          ).toBeVisible();
          await expect(
            security.tab(SECURITY_CENTER.tabs.securityReportAnalysis),
          ).toHaveCount(0);

          const rows = new CasesListPage(page).caseCards();
          const empty = main.getByText(SECURITY_CENTER.emptyMessage, {
            exact: true,
          });
          await expect(rows.first().or(empty)).toBeVisible({
            timeout: LOAD_TIMEOUT_MS,
          });

          const isEmpty = await empty.isVisible();
          const rowCount = isEmpty ? 0 : await rows.count();
          if (isEmpty) {
            test.info().annotations.push({
              type: "empty-drilldown",
              description: `${projectType}: ${card.label} lists no reports.`,
            });
          }
          // Outstanding lists no Closed report; Resolved lists only Closed.
          for (const row of await rows.all()) {
            const text = await row.innerText();
            expect(
              /\bClosed\b/.test(text),
              `${card.label} row should ${card.closed ? "" : "not "}be Closed: ${text}`,
            ).toBe(card.closed);
          }

          // Back restores the cards and tabs.
          await main
            .getByRole("button", { name: SECURITY_CENTER.statBackButton })
            .click();
          await expect(cardButton).toBeVisible();
          await expect(
            security.tab(SECURITY_CENTER.tabs.securityReportAnalysis),
          ).toBeVisible();
          if (isEmpty) return;

          // View one: drill in again and open the first report analysis.
          await cardButton.click();
          await expect(rows.first()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
          const caseNumber = /CS\d+/.exec(await rows.first().innerText())?.[0];
          expect(caseNumber, "the first row should show a CS number").toBeTruthy();
          await rows.first().click();
          await expect(page).toHaveURL(
            new RegExp(
              `/projects/${idPattern(project.id)}/${SECURITY_CENTER.pathSegment}/` +
                `${SECURITY_CENTER.reportAnalysisSegment}/${RECORD_ID_PATTERN}$`,
            ),
            { timeout: LOAD_TIMEOUT_MS },
          );
          // The detail page is about the report that was clicked. Matched by
          // number rather than CaseDetailPage.caseNumber(): a report analysis
          // can list other CS numbers too, which makes that locator ambiguous.
          await expect(
            main.getByText(caseNumber!, { exact: true }).first(),
          ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
          console.log(
            `Security Center (${projectType}): ${card.label} → ${rowCount} ` +
              `row(s) checked, opened ${caseNumber}`,
          );
        });
      }
    });
  }
});
