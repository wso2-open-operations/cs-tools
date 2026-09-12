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
  type Response,
  expect,
} from "../fixtures/test";
import { CASE_DETAIL, UPDATES } from "../utils/selectors";
import { SideNavPage } from "./SideNavPage";

/** How long to allow for the page and its filter options to resolve. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the Updates page's update-level search.
 */
export class UpdatesPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /**
   * Opens the page through the side nav, as a user would.
   *
   * @param projectId - Project whose updates to open.
   */
  async openViaSideNav(projectId: string): Promise<void> {
    const sideNav = new SideNavPage(this.page);
    await sideNav.open(projectId);
    await sideNav.clickItem(
      UPDATES.navItem,
      new RegExp(`/projects/${projectId}/${UPDATES.pathSegment}`),
    );
    await expect(this.sectionTitle()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /** The filter section's heading. */
  sectionTitle(): Locator {
    return this.main().getByText(UPDATES.sectionTitle, { exact: true });
  }

  /** The hint shown before any search has been run. */
  idleHint(): Locator {
    return this.main().getByText(UPDATES.idleHint, { exact: true });
  }

  productSelect(): Locator {
    return this.main().locator(`#${UPDATES.ids.product}`);
  }

  versionSelect(): Locator {
    return this.main().locator(`#${UPDATES.ids.version}`);
  }

  startLevelSelect(): Locator {
    return this.main().locator(`#${UPDATES.ids.startLevel}`);
  }

  endLevelSelect(): Locator {
    return this.main().locator(`#${UPDATES.ids.endLevel}`);
  }

  /**
   * Opens a select and picks an option.
   *
   * The options render in a portal at the document root, so they are looked up
   * page-wide. Waits for the control to be enabled first: the selects cascade,
   * and each stays disabled until the one before it has a value.
   *
   * @param select - The select to operate.
   * @param option - Exact option label.
   */
  async chooseOption(select: Locator, option: string): Promise<void> {
    await expect(select).toBeEnabled({ timeout: LOAD_TIMEOUT_MS });
    await select.click();
    await this.page
      .getByRole("option", { name: option, exact: true })
      .click({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Reads the options a select offers, then closes it again.
   *
   * Waits for the first option before reading: `allInnerTexts` resolves against
   * whatever matches at that moment and does not retry, so reading straight after
   * the click can return an empty list while the portal mounts.
   *
   * @param select - The select to open.
   * @returns The option labels, in order, with the disabled placeholder dropped.
   */
  async optionLabels(select: Locator): Promise<string[]> {
    await expect(select).toBeEnabled({ timeout: LOAD_TIMEOUT_MS });
    await select.click();

    const options = this.page.getByRole("option");
    await expect(options.first()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    const labels = await options.allInnerTexts();

    await this.page.keyboard.press("Escape");

    const placeholders: string[] = Object.values(UPDATES.placeholders);
    return labels
      .map((label) => label.trim())
      .filter((label) => label.length > 0 && !placeholders.includes(label));
  }

  /**
   * Fills the whole filter row, in the order the cascade requires.
   *
   * @param productName - Product option label.
   * @param productVersion - Version option label.
   * @param startLevel - Starting update level.
   * @param endLevel - Ending update level.
   */
  async fillSearch(
    productName: string,
    productVersion: string,
    startLevel: string,
    endLevel: string,
  ): Promise<void> {
    await this.chooseOption(this.productSelect(), productName);
    await this.chooseOption(this.versionSelect(), productVersion);
    await this.chooseOption(this.startLevelSelect(), startLevel);
    await this.chooseOption(this.endLevelSelect(), endLevel);
  }

  /**
   * A column header of the results table.
   *
   * @param label - The header's text.
   */
  resultsColumnHeader(label: string): Locator {
    return this.main().getByRole("columnheader", { name: label, exact: true });
  }

  /** The copy shown when the searched range holds no update levels. */
  noResultsMessage(): Locator {
    return this.main().getByText(UPDATES.results.emptyMessage, {
      exact: true,
    });
  }

  /**
   * Waits for a search to finish rendering.
   *
   * The table replaces a skeleton, so the response landing is not the same as the
   * results being on screen. Settles on either the table's first column header or
   * the no-results copy — a range with nothing in it is a legitimate outcome.
   */
  async waitForResults(): Promise<void> {
    await expect(
      this.resultsColumnHeader(UPDATES.results.headers[0]).or(
        this.noResultsMessage(),
      ),
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * The View control on a results row.
   *
   * The control's label is the same on every row, so it has to be reached through
   * the row carrying the level — and the row is identified by that level's *cell*,
   * not by its text. `hasText` matches `textContent`, which concatenates the cells
   * without separators ("1SecurityView"), so a word-boundary pattern on the level
   * finds no boundary after the digit and matches nothing. Verified live.
   *
   * @param level - The update level the row lists.
   */
  viewLevelButton(level: string): Locator {
    return this.main()
      .getByRole("row")
      .filter({
        has: this.page.getByRole("cell", { name: level, exact: true }),
      })
      .getByRole("button", {
        name: UPDATES.results.viewButton,
        exact: true,
      });
  }

  //
  // Update level details page.
  //

  /**
   * A summary label on the details page.
   *
   * @param label - The label's text.
   */
  levelSummaryLabel(label: string): Locator {
    return this.main().getByText(label, { exact: true });
  }

  /**
   * One of the details page's update filters — All, Security or Regular.
   *
   * @param label - The filter's label.
   */
  levelFilterButton(label: string): Locator {
    return this.main().getByRole("button", { name: label, exact: true });
  }

  /**
   * The back control on the level details page.
   *
   * It goes back through history rather than to a fixed route, so where it lands
   * depends on how the page was opened.
   */
  levelBackButton(): Locator {
    return this.main().getByRole("button", {
      name: UPDATES.levelDetails.backButton,
      exact: true,
    });
  }

  /** The listed updates, each rendering its number inline. */
  levelUpdateNumbers(): Locator {
    return this.main().getByText(
      new RegExp(`${UPDATES.levelDetails.updateNumberPrefix}\\s*\\S+`),
    );
  }

  //
  // Report download.
  //

  /**
   * The Download Report button.
   *
   * Disabled until a search has produced report data, and while the PDF is being
   * generated. There is no dialog behind it — it saves the file directly.
   */
  downloadReportButton(): Locator {
    return this.main().getByRole("button", {
      name: UPDATES.report.downloadButton,
      exact: true,
    });
  }

  /**
   * Downloads the report and returns the captured download.
   *
   * The PDF is built in the browser by jsPDF and saved through a generated
   * anchor, which the browser surfaces as a download event — so the listener has
   * to be armed before the click.
   *
   * @returns The captured download.
   */
  async downloadReport(): Promise<Download> {
    const [download] = await Promise.all([
      this.page.waitForEvent("download", { timeout: LOAD_TIMEOUT_MS }),
      this.downloadReportButton().click(),
    ]);
    return download;
  }

  /** The Search button. Disabled until all four filters are set. */
  searchButton(): Locator {
    return this.main().getByRole("button", {
      name: UPDATES.searchButton,
      exact: true,
    });
  }

  /** The Clear Filters button. */
  clearFiltersButton(): Locator {
    return this.main().getByRole("button", {
      name: UPDATES.clearFiltersButton,
      exact: true,
    });
  }

  /**
   * Runs the search and waits for the levels request to land.
   *
   * Waits whatever the status, then leaves the caller to assert on it: requiring
   * 2xx in the predicate would mean a rejected search never matches and the test
   * times out with no clue as to why.
   *
   * @returns The search response.
   */
  async search(): Promise<Response> {
    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname.endsWith("/updates/levels/search") &&
          r.request().method() === "POST",
        { timeout: LOAD_TIMEOUT_MS },
      ),
      this.searchButton().click(),
    ]);
    return response;
  }
}
