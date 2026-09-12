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

  /** The copy shown when the list has nothing to show. */
  emptyMessage(): Locator {
    return this.main().getByText(CHANGE_REQUESTS_LIST.emptyMessage);
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
