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
// Role gating on /time-cards. The Approvals tab is driven by useTimecardRole,
// which resolves isApprover from GET /users/me roles
// (sn_customerservice_timecard_approver, or the "admin" role). Approvers see
// it; plain engineers do not, even when the tab is forced via the URL.
//

import { test, expect, withRole } from "../../fixtures/test";
import { TimeCardsPage } from "../../pages/TimeCardsPage";

test.describe("time cards — approver role", () => {
  withRole(test, "approver");

  test("approver sees the Approvals tab", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await expect(tc.myTab()).toBeVisible();
    await expect(tc.approvalsTab()).toBeVisible();
  });

  test("approver can open the Approvals queue via URL", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto("?tab=approvals");
    await expect(tc.approvalsTab()).toBeVisible();
  });
});

test.describe("time cards — engineer role", () => {
  withRole(test, "engineer");

  test("engineer does NOT see the Approvals tab", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await expect(tc.myTab()).toBeVisible();
    await expect(tc.approvalsTab()).toHaveCount(0);
  });

  test("forcing ?tab=approvals falls back to My time sheets", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto("?tab=approvals");
    // No Approvals tab, and the My time sheets tab remains selected.
    await expect(tc.approvalsTab()).toHaveCount(0);
    await expect(tc.myTab()).toHaveAttribute("aria-selected", "true");
  });
});
