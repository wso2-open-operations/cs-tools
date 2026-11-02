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
// Opens existing cases — one per severity, across all three project types — and
// checks each detail page shows what it should.
//
// ✅ READ-ONLY. Cases are opened by URL and asserted; nothing is created,
// commented on or closed, so this is safe to run repeatedly.
//
// What is and is not pinned:
//
// - **Severity** is pinned per case, since that is the axis these fixtures exist
//   to cover.
// - **WSO2 case id** and **case number** are asserted by format, not by value —
//   the values differ per case and pinning them would make this a test of
//   staging's contents. The id prefix is per project and is not the project key:
//   Subscription cases read `AUTOMATIONTESTCUSSUB-<n>`, Cloud Support's
//   `AUTOMATIONTESTCUSCLSUB-<n>` and MCS's `AUTOMATIONTESTCUSMSSUB-<n>`.
// - **State** is asserted to be one of the real case states, so a placeholder or
//   a blank would fail.
// - **Subject** and **comment** are pinned per case, every value read off the
//   live pages. Per-case rather than shared because the naming is not uniform:
//   the Subscription and Cloud Support S4 cases carry no severity suffix and no
//   "…with severity" in their comment, while MCS's S4 does.
//
// The header carries the WSO2 case id as well as the Details tab, so those
// assertions run against the header. Comments live on the Activity tab, which is
// the default.
//
// The test then switches to the **Details** tab and checks its five sections and
// their field labels are displayed. Labels are asserted as "present at least
// once" rather than exactly once: several repeat legitimately — the header
// already shows a status and a severity — so requiring uniqueness would fail for
// the wrong reason. The checks are soft, so one run reports every missing field
// rather than stopping at the first.
//

import { test, expect, withSession } from "../../fixtures/test";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import {
  CASE_VIEWS,
  CASE_VIEW_EXPECTATIONS,
  PROJECTS,
} from "../../config/testData";
import { CASE_DETAIL, CASE_DETAILS_PANEL } from "../../utils/selectors";
import { idPattern } from "../../utils/ids";

withSession(test);

test.describe("View Case", () => {
  // Each case is a cold page load with several queries behind the header; the
  // 30s default is not enough.
  test.describe.configure({ timeout: 120_000 });

  for (const { projectType, wso2CaseIdPattern, cases } of CASE_VIEWS) {
    const project = PROJECTS[projectType];

    test.describe(projectType, () => {
      for (const { severity, caseId, subject, comment } of cases) {
        test(`shows details for a ${severity} case`, async ({ page }) => {
          test.skip(
            !project.id || !caseId,
            `${projectType} fixture needs a project id and a case id for ${severity}. ` +
              `Fill them in tests/e2e/config/testData.ts.`,
          );

          const caseDetail = new CaseDetailPage(page);

          // Capture the case's own data: whether the "Production Version" field
          // is rendered depends on it, and asserting from the fixture instead
          // means the test drifts whenever the case data changes.
          const caseResponse = page.waitForResponse(
            (r) =>
              // The page's OWN url also ends with /cases/<id>, so the document
              // navigation matches a naive path check and hands back HTML.
              // Excluding it is what makes this the API response.
              r.request().resourceType() !== "document" &&
              r.request().method() === "GET" &&
              new RegExp(`/cases/${idPattern(caseId)}$`).test(
                new URL(r.url()).pathname,
              ),
            { timeout: 60_000 },
          );

          await caseDetail.open(project.id, caseId);

          const caseData = (await (await caseResponse).json()) as {
            deployedProduct?: { version?: string | null };
          };
          const hasProductVersion = Boolean(
            caseData.deployedProduct?.version,
          );

          // Case number — format only; the value differs per case.
          await expect(caseDetail.caseNumber()).toBeVisible();
          await expect(caseDetail.caseNumber()).toHaveText(
            CASE_VIEW_EXPECTATIONS.caseNumberPattern,
          );

          // WSO2 case id — project-specific prefix plus an integer.
          await expect(
            caseDetail.wso2CaseId(wso2CaseIdPattern),
          ).toBeVisible();

          // Severity — the reason these four fixtures exist, so pinned exactly.
          await expect(caseDetail.severityChip(severity)).toBeVisible();

          // State — must be one of the real states rather than blank or a dash.
          const states = CASE_DETAIL.header.states;
          const visibleStates = await Promise.all(
            states.map((state) => caseDetail.stateLabel(state).count()),
          );
          expect(
            visibleStates.reduce((total, n) => total + n, 0),
            `expected one of ${states.join(", ")} to be shown as the case state`,
          ).toBeGreaterThan(0);

          // Subject — pinned per case.
          await expect(caseDetail.subject()).toHaveText(subject);

          // The comment, on the Activity tab (the default). Pinned per case: the S4
          // case's wording differs from the other three (see the fixture note).
          await expect(await caseDetail.scrollToComment(comment)).toBeVisible();

          // ── Details tab ──────────────────────────────────────────────────
          await caseDetail.openDetailsTab();

          const { sections, fields } = CASE_DETAILS_PANEL;

          // Sections are asserted present; Escalation Levels and Watch List
          // carry no fields of their own here, so their headings are the check.
          for (const section of Object.values(sections)) {
            await expect
              .soft(
                caseDetail.detailsText(section),
                `"${section}" section should be shown`,
              )
              .not.toHaveCount(0);
          }

          // Field labels, asserted as "displayed at least once" rather than
          // exactly once: several repeat legitimately — the header already shows
          // a status and a severity — so requiring uniqueness would fail for the
          // wrong reason.
          // "Production Version" is rendered only when the case carries a
          // deployed product version (CaseDetailsDetailsPanel guards it on
          // `deployedProduct.version`). Decided from the CASE's data rather than
          // a per-project fixture flag: which cases have a version is
          // environment data that changes, and a stale flag fails as a missing
          // field — which reads like a UI regression and is not.
          const productFields = hasProductVersion
            ? fields.productEnvironment
            : fields.productEnvironment.filter(
                (label) => label !== "Production Version",
              );

          // The inverse also has to hold, or "omit it when absent" would excuse
          // the field never rendering at all.
          if (!hasProductVersion) {
            await expect
              .soft(
                caseDetail.detailsText("Production Version"),
                "Production Version should be hidden when the case has no " +
                  "deployed product version",
              )
              .toHaveCount(0);
          }

          for (const label of [
            ...fields.caseOverview,
            ...productFields,
            ...fields.customerInformation,
          ]) {
            await expect
              .soft(
                caseDetail.detailsText(label),
                `"${label}" field should be shown`,
              )
              .not.toHaveCount(0);
          }
        });
      }
    });
  }
});
