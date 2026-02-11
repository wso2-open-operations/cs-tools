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
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import useGetCasesFilters from "@api/useGetCasesFilters";
import { mockCaseMetadata } from "@models/mockData";

// Mock providers and hooks
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
    isSignedIn: true,
    isLoading: false,
  }),
}));

const mockUseMockConfig = vi.fn().mockReturnValue({
  isMockEnabled: true,
});

vi.mock("@/providers/MockConfigProvider", () => ({
  useMockConfig: () => mockUseMockConfig(),
}));

vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    error: vi.fn(),
  }),
}));

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
});

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
);

describe("useGetCasesFilters", () => {
  beforeEach(() => {
    queryClient.clear();
    vi.clearAllMocks();
    vi.stubGlobal("config", {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.example.com",
    });
  });

  it("should return mock data when isMockEnabled is true", async () => {
    const { result } = renderHook(() => useGetCasesFilters("123"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toEqual(mockCaseMetadata);
  });

  it("should fetch data from API when isMockEnabled is false", async () => {
    mockUseMockConfig.mockReturnValue({ isMockEnabled: false });
    const mockResponse = {
      statuses: [{ id: "1", label: "Open" }],
      severities: [{ id: "2", label: "High" }],
      issueTypes: [{ id: "3", label: "Incident" }],
    };

    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue(mockResponse),
    });
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => useGetCasesFilters("123"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(mockResponse);
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/projects/123/cases/filters"),
      expect.any(Object),
    );
  });

  it("should fetch and return project case filters", async () => {
    mockUseMockConfig.mockReturnValue({ isMockEnabled: false });

    const mockFetch = vi.fn().mockResolvedValue({
      ok: false,
      statusText: "Internal Server Error",
    });
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => useGetCasesFilters("123"), { wrapper });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toContain("Internal Server Error");
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });
});
