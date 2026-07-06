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
// Approvals tab (approver/admin). The queue only ever shows OTHER users'
// submitted cards (useApprovalQueue excludes the signed-in user's own), so a
// single captured session can never populate it — genuinely testing
// approve/reject needs two identities, via openContextAs(browser, "engineer").
// That test is skipped when tests/e2e/storageState/engineer.json hasn't been
// captured (see auth/README.md).
//
// PATCH /time-cards/{id} 403s unless the signed-in user is the card's
// assigned approver — confirmed live: approving your own just-created card
// succeeds, approving another engineer's real (pre-existing) card 403s
// ("Access to the requested resource is forbidden!"). So the card created
// below must have the *approver* session (not the engineer) as its assigned
// approver, or this test would 403 for the same reason.
//

import { test, expect, withRole, hasSession, openContextAs, approverSearchQuery } from "../../fixtures/test";
import { TimeCardsPage } from "../../pages/TimeCardsPage";
import { LogTimeDialog } from "../../pages/LogTimeDialog";
import { e2eWorkLogComment } from "../../utils/selectors";

withRole(test, "approver");

test.describe("time cards — approvals queue", () => {
  test("queue tab and filters render", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await tc.openApprovals();
    await expect(page.getByText("Could not load the approval queue.")).toHaveCount(
      0,
      { timeout: 15_000 },
    );
    await expect(page.getByLabel("Project")).toBeVisible();
    await expect(page.getByLabel("State")).toBeVisible();
    await expect(page.getByLabel("Engineer")).toBeVisible();
  });

  test("approving a card created by a different signed-in user clears it from the queue", async ({
    page,
    browser,
  }) => {
    test.skip(
      !hasSession("engineer"),
      "No captured session for 'engineer'. See tests/e2e/auth/README.md to create " +
        "tests/e2e/storageState/engineer.json — this test needs a second identity " +
        "distinct from the approver session to create a card the approver didn't author.",
    );

    // The card's assigned approver must be the *approver* session, since
    // only the assigned approver can decide it (see note above) — derive the
    // search query from the primary `page` (approver), not the engineer.
    const approverQuery = await approverSearchQuery(page);

    // Create the card as "engineer" in a second, independent context.
    const engineerContext = await openContextAs(browser, "engineer");
    const engineerPage = await engineerContext.newPage();
    let caseNumber: string | null = null;
    try {
      await engineerPage.goto("/cases");
      const firstCase = engineerPage
        .locator('a[href^="/cases/"]:not([href="/cases/new"])')
        .first();
      const hasCase = await firstCase
        .waitFor({ state: "visible", timeout: 15_000 })
        .then(() => true)
        .catch(() => false);
      test.skip(!hasCase, "No cases visible to the 'engineer' session.");

      await firstCase.click();
      await expect(engineerPage).toHaveURL(/\/cases\/[^/]+$/);
      await engineerPage.getByRole("tab", { name: "Time tracking" }).click();
      const logTime = engineerPage.getByRole("button", { name: "Log time" });
      test.skip(
        !(await logTime.isVisible().catch(() => false)),
        "This case is closed — Log time isn't available.",
      );

      await logTime.click();
      const dialog = new LogTimeDialog(engineerPage);
      await dialog.waitForOpen();
      caseNumber = await dialog.caseNumber();
      await dialog.fillAndSubmit({
        hours: 1,
        workLogComment: e2eWorkLogComment("approvals cross-user"),
        approverQuery,
      });
    } finally {
      await engineerContext.close();
    }
    test.skip(!caseNumber, "Could not determine the created card's case number.");

    // Approve it as the approver session (the primary `page`, already
    // authenticated via withRole above).
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await tc.openApprovals();
    await page.getByLabel("Work item").fill(caseNumber!);
    await expect(tc.cardRow(caseNumber!)).toBeVisible({ timeout: 15_000 });

    await tc.cardButton(caseNumber!, "Approve").click();
    // TimeCardReviewDialog's confirm button reads "Accept", not "Approve".
    await page.getByRole("dialog").getByRole("button", { name: "Accept" }).click();

    await expect(tc.cardText(caseNumber!)).toHaveCount(0, { timeout: 15_000 });
  });
});
