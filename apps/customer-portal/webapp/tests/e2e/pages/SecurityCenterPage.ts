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
import { CASE_DETAIL, SECURITY_CENTER } from "../utils/selectors";
import { expectSuccess } from "../utils/caseFlows";
import { idPattern } from "../utils/ids";
import { SideNavPage } from "./SideNavPage";
import { caseSearchResponse } from "../utils/listSearch";

/** How long to allow for the list and its search results to resolve. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the Security Center's Security Report Analysis list
 * (`/projects/:projectId/security-center`).
 *
 * Exists so the create-report spec can check whether its report already exists
 * before raising another: reports are cases, and cases have no delete endpoint,
 * so an unguarded create accumulates permanent records on every run.
 */
export class SecurityCenterPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  searchInput(): Locator {
    return this.page.getByPlaceholder(SECURITY_CENTER.searchPlaceholder);
  }

  /**
   * Opens the Security Center and waits for the report list's search box.
   *
   * @param projectId - Project whose reports to list.
   */
  async open(projectId: string): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/${SECURITY_CENTER.pathSegment}`,
    );
    await expect(this.searchInput()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Searches the report list and reports whether anything matched.
   *
   * The list's search covers case number, title and description, so passing a
   * report's **description** finds it regardless of its generated title — which
   * carries the creation date and so differs day to day.
   *
   * Waits for the search response produced by this very term, so a result is
   * never read mid-flight.
   *
   * @param searchText - Text to search for, typically the report's description.
   * @returns True when the search returns at least one report.
   */
  async hasReportMatching(searchText: string): Promise<boolean> {
    // The report list is backed by the same /cases/search endpoint as the cases
    // list, so it uses the same term-matched wait. `networkidle` would be unsafe
    // here for the same reason: a premature read reports "no report" and the
    // caller raises a duplicate that cannot be deleted.
    const searchResponse = caseSearchResponse(this.page, searchText);
    await this.searchInput().fill(searchText);
    const response = await searchResponse;

    await expectSuccess(response, "report search");

    // A result row always carries a case number. Checking for one is a positive
    // signal, rather than inferring a hit from the empty message being absent —
    // which would also be true mid-load.
    const rows = this.main().getByText(/^CS\d+$/);
    return (await rows.count()) > 0;
  }

  //
  // Component Analysis tab (ProductVulnerabilitiesTable).
  //
  // Everything below narrows a dataset that was fetched once, in the browser.
  // No method here waits on a network response, and none should be given one:
  // typing in the search box or changing a filter issues no request at all.
  //

  /**
   * Opens Security Center through the side nav, as a user would.
   *
   * Returns only once the page's own content has rendered, not merely once the
   * URL says Security Center. The URL changes before the route swaps, so a
   * caller that immediately reads something non-retrying — `tab(...).count()`,
   * say — samples the previous page and gets zero. That does not fail loudly:
   * it looks like the tab is absent, and the spec skips itself reporting the
   * feature is not available when it is.
   *
   * The marker is "either tab's search box, whichever appears". Both are unique
   * to this page, which matters: a page-wide `getByRole("tab")` would be
   * satisfied by the tabs of whatever page is still mounted — a case detail has
   * several — so the wait would pass before Security Center rendered at all.
   *
   * Both are accepted because which tab lands first depends on the project's
   * permissions (SECURITY_PAGE_TABS is filtered), so waiting on the reports
   * search alone would hang for a project whose first tab is Component Analysis.
   */
  async openViaSideNav(projectId: string): Promise<void> {
    const sideNav = new SideNavPage(this.page);
    await sideNav.open(projectId);
    await sideNav.clickItem(
      SECURITY_CENTER.navItem,
      new RegExp(`/projects/${idPattern(projectId)}/${SECURITY_CENTER.pathSegment}`),
    );

    await expect(async () => {
      const reports = await this.searchInput().count();
      const components = await this.componentSearchInput().count();
      expect(
        reports + components,
        "Security Center should show either tab's search box",
      ).toBeGreaterThan(0);
    }).toPass({ timeout: LOAD_TIMEOUT_MS });
  }

  /** A tab in the Security Center's tab bar. */
  tab(label: string): Locator {
    return this.page.getByRole("tab", { name: label, exact: true });
  }

  /** The Component Analysis search box. */
  componentSearchInput(): Locator {
    return this.page.getByPlaceholder(
      SECURITY_CENTER.componentAnalysis.searchPlaceholder,
    );
  }

  /**
   * Switches to Component Analysis and waits for its data to land.
   *
   * Waits for a row rather than the search box: the box renders while the
   * fetch is still in flight, so a caller that searched on seeing it would be
   * filtering an empty table. Returns the row count so a caller can assert the
   * narrowing actually narrowed.
   *
   * @returns Total matching records, from the pagination footer.
   */
  async openComponentAnalysis(): Promise<number> {
    await this.tab(SECURITY_CENTER.tabs.componentAnalysis).click();

    // The tab is reflected in the URL, and the page reads its active tab back
    // out of that param — so this is the tab state, not decoration.
    await expect(this.page).toHaveURL(
      new RegExp(`[?&]tab=${SECURITY_CENTER.componentAnalysis.tabId}`),
    );

    await expect(this.componentSearchInput()).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });

    // Either rows arrive or the table legitimately has none; both are settled
    // states, and waiting for only the first would hang on an empty project.
    await expect(async () => {
      const rows = await this.componentRows().count();
      const empty = await this.page
        .getByText(SECURITY_CENTER.componentAnalysis.emptyMessage)
        .count();
      expect(rows + empty).toBeGreaterThan(0);
    }).toPass({ timeout: LOAD_TIMEOUT_MS });

    return this.componentTotal();
  }

  /** Rows of the Component Analysis table, identified by the CVE each carries. */
  componentRows(): Locator {
    return this.main()
      .getByRole("row")
      .filter({ hasText: SECURITY_CENTER.componentAnalysis.cvePattern });
  }

  /**
   * The size of the current result set, read from the pagination footer.
   *
   * NOT the number of rendered rows. The table paginates client-side at ten a
   * page, so on this tenant `componentRows()` returns 10 whether the filter
   * matched 10 records or 1265 — which makes any narrowing assertion built on
   * rendered rows quietly vacuous. The footer's "1–10 of 1265" is the only
   * place the real total appears.
   *
   * @returns Total matching records, or 0 when the empty state is showing.
   */
  async componentTotal(): Promise<number> {
    if (
      (await this.main()
        .getByText(SECURITY_CENTER.componentAnalysis.emptyMessage)
        .count()) > 0
    ) {
      return 0;
    }

    const label = await this.main()
      .getByText(/of\s+[\d,]+/)
      .first()
      .textContent();
    const match = label?.match(/of\s+([\d,]+)/);
    return match ? Number(match[1].replace(/,/g, "")) : 0;
  }

  /**
   * Types into the Component Analysis search and waits for the table to settle.
   *
   * The input is debounced and filters in-process, so there is no response to
   * await.
   *
   * @param query - CVE, component name or vulnerability id fragment.
   * @returns Number of matching records — the result-set total, not the page.
   */
  async searchComponents(query: string): Promise<number> {
    await this.componentSearchInput().fill(query);

    // The debounce must elapse before ANY count is meaningful. Polling for a
    // "stable" count without this waits zero time: the first two reads agree
    // because neither has seen the filter apply yet, which presents as a search
    // that matched every row.
    await this.page.waitForTimeout(
      SECURITY_CENTER.componentAnalysis.searchDebounceMs * 2,
    );

    // Only now is stability meaningful: require two consecutive identical reads
    // so a count is never taken mid-render.
    let previous = -1;
    await expect(async () => {
      const count = await this.componentTotal();
      const settled = count === previous;
      previous = count;
      expect(settled).toBe(true);
    }).toPass({ timeout: LOAD_TIMEOUT_MS });

    return this.componentTotal();
  }

  /** Clears the search box. */
  async clearComponentSearch(): Promise<void> {
    await this.componentSearchInput().fill("");
  }

  /** The Filters toggle, which reveals the severity/product/version selects. */
  filtersButton(): Locator {
    return this.main().getByRole("button", {
      name: SECURITY_CENTER.componentAnalysis.filtersButton,
    });
  }

  /** Opens the filter panel if it is not already showing. */
  async openFilters(): Promise<void> {
    await this.filtersButton().click();
    await expect(this.filterSelect(
      SECURITY_CENTER.componentAnalysis.filters.severityLabel,
    )).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /** One of the filter selects, addressed by its visible label.
   *
   * Matched exactly: "Product" is a prefix of "Product Version", so a substring
   * match resolves to both selects and fails on strict mode. */
  filterSelect(label: string): Locator {
    return this.main().getByLabel(label, { exact: true });
  }

  /**
   * Picks a real value in a filter select, skipping its no-filter option.
   *
   * The first option in each of these selects is an "All …" entry that clears
   * the filter rather than applying one — choosing it and then asserting the
   * list narrowed is a test that can only fail.
   *
   * @param label - The select's label.
   * @param allOptionLabel - That select's no-filter option, to skip over.
   * @returns The chosen value and how many real values were on offer, or null
   *   when the select offers nothing to filter by. The count matters to the
   *   caller: a filter can only be expected to NARROW a list when more than one
   *   value exists — with a single value, matching everything is correct.
   */
  async selectFilterValue(
    label: string,
    allOptionLabel: string,
  ): Promise<{ value: string; optionCount: number } | null> {
    await this.filterSelect(label).click();

    const options = this.page.getByRole("option");
    await expect(options.first()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });

    const labels = (await options.allTextContents())
      .map((text) => text.trim())
      .filter((text) => text && text !== allOptionLabel);

    if (labels.length === 0) {
      await this.page.keyboard.press("Escape");
      return null;
    }

    const chosen = labels[0];
    await options.filter({ hasText: chosen }).first().click();
    return { value: chosen, optionCount: labels.length };
  }
}
