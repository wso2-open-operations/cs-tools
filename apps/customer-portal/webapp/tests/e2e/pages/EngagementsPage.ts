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

import { type Locator, type Page, expect } from "../fixtures/test";
import { CASE_DETAIL, ENGAGEMENTS } from "../utils/selectors";
import { SideNavPage } from "./SideNavPage";

/** How long to allow for the page and its list query to resolve. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the engagements page: its stat cards, list and controls.
 */
export class EngagementsPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /**
   * Opens the page through the side nav, as a user would.
   *
   * @param projectId - Project whose engagements to open.
   */
  async openViaSideNav(projectId: string): Promise<void> {
    const sideNav = new SideNavPage(this.page);
    await sideNav.open(projectId);
    await sideNav.clickItem(
      ENGAGEMENTS.navItem,
      new RegExp(`/projects/${projectId}/${ENGAGEMENTS.pathSegment}`),
    );
    await expect(this.statCard(ENGAGEMENTS.statCards[0].label)).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * A stat card, located by its label.
   *
   * @param label - The card's label.
   */
  statCard(label: string): Locator {
    return this.main().getByText(label, { exact: true });
  }

  /**
   * The clickable form of a stat card.
   *
   * @param label - The card's label.
   */
  statCardButton(label: string): Locator {
    return this.main().getByRole("button", { name: new RegExp(label) });
  }

  /**
   * The list's heading, which names the filter in effect.
   *
   * @param title - Expected title.
   */
  heading(title: string): Locator {
    return this.main().getByRole("heading", { name: title, exact: true });
  }

  searchInput(): Locator {
    return this.main().getByPlaceholder(ENGAGEMENTS.searchPlaceholder);
  }

  /**
   * The engagement rows.
   *
   * Each row is a clickable card carrying the engagement's case number, which is
   * what separates a row from the page's other controls.
   */
  rows(): Locator {
    return this.main()
      .getByRole("button")
      .filter({ hasText: ENGAGEMENTS.numberPattern });
  }

  /**
   * The case number a row carries.
   *
   * @param index - Zero-based row.
   * @returns The number, or null when the row has none.
   */
  async rowNumber(index: number): Promise<string | null> {
    const match = ENGAGEMENTS.numberPattern.exec(
      await this.rows().nth(index).innerText(),
    );
    return match ? match[0] : null;
  }

  /**
   * Waits for the list to settle: a row, or the copy shown when a filter matches
   * nothing. Without this a caller can assert against a still-loading list.
   */
  async waitForList(): Promise<void> {
    await expect(
      this.rows()
        .first()
        .or(this.main().getByText(/No engagements/)),
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * The Back control.
   *
   * Named identically on the listing page and on an engagement's detail page, so
   * the caller's context decides which one this is.
   */
  backButton(): Locator {
    return this.main().getByRole("button", {
      name: ENGAGEMENTS.backButton,
      exact: true,
    });
  }

  /** Opens the filter panel. Reads "Clear Filters (n)" once one is applied. */
  filtersButton(): Locator {
    return this.main().getByRole("button", {
      name: ENGAGEMENTS.filtersButton,
      exact: true,
    });
  }

  /**
   * The same control once a filter is applied, where it clears rather than
   * toggles the panel.
   *
   * @param activeCount - How many filters are active, which the label carries.
   */
  clearFiltersButton(activeCount: number): Locator {
    return this.main().getByRole("button", {
      name: ENGAGEMENTS.clearFiltersButton(activeCount),
      exact: true,
    });
  }

  /** The Sort by select. */
  sortFieldSelect(): Locator {
    return this.main().locator(`#${ENGAGEMENTS.sort.fieldSelectId}`);
  }

  /** The Order by select. */
  sortOrderSelect(): Locator {
    return this.main().locator(`#${ENGAGEMENTS.sort.orderSelectId}`);
  }

  /**
   * Chooses a sort field or order.
   *
   * The option list renders in a portal at the document root, so it is looked up
   * page-wide.
   *
   * @param select - The select to operate.
   * @param label - Exact option label.
   */
  async chooseSortOption(select: Locator, label: string): Promise<void> {
    await select.click();
    await this.page.getByRole("option", { name: label, exact: true }).click();
  }
}
