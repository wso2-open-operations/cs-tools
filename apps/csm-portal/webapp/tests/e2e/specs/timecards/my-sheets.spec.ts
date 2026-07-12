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
// "My time sheets" tab, backed by the real POST /time-cards/search
// (client-grouped into weeks). The backend requires `filters.projectIds` to
// be non-empty to return anything (confirmed live) — useMyTimeSheets now
// defaults to every project the signed-in user can see when no explicit
// project filter is picked, so a just-created card is expected to appear
// here without the user touching the project filter.
//

import { test, expect, withRole, approverSearchQuery } from "../../fixtures/test";
import { TimeCardsPage } from "../../pages/TimeCardsPage";
import { LogTimeDialog } from "../../pages/LogTimeDialog";
import { e2eWorkLogComment } from "../../utils/selectors";

withRole(test, "approver");

/** Logs a real, uniquely-labelled time card from whichever case is first in
 * the list and returns its case number, or null if there's nothing to open. */
async function logTimeOnFirstCase(
  page: import("@playwright/test").Page,
  label: string,
): Promise<string | null> {
  const approverQuery = await approverSearchQuery(page);
  await page.goto("/cases");
  const firstCase = page
    .locator('a[href^="/cases/"]:not([href="/cases/new"])')
    .first();
  const hasCase = await firstCase
    .waitFor({ state: "visible", timeout: 15_000 })
    .then(() => true)
    .catch(() => false);
  if (!hasCase) return null;
  await firstCase.click();
  await expect(page).toHaveURL(/\/cases\/[^/]+$/);

  await page.getByRole("tab", { name: "Time tracking" }).click();
  const logTime = page.getByRole("button", { name: "Log time" });
  if (!(await logTime.isVisible().catch(() => false))) return null;

  await logTime.click();
  const dialog = new LogTimeDialog(page);
  await dialog.waitForOpen();
  const caseNumber = await dialog.caseNumber();
  await dialog.fillAndSubmit({
    hours: 1,
    workLogComment: e2eWorkLogComment(label),
    approverQuery,
  });
  return caseNumber;
}

test.describe("time cards — my time sheets", () => {
  test("page loads: tab, filters, and an empty/loaded state render", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await expect(tc.myTab()).toBeVisible();
    await tc.openFilters();
    await expect(page.getByLabel("Project")).toBeVisible();
    await expect(page.getByLabel("Work item")).toBeVisible();
    await expect(page.getByLabel("State")).toBeVisible();
    await expect(page.getByText("Could not load your time sheets.")).toHaveCount(0);
  });

  test("a newly logged card appears grouped in My time sheets", async ({ page }) => {
    const caseNumber = await logTimeOnFirstCase(page, "my-sheets display");
    test.skip(!caseNumber, "No open case available to log time against.");

    const tc = new TimeCardsPage(page);
    await tc.goto();
    await expect(tc.cardRow(caseNumber!)).toBeVisible({ timeout: 15_000 });
  });

  test("state and work-item filters narrow to the matching card", async ({ page }) => {
    // Creates a real card (network round trip against live staging) *and*
    // drives three sequential filter interactions afterward — comfortably
    // exceeds the config default of 30s.
    test.setTimeout(60_000);
    const caseNumber = await logTimeOnFirstCase(page, "my-sheets filters");
    test.skip(!caseNumber, "No open case available to log time against.");

    const tc = new TimeCardsPage(page);
    await tc.goto();
    await expect(tc.cardRow(caseNumber!)).toBeVisible({ timeout: 15_000 });

    // Work item filter narrows to the matching card. It's a multi-select
    // sourced from case numbers on the current page, not free text, so
    // there's no equivalent "type something that can't match" case anymore
    // (nothing to select if it doesn't exist) — clearing instead confirms
    // the filter was actually applied and removable.
    await tc.filterWorkItem(caseNumber!);
    await expect(tc.cardRow(caseNumber!)).toBeVisible();
    await tc.clearFilters();
    // "Clear filters" only shows up while a filter is active, so it going
    // away means the work item filter actually got removed.
    await expect(page.getByRole("button", { name: "Clear filters" })).toHaveCount(0);

    // State filter: the card was just created, so it's "submitted".
    await tc.filterState("Submitted");
    await expect(tc.cardRow(caseNumber!)).toBeVisible();
  });
});
