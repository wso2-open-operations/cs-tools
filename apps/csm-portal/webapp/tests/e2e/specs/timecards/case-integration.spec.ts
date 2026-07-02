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
// Case-detail integration: the "Time tracking" tab renders CaseTimeCardsPanel
// and (for open cases) exposes the "Log time" action which opens the log
// dialog. Cases come from the real backend, so this spec opens whichever case
// is first in the list and skips cleanly if there are none.
//

import { test, expect, withRole } from "../../fixtures/test";

withRole(test, "approver");

test.describe("time cards — case integration", () => {
  test("Time tracking tab shows the panel and can open Log time", async ({
    page,
  }) => {
    await page.goto("/cases");

    const firstCase = page
      .locator('a[href^="/cases/"]:not([href="/cases/new"])')
      .first();
    // The search round-trips a few backend calls before the list renders;
    // wait for a row rather than counting immediately (which raced the fetch).
    const hasCase = await firstCase
      .waitFor({ state: "visible", timeout: 15_000 })
      .then(() => true)
      .catch(() => false);
    if (!hasCase) {
      test.skip(true, "No cases available in this environment to open.");
    }
    await firstCase.click();
    await expect(page).toHaveURL(/\/cases\/[^/]+$/);

    // Open the Time tracking tab and assert the panel rendered.
    await page.getByRole("tab", { name: "Time tracking" }).click();
    await expect(page.getByText("Time tracked")).toBeVisible();

    // Open cases expose "Log time"; closed cases render the panel read-only.
    const logTime = page.getByRole("button", { name: "Log time" });
    if (await logTime.isVisible().catch(() => false)) {
      await logTime.click();
      const dialog = page.getByRole("dialog");
      await expect(
        dialog.getByRole("heading", { name: /Log time/ }),
      ).toBeVisible();
      await dialog.getByRole("button", { name: "Cancel" }).click();
      await expect(dialog).toBeHidden();
    }
  });
});
