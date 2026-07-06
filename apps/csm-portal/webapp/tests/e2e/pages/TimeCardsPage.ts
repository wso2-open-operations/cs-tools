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
   * The most recent row for a card. TimeSheetCard sets `data-testid`
   * per-row keyed by the card's own id (`timecard-row-<id>`) — unique per
   * row, since an engineer can log multiple cards against the same case in
   * one week. Tests only know the case *number* though (ids are
   * server-generated), so this filters the testid-tagged rows by their
   * visible case-number text instead of matching the testid directly.
   *
   * There's no delete endpoint, so repeated runs against the same case
   * accumulate multiple cards (and thus multiple matching rows) under the
   * same case number. `.first()` reliably lands on the newest one: sheets
   * render newest-week-first and cards within a sheet render
   * newest-createdOn-first (see `groupIntoSheets` in `useTimeSheets.ts`), so
   * the first DOM match for a case number is always its most recently
   * created card.
   */
  cardRow(caseNumber: string): Locator {
    return this.page
      .locator('[data-testid^="timecard-row-"]')
      .filter({ hasText: caseNumber })
      .first();
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
