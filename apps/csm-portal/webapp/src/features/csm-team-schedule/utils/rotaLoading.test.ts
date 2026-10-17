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

import { describe, expect, it } from "vitest";
import { rotaIsLoading, type RotaLoadingInput } from "./rotaLoading";

const settled: RotaLoadingInput = {
  view: "week",
  catalogueLoading: false,
  tabsKnown: true,
  mine: { isLoading: false },
  assignments: { isLoading: false, isPlaceholderData: false },
};
const loading = (over: Partial<RotaLoadingInput>): boolean => rotaIsLoading({ ...settled, ...over });

describe("rotaIsLoading: when the rota card shows its skeleton", () => {
  it("is not loading once the view's own rows are in", () => {
    for (const view of ["mine", "today", "week", "roster"] as const) {
      expect(loading({ view })).toBe(false);
    }
  });

  it("is loading until the catalogue and the reader's tabs are known, on every view", () => {
    for (const view of ["mine", "today", "week", "roster"] as const) {
      expect(loading({ view, catalogueLoading: true })).toBe(true);
      expect(loading({ view, tabsKnown: false })).toBe(true);
    }
  });

  it("is loading on the first read of a view", () => {
    expect(loading({ view: "week", assignments: { isLoading: true } })).toBe(true);
    expect(loading({ view: "roster", assignments: { isLoading: true } })).toBe(true);
    expect(loading({ view: "mine", mine: { isLoading: true } })).toBe(true);
  });

  it("is loading while the week or day still holds the previous range's rows", () => {
    // Stepping to a week not read yet kept last week's rows on screen; drawn
    // against the new dates they matched no day, so the week read as an empty
    // grid of dashes -- nobody on the rota -- until the answer arrived.
    const held = { isLoading: false, isPlaceholderData: true };
    expect(loading({ view: "week", assignments: held })).toBe(true);
    expect(loading({ view: "today", assignments: held })).toBe(true);
  });

  it("leaves the roster and My week to their own reads", () => {
    const held = { isLoading: false, isPlaceholderData: true };
    // The roster reads month by month and says it is loading on its own.
    expect(loading({ view: "roster", assignments: held })).toBe(false);
    // My week draws from its own read, not the week's.
    expect(loading({ view: "mine", assignments: held })).toBe(false);
  });
});
