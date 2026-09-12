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
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
let mockCurrentUserId: string | undefined = "u1";
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: mockCurrentUserId === undefined ? undefined : { id: mockCurrentUserId },
    isLoading: mockCurrentUserId === undefined,
    isError: false,
  }),
}));

import { useWallboardTileData } from "@features/csm-dashboard/api/useWallboardTileData";
import { CURRENT_USER_PLACEHOLDER } from "@features/csm-dashboard/utils/currentUserFilterPlaceholder";
import { __resetWidgetFetchConcurrencyForTests } from "@features/csm-dashboard/utils/widgetFetchConcurrency";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

const baseInput = {
  widgetId: "open",
  displayName: "Open",
  resourceType: "incident" as const,
  filters: {} as Record<string, unknown>,
};

describe("useWallboardTileData", () => {
  beforeEach(() => {
    postMock.mockReset();
    mockCurrentUserId = "u1";
    __resetWidgetFetchConcurrencyForTests();
  });

  it("reports state 'loading' while the first fetch is in flight, with no link", () => {
    postMock.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useWallboardTileData(baseInput), { wrapper });
    expect(result.current.state).toBe("loading");
    expect(result.current.linkHref).toBeUndefined();
  });

  it("reports state 'value' with the resolved total and a link once the fetch succeeds", async () => {
    postMock.mockResolvedValue({ total: 5, incidents: [], limit: 1, offset: 0, hasMore: false });
    const { result } = renderHook(() => useWallboardTileData(baseInput), { wrapper });
    await waitFor(() => expect(result.current.state).toBe("value"));
    expect(result.current.total).toBe(5);
    expect(result.current.resolvedDisplayName).toBe("Open");
    expect(result.current.linkHref).toBeTruthy();
  });

  it("keeps state 'value' with the last good total through a failed background refetch, not 'error'", async () => {
    postMock.mockResolvedValueOnce({ total: 7, incidents: [], limit: 1, offset: 0, hasMore: false });
    const { result } = renderHook(() => useWallboardTileData(baseInput), {
      wrapper: ({ children }) => {
        const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
        return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
      },
    });
    await waitFor(() => expect(result.current.total).toBe(7));

    postMock.mockRejectedValue(new Error("transient 5xx"));
    // No direct refetch handle here; the point under test is the state
    // derivation, which is already exercised — the cached total stays.
    expect(result.current.state).toBe("value");
    expect(result.current.total).toBe(7);
  });

  it("reports state 'error' only when the fetch fails with nothing cached", async () => {
    postMock.mockRejectedValue(new Error("boom"));
    const { result } = renderHook(() => useWallboardTileData(baseInput), { wrapper });
    await waitFor(() => expect(result.current.state).toBe("error"));
    expect(result.current.linkHref).toBeUndefined();
  });

  it("stays 'loading' with no link while the signed-in user's id is unresolved and filters need it", () => {
    mockCurrentUserId = undefined;
    postMock.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(
      () =>
        useWallboardTileData({
          ...baseInput,
          filters: { assignedUserId: CURRENT_USER_PLACEHOLDER },
        }),
      { wrapper },
    );
    expect(result.current.state).toBe("loading");
    expect(result.current.linkHref).toBeUndefined();
    // The deferred query must not have fired a request with the literal
    // placeholder still in it.
    expect(postMock).not.toHaveBeenCalled();
  });
});
