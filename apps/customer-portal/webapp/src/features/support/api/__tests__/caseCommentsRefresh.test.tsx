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

import { QueryClient, type InfiniteData } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { CaseCommentsResponse } from "@features/support/types/cases";
import {
  MAX_INCREMENTAL_REFRESH_PAGES,
  countLoadedComments,
  dedupeCommentPages,
  refreshCaseComments,
} from "../caseCommentsRefresh";
import useGetCaseCommentsInfinite from "../useGetCaseCommentsInfinite";

const mockAuthFetch = vi.fn();
vi.mock("@/hooks/useAuthApiClient", () => ({ useAuthApiClient: () => mockAuthFetch }));
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: true, isLoading: false }),
}));
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn(), warn: vi.fn(), info: vi.fn() }),
}));

const KEY = [ApiQueryKeys.CASE_COMMENTS, "project-1", "case-1", "infinite"];
const logger = { debug: vi.fn() };

const comment = (n: number) => ({
  id: `c${n}`,
  content: `body ${n}`,
  createdOn: `2026-01-01 00:00:${String(n).padStart(2, "0")}`,
  createdBy: "user",
  type: "comments",
  isEscalated: false,
});

/** Server state, newest first. ids c<n> with a higher n are newer. */
let server: number[] = [];

function mockServer() {
  mockAuthFetch.mockImplementation(async (_url: string, init: RequestInit) => {
    const { offset, limit } = JSON.parse(init.body as string).pagination;
    const slice = server.slice(offset, offset + limit);
    return {
      ok: true,
      json: () =>
        Promise.resolve({
          activities: slice.map(comment),
          totalRecords: server.length,
          offset,
          limit,
        }),
    } as Response;
  });
}

function seed(queryClient: QueryClient, pageSizes: number[]) {
  // Builds cached pages from the current `server` state, newest first.
  let cursor = 0;
  const pages: CaseCommentsResponse[] = pageSizes.map((size) => {
    const slice = server.slice(cursor, cursor + size);
    const page = {
      comments: slice.map(comment),
      totalRecords: server.length,
      offset: cursor,
      limit: 10,
    };
    cursor += size;
    return page;
  });
  queryClient.setQueryData<InfiniteData<CaseCommentsResponse>>(KEY, {
    pages,
    pageParams: pageSizes.map((_, i) => i * 10),
  });
}

const cachedIds = (queryClient: QueryClient) =>
  queryClient
    .getQueryData<InfiniteData<CaseCommentsResponse>>(KEY)!
    .pages.flatMap((p) => p.comments.map((c) => c.id));

const range = (from: number, to: number) =>
  Array.from({ length: to - from + 1 }, (_, i) => to - i); // to ... from

describe("refreshCaseComments", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("config", { CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test" });
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    server = range(1, 25);
    mockServer();
  });

  it("makes exactly one request for a single new comment", async () => {
    seed(queryClient, [10, 10, 5]);
    server = [26, ...server];

    await refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger);

    expect(mockAuthFetch).toHaveBeenCalledTimes(1);
    expect(JSON.parse(mockAuthFetch.mock.calls[0][1].body)).toEqual({
      pagination: { offset: 0, limit: 10 },
    });
    const data = queryClient.getQueryData<InfiniteData<CaseCommentsResponse>>(KEY)!;
    expect(data.pages).toHaveLength(3);
    expect(cachedIds(queryClient).slice(0, 3)).toEqual(["c26", "c25", "c24"]);
    expect(cachedIds(queryClient)).toHaveLength(26);
    expect(data.pages.every((p) => p.totalRecords === 26)).toBe(true);
  });

  it("maps new comments to the same shape as the query", async () => {
    seed(queryClient, [10]);
    server = [26, ...server];
    await refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger);
    const first = queryClient.getQueryData<InfiniteData<CaseCommentsResponse>>(KEY)!.pages[0]
      .comments[0];
    expect(first).toMatchObject({ id: "c26", content: "body 26", isEscalated: false });
  });

  it("walks a second page when new comments span two pages, then stops", async () => {
    seed(queryClient, [10, 10, 5]);
    server = [...range(26, 37), ...server]; // 12 new

    await refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger);

    expect(mockAuthFetch).toHaveBeenCalledTimes(2);
    expect(cachedIds(queryClient)).toEqual(server.map((n) => `c${n}`));
  });

  it("falls back to invalidating when no known comment is found within the cap", async () => {
    seed(queryClient, [10]);
    server = [...range(100, 100 + MAX_INCREMENTAL_REFRESH_PAGES * 10 + 4), ...server];
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const before = cachedIds(queryClient);

    await refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger);

    expect(mockAuthFetch).toHaveBeenCalledTimes(MAX_INCREMENTAL_REFRESH_PAGES);
    expect(invalidate).toHaveBeenCalledWith({ queryKey: KEY });
    expect(cachedIds(queryClient)).toEqual(before);
  });

  it("does nothing when the case has no cached comments", async () => {
    await refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger);
    expect(mockAuthFetch).not.toHaveBeenCalled();

    queryClient.setQueryData(KEY, { pages: [], pageParams: [] });
    await refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger);
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("ignores cached comments of other cases", async () => {
    queryClient.setQueryData([ApiQueryKeys.CASE_COMMENTS, "project-1", "case-2", "infinite"], {
      pages: [{ comments: [comment(1)], totalRecords: 1, offset: 0, limit: 10 }],
      pageParams: [0],
    });
    await refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger);
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("adds nothing when there are no new comments", async () => {
    seed(queryClient, [10, 10, 5]);
    await refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger);
    expect(mockAuthFetch).toHaveBeenCalledTimes(1);
    expect(cachedIds(queryClient)).toHaveLength(25);
  });

  it("fills a cache that holds an empty first page", async () => {
    server = [];
    seed(queryClient, [0]);
    server = [1];
    await refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger);
    expect(mockAuthFetch).toHaveBeenCalledTimes(1);
    expect(cachedIds(queryClient)).toEqual(["c1"]);
  });

  it("swallows request failures, logs at debug and leaves the cache untouched", async () => {
    seed(queryClient, [10]);
    const before = queryClient.getQueryData(KEY);
    mockAuthFetch.mockRejectedValue(new Error("network down"));

    await expect(
      refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger),
    ).resolves.toBeUndefined();

    expect(logger.debug).toHaveBeenCalled();
    expect(queryClient.getQueryData(KEY)).toBe(before);
  });

  it("coalesces concurrent refreshes into one run plus one follow-up", async () => {
    seed(queryClient, [10, 10, 5]);
    server = [26, ...server];

    const runs = [
      refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger),
      refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger),
      refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger),
    ];
    await Promise.all(runs);

    // First run adds c26; the queued follow-up finds nothing new (one request).
    expect(mockAuthFetch).toHaveBeenCalledTimes(2);
    expect(cachedIds(queryClient).filter((id) => id === "c26")).toHaveLength(1);
  });

  it("picks up a comment that lands while a refresh is in flight", async () => {
    seed(queryClient, [10]);
    server = [26, ...server];
    const first = refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger);
    server = [27, ...server];
    await Promise.all([first, refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger)]);
    expect(cachedIds(queryClient).slice(0, 2)).toEqual(["c27", "c26"]);
  });
});

describe("comment page helpers", () => {
  const page = (ids: number[], totalRecords = 30): CaseCommentsResponse => ({
    comments: ids.map(comment),
    totalRecords,
    offset: 0,
    limit: 10,
  });

  it("counts distinct loaded comments", () => {
    expect(countLoadedComments([page([3, 2]), page([2, 1])])).toBe(3);
  });

  it("dedupes comments repeated across pages and keeps untouched pages by reference", () => {
    const clean: InfiniteData<CaseCommentsResponse> = {
      pages: [page([3, 2]), page([1])],
      pageParams: [0, 2],
    };
    expect(dedupeCommentPages(clean)).toBe(clean);

    const overlapping: InfiniteData<CaseCommentsResponse> = {
      pages: [page([3, 2]), page([2, 1])],
      pageParams: [0, 2],
    };
    const result = dedupeCommentPages(overlapping);
    expect(result.pages.flatMap((p) => p.comments.map((c) => c.id))).toEqual(["c3", "c2", "c1"]);
  });
});

// react-query only re-renders for result fields that were read during render,
// so read every field the way a component would (spread invokes the getters).
function useSubscribed() {
  const query = useGetCaseCommentsInfinite("project-1", "case-1");
  return { ...query };
}

describe("useGetCaseCommentsInfinite next page after a refresh", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("config", { CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test" });
    server = range(1, 25);
    mockServer();
  });

  it("requests older comments at the loaded count, without duplicates or gaps", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useSubscribed(), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.hasNextPage).toBe(true);

    // Two new comments arrive and are spliced in: every older offset shifts by 2.
    server = [27, 26, ...server];
    await refreshCaseComments(queryClient, mockAuthFetch, "case-1", logger);
    await waitFor(() =>
      expect(result.current.data?.pages.flatMap((p) => p.comments)).toHaveLength(12),
    );
    mockAuthFetch.mockClear();

    await result.current.fetchNextPage();
    expect(JSON.parse(mockAuthFetch.mock.calls[0][1].body).pagination.offset).toBe(12);
    await waitFor(() =>
      expect(result.current.data?.pages.flatMap((p) => p.comments)).toHaveLength(22),
    );
    await result.current.fetchNextPage();
    await waitFor(() => expect(result.current.hasNextPage).toBe(false));

    const ids = result.current.data!.pages.flatMap((p) => p.comments.map((c) => c.id));
    expect(ids).toEqual(server.map((n) => `c${n}`));
  });

  it("drops an overlapping comment when an older page repeats one already loaded", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useSubscribed(), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // A comment arrives that the cache has not seen: page 2 repeats c15.
    server = [26, ...server];
    await result.current.fetchNextPage();
    await waitFor(() => expect(result.current.data?.pages).toHaveLength(2));

    const ids = result.current.data!.pages.flatMap((p) => p.comments.map((c) => c.id));
    expect(new Set(ids).size).toBe(ids.length);
  });
});
