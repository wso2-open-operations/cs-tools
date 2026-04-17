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
import { describe, expect, it, vi, beforeEach } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import useGetTimeTrackingDetails from "@features/usage-metrics/api/useGetTimeTrackingDetails";
import type { ReactNode } from "react";

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
    isSignedIn: true,
    isLoading: false,
  }),
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

vi.mock("@/utils/useAuthApiClient", () => ({
  useAuthApiClient: () =>
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/timetracking")) {
        return {
          ok: true,
          status: 200,
          json: async () => ({
            timeLogs: [
              {
                id: "1",
                badges: [],
                description: "Working on case #123",
                user: null,
                role: null,
                date: null,
                hours: null,
              },
            ],
          }),
        };
      }
      throw new Error(`Unexpected request: ${url}`);
    }),
}));

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

describe("useGetTimeTrackingDetails", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (
      window as unknown as {
        config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string };
      }
    ).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    };
  });

  it("should return time tracking details from API", async () => {
    const projectId = "project-1";

    const { result } = renderHook(() => useGetTimeTrackingDetails(projectId), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const data = result.current.data;
    expect(data).toBeDefined();
    expect(data?.timeLogs).toHaveLength(1);
    expect(data?.timeLogs[0].id).toBe("1");
    expect(data?.timeLogs[0].description).toBe("Working on case #123");
  });
});
