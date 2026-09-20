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
import { CASE_DETAIL, SUPPORT_CENTER } from "../utils/selectors";
import { SideNavPage } from "./SideNavPage";
import { projectPathPattern } from "../utils/ids";

/** How long to allow for the shell and the card's own queries to resolve. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the Support Center landing page
 * (`/projects/:projectId/support`).
 *
 * Its overview cards each carry footer buttons that navigate into a pre-filtered
 * list — notably Outstanding Cases → "View my cases", which is the only place in
 * the UI that reaches `?createdByMe=true`.
 */
export class SupportCenterPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /**
   * Opens Support Center through the side nav, as a user would.
   *
   * Goes via the nav rather than a direct URL so the nav item itself is
   * exercised — the same reason the create-case specs go through Get Help.
   *
   * @param projectId - Project whose support centre to open.
   */
  async openViaSideNav(projectId: string): Promise<void> {
    const sideNav = new SideNavPage(this.page);
    await sideNav.open(projectId);
    await sideNav.clickItem(
      SUPPORT_CENTER.navItem,
      projectPathPattern(projectId, `${SUPPORT_CENTER.pathSegment}$`),
    );
    await expect(this.outstandingCasesCard()).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * One of the four stat cards across the top.
   *
   * Matched as a button rather than by its text: ListStatGrid makes each card a
   * `role="button"` when it is clickable, and the "Outstanding Cases" wording
   * also appears below as the overview card's title — so a text match would be
   * ambiguous. The accessible name runs the count into the label
   * ("36Outstanding Cases"), hence a substring match on the label.
   *
   * @param label - The card's label.
   * @returns Locator for the card.
   */
  statCard(label: string): Locator {
    return this.main().getByRole("button", { name: label });
  }

  /**
   * All four stat cards, for counting them.
   *
   * Identified by the accessible name starting with the stat's own value — the
   * card renders the number above the label, so every one of them reads
   * "<count><label>" and nothing else on the page does.
   */
  statCards(): Locator {
    return this.main().getByRole("button", { name: /^\d/ });
  }

  /**
   * The back control on a list opened from here.
   *
   * Doubles as the marker that the destination has actually rendered. The URL
   * changes before the route swaps, so a page that has only just been navigated
   * to still has Support Center in the DOM — and since the overview card's title
   * repeats a stat card's label, asserting on the destination's heading during
   * that overlap hits two elements and fails on strict mode rather than
   * retrying. This control exists only on the destination.
   */
  backButton(): Locator {
    return this.page.getByRole("button", {
      name: SUPPORT_CENTER.backButton,
      exact: true,
    });
  }

  /**
   * Waits for a list opened from a card to finish rendering.
   */
  async waitForList(): Promise<void> {
    await expect(this.backButton()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Returns to Support Center from a list opened by a card.
   *
   * The back control only carries this label when the list was reached from
   * here — it depends on the `returnTo` the card set — so clicking it is also
   * what proves the round trip is wired up.
   *
   * Asserts the whole project-scoped path, not just the trailing segment: the
   * `returnTo` the card set carries a project id, so landing on another
   * project's Support Center is exactly the kind of mistake this should catch.
   *
   * @param projectId - Project whose Support Center the list was opened from.
   */
  async returnFromList(projectId: string): Promise<void> {
    await this.backButton().click();
    await expect(this.page).toHaveURL(
      projectPathPattern(projectId, `${SUPPORT_CENTER.pathSegment}$`),
      { timeout: LOAD_TIMEOUT_MS },
    );
  }

  /**
   * The Outstanding Cases card.
   *
   * Filtered by its own title rather than taken by position: the page lays out
   * several near-identical overview cards, so document order is not a stable
   * handle on any one of them.
   */
  outstandingCasesCard(): Locator {
    return this.main()
      .locator("div")
      .filter({
        has: this.page.getByText(SUPPORT_CENTER.outstandingCases.title, {
          exact: true,
        }),
      })
      .filter({
        has: this.page.getByRole("button", {
          name: SUPPORT_CENTER.outstandingCases.myCasesButton,
          exact: true,
        }),
      })
      .last();
  }

  /**
   * The Chat History card.
   *
   * Located the same way as the Outstanding Cases card — by its title *and* one
   * of its own buttons, so nested containers resolve to the card itself.
   */
  chatHistoryCard(): Locator {
    return this.main()
      .locator("div")
      .filter({
        has: this.page.getByText(SUPPORT_CENTER.chatHistory.title, {
          exact: true,
        }),
      })
      .filter({
        has: this.page.getByRole("button", {
          name: SUPPORT_CENTER.chatHistory.allChatHistoryButton,
          exact: true,
        }),
      })
      .last();
  }

  /**
   * A footer button of the Chat History card.
   *
   * @param name - Exact button label.
   */
  chatHistoryFooterButton(name: string): Locator {
    return this.chatHistoryCard().getByRole("button", {
      name,
      exact: true,
    });
  }

  /**
   * A footer button of the Outstanding Cases card.
   *
   * Scoped to the card: the "View all cases" wording is close enough to other
   * cards' footers that a page-wide match risks landing on the wrong list.
   *
   * @param name - Exact button label.
   * @returns Locator for the button.
   */
  outstandingCasesFooterButton(name: string): Locator {
    return this.outstandingCasesCard().getByRole("button", {
      name,
      exact: true,
    });
  }
}
