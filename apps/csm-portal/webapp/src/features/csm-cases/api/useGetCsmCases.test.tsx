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

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();

// The real client + config read runtime config at module load, which isn't
// present under vitest (same approach as useQuickCaseSearch.test.tsx).
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({ useIdTokenClaims: () => ({ email: "me@wso2.com" }) }));
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" } }),
}));

import { useGetCsmCases } from "@features/csm-cases/api/useGetCsmCases";
import { DEFAULT_CASES_FILTERS } from "@features/csm-cases/utils/casesFiltersUrl";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

function beCase(id: string, number: string) {
  return { id, number, subject: `subject ${number}`, state: "new" };
}

/** The `filters` object of the Nth `POST /cases/search` call. */
function filtersOfCall(n: number) {
  return postMock.mock.calls[n][1].filters;
}

/**
 * Which of the three parallel legs a request body represents:
 * - `exact`   — the indexed identifier lookup
 * - `text`    — the free-text CONTAINS scan
 * - `overlap` — both together, whose `total` says how many exact hits the scan
 *               already counts (see `useGetCsmCases`)
 */
function legOf(body: { filters: { searchQuery?: string; filters?: { op: string }[] } }) {
  const hasExact = (body.filters.filters ?? []).some((x) => x.op === "eq");
  const hasText = !!body.filters.searchQuery;
  if (hasExact && hasText) return "overlap";
  return hasExact ? "exact" : "text";
}

describe("useGetCsmCases — identifier search runs both legs in parallel", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("issues an exact-match AND a free-text search for a case number, pinning the exact hit first", async () => {
    // Leg 1 (exact) resolves the case actually being looked up; leg 2
    // (free-text) resolves an unrelated case that merely mentions that number
    // in its description — the reported failure mode.
    postMock.mockImplementation((_url, body) => {
      const leg = legOf(body);
      if (leg === "overlap") {
        // The scan does not cover the exact hit here.
        return Promise.resolve({ cases: [], total: 0, limit: 1, offset: 0 });
      }
      return Promise.resolve(
        leg === "exact"
          ? { cases: [beCase("id-target", "CS0346083")], total: 1, limit: 5, offset: 0 }
          : { cases: [beCase("id-other", "CS0361878")], total: 1, limit: 20, offset: 0 },
      );
    });

    const filters = { ...DEFAULT_CASES_FILTERS, search: "CS0346083" };
    const { result } = renderHook(() => useGetCsmCases(filters, 0, 20), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledTimes(3);

    // One leg exact, one leg free-text (the third resolves their overlap).
    const legs = [filtersOfCall(0), filtersOfCall(1), filtersOfCall(2)].filter(
      (f) => !(f.searchQuery && f.filters?.length),
    );
    const exactLeg = legs.find((f) => f.filters?.length);
    const textLeg = legs.find((f) => f.searchQuery);
    expect(exactLeg.filters).toEqual([
      { field: "number", op: "eq", values: ["CS0346083"] },
    ]);
    expect(textLeg.searchQuery).toBe("CS0346083");

    // The exact hit is pinned above the free-text hit, and nothing is lost.
    expect(result.current.data?.cases.map((c) => c.id)).toEqual(["id-target", "id-other"]);
  });

  it("does not drop or duplicate a case that both legs return", async () => {
    postMock.mockImplementation((_url, body) => {
      const leg = legOf(body);
      if (leg === "overlap") {
        // The scan already counts the exact hit, so it adds nothing to total.
        return Promise.resolve({ cases: [], total: 1, limit: 1, offset: 0 });
      }
      return Promise.resolve(
        leg === "exact"
          ? { cases: [beCase("id-target", "CS0346083")], total: 1, limit: 5, offset: 0 }
          : {
              // Free-text also matches the target (it contains its own number).
              cases: [beCase("id-target", "CS0346083"), beCase("id-other", "CS0361878")],
              total: 2,
              limit: 20,
              offset: 0,
            },
      );
    });

    const filters = { ...DEFAULT_CASES_FILTERS, search: "CS0346083" };
    const { result } = renderHook(() => useGetCsmCases(filters, 0, 20), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.cases.map((c) => c.id)).toEqual(["id-target", "id-other"]);
    // Free-text already counted it, so the total isn't inflated.
    expect(result.current.data?.total).toBe(2);
  });

  it("keeps pinned rows off later pages so they can't appear twice", async () => {
    postMock.mockImplementation((_url, body) => {
      const leg = legOf(body);
      if (leg === "overlap") {
        return Promise.resolve({ cases: [], total: 1, limit: 1, offset: 0 });
      }
      return Promise.resolve(
        leg === "exact"
          ? { cases: [beCase("id-target", "CS0346083")], total: 1, limit: 5, offset: 0 }
          : {
              cases: [beCase("id-target", "CS0346083"), beCase("id-p2", "CS0400000")],
              total: 40,
              limit: 20,
              offset: 20,
            },
      );
    });

    const filters = { ...DEFAULT_CASES_FILTERS, search: "CS0346083" };
    const { result } = renderHook(() => useGetCsmCases(filters, 1, 20), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // Page 1: no pinned rows, and the pinned id is filtered out of the page.
    expect(result.current.data?.cases.map((c) => c.id)).toEqual(["id-p2"]);
  });

  it("reports the same total on every page, and never exceeds pageSize", async () => {
    // The exact hit sits outside the first free-text page (it only shows up
    // deep in the scan), which is what used to make the total change as the
    // user paged: the pinned row counted as "extra" on pages that didn't
    // contain it, and as already-counted on the page that did.
    const pageSizeUnderTest = 3;
    postMock.mockImplementation((_url, body) => {
      const leg = legOf(body);
      if (leg === "overlap") {
        // The scan covers the exact hit (it is row 7 below), so it adds nothing.
        return Promise.resolve({ cases: [], total: 1, limit: 1, offset: 0 });
      }
      if (leg === "exact") {
        return Promise.resolve({
          cases: [beCase("id-target", "CS0346083")],
          total: 1,
          limit: 5,
          offset: 0,
        });
      }
      // A free-text set of 9, with the exact hit buried at index 7.
      const all = Array.from({ length: 9 }, (_, i) =>
        i === 7 ? beCase("id-target", "CS0346083") : beCase(`id-${i}`, `CS040000${i}`),
      );
      const { offset, limit } = body.pagination;
      return Promise.resolve({
        cases: all.slice(offset, offset + limit),
        total: all.length,
        limit,
        offset,
      });
    });

    const filters = { ...DEFAULT_CASES_FILTERS, search: "CS0346083" };
    const totals: number[] = [];
    for (const page of [0, 1, 2]) {
      const { result } = renderHook(
        () => useGetCsmCases(filters, page, pageSizeUnderTest),
        { wrapper },
      );
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      totals.push(result.current.data!.total);
      expect(result.current.data!.cases.length).toBeLessThanOrEqual(pageSizeUnderTest);
    }

    // Stable across pages -- the actual regression being guarded.
    expect(new Set(totals).size).toBe(1);
    expect(totals[0]).toBe(9);
  });

  it("does not skip the row displaced off page 0 by a pinned hit", async () => {
    // Pinning consumes a slot at the front, so page 1 must start one row early
    // -- otherwise the row pushed off the end of page 0 is never shown at all.
    const pageSizeUnderTest = 3;
    const all = Array.from({ length: 6 }, (_, i) => beCase(`id-${i}`, `CS040000${i}`));
    postMock.mockImplementation((_url, body) => {
      const leg = legOf(body);
      if (leg === "overlap") {
        return Promise.resolve({ cases: [], total: 0, limit: 1, offset: 0 });
      }
      if (leg === "exact") {
        return Promise.resolve({
          cases: [beCase("id-target", "CS0346083")],
          total: 1,
          limit: 5,
          offset: 0,
        });
      }
      const { offset, limit } = body.pagination;
      return Promise.resolve({
        cases: all.slice(offset, offset + limit),
        total: all.length,
        limit,
        offset,
      });
    });

    const filters = { ...DEFAULT_CASES_FILTERS, search: "CS0346083" };
    const seen: string[] = [];
    for (const page of [0, 1]) {
      const { result } = renderHook(
        () => useGetCsmCases(filters, page, pageSizeUnderTest),
        { wrapper },
      );
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      seen.push(...result.current.data!.cases.map((c) => c.id));
    }

    // Pinned first, then the free-text rows in order with none skipped.
    expect(seen).toEqual(["id-target", "id-0", "id-1", "id-2", "id-3", "id-4"]);
  });

  it("counts an exact-only hit in the total and keeps every row reachable", async () => {
    // The scan misses the identifier entirely (the anomaly this feature exists
    // for), so the merged sequence is 1 exact-only row + 9 scan rows. If the
    // total under-reported those 9, the paginator would stop offering pages
    // before the last row and it could never be reached.
    const pageSizeUnderTest = 3;
    const scanRows = Array.from({ length: 9 }, (_, i) =>
      beCase(`id-${i}`, `CS040000${i}`),
    );
    postMock.mockImplementation((_url, body) => {
      const leg = legOf(body);
      if (leg === "overlap") {
        return Promise.resolve({ cases: [], total: 0, limit: 1, offset: 0 });
      }
      if (leg === "exact") {
        return Promise.resolve({
          cases: [beCase("id-target", "CS0346083")],
          total: 1,
          limit: 5,
          offset: 0,
        });
      }
      const { offset, limit } = body.pagination;
      return Promise.resolve({
        cases: scanRows.slice(offset, offset + limit),
        total: scanRows.length,
        limit,
        offset,
      });
    });

    const filters = { ...DEFAULT_CASES_FILTERS, search: "CS0346083" };
    const seen: string[] = [];
    const totals: number[] = [];
    // 10 rows over a page size of 3 => 4 pages, the last holding a single row.
    for (const page of [0, 1, 2, 3]) {
      const { result } = renderHook(
        () => useGetCsmCases(filters, page, pageSizeUnderTest),
        { wrapper },
      );
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      totals.push(result.current.data!.total);
      expect(result.current.data!.cases.length).toBeLessThanOrEqual(pageSizeUnderTest);
      seen.push(...result.current.data!.cases.map((c) => c.id));
    }

    expect(new Set(totals)).toEqual(new Set([10]));
    // All ten rows reachable, in merged order, none repeated.
    expect(seen).toEqual([
      "id-target",
      ...scanRows.map((c) => c.id),
    ]);
    expect(new Set(seen).size).toBe(10);
  });

  it("issues only ONE search for a plain free-text query", async () => {
    postMock.mockResolvedValue({ cases: [], total: 0, limit: 20, offset: 0 });

    const filters = { ...DEFAULT_CASES_FILTERS, search: "printer jam" };
    const { result } = renderHook(() => useGetCsmCases(filters, 0, 20), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(filtersOfCall(0).searchQuery).toBe("printer jam");
  });

  it("issues only ONE search when there is no query at all", async () => {
    postMock.mockResolvedValue({ cases: [], total: 0, limit: 20, offset: 0 });

    const { result } = renderHook(() => useGetCsmCases(DEFAULT_CASES_FILTERS, 0, 20), {
      wrapper,
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(filtersOfCall(0).searchQuery).toBeUndefined();
  });
});

describe("useGetCsmCases — queryKey covers every manually-toggleable filter", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ cases: [], total: 0, limit: 20, offset: 0 });
  });

  // Regression: `csTeams` (and, at the time, several other CasesFilters
  // fields) reached the search payload via `buildCaseSearchFilters` but had
  // no entry in the queryKey array below -- so picking a team from the bar's
  // "CRE Team" control changed `filters.csTeams` without changing the queryKey,
  // and React Query treated it as the identical query and never refetched.
  // Reported live: "when i select a team, no network call goes in the team
  // filter."
  it("refetches when only csTeams changes", async () => {
    const { result, rerender } = renderHook(
      ({ filters }) => useGetCsmCases(filters, 0, 20),
      { wrapper, initialProps: { filters: DEFAULT_CASES_FILTERS } },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(postMock).toHaveBeenCalledTimes(1);

    rerender({ filters: { ...DEFAULT_CASES_FILTERS, csTeams: ["g-1"] } });
    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(2));
  });

  it("refetches when only onboardingStatuses changes", async () => {
    const { result, rerender } = renderHook(
      ({ filters }) => useGetCsmCases(filters, 0, 20),
      { wrapper, initialProps: { filters: DEFAULT_CASES_FILTERS } },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(postMock).toHaveBeenCalledTimes(1);

    rerender({
      filters: { ...DEFAULT_CASES_FILTERS, onboardingStatuses: ["In-Progress"] },
    });
    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(2));
  });
});

describe("useGetCsmCases — relative-date `createdOn` placeholders", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ cases: [], total: 0, limit: 20, offset: 0 });
  });

  /** The `createdOn` entry with the given op from the Nth call's field filters. */
  function createdOnEntry(n: number, op: "gte" | "lte") {
    return filtersOfCall(n).filters.find(
      (f: { field: string; op: string }) => f.field === "createdOn" && f.op === op,
    );
  }

  it("resolves `__daysAgo:N__` to a concrete instant before hitting /cases/search, not the raw placeholder", async () => {
    const { result } = renderHook(
      () =>
        useGetCsmCases(
          { ...DEFAULT_CASES_FILTERS, createdOnLte: "__daysAgo:30__" },
          0,
          20,
        ),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const entry = createdOnEntry(0, "lte");
    expect(entry).toBeDefined();
    const sent = entry.values[0] as string;
    expect(sent).not.toBe("__daysAgo:30__");
    // A resolved RFC3339 instant, not the raw placeholder string.
    expect(() => new Date(sent).toISOString()).not.toThrow();
    expect(Number.isNaN(new Date(sent).getTime())).toBe(false);
  });

  it("leaves a literal date untouched", async () => {
    const { result } = renderHook(
      () =>
        useGetCsmCases(
          { ...DEFAULT_CASES_FILTERS, createdOnGte: "2026-01-01" },
          0,
          20,
        ),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(createdOnEntry(0, "gte").values[0]).toBe("2026-01-01");
  });
});
