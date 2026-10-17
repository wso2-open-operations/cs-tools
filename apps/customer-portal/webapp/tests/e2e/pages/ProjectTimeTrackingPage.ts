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
  type Request,
  expect,
} from "../fixtures/test";
import { CASE_DETAIL, PROJECT_DETAILS, TIME_TRACKING } from "../utils/selectors";

/** The tab renders seven card skeletons while its query resolves, and the
 * project-details page loads behind it — both well past the 5s default. */
const LOAD_TIMEOUT_MS = 60_000;

/** Counts read off the "Showing n of total time cards" line. */
export interface TimeCardCounts {
  /** Cards rendered on the current page. */
  shown: number;
  /** Matching cards across all pages. */
  total: number;
}

/**
 * Page object for the Time Tracking tab of the project details page
 * (`/projects/:projectId/project-details`).
 */
export class ProjectTimeTrackingPage {
  constructor(private readonly page: Page) {}

  /** The app's `<main>` region. */
  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /** The Time Tracking tab itself. */
  timeTrackingTab(): Locator {
    return this.page.getByRole("tab", {
      name: PROJECT_DETAILS.tabs.timeTracking,
    });
  }

  /**
   * Navigates from the dashboard to the Time Tracking tab via the side nav,
   * then waits for the time cards query to resolve.
   *
   * @param projectId - Project whose time tracking to open.
   */
  async openTimeTrackingTab(projectId: string): Promise<void> {
    await this.page.goto(`/projects/${projectId}/dashboard`);

    const navItem = this.page.getByRole("button", {
      name: PROJECT_DETAILS.navItem,
    });
    await expect(navItem).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await navItem.click();
    await expect(this.page).toHaveURL(
      new RegExp(`/projects/${projectId}/${PROJECT_DETAILS.pathSegment}`),
    );

    await expect(this.timeTrackingTab()).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
    await this.timeTrackingTab().click();

    // The summary line is rendered from the query's own result, so it marks the
    // point where the list is real rather than skeletonised.
    await this.waitForSummary();
  }

  /** The "Showing n of total time cards" line. */
  summary(): Locator {
    return this.main().getByText(TIME_TRACKING.summaryPattern);
  }

  /** Waits for the summary line, i.e. for the time cards query to resolve. */
  async waitForSummary(): Promise<void> {
    await expect(this.summary()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Reads the rendered and total card counts.
   *
   * Parsed from the summary rather than counted off the DOM: `total` spans all
   * pages and is stated nowhere else, and reading both from one line keeps them
   * consistent with each other.
   *
   * @returns Cards shown on this page, and the matching total.
   */
  async counts(): Promise<TimeCardCounts> {
    const text = (await this.summary().innerText()).trim();
    const match = TIME_TRACKING.summaryPattern.exec(text);
    expect(match, `could not read card counts from "${text}"`).not.toBeNull();
    return { shown: Number(match![1]), total: Number(match![2]) };
  }

  /** The case-number chip on each rendered time card — one per entry. */
  entries(): Locator {
    return this.main().getByText(TIME_TRACKING.caseNumberPattern);
  }

  /** The empty state shown when no time cards match. */
  emptyMessage(): Locator {
    return this.main().getByText(TIME_TRACKING.emptyMessage);
  }

  /** The panel-level error state. */
  errorMessage(): Locator {
    return this.main().getByText(TIME_TRACKING.errorMessage);
  }

  /**
   * One of the two date fields.
   *
   * ⚠️ MUI X v8+ renders these with the accessible field DOM structure: a
   * `role="group"` holding one span per section (month, day, year), with the
   * real `<input>` hidden. `fill()` rejects that outright, so the group is
   * clicked and typed into instead — see {@link setDateRange}.
   *
   * @param label - "Start date" or "End date".
   * @returns Locator for that field's group.
   */
  dateField(label: string): Locator {
    return this.page.getByRole("group", { name: label });
  }

  /**
   * The filter row's Clear button — present only while a range is set.
   *
   * Filtered by text content, because each picker also renders a `clearable`
   * icon button titled "Clear": three buttons share the accessible name, and
   * only this one has visible text. Clicking a picker's icon would clear one
   * end of the range rather than the filter.
   */
  clearButton(): Locator {
    return this.page
      .getByRole("button", {
        name: TIME_TRACKING.dateFilter.clearButton,
        exact: true,
      })
      .filter({ hasText: TIME_TRACKING.dateFilter.clearButton });
  }

  /**
   * Types a date into one of the pickers.
   *
   * Typed digit by digit rather than assigned: the field advances from month to
   * day to year as each section fills, which only happens through real key
   * events. The field is clicked first so the caret lands on its first section,
   * so a value already present is overwritten rather than appended to.
   *
   * @param label - "Start date" or "End date".
   * @param date - The date to enter.
   */
  private async typeDate(label: string, date: Date): Promise<void> {
    const field = this.dateField(label);
    await expect(field).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await field.click();

    const mm = String(date.getMonth() + 1).padStart(2, "0");
    const dd = String(date.getDate()).padStart(2, "0");
    const yyyy = String(date.getFullYear());
    await this.page.keyboard.type(`${mm}${dd}${yyyy}`);
  }

  /** `YYYY-MM-DD`, the form the filter sends and the pickers hold. */
  private static isoDate(date: Date): string {
    const mm = String(date.getMonth() + 1).padStart(2, "0");
    const dd = String(date.getDate()).padStart(2, "0");
    return `${date.getFullYear()}-${mm}-${dd}`;
  }

  /** The filters a time-cards search request carried. */
  private static requestFilters(
    request: Request,
  ): { startDate?: string; endDate?: string } {
    const body = request.postDataJSON() as {
      filters?: { startDate?: string; endDate?: string };
    } | null;
    return body?.filters ?? {};
  }

  /**
   * Runs an action and waits for the time-cards search it triggers, then for
   * the DOM to catch up with that response.
   *
   * Waiting on the summary alone is NOT enough and was a real bug here: the
   * previous summary stays on screen while the refetch is in flight, so an
   * assertion made straight after a filter change reads the OLD counts and
   * passes or fails against stale data. Tying the wait to the response that
   * carries the expected filters — and then polling until the rendered total
   * equals that response's own `totalRecords` — removes the race in both
   * directions.
   *
   * @param matchFilters - Recognises the request this change should produce.
   * @param action - The interaction that triggers it.
   * @returns Counts once the DOM agrees with the response.
   */
  private async applyFilterChange(
    matchFilters: (filters: { startDate?: string; endDate?: string }) => boolean,
    action: () => Promise<void>,
  ): Promise<TimeCardCounts> {
    const responsePromise = this.page.waitForResponse(
      (response) =>
        response.request().method() === "POST" &&
        /\/cases\/time-cards\/search$/.test(
          new URL(response.url()).pathname,
        ) &&
        matchFilters(ProjectTimeTrackingPage.requestFilters(response.request())),
      { timeout: LOAD_TIMEOUT_MS },
    );

    await action();

    const response = await responsePromise;
    expect(
      response.ok(),
      `the time cards search failed (${response.status()})`,
    ).toBe(true);

    const payload = (await response.json()) as { totalRecords?: number };
    const expectedTotal = payload.totalRecords ?? 0;

    // Poll rather than read once: the response landing and React rendering it
    // are separate events.
    await expect
      .poll(async () => (await this.counts()).total, {
        timeout: LOAD_TIMEOUT_MS,
      })
      .toBe(expectedTotal);

    return this.counts();
  }

  /**
   * Sets both ends of the range and waits for the matching refetch.
   *
   * Start is entered first on purpose: the end picker takes `minDate` from the
   * start, so entering them the other way round can have the end value rejected
   * by a constraint that is about to change.
   *
   * @param start - Range start.
   * @param end - Range end.
   * @returns Counts for the filtered list.
   */
  async setDateRange(start: Date, end: Date): Promise<TimeCardCounts> {
    const startIso = ProjectTimeTrackingPage.isoDate(start);
    const endIso = ProjectTimeTrackingPage.isoDate(end);

    // Typing the start alone fires its own request, so the wait is armed around
    // the END keystrokes — only then do both filters carry their final values.
    await this.typeDate(TIME_TRACKING.dateFilter.startField, start);

    return this.applyFilterChange(
      (filters) =>
        filters.startDate === startIso && filters.endDate === endIso,
      async () => {
        await this.typeDate(TIME_TRACKING.dateFilter.endField, end);
        // Dismiss the picker popup so it cannot cover the list underneath.
        await this.page.keyboard.press("Escape");
      },
    );
  }

  /**
   * Clears the range and waits for the unfiltered refetch.
   *
   * @returns Counts for the unfiltered list.
   */
  async clearDateRange(): Promise<TimeCardCounts> {
    await expect(this.clearButton()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    return this.applyFilterChange(
      (filters) => !filters.startDate && !filters.endDate,
      async () => {
        await this.clearButton().click();
      },
    );
  }

  /** The Export button. */
  exportButton(): Locator {
    return this.main().getByRole("button", {
      name: TIME_TRACKING.export.button,
    });
  }

  /** A menu item in the open export menu. */
  exportMenuItem(label: string): Locator {
    return this.page.getByRole("menuitem", { name: label, exact: true });
  }

  /** Opens the export menu and waits for its items. */
  async openExportMenu(): Promise<void> {
    // Disabled while loading, on error, and when the result set is empty — so
    // enabled also confirms there is something to export.
    await expect(this.exportButton()).toBeEnabled({ timeout: LOAD_TIMEOUT_MS });
    await this.exportButton().click();
    await expect(
      this.exportMenuItem(TIME_TRACKING.export.csvItem),
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Picks an export format and captures the resulting download.
   *
   * Both formats are built in the browser and handed over through an anchor's
   * `download` attribute, so there is no request to wait on — the download
   * event is the only evidence a file was produced.
   *
   * @param label - "Export to CSV" or "Export to PDF".
   * @returns The download.
   */
  async exportAs(label: string): Promise<Download> {
    const [download] = await Promise.all([
      this.page.waitForEvent("download", { timeout: LOAD_TIMEOUT_MS }),
      this.exportMenuItem(label).click(),
    ]);
    return download;
  }
}
