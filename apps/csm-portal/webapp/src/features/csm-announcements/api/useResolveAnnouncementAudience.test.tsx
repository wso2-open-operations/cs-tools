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
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi, beforeEach } from "vitest";

const postMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

// Imported after the mock above so the module picks it up.
import { useResolveAnnouncementAudience } from "@features/csm-announcements/api/useResolveAnnouncementAudience";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

function project(id: string) {
  return { id, name: `Project ${id}`, key: id.toUpperCase(), account: { id: "a", name: "Acme" } };
}

describe("useResolveAnnouncementAudience", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("stays disabled and issues no request until enabled", () => {
    const { result } = renderHook(
      () =>
        useResolveAnnouncementAudience(false, {
          excludeClosureStates: [],
          excludeSubscriptionTypes: [],
        }),
      { wrapper },
    );
    expect(postMock).not.toHaveBeenCalled();
    expect(result.current.total).toBe(0);
    expect(result.current.isLoading).toBe(false);
  });

  it("pages through every match before reporting a total, not just the first page", async () => {
    // 3 pages of 2 rows each (page size is fixed inside the hook at 50, but
    // hasMore/empty-page still end the loop correctly at any page size the
    // backend actually returns).
    postMock
      .mockResolvedValueOnce({
        projects: [project("p1"), project("p2")],
        total: 5,
        limit: 50,
        offset: 0,
        hasMore: true,
      })
      .mockResolvedValueOnce({
        projects: [project("p3"), project("p4")],
        total: 5,
        limit: 50,
        offset: 2,
        hasMore: true,
      })
      .mockResolvedValueOnce({
        projects: [project("p5")],
        total: 5,
        limit: 50,
        offset: 4,
        hasMore: false,
      });

    const { result } = renderHook(
      () =>
        useResolveAnnouncementAudience(true, {
          excludeClosureStates: ["Restricted"],
          excludeSubscriptionTypes: [],
        }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(postMock).toHaveBeenCalledTimes(3);
    expect(result.current.total).toBe(5);
    expect(result.current.projects.map((p) => p.id)).toEqual(["p1", "p2", "p3", "p4", "p5"]);
    // Offset advances by rows actually received, not assumed from the request.
    expect(postMock).toHaveBeenNthCalledWith(
      2,
      "/announcements/audience/search",
      expect.objectContaining({ pagination: { offset: 2, limit: 50 } }),
    );
    expect(postMock).toHaveBeenNthCalledWith(
      3,
      "/announcements/audience/search",
      expect.objectContaining({ pagination: { offset: 4, limit: 50 } }),
    );
  });

  it("stops paging when a page comes back empty even if hasMore is true", async () => {
    postMock.mockResolvedValueOnce({
      projects: [],
      total: 0,
      limit: 50,
      offset: 0,
      hasMore: true,
    });

    const { result } = renderHook(
      () =>
        useResolveAnnouncementAudience(true, {
          excludeClosureStates: [],
          excludeSubscriptionTypes: [],
        }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(postMock).toHaveBeenCalledTimes(1);
    expect(result.current.total).toBe(0);
  });

  it("errors rather than silently reporting a truncated audience when the safety bound is hit", async () => {
    // hasMore stays true forever, so the loop always exhausts
    // MAX_AUDIENCE_PAGES (100) before the upstream match set ever ends.
    postMock.mockImplementation((_url: string, body: { pagination: { offset: number } }) =>
      Promise.resolve({
        projects: [project(`p${body.pagination.offset}`)],
        total: 1_000_000,
        limit: 50,
        offset: body.pagination.offset,
        hasMore: true,
      }),
    );

    const { result } = renderHook(
      () =>
        useResolveAnnouncementAudience(true, {
          excludeClosureStates: [],
          excludeSubscriptionTypes: [],
        }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isError).toBe(true);
    expect(result.current.total).toBe(0);
  });
});
