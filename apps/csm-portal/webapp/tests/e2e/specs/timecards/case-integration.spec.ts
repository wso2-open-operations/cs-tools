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
// and (for open cases) exposes "Log time", which creates a REAL time card via
// POST /time-cards — there is no mock data and no delete endpoint, so this
// leaves a permanent record in staging (see utils/selectors.ts E2E_TAG).
// Cases come from the real backend, so this spec opens whichever case is
// first in the list and skips cleanly if there are none.
//
// The panel is scoped by useCaseTimeCards(caseId, projectId) — the backend
// requires POST /time-cards/search's filters.projectIds to be non-empty to
// return anything (confirmed live), so it searches the case's own project
// and filters to this case client-side. The panel doesn't render a new
// card's workLogComment, so this asserts on the "Across N entries" count
// instead of a specific card's content — a stable, id-free signal that the
// just-created card round-tripped through search and rendered.
//

import { test, expect, withRole, approverSearchQuery } from "../../fixtures/test";
import { LogTimeDialog } from "../../pages/LogTimeDialog";
import { e2eWorkLogComment } from "../../utils/selectors";

withRole(test, "approver");

test.describe("time cards — case integration", () => {
  test("logging time from a case appears in the panel (real create + display)", async ({
    page,
  }) => {
    const approverQuery = await approverSearchQuery(page);
    await page.goto("/cases");

    const firstCase = page
      .locator('a[href^="/cases/"]:not([href="/cases/new"])')
      .first();
    const hasCase = await firstCase
      .waitFor({ state: "visible", timeout: 15_000 })
      .then(() => true)
      .catch(() => false);
    if (!hasCase) {
      test.skip(true, "No cases available in this environment to open.");
    }
    await firstCase.click();
    await expect(page).toHaveURL(/\/cases\/[^/]+$/);

    // The panel renders "Across 0 entries" immediately (before its data has
    // loaded — see CaseTimeCardsPanel, cards defaults to [] while isLoading)
    // and then updates once useCaseTimeCards resolves. Wait for that first
    // real response before reading the "before" count, or a slow initial
    // load can be misread as "0 entries" and produce a false mismatch below.
    const [searchResponse] = await Promise.all([
      page.waitForResponse((r) => r.url().includes("/time-cards/search")),
      page.getByRole("tab", { name: "Time tracking" }).click(),
    ]);
    await searchResponse.finished();
    await expect(page.getByText("Time tracked")).toBeVisible();
    // Give React a beat to commit the response into state after the network
    // call resolves, so the very next read isn't racing the render.
    await page.waitForTimeout(300);

    const logTime = page.getByRole("button", { name: "Log time" });
    if (!(await logTime.isVisible().catch(() => false))) {
      test.skip(true, "This case is closed — Log time isn't available.");
    }

    const entryCount = page.getByText(/^Across \d+ entr(y|ies)$/);
    const beforeText = (await entryCount.textContent()) ?? "";
    const beforeN = Number(beforeText.match(/\d+/)?.[0] ?? "0");

    await logTime.click();
    const dialog = new LogTimeDialog(page);
    await dialog.waitForOpen();
    await dialog.fillAndSubmit({
      hours: 1,
      workLogComment: e2eWorkLogComment("case-integration create"),
      approverQuery,
    });

    // The panel's search query is invalidated on create success and refetches
    // (staleTime 5s) — poll until the count reflects the new card rather than
    // asserting immediately.
    await expect(entryCount).toHaveText(
      new RegExp(`^Across ${beforeN + 1} entr(y|ies)$`),
      { timeout: 15_000 },
    );
  });
});
