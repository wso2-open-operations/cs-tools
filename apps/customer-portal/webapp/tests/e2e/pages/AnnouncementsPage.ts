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
import {
  ANNOUNCEMENTS_LIST,
  CASE_DETAIL,
  MUI_PAGINATION,
} from "../utils/selectors";
import { SideNavPage } from "./SideNavPage";

/** How long to allow for the list and its search to resolve. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the announcements list.
 */
export class AnnouncementsPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /**
   * Opens the list through the side nav, as a user would.
   *
   * @param projectId - Project whose announcements to list.
   */
  async openViaSideNav(projectId: string): Promise<void> {
    const sideNav = new SideNavPage(this.page);
    await sideNav.open(projectId);
    await sideNav.clickItem(
      ANNOUNCEMENTS_LIST.navItem,
      new RegExp(`/projects/${projectId}/${ANNOUNCEMENTS_LIST.pathSegment}`),
    );
    await expect(this.heading()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /** The list's heading. */
  heading(): Locator {
    return this.main().getByRole("heading", {
      name: ANNOUNCEMENTS_LIST.title,
      exact: true,
    });
  }

  /** The list's description, beneath the heading. */
  description(): Locator {
    return this.main().getByText(ANNOUNCEMENTS_LIST.description, {
      exact: true,
    });
  }

  /**
   * The announcement rows.
   *
   * Each row is a clickable card carrying the announcement's case number, which
   * is what separates a row from the page's other controls.
   */
  rows(): Locator {
    return this.main()
      .getByRole("button")
      .filter({ hasText: ANNOUNCEMENTS_LIST.numberPattern });
  }

  /** The copy shown when a project has no announcements. */
  emptyMessage(): Locator {
    return this.main().getByText(ANNOUNCEMENTS_LIST.emptyMessage);
  }

  /**
   * Waits for the list to settle into one of its two real states — rows, or the
   * empty copy — so a caller cannot assert against a still-loading page.
   */
  async waitForList(): Promise<void> {
    await expect(this.rows().first().or(this.emptyMessage())).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /** The "Showing X of Y announcements" bar. */
  resultsBar(): Locator {
    return this.main().getByText(ANNOUNCEMENTS_LIST.resultsCountPattern);
  }

  /**
   * The totals the results bar reports.
   *
   * @returns The shown and total counts, or null when the bar is absent.
   */
  async resultsCounts(): Promise<{ shown: number; total: number } | null> {
    const match = ANNOUNCEMENTS_LIST.resultsCountPattern.exec(
      await this.resultsBar().innerText(),
    );
    return match
      ? { shown: Number(match[1]), total: Number(match[2]) }
      : null;
  }

  //
  // Filters.
  //

  /** Opens the filter panel. Reads "Clear Filters (n)" once one is applied. */
  filtersButton(): Locator {
    return this.main().getByRole("button", {
      name: ANNOUNCEMENTS_LIST.filtersButton,
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
      name: ANNOUNCEMENTS_LIST.clearFiltersButton(activeCount),
      exact: true,
    });
  }

  /** The Status filter select, addressed by the id its definition sets. */
  statusFilterSelect(): Locator {
    return this.main().locator(`#${ANNOUNCEMENTS_LIST.statusFilter.selectId}`);
  }

  /**
   * Reads the options the Status filter offers.
   *
   * Waits for the first option before reading: `allInnerTexts` does not retry, so
   * reading straight after the click can return an empty list while the portal
   * mounts.
   *
   * @returns The option labels, in order.
   */
  async statusFilterOptions(): Promise<string[]> {
    await this.statusFilterSelect().click();

    const options = this.page.getByRole("option");
    await expect(options.first()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    const labels = await options.allInnerTexts();

    await this.page.keyboard.press("Escape");
    return labels.map((label) => label.trim());
  }

  /**
   * Chooses a status in the filter panel.
   *
   * The select is multi-select, so its menu stays open after a click — Escape
   * closes it, which also clears the overlay that would intercept later clicks.
   *
   * @param label - Option label to choose.
   */
  async selectStatusFilter(label: string): Promise<void> {
    await this.statusFilterSelect().click();
    await this.page.getByRole("option", { name: label, exact: true }).click();
    await this.page.keyboard.press("Escape");
  }

  //
  // Sorting.
  //

  /** The Sort by select. */
  sortFieldSelect(): Locator {
    return this.main().locator(`#${ANNOUNCEMENTS_LIST.sort.fieldSelectId}`);
  }

  /** The Order by select. */
  sortOrderSelect(): Locator {
    return this.main().locator(`#${ANNOUNCEMENTS_LIST.sort.orderSelectId}`);
  }

  /**
   * Chooses a sort field or order.
   *
   * @param select - The select to operate.
   * @param label - Exact option label.
   */
  async chooseSortOption(select: Locator, label: string): Promise<void> {
    await select.click();
    await this.page.getByRole("option", { name: label, exact: true }).click();
  }

  //
  // Pagination.
  //

  /** The Rows per page control. Accepts either role, as MUI changed the
   * trigger's role across versions. */
  rowsPerPageSelect(): Locator {
    const name = new RegExp(MUI_PAGINATION.rowsPerPageLabel.replace(":", ":?"));
    return this.main()
      .getByRole("combobox", { name })
      .or(this.main().getByRole("button", { name }))
      .first();
  }

  /**
   * Changes the list's page size.
   *
   * @param rows - Page size to choose.
   */
  async selectRowsPerPage(rows: number): Promise<void> {
    await this.rowsPerPageSelect().click();
    await this.page
      .getByRole("option", { name: String(rows), exact: true })
      .click();
  }

  /** Next-page control. Disabled on the last page. */
  nextPageButton(): Locator {
    return this.main().getByRole("button", {
      name: MUI_PAGINATION.nextPageButton,
      exact: true,
    });
  }

  /** Previous-page control. Disabled on the first page. */
  previousPageButton(): Locator {
    return this.main().getByRole("button", {
      name: MUI_PAGINATION.previousPageButton,
      exact: true,
    });
  }

  /** The "1–10 of 24" range text beside the page controls. */
  displayedRows(): Locator {
    return this.main().getByText(MUI_PAGINATION.displayedRowsPattern);
  }

  /**
   * The range the pagination reports.
   *
   * Guarded on the control existing: `innerText` on a missing element waits for
   * it and then throws, so without this the "not rendered" case would surface as
   * a timeout rather than as the null a caller can act on.
   *
   * Absent is a normal state, not only an empty-list one: ListPagination returns
   * null whenever `totalRecords <= rowsPerPage`, so raising the page size can
   * remove the control that was there a moment ago. A caller reading this after a
   * resize has to treat null as "everything fits on one page" and count the rows
   * instead.
   *
   * @returns from/to/total, or null when the range is not rendered.
   */
  async displayedRange(): Promise<{
    from: number;
    to: number;
    total: number;
  } | null> {
    if ((await this.displayedRows().count()) === 0) return null;

    const match = MUI_PAGINATION.displayedRowsPattern.exec(
      await this.displayedRows().innerText(),
    );
    return match
      ? {
          from: Number(match[1]),
          to: Number(match[2]),
          total: Number(match[3]),
        }
      : null;
  }

  /**
   * The case number a row carries.
   *
   * @param index - Zero-based row.
   * @returns The number, or null when the row has none.
   */
  async rowNumber(index: number): Promise<string | null> {
    const match = ANNOUNCEMENTS_LIST.numberPattern.exec(
      await this.rows().nth(index).innerText(),
    );
    return match ? match[0] : null;
  }
}
