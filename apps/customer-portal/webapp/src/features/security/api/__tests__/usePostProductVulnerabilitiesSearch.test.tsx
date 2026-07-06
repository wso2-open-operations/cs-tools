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
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import type { ReactNode } from "react";
import { usePostProductVulnerabilitiesSearch } from "@features/security/api/usePostProductVulnerabilitiesSearch";

const mockLogger = { debug: vi.fn(), error: vi.fn() };
vi.mock("@hooks/useLogger", () => ({ useLogger: () => mockLogger }));

let mockIsSignedIn = true;
let mockIsAuthLoading = false;
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: mockIsSignedIn, isLoading: mockIsAuthLoading }),
}));

const mockAuthFetch = vi.fn();
vi.mock("@/hooks/useAuthApiClient", () => ({ useAuthApiClient: () => mockAuthFetch }));

describe("usePostProductVulnerabilitiesSearch", () => {
  let queryClient: QueryClient;
  const originalConfig = window.config;
  const request = { filters: { severityId: 1 }, pagination: { offset: 0, limit: 10 } };

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  beforeEach(() => {
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockIsSignedIn = true;
    mockIsAuthLoading = false;
    vi.clearAllMocks();
    vi.stubGlobal("config", { CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test" });
  });

  afterEach(() => {
    window.config = originalConfig;
    vi.unstubAllGlobals();
  });

  it("posts search request and returns response payload", async () => {
    const payload = { productVulnerabilities: [], totalRecords: 0, offset: 0, limit: 10 };
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve(payload),
    } as Response);

    const { result } = renderHook(
      () => usePostProductVulnerabilitiesSearch(request),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(payload);
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/products/vulnerabilities/search",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(request),
      }),
    );
  });

  it("parses api error response text", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: false,
      status: 400,
      statusText: "Bad Request",
      text: () => Promise.resolve('{"message":"Invalid filter payload"}'),
    } as Response);

    const { result } = renderHook(
      () => usePostProductVulnerabilitiesSearch(request),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe("Invalid filter payload");
  });
});
