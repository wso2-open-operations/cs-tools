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

import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useOverview, useTaxonomy } from "./hooks";

/** Wraps a hook under test in its own fresh QueryClient. */
function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("api hooks", () => {
  beforeEach(() => {
    // @ts-expect-error -- partial config is fine for these tests
    window.config = { GID_BACKEND_BASE_URL: "https://backend.example.test" };
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    // @ts-expect-error -- test-only cleanup of the global window.config
    delete window.config;
  });

  it("useTaxonomy fetches GET /taxonomy and resolves with the response", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ statuses: [], csStatuses: ["WOC"] }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const { result } = renderHook(() => useTaxonomy(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual({ statuses: [], csStatuses: ["WOC"] });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url] = fetchMock.mock.calls[0] as [string];
    expect(url).toBe("https://backend.example.test/taxonomy");
  });

  it("useOverview requests /metrics/overview with the given filters", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ refreshedAt: "2026-01-01T00:00:00Z" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const { result } = renderHook(() => useOverview({ abtTeam: "Atlas" }), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url] = fetchMock.mock.calls[0] as [string];
    expect(url).toBe("https://backend.example.test/metrics/overview?abtTeam=Atlas");
  });

  it("useOverview keeps the previous filter's data on screen while the new filter's fetch is pending", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      const body = url.includes("repo=b") ? { repo: "b" } : { repo: "a" };
      return Promise.resolve(
        new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }),
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    // A single shared QueryClient across both renders, so the key change on
    // rerender can hit the first render's cache entry as its placeholder.
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    const { result, rerender } = renderHook(({ repo }: { repo: string }) => useOverview({ repo }), {
      wrapper: localWrapper,
      initialProps: { repo: "a" },
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual({ repo: "a" });

    rerender({ repo: "b" });

    // The new query key is a cache miss, but keepPreviousData holds the
    // prior filter's data on screen instead of flipping isLoading.
    expect(result.current.isLoading).toBe(false);
    expect(result.current.isPlaceholderData).toBe(true);
    expect(result.current.data).toEqual({ repo: "a" });

    await waitFor(() => expect(result.current.data).toEqual({ repo: "b" }));
    expect(result.current.isPlaceholderData).toBe(false);
  });
});
