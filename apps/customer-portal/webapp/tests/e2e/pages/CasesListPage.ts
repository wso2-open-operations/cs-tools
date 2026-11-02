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

import {
  type Download,
  type Locator,
  type Page,
  expect,
} from "../fixtures/test";
import { CASES_LIST, CASE_DETAIL } from "../utils/selectors";
import { expectSuccess } from "../utils/caseFlows";
import { caseSearchResponse } from "../utils/listSearch";

/** How long to allow for the list and its search results to resolve. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for a project's cases list
 * (`/projects/:projectId/support/cases`).
 */
export class CasesListPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  searchInput(): Locator {
    return this.page.getByPlaceholder(CASES_LIST.searchPlaceholder);
  }

  /**
   * The search panel's text input, scoped to the main content area.
   *
   * AllCasesPage renders ListSearchPanel (search input + Filters toggle) only
   * when the list is unfiltered: with `?statusFilter=active|resolved` or a
   * dashboard `?severityId=` it renders a bare Divider in its place
   * (`hideSearchPanel`). The panel is therefore absent from the DOM, not merely
   * hidden, so callers assert absence with `toHaveCount(0)`.
   *
   * Scoped to `app-main` so the header's global search box can never satisfy
   * it, whatever its placeholder.
   *
   * @returns Locator for the input.
   */
  searchPanelInput(): Locator {
    return this.main().getByPlaceholder(CASES_LIST.searchPlaceholder);
  }

  /**
   * The search panel's Filters toggle, in either of its labels.
   *
   * @returns Locator for the button.
   */
  searchPanelFiltersButton(): Locator {
    return this.main().getByRole("button", {
      name: CASES_LIST.filtersButtonAnyState,
    });
  }

  /**
   * Opens the list and waits for the search box.
   *
   * @param projectId - Project whose cases to list.
   */
  async open(projectId: string): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/${CASES_LIST.pathSegment}`,
    );
    await expect(this.searchInput()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Searches the list and reports whether a case with this exact subject is
   * listed.
   *
   * The search matches id, title or description, so it can return near misses —
   * "subscription case S1" also matches "subscription case S10". The exact-text
   * check is what makes this a reliable existence test rather than a fuzzy one.
   *
   * @param subject - Exact case subject to look for.
   * @returns True when a row with that subject is present.
   */
  async hasCaseWithSubject(subject: string): Promise<boolean> {
    // Wait for the search request carrying THIS subject — see listSearch for why
    // `networkidle` is not a safe signal here.
    const searchResponse = caseSearchResponse(this.page, subject);

    await this.searchInput().fill(subject);
    const response = await searchResponse;

    // A failed search returns no rows, which is indistinguishable from "no such
    // case" — so assert it succeeded rather than silently treating it as absent.
    await expectSuccess(response, "case search");

    const match = this.main().getByText(subject, { exact: true });
    return (await match.count()) > 0;
  }

  /**
   * Searches the list and waits for the results of THIS query.
   *
   * The same term-matched wait `hasCaseWithSubject` uses, but returning the row
   * count rather than a subject match — the export spec searches by case
   * number, which the row renders, whereas the subject check compares exact
   * text that a number-only search will not produce.
   *
   * @param term - Case number, title or description fragment.
   * @returns Number of rows the search returned.
   */
  async search(term: string): Promise<number> {
    const searchResponse = caseSearchResponse(this.page, term);

    await this.searchInput().fill(term);
    const response = await searchResponse;

    // A failed search renders no rows, which is indistinguishable from "no
    // match" — assert it succeeded rather than reporting zero.
    await expectSuccess(response, "case search");

    // The response arriving is not the list having re-rendered. Counting rows
    // here reads whatever is still on screen — the PREVIOUS search's results,
    // or none at all — so the body's own total is used as the signal to wait
    // against rather than a bare count.
    const { totalRecords = 0 } = (await response.json()) as {
      totalRecords?: number;
    };

    if (totalRecords === 0) {
      await expect(
        this.rows(),
        "a search with no matches should render no rows",
      ).toHaveCount(0, { timeout: LOAD_TIMEOUT_MS });
      return 0;
    }

    // Rows are paginated, so the rendered count is capped — the assertion is
    // that the list has caught up with the response, not that every match is
    // on screen.
    await expect
      .poll(() => this.rows().count(), {
        timeout: LOAD_TIMEOUT_MS,
        message:
          `the list should render results for "${term}" ` +
          `(${totalRecords} matched)`,
      })
      .toBeGreaterThan(0);

    return this.rows().count();
  }

  /** The list's heading, which names which list is on screen. */
  heading(name: string): Locator {
    return this.main().getByText(name, { exact: true });
  }

  /**
   * A filter's label inside the panel.
   *
   * Returns every match rather than one: MUI renders a Select's label twice —
   * once as the floating InputLabel and once inside the outline's legend — so a
   * single-element locator would be a strict-mode violation. Callers assert on
   * the count.
   *
   * @param label - Filter label as rendered.
   * @returns Locator for its label nodes.
   */
  filterLabel(label: string): Locator {
    return this.main().getByText(label, { exact: true });
  }

  /** The Created By filter's label. Withheld on My Cases. */
  createdByFilter(): Locator {
    return this.filterLabel(CASES_LIST.createdByFilterLabel);
  }

  /**
   * Opens the filter panel.
   *
   * The panel is collapsed on load and its contents are unmounted, not hidden —
   * so before this runs, *no* filter is in the DOM and "filter X is absent"
   * holds trivially. Waits for a filter every list offers, which is what makes
   * an absence assertion afterwards meaningful.
   */
  async openFilters(): Promise<void> {
    await this.main()
      .getByRole("button", { name: CASES_LIST.filtersButton, exact: true })
      .click();
    await expect(
      this.filterLabel(CASES_LIST.severityFilterLabel).first(),
      "the filter panel should be open",
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * The case rows.
   *
   * ListCard sets `role="button"` on each row (it is clickable), so the rows are
   * buttons — but so are the page's controls, hence the narrowing below by the
   * per-row "Created by" text is done in `rowCreators()` rather than here.
   */
  rows(): Locator {
    return this.main()
      .getByRole("button")
      .filter({ hasText: CASES_LIST.createdByPrefix });
  }

  /**
   * Every case card on the current page, whether or not it names a creator.
   *
   * `rows()` narrows by "Created by", which ListCard omits for cases without a
   * creator — fine for comparing creators, but it under-counts. This narrows by
   * the "Updated …" footer every card renders, so it is the one to use when the
   * number of rendered rows is itself the assertion.
   *
   * @returns Locator for the cards.
   */
  caseCards(): Locator {
    return this.main()
      .getByRole("button")
      .filter({ hasText: CASES_LIST.rowUpdatedPattern });
  }

  /**
   * The card for one case, identified by its case number.
   *
   * ListCard renders the number as its own text node, so an exact match on it
   * picks out exactly one card even when titles repeat.
   *
   * @param caseNumber - The case's `number` from the search response.
   * @returns Locator for the card.
   */
  caseCard(caseNumber: string): Locator {
    return this.caseCards().filter({
      has: this.page.getByText(caseNumber, { exact: true }),
    });
  }

  /**
   * The list's empty-state message (either wording).
   *
   * @returns Locator for the message.
   */
  emptyState(): Locator {
    return this.main().getByText(CASES_LIST.emptyStatePattern);
  }

  /**
   * The creator named on each row, in list order.
   *
   * ListCard renders "Created by <x>" only when the case carries a creator, so
   * rows without one are simply absent from the result rather than empty
   * strings — callers compare the distinct values, which an empty entry would
   * distort.
   *
   * @returns One creator per row that names one.
   */
  async rowCreators(): Promise<string[]> {
    const texts = await this.main()
      .getByText(new RegExp(`^${CASES_LIST.createdByPrefix}\\S`))
      .allInnerTexts();
    return texts.map((text) =>
      text.replace(CASES_LIST.createdByPrefix, "").trim(),
    );
  }

  /**
   * Total number of cases the list reports, from the "Showing X of Y cases" bar.
   *
   * The total — not the shown count — is the filtered size of the whole result
   * set, so it is what a comparison between two lists has to use; the shown
   * count is capped by the page size.
   *
   * @returns The total, or null when the bar is not rendered.
   */
  async totalCount(): Promise<number | null> {
    const bar = this.main().getByText(CASES_LIST.resultsCountPattern).first();
    await expect(bar).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    const match = CASES_LIST.resultsCountPattern.exec(await bar.innerText());
    return match ? Number(match[2]) : null;
  }

  //
  // Export. The button opens a menu; the menu item performs the download.
  //

  /** The Export button on the list toolbar. */
  exportButton(): Locator {
    return this.main().getByRole("button", {
      name: CASES_LIST.export.button,
      exact: true,
    });
  }

  /** A format in the Export menu, e.g. "Export to CSV". */
  exportMenuItem(label: string): Locator {
    return this.page.getByRole("menuitem", { name: label, exact: true });
  }

  /**
   * Opens the Export menu.
   *
   * Waits for the button to be enabled first: it is disabled while a previous
   * export is still running and until the project id resolves, and a click in
   * that window is silently dropped.
   */
  async openExportMenu(): Promise<void> {
    await expect(this.exportButton()).toBeEnabled({ timeout: LOAD_TIMEOUT_MS });
    await this.exportButton().click();
    await expect(this.exportMenuItem(CASES_LIST.export.csvItem)).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * Picks an export format and returns the file the browser received.
   *
   * The download is awaited alongside the click rather than after it: both
   * formats build the file in-page and trigger an anchor click, so the event
   * can land before a sequential `waitForEvent` starts listening.
   *
   * @param label - Menu item label, e.g. "Export to PDF".
   * @returns The download, for asserting on its filename and contents.
   */
  async exportAs(label: string): Promise<Download> {
    const [download] = await Promise.all([
      this.page.waitForEvent("download", { timeout: LOAD_TIMEOUT_MS }),
      this.exportMenuItem(label).click(),
    ]);
    return download;
  }
}
