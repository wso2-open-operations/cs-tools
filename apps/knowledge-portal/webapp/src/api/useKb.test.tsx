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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { searchArticles } from "@api/backend/client";
import type { KBArticle, SearchKBArticlesResponse } from "@api/backend/types";
import { getNextOffset, trailingEmptyPages } from "@api/pagination";
import { useArticleSearch } from "@api/useKb";

vi.mock("@api/backend/client", () => ({
  countArticles: vi.fn(),
  searchArticles: vi.fn(),
  listKnowledgeBases: vi.fn(),
  getArticle: vi.fn(),
  getRelatedArticles: vi.fn(),
}));

const article = (id: string): KBArticle => ({
  id,
  knowledgeBaseId: "kb",
  title: id,
  body: "",
  createdOn: "2026-01-01T00:00:00Z",
  updatedOn: "2026-01-01T00:00:00Z",
});

const page = (ids: string[], offset: number, total: number, hasMore: boolean): SearchKBArticlesResponse => ({
  articles: ids.map(article),
  total,
  limit: 2,
  offset,
  hasMore,
});

const wrapper = ({ children }: { children: ReactNode }) => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
};

const search = vi.mocked(searchArticles);

describe("getNextOffset", () => {
  it("advances by the page limit while hasMore", () => {
    expect(getNextOffset(page(["a", "b"], 0, 5, true))).toBe(2);
  });
  it("stops when hasMore is false", () => {
    expect(getNextOffset(page(["a"], 4, 5, false))).toBeUndefined();
  });
  it("stops at the backend's maximum offset", () => {
    expect(getNextOffset({ ...page(["a"], 9999, 99999, true), limit: 50 })).toBeUndefined();
    expect(getNextOffset({ ...page(["a"], 9950, 99999, true), limit: 50 })).toBe(10000);
  });
});

describe("paging guards", () => {
  it("stops when a page reports hasMore with a non-positive limit", () => {
    expect(getNextOffset({ ...page(["a"], 0, 5, true), limit: 0 })).toBeUndefined();
  });
  it("counts trailing empty pages", () => {
    expect(trailingEmptyPages([page(["a"], 0, 5, true), page([], 2, 5, true), page([], 4, 5, true)])).toBe(2);
    expect(trailingEmptyPages([page([], 0, 5, true), page(["a"], 2, 5, true)])).toBe(0);
  });
});

describe("useArticleSearch", () => {
  beforeEach(() => search.mockReset());

  it("sends limit and offset and appends pages on loadMore", async () => {
    search
      .mockResolvedValueOnce(page(["a", "b"], 0, 3, true))
      .mockResolvedValueOnce(page(["c"], 2, 3, false));

    const { result } = renderHook(() => useArticleSearch({ knowledgeBaseId: "kb", searchQuery: "q" }), { wrapper });

    await waitFor(() => expect(result.current.articles.map((a) => a.id)).toEqual(["a", "b"]));
    expect(result.current.total).toBe(3);
    expect(result.current.hasMore).toBe(true);
    expect(search.mock.calls[0][0]).toEqual({ knowledgeBaseId: "kb", searchQuery: "q", limit: 20, offset: 0 });

    act(() => result.current.loadMore());

    await waitFor(() => expect(result.current.articles.map((a) => a.id)).toEqual(["a", "b", "c"]));
    expect(result.current.hasMore).toBe(false);
    expect(search.mock.calls[1][0]).toMatchObject({ offset: 2, limit: 20 });
  });

  it("omits empty filters from the request", async () => {
    search.mockResolvedValue(page([], 0, 0, false));
    const { result } = renderHook(() => useArticleSearch({}), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(search.mock.calls[0][0]).toEqual({ limit: 20, offset: 0 });
  });

  it("keeps the previous results visible while a new filter loads", async () => {
    let release!: (value: SearchKBArticlesResponse) => void;
    search.mockResolvedValueOnce(page(["a"], 0, 1, false));
    search.mockImplementationOnce(() => new Promise((resolve) => { release = resolve; }));

    const { result, rerender } = renderHook(
      ({ q }: { q: string }) => useArticleSearch({ searchQuery: q }),
      { wrapper, initialProps: { q: "one" } },
    );
    await waitFor(() => expect(result.current.articles).toHaveLength(1));

    rerender({ q: "two" });
    await waitFor(() => expect(result.current.isRefreshing).toBe(true));
    expect(result.current.articles.map((a) => a.id)).toEqual(["a"]);
    expect(result.current.isLoading).toBe(false);

    await act(async () => release(page(["z"], 0, 1, false)));
    await waitFor(() => expect(result.current.articles.map((a) => a.id)).toEqual(["z"]));
    expect(result.current.isRefreshing).toBe(false);
  });
});
