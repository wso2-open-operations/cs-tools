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
import { beforeEach, describe, expect, it, vi } from "vitest";
import useSearchProjectCaseTimeCards from "@features/usage-metrics/api/useSearchProjectCaseTimeCards";

const authFetchMock = vi.fn();

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isSignedIn: true,
    isLoading: false,
  }),
}));

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => authFetchMock,
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    error: vi.fn(),
  }),
}));

function createWrapper() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
}

describe("useSearchProjectCaseTimeCards", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (
      window as unknown as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }
    ).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    };
    authFetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        offset: 0,
        limit: 10,
        totalRecords: 1,
        caseTimeCards: [],
      }),
    });
  });

  it("calls case time-cards endpoint with pagination payload", async () => {
    const { result } = renderHook(
      () =>
        useSearchProjectCaseTimeCards({
          projectId: "p2",
          startDate: "2026-02-01",
          endDate: "2026-02-28",
          states: ["SUBMITTED"],
        }),
      { wrapper: createWrapper() },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(authFetchMock).toHaveBeenCalledWith(
      "https://api.test/projects/p2/cases/time-cards/search",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          filters: {
            startDate: "2026-02-01",
            endDate: "2026-02-28",
            states: ["SUBMITTED"],
          },
          pagination: { limit: 10, offset: 0 },
        }),
      }),
    );
  });
});

