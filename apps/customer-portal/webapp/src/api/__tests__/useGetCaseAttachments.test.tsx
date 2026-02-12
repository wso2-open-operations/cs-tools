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
import useGetCaseAttachments from "@api/useGetCaseAttachments";
import { mockCaseAttachments } from "@models/mockData";

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

describe("useGetCaseAttachments", () => {
  beforeEach(() => {
    queryClient.clear();
    vi.clearAllMocks();
    mockUseMockConfig.mockReturnValue({ isMockEnabled: true });
  });

  it("should return mock attachments when isMockEnabled is true", async () => {
    const { result } = renderHook(() => useGetCaseAttachments("case-001"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeDefined();
    expect(result.current.data?.attachments).toBeDefined();
    expect(result.current.data?.attachments.length).toBe(
      mockCaseAttachments.length,
    );
    expect(result.current.data?.totalRecords).toBe(mockCaseAttachments.length);
    expect(result.current.data?.offset).toBe(0);
    expect(result.current.data?.limit).toBe(50);
  });

  it("should not fetch when caseId is missing", () => {
    const { result } = renderHook(() => useGetCaseAttachments(""), { wrapper });
    expect(result.current.isFetching).toBe(false);
  });

  it("should use correct query key", () => {
    renderHook(() => useGetCaseAttachments("case-001"), { wrapper });
    const query = queryClient.getQueryCache().findAll({
      queryKey: ["case-attachments", "case-001", 50, 0, true],
    })[0];
    expect(query).toBeDefined();
  });

  it("should respect limit and offset options when mock is enabled", async () => {
    const { result } = renderHook(
      () => useGetCaseAttachments("case-001", { limit: 2, offset: 1 }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.limit).toBe(2);
    expect(result.current.data?.offset).toBe(1);
    expect(result.current.data?.attachments.length).toBeLessThanOrEqual(2);
    expect(result.current.data?.totalRecords).toBe(mockCaseAttachments.length);
  });
});
