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
import { useResolveProductVersionAudience } from "@features/csm-announcements/api/useResolveProductVersionAudience";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useResolveProductVersionAudience", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("stays disabled until both productId and productVersionId are chosen", () => {
    const { result } = renderHook(() => useResolveProductVersionAudience(undefined, undefined), {
      wrapper,
    });
    expect(postMock).not.toHaveBeenCalled();
    expect(result.current.total).toBe(0);
  });

  it("pages through every match before reporting a total", async () => {
    postMock
      .mockResolvedValueOnce({
        projects: [{ id: "p1", name: "Project 1" }],
        total: 2,
        limit: 50,
        offset: 0,
        hasMore: true,
      })
      .mockResolvedValueOnce({
        projects: [{ id: "p2", name: "Project 2" }],
        total: 2,
        limit: 50,
        offset: 1,
        hasMore: false,
      });

    const { result } = renderHook(() => useResolveProductVersionAudience("prod-1", "ver-1"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.total).toBe(2);
    expect(result.current.projects.map((p) => p.id)).toEqual(["p1", "p2"]);
  });

  it("errors rather than silently reporting a truncated audience when the safety bound is hit", async () => {
    // hasMore stays true forever, so the loop always exhausts
    // MAX_AUDIENCE_PAGES (100) before the upstream match set ever ends.
    postMock.mockImplementation((_url: string, body: { pagination: { offset: number } }) =>
      Promise.resolve({
        projects: [{ id: `p${body.pagination.offset}`, name: "Project" }],
        total: 1_000_000,
        limit: 50,
        offset: body.pagination.offset,
        hasMore: true,
      }),
    );

    const { result } = renderHook(() => useResolveProductVersionAudience("prod-1", "ver-1"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isError).toBe(true);
    expect(result.current.total).toBe(0);
  });
});
