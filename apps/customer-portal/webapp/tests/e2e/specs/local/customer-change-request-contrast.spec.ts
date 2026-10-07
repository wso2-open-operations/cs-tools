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
// The answer buttons are legible in BOTH themes: the text of Approve, Reject and Propose
// New Time (Customer Approval), of Successful and Unsuccessful (Customer Review) and of the
// reject confirmation's button reaches WCAG AA (4.5 : 1 for text this small) against what is
// really painted behind it. The page's background is a gradient, so the colour behind the text
// is sampled from a screenshot rather than read from the CSS. A single fixed shade for both
// themes read 3.6 to 4.3 : 1 on the dark page.
//
// Read-only apart from the fixtures' reset (E2E_POSTGRES_CONTAINER, like the other
// state-changing specs, so it SKIPS without it): nothing is answered.
//

import { test, expect } from "../../fixtures/test";
import { withLocalSession } from "../../auth/localSessions";
import { ChangeRequestDetailsPage } from "../../pages/ChangeRequestDetailsPage";
import { worstTextContrast } from "../../utils/contrast";
import { FIXTURES, resetFixtures, withFixtureStack } from "../../utils/localStack";
import { CHANGE_REQUEST_DETAILS as UI } from "../../utils/selectors";

withLocalSession(test, "dave");
withFixtureStack(test);

const { approval, review, projectId } = FIXTURES;
const AA_TEXT = 4.5;

for (const scheme of ["light", "dark"] as const) {
  test.describe(`Local stack — answer buttons in ${scheme} mode`, () => {
    test.use({ colorScheme: scheme });
    test.describe.configure({ timeout: 120_000 });

    test.beforeEach(async () => {
      await resetFixtures();
    });

    test(`Propose New Time, Approve, Reject and the reject confirmation read at AA`, async ({ page }) => {
      const dave = new ChangeRequestDetailsPage(page);
      await dave.open(projectId, approval.id, approval.number);
      for (const name of [UI.buttons.proposeNewTime, UI.buttons.approve, UI.buttons.reject]) {
        const ratio = await worstTextContrast(page, dave.button(name));
        expect(ratio, `${name} in ${scheme} mode: ${ratio.toFixed(2)} : 1`).toBeGreaterThanOrEqual(AA_TEXT);
      }

      await dave.button(UI.buttons.reject).click();
      const confirm = dave.rejectConfirmButton(UI.rejectConfirm.approvalConfirm);
      await expect(confirm).toBeVisible();
      // The dialog fades in: measure once it has settled.
      await page.waitForTimeout(500);
      const ratio = await worstTextContrast(page, confirm);
      expect(ratio, `the reject confirmation in ${scheme} mode: ${ratio.toFixed(2)} : 1`).toBeGreaterThanOrEqual(AA_TEXT);
    });

    test(`Successful and Unsuccessful read at AA`, async ({ page }) => {
      const dave = new ChangeRequestDetailsPage(page);
      await dave.open(projectId, review.id, review.number);
      for (const name of [UI.buttons.successful, UI.buttons.unsuccessful]) {
        const ratio = await worstTextContrast(page, dave.button(name));
        expect(ratio, `${name} in ${scheme} mode: ${ratio.toFixed(2)} : 1`).toBeGreaterThanOrEqual(AA_TEXT);
      }
    });
  });
}
