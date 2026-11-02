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
// The change requests list: its unfiltered view, the List/Calendar switch, and
// opening a request from a row.
//
// ✅ READ-ONLY. Nothing here approves, rejects or otherwise changes a request —
// those are irreversible from the portal and would consume a real record.
//
// Scoped to Managed Cloud Subscription: change requests need CR access, which is
// the same flag that puts Operations in the side nav, and only that project has
// it.
//
// The filtered variants of this page — Outstanding, Action Required, Upcoming —
// are selected by router state rather than by the URL, so they cannot be reached
// by navigating directly and are covered where they are actually opened from:
// the dashboard's operations donut, and (once written) the Operations hub cards.
//

import { promises as fs } from "node:fs";
import { test, expect, withSession } from "../../fixtures/test";
import { ChangeRequestsPage } from "../../pages/ChangeRequestsPage";
import { SideNavPage } from "../../pages/SideNavPage";
import { PROJECTS, ProjectType } from "../../config/testData";
import {
  CHANGE_REQUESTS_LIST,
  CHANGE_REQUEST_PDF,
} from "../../utils/selectors";
import {
  RECORD_ID_PATTERN,
  projectPathPattern,
} from "../../utils/ids";

withSession(test);

test.describe("Change Requests", () => {
  // A cold list load behind a project-features query; the 30s default is not
  // enough.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[ProjectType.MANAGED_CLOUD_SUBSCRIPTION];

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — lists all change requests`, async ({
    page,
  }) => {
    test.skip(
      !project.id,
      `${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} needs a project id. ` +
        `Fill it in tests/e2e/config/testData.ts.`,
    );

    const changeRequests = new ChangeRequestsPage(page);
    await changeRequests.open(project.id);

    // Navigating directly gets the unfiltered view — the other titles come from
    // state the hub passes, which a URL cannot carry.
    await expect(
      changeRequests.heading(CHANGE_REQUESTS_LIST.titles.all),
    ).toBeVisible();

    // Both view modes are on offer, with the list selected to begin with.
    await expect(
      changeRequests.viewTab(CHANGE_REQUESTS_LIST.viewTabs.list),
    ).toHaveAttribute("aria-selected", "true");
    await expect(
      changeRequests.viewTab(CHANGE_REQUESTS_LIST.viewTabs.calendar),
    ).toBeVisible();

    await changeRequests.waitForList();

    // How many requests exist is environment data, so an empty list is a
    // legitimate result — what matters is that the list resolved into one of its
    // real states rather than a skeleton.
    const rows = await changeRequests.rows().count();
    console.log(
      `Change requests (${ProjectType.MANAGED_CLOUD_SUBSCRIPTION}): ${rows} listed`,
    );
  });

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — switches between list and calendar views`, async ({
    page,
  }) => {
    test.skip(!project.id, `${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} needs a project id.`);

    const changeRequests = new ChangeRequestsPage(page);
    await changeRequests.open(project.id);
    await changeRequests.waitForList();

    // Into the calendar. The rows are a list-view rendering, so their absence is
    // what shows the switch actually changed the view rather than only the tab.
    await changeRequests.openView(CHANGE_REQUESTS_LIST.viewTabs.calendar);
    await expect(
      changeRequests.viewTab(CHANGE_REQUESTS_LIST.viewTabs.list),
    ).toHaveAttribute("aria-selected", "false");
    await expect(changeRequests.rows()).toHaveCount(0);

    // And back, where the list returns to whichever state it was in.
    await changeRequests.openView(CHANGE_REQUESTS_LIST.viewTabs.list);
    await expect(
      changeRequests.viewTab(CHANGE_REQUESTS_LIST.viewTabs.calendar),
    ).toHaveAttribute("aria-selected", "false");
    await changeRequests.waitForList();

    console.log(
      `Change requests (${ProjectType.MANAGED_CLOUD_SUBSCRIPTION}): switched to ` +
        `calendar and back`,
    );
  });

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — opens a change request from the list`, async ({
    page,
  }) => {
    test.skip(!project.id, `${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} needs a project id.`);

    const changeRequests = new ChangeRequestsPage(page);
    await changeRequests.open(project.id);
    await changeRequests.waitForList();

    const rowCount = await changeRequests.rows().count();
    test.skip(
      rowCount === 0,
      `${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} has no change requests to open. ` +
        `This test needs at least one on the project.`,
    );

    // Read the number off the row before clicking, so the detail page can be
    // checked against the row that was actually opened rather than against an id
    // from config.
    const number = await changeRequests.rowNumber(0);
    expect(number, "the row should carry a change request number").toBeTruthy();

    await changeRequests.rows().first().click();

    // A change request sysid is a 32-character hex string; anchoring rules out
    // landing back on the list.
    await expect(page).toHaveURL(
      projectPathPattern(
        project.id,
        `${CHANGE_REQUESTS_LIST.pathSegment}/${RECORD_ID_PATTERN}$`,
      ),
    );

    // The request that was clicked, not merely some request.
    await expect(
      page.getByTestId("app-main").getByText(number as string, { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });

    console.log(
      `Change requests (${ProjectType.MANAGED_CLOUD_SUBSCRIPTION}): opened ${number}`,
    );
  });

  // ✅ READ-ONLY. The PDF is generated in the browser from the loaded record.
  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — downloads a change request as PDF`, async ({
    page,
  }) => {
    test.skip(!project.id, `${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} needs a project id.`);

    // Operations in the side menu, then the hub's "View all change requests".
    await new SideNavPage(page).open(project.id);
    await new SideNavPage(page).clickItem(
      CHANGE_REQUEST_PDF.operationsNavItem,
      projectPathPattern(project.id, "operations"),
    );
    await page
      .getByTestId("app-main")
      .getByRole("button", { name: CHANGE_REQUEST_PDF.viewAllButton })
      .click();
    await expect(page).toHaveURL(
      projectPathPattern(project.id, `${CHANGE_REQUESTS_LIST.pathSegment}(\\?.*)?$`),
      { timeout: 60_000 },
    );

    const changeRequests = new ChangeRequestsPage(page);
    await changeRequests.waitForList();
    test.skip(
      (await changeRequests.rows().count()) === 0,
      `${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} has no change requests to open.`,
    );
    const number = await changeRequests.rowNumber(0);
    await changeRequests.rows().first().click();
    await expect(page).toHaveURL(
      projectPathPattern(project.id, `${CHANGE_REQUESTS_LIST.pathSegment}/${RECORD_ID_PATTERN}$`),
    );
    await expect(
      page.getByTestId("app-main").getByText(number as string, { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });

    // Download: a real, non-empty PDF.
    const button = page.getByRole("button", { name: CHANGE_REQUEST_PDF.downloadButton });
    await expect(button).toBeEnabled({ timeout: 30_000 });
    const [download] = await Promise.all([
      page.waitForEvent("download", { timeout: 60_000 }),
      button.click(),
    ]);
    expect(download.suggestedFilename()).toMatch(/\.pdf$/i);
    const bytes = await fs.readFile((await download.path())!);
    expect(bytes.length, "the PDF should not be empty").toBeGreaterThan(1000);
    expect(bytes.subarray(0, 5).toString(), "a PDF file").toBe("%PDF-");

    console.log(
      `Change requests (${ProjectType.MANAGED_CLOUD_SUBSCRIPTION}): downloaded ` +
        `${download.suggestedFilename()} for ${number} (${bytes.length} bytes)`,
    );
  });
});
