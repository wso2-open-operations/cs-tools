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
import { describe, it, expect, vi, beforeEach } from "vitest";
import useGetCaseDetails from "@api/useGetCaseDetails";
import { mockCaseDetails } from "@models/mockData";

vi.mock("@constants/apiConstants", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@constants/apiConstants")>();
  return {
    ...actual,
    API_MOCK_DELAY: 0,
  };
});

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
    isSignedIn: true,
    isLoading: false,
  }),
}));

const mockUseMockConfig = vi.fn().mockReturnValue({ isMockEnabled: true });

vi.mock("@providers/MockConfigProvider", () => ({
  useMockConfig: () => mockUseMockConfig(),
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn() }),
}));

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
);

describe("useGetCaseDetails", () => {
  beforeEach(() => {
    queryClient.clear();
    vi.clearAllMocks();
    mockUseMockConfig.mockReturnValue({ isMockEnabled: true });
  });

  it("should return mock case details when isMockEnabled is true", async () => {
    const { result } = renderHook(
      () => useGetCaseDetails("project-1", "case-001"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeDefined();
    expect(result.current.data?.id).toBe("case-001");
    expect(result.current.data?.number).toBe(mockCaseDetails.number);
    expect(result.current.data?.title).toBe(mockCaseDetails.title);
  });

  it("should not fetch when projectId or caseId is missing", () => {
    const { result: r1 } = renderHook(() => useGetCaseDetails("", "case-001"), {
      wrapper,
    });
    expect(r1.current.isFetching).toBe(false);

    const { result: r2 } = renderHook(
      () => useGetCaseDetails("project-1", ""),
      { wrapper },
    );
    expect(r2.current.isFetching).toBe(false);
  });

  it("should use correct query key", () => {
    renderHook(() => useGetCaseDetails("project-1", "case-001"), { wrapper });
    const query = queryClient.getQueryCache().findAll({
      queryKey: ["case-details", "project-1", "case-001", true],
    })[0];
    expect(query).toBeDefined();
  });
});
