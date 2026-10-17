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
import { GLOBAL_SEARCH } from "../utils/selectors";

/** The dropdown renders skeletons while both the cases and the change-requests
 * queries are in flight, which outlasts the 5s default expect timeout. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the header search bar (`SearchBar.tsx`).
 *
 * Lives in the header, so it is reachable from any project-scoped page rather
 * than belonging to one — but it searches only the SELECTED project, so every
 * method here assumes a project page is already open.
 */
export class GlobalSearchPage {
  constructor(private readonly page: Page) {}

  /**
   * The search input itself.
   *
   * Descends to the `<input>` deliberately: MUI puts `data-testid` on the
   * TextField ROOT — a `<div>` wrapping the adornments and the field — so the
   * test id alone resolves to something `fill()` rejects outright with
   * "Element is not an <input>…". Verified live.
   */
  input(): Locator {
    return this.page.getByTestId(GLOBAL_SEARCH.inputTestId).locator("input");
  }

  /**
   * The results dropdown.
   *
   * Rendered through a portal onto `document.body`, not inside the header, so
   * this is deliberately page-scoped — scoping it to the header would never
   * match.
   */
  dropdown(): Locator {
    return this.page.getByTestId(GLOBAL_SEARCH.dropdownTestId);
  }

  /**
   * Every result card currently rendered.
   *
   * Cases and change requests alike are `Form.CardButton`s carrying
   * `role="button"`, and the dropdown holds no other buttons, so the role alone
   * identifies results — the two card types are deliberately not distinguished
   * here, because a search returns one merged list.
   */
  resultCards(): Locator {
    return this.dropdown().getByRole("button");
  }

  /**
   * The result card naming `text` — a case number or a WSO2 case id.
   *
   * Filtered by text rather than located by it: the id is one `Typography`
   * among several inside the card, and the CARD is what has to be clicked.
   *
   * @param text - Identifier shown on the card.
   * @returns The matching card, as a locator.
   */
  resultCard(text: string): Locator {
    return this.resultCards().filter({ hasText: text });
  }

  /**
   * Types a query and waits for the dropdown to finish resolving it.
   *
   * The field is cleared first and the debounce waited out, so a query issued
   * after a previous one cannot be read against the previous one's results.
   *
   * Returns once the dropdown has settled into a real outcome — results or one
   * of the empty states — rather than after a fixed sleep, which would race the
   * two queries behind it.
   *
   * @param query - Text to search for.
   */
  async search(query: string): Promise<void> {
    const input = this.input();
    await expect(input).toBeVisible({ timeout: LOAD_TIMEOUT_MS });

    // Clearing and retyping rather than overwriting: `fill` on a non-empty
    // field would leave the previous debounce timer running, and a stale result
    // set briefly rendered under the new query reads as a result for it.
    await input.fill("");
    await input.fill(query);

    await expect(this.dropdown()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });

    // Settled = at least one card, or one of the terminal empty states. The
    // skeleton placeholders carry no role, so "a button exists" cannot be
    // satisfied by the loading state.
    const settled = this.resultCards()
      .first()
      .or(this.dropdown().getByText(GLOBAL_SEARCH.emptyStates.noResults))
      .or(this.dropdown().getByText(GLOBAL_SEARCH.emptyStates.error));
    await expect(settled.first()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Clicks a result and waits for the dropdown to close behind it.
   *
   * The dropdown closing is part of the contract, not incidental: the handler
   * clears the query and closes the list as it navigates, and one left open
   * over the destination page would cover it.
   *
   * @param card - The result card to open.
   */
  async openResult(card: Locator): Promise<void> {
    await expect(card).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await card.click();
    await expect(this.dropdown()).toBeHidden({ timeout: LOAD_TIMEOUT_MS });
  }
}
