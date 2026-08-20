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
import type { BeProduct, BeProductSearchResponse } from "@api/backend/types";

const postMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useQuickCaseSearch.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useInfiniteProductSearch } from "@features/csm-cases/api/useProductSearch";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

/** Builds a page of `count` rows all sharing `name`, matching the real
 * catalogue shape where one family can monopolize many consecutive rows. */
function page(
  name: string,
  count: number,
  hasMore: boolean,
  offset: number,
): BeProductSearchResponse {
  const products: BeProduct[] = Array.from({ length: count }, (_, i) => ({
    id: `${name}-${offset + i}`,
    name,
  }));
  return { products, total: 117, limit: count, offset, hasMore };
}

describe("useInfiniteProductSearch", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("keeps paging in the background past a first page monopolized by one family, without a manual scroll fetch", async () => {
    // Mirrors the live DEV-tenant bug: the first 50-row page is entirely one
    // family ("API Manager"), so the dropdown would render only 1 distinct
    // name and have nothing to scroll unless something else keeps fetching.
    postMock
      .mockResolvedValueOnce(page("API Manager", 50, true, 0))
      .mockResolvedValueOnce(page("API Manager", 50, true, 50))
      .mockResolvedValueOnce(page("Identity Server", 17, false, 100));

    const { result } = renderHook(() => useInfiniteProductSearch("", true), {
      wrapper,
    });

    // Without calling fetchNextPage manually, the background effect should
    // still walk every page until hasMore is false.
    await waitFor(() =>
      expect(result.current.productNames).toEqual([
        "API Manager",
        "Identity Server",
      ]),
    );
    await waitFor(() => expect(result.current.hasNextPage).toBe(false));
    expect(postMock).toHaveBeenCalledTimes(3);
  });

  it("does not fetch at all while disabled (dropdown closed)", () => {
    postMock.mockResolvedValue(page("API Manager", 50, true, 0));

    renderHook(() => useInfiniteProductSearch("", false), { wrapper });

    expect(postMock).not.toHaveBeenCalled();
  });

  it("stops auto-fetching at the safety cap even if the catalogue has more pages", async () => {
    // 21 pages of 50 rows each, every one reporting hasMore: true — well
    // past MAX_AUTO_PAGES (20). The background effect must not spin forever.
    postMock.mockImplementation(() =>
      Promise.resolve(page("API Manager", 50, true, 0)),
    );

    const { result } = renderHook(() => useInfiniteProductSearch("", true), {
      wrapper,
    });

    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(20));
    // Give the effect a chance to fire once more if it were unbounded.
    await new Promise((r) => setTimeout(r, 50));
    expect(postMock).toHaveBeenCalledTimes(20);
    expect(result.current.hasNextPage).toBe(true);
  });
});
