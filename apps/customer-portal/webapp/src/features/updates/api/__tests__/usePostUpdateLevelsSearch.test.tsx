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
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { usePostUpdateLevelsSearch } from "@features/updates/api/usePostUpdateLevelsSearch";

const mockAuthFetch = vi.fn();

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => mockAuthFetch,
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn() }),
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: true, isLoading: false }),
}));

describe("usePostUpdateLevelsSearch", () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("config", {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    });
  });

  it("stays idle when params are null", () => {
    const { result } = renderHook(() => usePostUpdateLevelsSearch(null), {
      wrapper,
    });

    expect(result.current.fetchStatus).toBe("idle");
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("posts search payload and returns response data", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          10: {
            updateType: "regular",
            updateDescriptionLevels: [],
          },
        }),
    } as Response);

    const params = {
      productName: "wso2am",
      productVersion: "4.4.0",
      startingUpdateLevel: 0,
      endingUpdateLevel: 10,
    };

    const { result } = renderHook(() => usePostUpdateLevelsSearch(params), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/updates/levels/search",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(params),
      }),
    );
    expect(result.current.data).toEqual({
      10: {
        updateType: "regular",
        updateDescriptionLevels: [],
      },
    });
  });
});

