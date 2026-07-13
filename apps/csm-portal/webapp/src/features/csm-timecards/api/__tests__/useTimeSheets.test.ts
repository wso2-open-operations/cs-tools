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

import { describe, expect, it, vi } from "vitest";
import { searchTimeCards } from "@features/csm-timecards/api/useTimeSheets";
import type { BackendApi } from "@api/backend/client";
import type { BeSearchTimeCardsResponse, BeTimeCardState, BeTimeCardView } from "@api/backend/types";

// useTimeSheets.ts pulls in @config/apiConfig transitively (via useGetUsersMe)
// and @api/backend/client directly; both read window.config at module load
// and throw outside a configured runtime. Mock both so importing
// searchTimeCards below doesn't trip either (vi.mock calls are hoisted above
// these imports by vitest). searchTimeCards takes its own BackendApi instance
// directly, so neither the hook nor the real config is ever exercised here.
vi.mock("@api/backend/client", () => ({ useBackendApi: vi.fn() }));
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));

function beCard(id: string, state: BeTimeCardState): BeTimeCardView {
  return {
    id,
    totalTime: 30,
    workDate: "2026-07-13",
    createdOn: "2026-07-13",
    hasBillable: true,
    state,
    user: { id: "user-1", name: "Jane Doe" },
    project: { id: "proj-1", name: "Acme" },
    case: { id: "case-1", number: "CS0000001", name: "Issue" },
  };
}

function bePage(timeCards: BeTimeCardView[], total: number, offset: number, limit: number): BeSearchTimeCardsResponse {
  return { timeCards, total, offset, limit };
}

/** A mock `BackendApi` whose `post` returns each of `pages` in call order —
 * safe because `searchTimeCards` awaits each raw-page fetch before issuing
 * the next, so there's never more than one in-flight `post` call. */
function mockApi(pages: BeSearchTimeCardsResponse[]): { api: BackendApi; post: ReturnType<typeof vi.fn> } {
  let call = 0;
  const post = vi.fn(async () => pages[Math.min(call++, pages.length - 1)]);
  const api: BackendApi = {
    get: vi.fn(),
    post: post as unknown as BackendApi["post"],
    patch: vi.fn(),
    postEmpty: vi.fn(),
    del: vi.fn(),
    getBlob: vi.fn(),
  };
  return { api, post };
}

describe("searchTimeCards", () => {
  it("makes a single request and returns the page as-is when no states filter is set", async () => {
    const cards = [beCard("a", "submitted"), beCard("b", "approved")];
    const { api, post } = mockApi([bePage(cards, 2, 0, 20)]);

    const result = await searchTimeCards(api, undefined, { page: 0, rowsPerPage: 20 });

    expect(post).toHaveBeenCalledTimes(1);
    expect(result.cards.map((c) => c.id)).toEqual(["a", "b"]);
    expect(result.total).toBe(2);
  });

  it("doesn't walk forward when the first page already has enough matches", async () => {
    const cards = [beCard("a", "approved"), beCard("b", "approved")];
    const { api, post } = mockApi([bePage(cards, 2, 0, 20)]);

    const result = await searchTimeCards(api, { states: ["approved"] }, { page: 0, rowsPerPage: 20 });

    expect(post).toHaveBeenCalledTimes(1);
    expect(result.cards.map((c) => c.id)).toEqual(["a", "b"]);
  });

  // Regression test for the reported bug: a state filter reading as "no
  // results" purely because the filter excluded everything on the first raw
  // page, even though a matching card existed a page later.
  it("walks forward into later raw pages to find matches the first page excluded", async () => {
    const page0 = [beCard("a", "submitted"), beCard("b", "rejected")];
    const page1 = [beCard("c", "approved")];
    const { api, post } = mockApi([
      bePage(page0, 3, 0, 2),
      bePage(page1, 3, 2, 2),
    ]);

    const result = await searchTimeCards(api, { states: ["approved"] }, { page: 0, rowsPerPage: 2 });

    expect(post).toHaveBeenCalledTimes(2);
    expect(result.cards.map((c) => c.id)).toEqual(["c"]);
  });

  it("stops walking once it has collected a full page of matches", async () => {
    const page0 = [beCard("a", "submitted")];
    const page1 = [beCard("b", "approved"), beCard("c", "approved")];
    const page2 = [beCard("d", "approved")];
    const { api, post } = mockApi([
      bePage(page0, 10, 0, 2),
      bePage(page1, 10, 2, 2),
      bePage(page2, 10, 4, 2),
    ]);

    const result = await searchTimeCards(api, { states: ["approved"] }, { page: 0, rowsPerPage: 2 });

    // Found 2 matches (== rowsPerPage) on the walk's first extra page — the
    // third page must never be requested.
    expect(post).toHaveBeenCalledTimes(2);
    expect(result.cards.map((c) => c.id)).toEqual(["b", "c"]);
  });

  it("stops walking once the scope is exhausted, without over-fetching", async () => {
    const page0 = [beCard("a", "submitted")];
    const page1 = [beCard("b", "rejected")];
    const { api, post } = mockApi([
      bePage(page0, 2, 0, 1),
      bePage(page1, 2, 1, 1),
    ]);

    const result = await searchTimeCards(api, { states: ["approved"] }, { page: 0, rowsPerPage: 1 });

    // total is 2 and both raw records were consumed (offsets 0 and 1) — the
    // walk must stop there rather than requesting a third, out-of-range page.
    expect(post).toHaveBeenCalledTimes(2);
    expect(result.cards).toHaveLength(0);
  });

  it("caps the walk at MAX_STATE_FILTER_WALK_PAGES extra requests even if nothing matches", async () => {
    // A large unmatching scope (bigger than the walk budget could ever cover).
    const pages = Array.from({ length: 10 }, (_, i) => bePage([beCard(`x${i}`, "submitted")], 1000, i, 1));
    const { api, post } = mockApi(pages);

    const result = await searchTimeCards(api, { states: ["approved"] }, { page: 0, rowsPerPage: 1 });

    // 1 initial fetch + 5 walked extra fetches (MAX_STATE_FILTER_WALK_PAGES) = 6.
    expect(post).toHaveBeenCalledTimes(6);
    expect(result.cards).toHaveLength(0);
  });
});
