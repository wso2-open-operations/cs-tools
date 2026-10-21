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
// Opens existing announcements and checks the header shows what it should.
//
// ✅ READ-ONLY. Announcements are opened by URL and asserted; nothing is created
// or modified, so this is safe to run repeatedly.
//
// An announcement reuses the case detail header — verified live: the header row
// reads `<wso2 case id> | <case number> <state>`, followed by the subject — so
// the same locators apply. It carries no severity chip.
//
// What is and is not pinned:
//
// - **Subject** is pinned. The same advisory is published to all three projects,
//   so they share it while each keeps its own number and id.
// - **WSO2 case id** and **case number** are asserted by format, since the values
//   differ per project. The id prefix is per project.
// - **State** must be one of the real case states, so a blank or placeholder
//   fails.
//

import { test, expect, withSession } from "../../fixtures/test";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import {
  ANNOUNCEMENT_VIEWS,
  CASE_VIEW_EXPECTATIONS,
  PROJECTS,
} from "../../config/testData";
import { ANNOUNCEMENT_DETAIL, CASE_DETAIL } from "../../utils/selectors";

withSession(test);

test.describe("View Announcement", () => {
  // Each announcement is a cold page load with several queries behind the
  // header, and this page resolves more slowly than the case pages do — the
  // 30s default is not enough.
  test.describe.configure({ timeout: 120_000 });

  for (const {
    projectType,
    announcementId,
    subject,
    wso2CaseIdPattern,
  } of ANNOUNCEMENT_VIEWS) {
    const project = PROJECTS[projectType];

    test.describe(projectType, () => {
      test("shows details for an announcement", async ({ page }) => {
        test.skip(
          !project.id || !announcementId,
          `${projectType} needs a project id and an announcement id. ` +
            `Fill them in tests/e2e/config/testData.ts.`,
        );

        const announcement = new CaseDetailPage(page);
        await announcement.openAnnouncement(project.id, announcementId);

        // `openAnnouncement` has already waited for the page to settle on one of
        // its two outcomes, so this reads a finished state rather than racing a
        // still-loading one — the distinction matters, since "not rendered yet"
        // and "not rendered" are the same to a count.
        //
        // A FAILURE, not a skip: a fixture naming a record that no longer
        // exists is exactly the kind of rot that should be fixed rather than
        // tolerated, and a skip would report green while this page went
        // unchecked.
        await expect(
          page.getByText(ANNOUNCEMENT_DETAIL.notFoundMessage),
          `Announcement ${announcementId} no longer exists on ${projectType} ` +
            `— the page renders "${ANNOUNCEMENT_DETAIL.notFoundMessage}". ` +
            `Repoint ANNOUNCEMENT_VIEWS in tests/e2e/config/testData.ts at a ` +
            `live announcement from that project's list.`,
        ).toHaveCount(0);

        // Case number — format only; the value differs per project.
        await expect(announcement.caseNumber()).toBeVisible();
        await expect(announcement.caseNumber()).toHaveText(
          CASE_VIEW_EXPECTATIONS.caseNumberPattern,
        );

        // WSO2 case id — project-specific prefix plus an integer.
        await expect(
          announcement.wso2CaseId(wso2CaseIdPattern),
        ).toBeVisible();

        // State — must be one of the real states rather than blank or a dash.
        const states = CASE_DETAIL.header.states;
        const visibleStates = await Promise.all(
          states.map((state) => announcement.stateLabel(state).count()),
        );
        expect(
          visibleStates.reduce((total, n) => total + n, 0),
          `expected one of ${states.join(", ")} to be shown as the state`,
        ).toBeGreaterThan(0);

        // Subject — the advisory title, shared across the three projects.
        await expect(announcement.subject()).toHaveText(subject);
      });
    });
  }
});
