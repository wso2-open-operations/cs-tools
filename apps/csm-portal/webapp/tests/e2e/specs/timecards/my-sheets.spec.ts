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
// "My time sheets" tab — the signed-in user's own weekly cards. Backed by the
// FE-first store, which seeds one pending (CS0353001) and one approved
// (CS0352900) card per user. The store resets on full page load, so each test
// starts from the same seeded state.
//

import { test, expect, withRole } from "../../fixtures/test";
import { TimeCardsPage } from "../../pages/TimeCardsPage";
import { MY_SEED } from "../../utils/selectors";

withRole(test, "approver");

test.describe("time cards — my time sheets", () => {
  test("shows the seeded pending and approved cards", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await expect(tc.cardText(MY_SEED.pending.caseNumber)).toBeVisible();
    await expect(tc.cardText(MY_SEED.approved.caseNumber)).toBeVisible();
  });

  test("submitting the pending card moves it to Submitted", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await tc.cardButton(MY_SEED.pending.caseNumber, "Submit").click();
    // The card stays in the sheet; its status chip flips to Submitted and the
    // owner can no longer Submit it.
    await expect(
      tc.cardRow(MY_SEED.pending.caseNumber).getByText("Submitted", { exact: true }),
    ).toBeVisible();
    await expect(
      tc.cardButton(MY_SEED.pending.caseNumber, "Submit"),
    ).toHaveCount(0);
  });

  test("editing the pending card opens and saves the dialog", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await tc.cardButton(MY_SEED.pending.caseNumber, "Edit").click();

    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByRole("heading", { name: new RegExp(`Edit time card`) }),
    ).toBeVisible();
    await dialog
      .getByLabel("Work log comment")
      .fill("Updated by e2e: refined the breakdown.");
    await dialog.getByRole("button", { name: "Save changes" }).click();
    await expect(dialog).toBeHidden();
  });

  test("deleting the pending card removes it", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await tc.cardButton(MY_SEED.pending.caseNumber, "Delete").click();

    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByRole("heading", { name: "Delete time card?" }),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "Delete", exact: true }).click();

    await expect(dialog).toBeHidden();
    await expect(tc.cardText(MY_SEED.pending.caseNumber)).toHaveCount(0);
  });

  test("state filter narrows to the selected state", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await tc.filterState(MY_SEED.approved.state); // "Approved"
    await expect(tc.cardText(MY_SEED.approved.caseNumber)).toBeVisible();
    await expect(tc.cardText(MY_SEED.pending.caseNumber)).toHaveCount(0);
  });
});
