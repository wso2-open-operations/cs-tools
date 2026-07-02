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
// Approvals tab (approver/admin). The queue seeds two other engineers'
// submitted sheets: Sajith (CS0352584) and Nimal (CS0349881). Approving/
// rejecting a card goes through the review dialog; once a sheet has no more
// submitted cards it leaves the queue.
//

import { test, expect, withRole } from "../../fixtures/test";
import { TimeCardsPage } from "../../pages/TimeCardsPage";
import { QUEUE_SEED } from "../../utils/selectors";

withRole(test, "approver");

test.describe("time cards — approvals queue", () => {
  test("queue lists other engineers' submitted sheets", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await tc.openApprovals();

    await expect(page.getByText(QUEUE_SEED.sajith.name)).toBeVisible();
    await expect(tc.cardText(QUEUE_SEED.sajith.submitted)).toBeVisible();
    await expect(page.getByText(QUEUE_SEED.nimal.name)).toBeVisible();
    await expect(tc.cardText(QUEUE_SEED.nimal.submitted)).toBeVisible();
  });

  test("accepting a submitted card clears it from the queue", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await tc.openApprovals();

    // Open the review dialog from the card's Approve button.
    await tc.cardButton(QUEUE_SEED.sajith.submitted, "Approve").click();
    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByRole("heading", {
        name: new RegExp(`Review time card.*${QUEUE_SEED.sajith.submitted}`),
      }),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "Accept" }).click();

    // Dialog closes and Sajith's card is no longer awaiting approval.
    await expect(dialog).toBeHidden();
    await expect(
      tc.cardButton(QUEUE_SEED.sajith.submitted, "Approve"),
    ).toHaveCount(0);
  });

  test("rejecting a submitted card records a lead comment", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await tc.openApprovals();

    await tc.cardButton(QUEUE_SEED.nimal.submitted, "Reject").click();
    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByRole("heading", {
        name: new RegExp(`Review time card.*${QUEUE_SEED.nimal.submitted}`),
      }),
    ).toBeVisible();
    await dialog
      .getByLabel("Lead's comment (optional)")
      .fill("Please split the hours by activity.");
    await dialog.getByRole("button", { name: "Reject" }).click();

    await expect(dialog).toBeHidden();
    await expect(
      tc.cardButton(QUEUE_SEED.nimal.submitted, "Reject"),
    ).toHaveCount(0);
  });

  test("opens the delegate-approvals dialog", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await tc.openApprovals();

    await page
      .getByRole("button", { name: /Delegate approvals|Manage delegation/ })
      .click();
    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByRole("heading", { name: "Delegate approvals" }),
    ).toBeVisible();
  });
});
