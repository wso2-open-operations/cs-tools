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

import { type Locator, type Page, expect } from "@playwright/test";
import { TIMECARDS } from "../utils/selectors";

/**
 * Page object for `/time-cards`. Selectors key off accessible roles and the
 * seeded cards' stable case numbers (never ids/dates). A "card row" is the
 * smallest element that contains both a card's case number and its action
 * buttons, which lets us scope actions to a specific card.
 */
export class TimeCardsPage {
  constructor(private readonly page: Page) {}

  async goto(query = ""): Promise<void> {
    await this.page.goto(`${TIMECARDS.path}${query}`);
    await expect(
      this.page.getByRole("heading", { name: TIMECARDS.heading }),
    ).toBeVisible();
  }

  approvalsTab(): Locator {
    return this.page.getByRole("tab", { name: TIMECARDS.tabs.approvals });
  }

  myTab(): Locator {
    return this.page.getByRole("tab", { name: TIMECARDS.tabs.mine });
  }

  async openApprovals(): Promise<void> {
    await this.approvalsTab().click();
  }

  /**
   * The row for one card, via the stable `data-testid` TimeSheetCard sets on
   * each row (`timecard-row-<caseNumber>`). DOM-heuristic scoping (hasText +
   * last(), or "nearest ancestor with a button") is fragile here: once a card
   * has no remaining owner actions (e.g. right after submit, when the sheet's
   * own rolled-up status chip also reads the same state as the card), those
   * heuristics over-match well past the intended row.
   */
  cardRow(caseNumber: string): Locator {
    return this.page.getByTestId(`timecard-row-${caseNumber}`);
  }

  /** Any element showing this case number (e.g. to assert presence/absence). */
  cardText(caseNumber: string): Locator {
    return this.page.getByText(caseNumber, { exact: false }).first();
  }

  cardButton(caseNumber: string, name: string): Locator {
    return this.cardRow(caseNumber).getByRole("button", { name, exact: true });
  }

  /** Sheet-level action button (e.g. "Submit week", "Approve remaining"). */
  sheetButton(name: string): Locator {
    return this.page.getByRole("button", { name, exact: true });
  }

  /** Set the State filter select (label "State") to an option label. */
  async filterState(optionLabel: string): Promise<void> {
    await this.page.getByLabel("State").click();
    await this.page.getByRole("option", { name: optionLabel, exact: true }).click();
  }

  reviewDialog(): Locator {
    return this.page.getByRole("dialog");
  }
}
