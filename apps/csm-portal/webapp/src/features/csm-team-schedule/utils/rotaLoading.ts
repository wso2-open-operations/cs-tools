/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/** The page's views, as far as loading is concerned. */
export type RotaView = "mine" | "today" | "week" | "roster" | "paging" | "access";

export interface RotaLoadingInput {
  view: RotaView;
  /** The catalogue (zones, windows, kinds) is still on its way. */
  catalogueLoading: boolean;
  /** Whether it is known yet which tabs the reader gets. */
  tabsKnown: boolean;
  /** My week's own read of the reader's rota. */
  mine: { isLoading: boolean };
  /** The read the current view draws from. `isPlaceholderData` is set while
   *  it still holds the previous range's rows for a range not read yet. */
  assignments: { isLoading: boolean; isPlaceholderData?: boolean };
}

/**
 * Whether the rota card should show its loading skeleton rather than the view.
 *
 * The week and day reads keep the previous range's rows on screen while the
 * next one loads, so the card does not blank on every step. But those rows
 * are for other dates: drawn against the new week they matched no day, and
 * the week read as an empty grid of dashes -- nobody on the rota -- until the
 * answer arrived. So on those two views, rows held over from another range
 * count as still loading. The roster reads month by month and says it is
 * loading on its own; My week has its own read.
 */
export function rotaIsLoading({ view, catalogueLoading, tabsKnown, mine, assignments }: RotaLoadingInput): boolean {
  if (catalogueLoading || !tabsKnown) return true;
  if (view === "mine") return mine.isLoading;
  if ((view === "week" || view === "today") && assignments.isPlaceholderData) return true;
  return assignments.isLoading;
}
