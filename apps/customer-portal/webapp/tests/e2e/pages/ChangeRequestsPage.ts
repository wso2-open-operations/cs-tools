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
import { CASE_DETAIL, CHANGE_REQUESTS_LIST } from "../utils/selectors";

/** How long to allow for the list and its search to resolve. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the change requests list and its two view modes.
 */
export class ChangeRequestsPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /**
   * Opens the list directly.
   *
   * Always lands on the unfiltered view: the filtered variants are selected by
   * router state that the hub and the dashboard donut pass, and a URL cannot
   * carry it.
   *
   * @param projectId - Project whose change requests to list.
   */
  async open(projectId: string): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/${CHANGE_REQUESTS_LIST.pathSegment}`,
    );
    await expect(this.heading(CHANGE_REQUESTS_LIST.titles.all)).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * The list's heading.
   *
   * @param title - Expected title.
   */
  heading(title: string): Locator {
    return this.main().getByRole("heading", { name: title, exact: true });
  }

  /**
   * A view tab — List View or Calendar View.
   *
   * @param label - The tab's label.
   */
  viewTab(label: string): Locator {
    return this.main().getByRole("tab", { name: label, exact: true });
  }

  /**
   * Switches view and waits for the tab to be selected.
   *
   * @param label - The view to open.
   */
  async openView(label: string): Promise<void> {
    await this.viewTab(label).click();
    await expect(this.viewTab(label)).toHaveAttribute("aria-selected", "true", {
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * The change request rows.
   *
   * Each row is a clickable card carrying the request's number, which is what
   * separates a row from the surrounding controls.
   */
  rows(): Locator {
    return this.main()
      .getByRole("button")
      .filter({ hasText: CHANGE_REQUESTS_LIST.numberPattern });
  }

  /**
   * EVERY row of the list, whatever its number looks like.
   *
   * {@link rows} keys on `CHG<digits>`, ServiceNow's numbering, so on a database
   * with another scheme — the local compose stack's seeded `CHG-FIXED-007`, or its
   * generated `CR-1013` — it finds nothing, and an assertion that "no row belongs
   * to another customer" would pass vacuously. Every row card prints its
   * "Updated:" stamp whatever it is numbered, so that is the key here.
   */
  allRows(): Locator {
    return this.main().getByRole("button").filter({ hasText: "Updated:" });
  }

  /**
   * The row carrying a given number, in any numbering scheme.
   *
   * @param number - Change request number, e.g. `CHG-FIXED-007`.
   */
  rowByNumber(number: string): Locator {
    return this.allRows().filter({ hasText: number });
  }

  /**
   * The number each row of the list carries, in render order.
   *
   * The number is read off each card as the first token shaped like one —
   * `CHG0038759`, `CHG-FIXED-007` or `CR-1013` — so a row whose number has none of
   * those shapes is skipped here but still counted by {@link allRows}.
   */
  async listedNumbers(): Promise<string[]> {
    const texts = await this.allRows().allInnerTexts();
    return texts.flatMap((text) => {
      const match = /\b(?:CHG|CR)[-A-Za-z]*-?\d+\b/.exec(text);
      return match ? [match[0]] : [];
    });
  }

  /** The copy shown when the list has nothing to show. */
  emptyMessage(): Locator {
    return this.main().getByText(CHANGE_REQUESTS_LIST.emptyMessage);
  }

  /**
   * Waits for the list to settle into one of its two real states — rows, or the
   * empty copy — so a caller cannot assert against a still-loading page.
   *
   * A row is recognised by {@link allRows}, not {@link rows}: a project whose
   * requests are numbered `CS-PORTAL-000026` (every request raised through the CSM
   * portal on the local stack) has no `CHG<digits>` row, so keying on that number
   * left this waiting for a state the page never reaches.
   */
  async waitForList(): Promise<void> {
    await expect(
      this.allRows().first().or(this.emptyMessage()).or(this.main().getByText(CHANGE_REQUESTS_LIST.emptyRefinedMessage)),
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Takes the applied filters off again (the list remembers them across visits, like its search) and waits for the
   * unfiltered list. Does nothing when none is applied.
   */
  async clearFilters(): Promise<void> {
    const clear = this.main().getByRole("button", { name: /^Clear Filters/ });
    if ((await clear.count()) === 0) return;
    const answered = this.page.waitForResponse(
      (r) => r.url().includes("/change-requests/search") && r.request().method() === "POST",
      { timeout: LOAD_TIMEOUT_MS },
    );
    await clear.click();
    await answered;
    await this.waitForList();
  }

  /** Opens the filter panel (State, Impact). */
  async openFilters(): Promise<void> {
    await this.main().getByRole("button", { name: /^Filters/ }).click();
    await expect(this.stateFilter()).toBeVisible();
  }

  /** The State filter's select (the panel is a popover, so it is looked for on the page, not inside the main region). */
  stateFilter(): Locator {
    return this.page.locator("#state");
  }

  /**
   * The states the State filter offers, in its order (the panel must be open). Opens and closes the select.
   */
  async stateFilterOptions(): Promise<string[]> {
    await this.stateFilter().click();
    await expect(this.page.getByRole("option").first()).toBeVisible();
    const options = await this.page.getByRole("option").allInnerTexts();
    await this.page.keyboard.press("Escape");
    return options;
  }

  /**
   * Filters the list to one state (the panel must be open) and waits for the search that carries it.
   *
   * @param label - The state as the filter words it, e.g. `Authorize`.
   */
  async filterByState(label: string): Promise<void> {
    const answered = this.page.waitForResponse(
      (r) => r.url().includes("/change-requests/search") && r.request().method() === "POST",
      { timeout: LOAD_TIMEOUT_MS },
    );
    await this.stateFilter().click();
    await this.page.getByRole("option", { name: label, exact: true }).click();
    await this.page.keyboard.press("Escape");
    await expect(this.page.getByRole("listbox")).toBeHidden();
    await answered;
    await expect(
      this.allRows().first().or(this.main().getByText(CHANGE_REQUESTS_LIST.emptyRefinedMessage)),
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /** The list's search box ("Search change requests by number, title, or description..."). */
  searchBox(): Locator {
    return this.main().getByPlaceholder(/^Search change requests/);
  }

  /**
   * Searches the list for `text` (a number, say) and waits until the search that carries it has answered, so a
   * caller that then counts the rows (or the empty copy) is not looking at the list from before it.
   *
   * @param text - What to type.
   */
  async searchFor(text: string): Promise<void> {
    const answered = this.page.waitForResponse(
      (r) =>
        r.url().includes("/change-requests/search") &&
        r.request().method() === "POST" &&
        (r.request().postData() ?? "").includes(text),
      { timeout: LOAD_TIMEOUT_MS },
    );
    await this.searchBox().fill(text);
    await answered;
    await expect(
      this.allRows().first().or(this.main().getByText(CHANGE_REQUESTS_LIST.emptyRefinedMessage)),
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /** Empties the search box (the list remembers the last search across visits) and waits for the full list again. */
  async clearSearch(): Promise<void> {
    if ((await this.searchBox().inputValue()) === "") return;
    const answered = this.page.waitForResponse(
      (r) => r.url().includes("/change-requests/search") && r.request().method() === "POST",
      { timeout: LOAD_TIMEOUT_MS },
    );
    await this.searchBox().fill("");
    await answered;
    await expect(
      this.allRows().first().or(this.emptyMessage()).or(this.main().getByText(CHANGE_REQUESTS_LIST.emptyRefinedMessage)),
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * The change request number a row carries.
   *
   * @param index - Zero-based row.
   * @returns The number, or null when the row has none.
   */
  async rowNumber(index: number): Promise<string | null> {
    const text = await this.rows().nth(index).innerText();
    const match = /CHG\d+/.exec(text);
    return match ? match[0] : null;
  }
}
