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
import { CASES_LIST, CASE_DETAIL, DASHBOARD } from "../utils/selectors";
import { SideNavPage } from "./SideNavPage";

/** How long to allow for the shell and the dashboard's own queries to resolve. */
const LOAD_TIMEOUT_MS = 60_000;

/** Escapes a label for use inside a RegExp. The card labels carry "." and
 * parentheses, which would otherwise be metacharacters. */
function escapeForRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * Page object for a project's dashboard.
 */
export class DashboardPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /**
   * Opens the dashboard through the side nav, as a user would.
   *
   * @param projectId - Project whose dashboard to open.
   */
  async openViaSideNav(projectId: string): Promise<void> {
    const sideNav = new SideNavPage(this.page);
    await sideNav.open(projectId);
    await sideNav.clickItem(
      DASHBOARD.navItem,
      new RegExp(`/projects/${projectId}/${DASHBOARD.pathSegment}$`),
    );

    // The cards are skeletons until the stats queries resolve, so the first
    // card's label marks the point where the grid is real.
    await expect(this.statCard(DASHBOARD.statCards[0].label)).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * A stat card, located by its label.
   *
   * Matched on the label text rather than by role: only three of the four cards
   * are buttons — Avg. Response Time is non-clickable — so a role-based locator
   * could not address them uniformly.
   *
   * @param label - The card's label.
   * @returns Locator for the label element.
   */
  statCard(label: string): Locator {
    return this.main().getByText(label, { exact: true });
  }

  /**
   * The clickable form of a stat card, for the three that open a list.
   *
   * @param label - The card's label.
   * @returns Locator for the card button.
   */
  statCardButton(label: string): Locator {
    return this.main().getByRole("button", {
      name: new RegExp(escapeForRegExp(label)),
    });
  }

  /**
   * The number a clickable stat card shows, read once the card has settled.
   *
   * Polled rather than read once, because a card passes through two
   * non-numeric states first: a skeleton while the stats query is loading, and
   * "--" for the moment the query is still disabled waiting on the project
   * context (react-query reports a disabled query as not loading, so the grid
   * falls through to its "no value" branch). Neither is the card's answer.
   *
   * The value is not in the button's accessible name, so it is parsed from the
   * card's rendered text: the one line that is a bare integer.
   *
   * @param label - The card's label. Must be one of the clickable cards.
   * @returns The card's value, or null when it settled on the error indicator.
   */
  async statCardValue(label: string): Promise<number | null> {
    const card = this.statCardButton(label);
    let value: number | null = null;
    await expect(async () => {
      if ((await card.getByTestId(DASHBOARD.errorIndicatorTestId).count()) > 0) {
        value = null;
        return;
      }
      const line = (await card.innerText())
        .split("\n")
        .map((part) => part.trim())
        .find((part) => /^\d+$/.test(part));
      expect(line, `the ${label} card should show a number`).toBeDefined();
      value = Number(line);
    }).toPass({ timeout: LOAD_TIMEOUT_MS });
    return value;
  }

  /**
   * The accordion section headers on a dashboard item page — "Cases",
   * "Service Requests" and the rest of {@link DASHBOARD.itemSections}.
   *
   * Each header is the accordion's button, and its text is the section label
   * followed by the count chip — or by nothing while that section's search is
   * still loading, when the chip is a skeleton.
   */
  itemSectionHeaders(): Locator {
    const labels = DASHBOARD.itemSections.map(escapeForRegExp).join("|");
    return this.main().getByRole("button", {
      name: new RegExp(`^(${labels})\\s*\\d*$`),
    });
  }

  /**
   * The total each section of a dashboard item page reports, from its count
   * chip, once every section has finished loading.
   *
   * The chips — not the rows — are the list's totals: each holds the section
   * search's `totalRecords`, while the rows beneath stop at the first page of
   * 10. A section with nothing in it is not rendered at all, so the result
   * holds only the sections the page shows, and is empty when the page shows
   * its empty-state copy instead.
   *
   * @param emptyMessage - The mode's empty-state copy, which settles the page
   *   as "nothing listed" rather than leaving the wait to time out.
   * @returns Section label to its total.
   */
  async itemSectionTotals(
    emptyMessage: string,
  ): Promise<Record<string, number>> {
    const labels = DASHBOARD.itemSections.map(escapeForRegExp).join("|");
    const loaded = new RegExp(`^(${labels}) (\\d+)$`);
    let totals: Record<string, number> = {};

    await expect(async () => {
      if (await this.emptyItemsMessage(emptyMessage).isVisible()) {
        totals = {};
        return;
      }
      const headers = (await this.itemSectionHeaders().allInnerTexts()).map(
        (text) => text.replace(/\s+/g, " ").trim(),
      );
      expect(
        headers.length,
        "the item page should list a section or say it has none",
      ).toBeGreaterThan(0);

      const read: Record<string, number> = {};
      for (const header of headers) {
        const match = loaded.exec(header);
        expect(match, `section "${header}" should have loaded its total`).not.toBeNull();
        read[(match as RegExpExecArray)[1]] = Number((match as RegExpExecArray)[2]);
      }
      totals = read;
    }).toPass({ timeout: LOAD_TIMEOUT_MS });

    return totals;
  }

  /** Error indicators anywhere in the main area — a failed section search
   * renders one in place of its rows and leaves its chip reading 0. */
  errorIndicators(): Locator {
    return this.main().getByTestId(DASHBOARD.errorIndicatorTestId);
  }

  /**
   * A section heading below the stat cards.
   *
   * Returns every match rather than narrowing to one: "Outstanding Support
   * Cases" titles both the severity donut and the cases table under it, so
   * callers assert the count is non-zero instead of asserting visibility on what
   * is legitimately two elements.
   *
   * @param title - The section's heading.
   * @returns Locator for the matching headings.
   */
  section(title: string): Locator {
    return this.main().getByRole("heading", { name: title, exact: true });
  }

  /**
   * An entry in one of the donut legends — severity, operations or
   * engagements.
   *
   * Located by text rather than by role: the entries carry no button role, and
   * the legend row itself is what handles the click.
   *
   * Unscoped by default, which suits the severity and operations labels — they
   * are distinctive enough to occur once on the page. Pass `chartHeading` for
   * plain-word labels such as "Migration" or "Follow Up", which a case title in
   * the table below could equally carry; the entry is then looked up inside
   * that chart card only (see {@link chartByHeading}).
   *
   * @param label - The legend entry, e.g. "S1 - Critical".
   * @param chartHeading - The chart card to scope to, if any.
   * @returns Locator for the entry.
   */
  legendEntry(label: string, chartHeading?: string): Locator {
    const scope = chartHeading ? this.chartByHeading(chartHeading) : this.main();
    return scope.getByText(label, { exact: true });
  }

  /**
   * The copy the Outstanding Engagements card shows in place of its donut and
   * legend when the project has no outstanding engagements.
   */
  engagementsChartEmpty(): Locator {
    return this.main().getByText(DASHBOARD.engagementsChartEmptyMessage, {
      exact: true,
    });
  }

  /**
   * A donut chart, located by its own heading.
   *
   * Narrowed by *both* the heading and the presence of a slice: "Outstanding
   * Support Cases" also titles the cases table below, which has no chart.
   * `.last()` then takes the innermost such container — the chart card rather
   * than a wrapper around the whole page.
   *
   * @param heading - The chart card's heading.
   * @returns Locator for the chart card.
   */
  chartByHeading(heading: string): Locator {
    return this.main()
      .locator("div")
      .filter({
        has: this.page.getByRole("heading", { name: heading, exact: true }),
      })
      .filter({ has: this.page.locator(DASHBOARD.severityChartSlice) })
      .last();
  }

  /**
   * A donut's slices, in legend order.
   *
   * Only series with a non-zero count are drawn: Recharts' `minAngle` applies to
   * non-zero values, so a zero-count series has no slice at all. Callers compare
   * the count against what they expect rather than assuming one per legend row.
   *
   * @param heading - The chart card's heading.
   */
  chartSlices(heading: string): Locator {
    return this.chartByHeading(heading).locator(DASHBOARD.severityChartSlice);
  }

  /**
   * Clicks a donut slice.
   *
   * Dispatches the event rather than clicking at a point. A slice is an arc, so
   * the centre of its bounding box falls in the donut's hole — a normal click
   * lands on nothing, and `force` clicks that same empty centre. Verified live:
   * forcing a click on the first slice left the dashboard where it was.
   *
   * @param heading - The chart card's heading.
   * @param index - Zero-based slice, in legend order.
   */
  async clickChartSlice(heading: string, index: number): Promise<void> {
    await this.chartSlices(heading).nth(index).dispatchEvent("click");
  }

  /** The Outstanding Support Cases donut's slices. */
  severityChartSlices(): Locator {
    return this.chartSlices(DASHBOARD.sections.outstandingCases);
  }

  /**
   * Clicks a slice of the Outstanding Support Cases donut.
   *
   * @param index - Zero-based slice, in legend order.
   */
  async clickSeverityChartSlice(index: number): Promise<void> {
    await this.clickChartSlice(DASHBOARD.sections.outstandingCases, index);
  }

  /**
   * Data rows of the Outstanding Support Cases table.
   *
   * Filtered on the case id so the header row is excluded — it is a `role="row"`
   * too, and taking `nth(1)` to skip it would silently start clicking the wrong
   * thing if a column were ever added above.
   */
  /**
   * Waits for the Outstanding Support Cases table to fill.
   *
   * The table is populated by its own request and routinely takes longer than
   * the 5s default assertion timeout — measured at roughly twelve seconds
   * against staging. Asserting a row without an explicit timeout fails with
   * "element(s) not found", which reads like a wrong selector and sends people
   * looking in the wrong place; the rows are there, just not yet.
   *
   * Encapsulated here rather than left to each call site, which is where the
   * timeout kept being forgotten.
   *
   * @returns The first row, for a caller that wants to read or click it.
   */
  async waitForCasesTable(): Promise<Locator> {
    const firstRow = this.casesTableRows().first();
    await expect(
      firstRow,
      "the Outstanding Support Cases table should list at least one case",
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    return firstRow;
  }

  /**
   * Waits for the cases table to reach a resting state and reports its size.
   *
   * Unlike {@link waitForCasesTable}, this tolerates an empty table: switching
   * to My Cases as an account that raised nothing on the project shows "No
   * outstanding cases.", which is a correct result rather than a failure.
   * Waiting only for rows would hang there for the full timeout and then blame
   * the selector.
   *
   * @returns The number of rows, 0 when the empty state is showing.
   */
  async waitForCasesTableSettled(): Promise<number> {
    await expect(async () => {
      const rows = await this.casesTableRows().count();
      const empty = await this.main()
        .getByText(DASHBOARD.casesTable.emptyMessage)
        .count();
      expect(
        rows + empty,
        "the cases table should show rows or say it has none",
      ).toBeGreaterThan(0);
    }).toPass({ timeout: LOAD_TIMEOUT_MS });

    return this.casesTableRows().count();
  }

  casesTableRows(): Locator {
    return this.main()
      .getByRole("row")
      .filter({ hasText: DASHBOARD.casesTable.rowIdPattern });
  }

  /**
   * A view tab of the Outstanding Support Cases table — My Cases or All Cases.
   *
   * @param label - The tab's label.
   * @returns Locator for the tab.
   */
  casesTableViewTab(label: string): Locator {
    return this.main().getByRole("tab", { name: label, exact: true });
  }

  /**
   * The email addresses named in the table's Created by column, one per row.
   *
   * Parsed out of each row's text: the column has no test id, and reading the
   * cells positionally would break the moment a column is added.
   *
   * @returns One address per row that names one.
   */
  async casesTableCreators(): Promise<string[]> {
    const rows = await this.casesTableRows().allInnerTexts();
    return rows
      .map((row) => /([\w.+-]+@[\w.-]+)/.exec(row)?.[1])
      .filter((creator): creator is string => !!creator);
  }

  /**
   * The Rows per page control at the foot of the cases table.
   *
   * MUI's TablePagination points the select's `aria-labelledby` at the "Rows per
   * page:" text, so the accessible name carries it — but the trigger's role
   * changed from `button` to `combobox` across MUI versions, so this accepts
   * either rather than pinning one.
   */
  rowsPerPageSelect(): Locator {
    const label = DASHBOARD.casesTable.pagination.rowsPerPageLabel;
    const asCombobox = this.main().getByRole("combobox", {
      name: new RegExp(escapeForRegExp(label)),
    });
    const asButton = this.main().getByRole("button", {
      name: new RegExp(escapeForRegExp(label)),
    });
    return asCombobox.or(asButton).first();
  }

  /**
   * Changes the cases table's page size.
   *
   * The option list renders in a portal at the document root rather than inside
   * the table, so it is looked up page-wide.
   *
   * @param rows - Page size to choose.
   */
  async selectRowsPerPage(rows: number): Promise<void> {
    await this.rowsPerPageSelect().click();
    await this.page
      .getByRole("option", { name: String(rows), exact: true })
      .click();
  }

  /**
   * Item rows on a dashboard item page — Action Required and its siblings.
   *
   * Those pages render the same ListCard as the cases list, so a row is a
   * clickable element carrying "Created by".
   */
  itemRows(): Locator {
    return this.main()
      .getByRole("button")
      .filter({ hasText: CASES_LIST.createdByPrefix });
  }

  /**
   * The copy a dashboard item page shows when it has nothing to list.
   *
   * Each mode has its own wording, and it is the *page's* EmptyState rather than
   * ListItems' — sections with no items are filtered out before they render, so
   * the list component's own empty copy never appears here.
   *
   * @param message - The mode's empty message.
   * @returns Locator for it.
   */
  emptyItemsMessage(message: string): Locator {
    return this.main().getByText(message, { exact: true });
  }

  /** Opens the cases table's filter panel. */
  casesTableFiltersButton(): Locator {
    return this.main().getByRole("button", {
      name: DASHBOARD.casesTable.filtersButton,
      exact: true,
    });
  }

  /**
   * The same control as the Filters button once a filter is applied, where it
   * clears rather than toggles the panel.
   *
   * @param activeCount - How many filters are active, which the label carries.
   * @returns Locator for the clear control.
   */
  clearFiltersButton(activeCount: number): Locator {
    return this.main().getByRole("button", {
      name: DASHBOARD.casesTable.clearFiltersButton(activeCount),
      exact: true,
    });
  }

  /** The Severity filter Select, addressed by the element id the field sets. */
  severityFilterSelect(): Locator {
    return this.filterSelect(DASHBOARD.casesTable.filters.severity.selectId);
  }

  /**
   * A filter Select in the cases table's filter panel.
   *
   * Addressed by the element id CasesFilters gives each field — its `filterKey`
   * (see {@link DASHBOARD.casesTable.filters}). The Selects' accessible names
   * would serve too, but a multi-select's name also carries its current value,
   * so the id is the one handle that does not change as options are ticked.
   *
   * @param selectId - The field's `filterKey`, e.g. "statusIds".
   * @returns Locator for the Select trigger.
   */
  filterSelect(selectId: string): Locator {
    return this.main().locator(`#${selectId}`);
  }

  /**
   * A filter Select's floating label. CasesFilters ids it `<filterKey>-label`
   * and points the Select's `aria-labelledby` at it.
   *
   * @param selectId - The field's `filterKey`.
   * @returns Locator for the label.
   */
  filterLabel(selectId: string): Locator {
    return this.main().locator(`#${selectId}-label`);
  }

  /**
   * Chooses a severity in the cases table's filter panel.
   *
   * @param label - Option label, e.g. "S4(Query)".
   */
  async selectSeverityFilter(label: string): Promise<void> {
    await this.selectFilterOptions(
      DASHBOARD.casesTable.filters.severity.selectId,
      [label],
    );
  }

  /**
   * Ticks one or more options of a multi-select filter, then closes its menu.
   *
   * Every filter on this table is multi-select, so the menu stays open after a
   * click — Escape closes it, which both commits the choice and clears the
   * overlay that would otherwise intercept clicks on the table beneath. Each
   * tick re-queries the table on its own, so a caller waiting on the request
   * should wait for the one carrying the full selection.
   *
   * The option list renders in a portal at the document root, so it is looked
   * up page-wide rather than inside the panel.
   *
   * @param selectId - The field's `filterKey`.
   * @param labels - Option labels to tick, in order.
   */
  async selectFilterOptions(
    selectId: string,
    labels: readonly string[],
  ): Promise<void> {
    await this.filterSelect(selectId).click();
    for (const label of labels) {
      await this.page
        .getByRole("listbox")
        .getByRole("option", { name: label, exact: true })
        .click();
    }
    await this.page.keyboard.press("Escape");
  }

  /**
   * The case numbers the cases table is showing, one per row.
   *
   * @returns Case numbers, e.g. "CS0001234", in row order.
   */
  async casesTableCaseNumbers(): Promise<string[]> {
    const rows = await this.casesTableRows().allInnerTexts();
    return rows
      .map((row) => DASHBOARD.casesTable.rowCaseNumberPattern.exec(row)?.[1])
      .filter((value): value is string => !!value);
  }

  /** The cases table's next-page control. Disabled on the last page. */
  nextPageButton(): Locator {
    return this.main().getByRole("button", {
      name: DASHBOARD.casesTable.pagination.nextPageButton,
      exact: true,
    });
  }

  /** The cases table's previous-page control. Disabled on the first page. */
  previousPageButton(): Locator {
    return this.main().getByRole("button", {
      name: DASHBOARD.casesTable.pagination.previousPageButton,
      exact: true,
    });
  }

  /** The "1–5 of 36" range text MUI renders beside the page controls. */
  displayedRows(): Locator {
    return this.main().getByText(
      DASHBOARD.casesTable.pagination.displayedRowsPattern,
    );
  }

  /**
   * The first row number the table is currently showing, from the range text.
   *
   * @returns The "from" of "from–to of count", or null when it is not rendered.
   */
  async displayedFromRow(): Promise<number | null> {
    const text = await this.displayedRows().innerText();
    const match = DASHBOARD.casesTable.pagination.displayedRowsPattern.exec(
      text,
    );
    return match ? Number(match[1]) : null;
  }

  /**
   * Every stat card label on the page, for counting them.
   *
   * A union of the expected labels rather than a structural selector: the grid's
   * cards carry no test id, and each label appears exactly once on the page
   * (verified live), so this counts cards without depending on the DOM shape.
   */
  statCardLabels(): Locator {
    const pattern = DASHBOARD.statCards
      .map((card) => escapeForRegExp(card.label))
      .join("|");
    return this.main().getByText(new RegExp(`^(${pattern})$`));
  }
}
