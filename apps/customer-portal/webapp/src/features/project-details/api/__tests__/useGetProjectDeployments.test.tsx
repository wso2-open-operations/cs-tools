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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { usePostProjectDeploymentsSearchAll } from "@api/usePostProjectDeploymentsSearch";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";

const mockProjectDeploymentsResponse = {
  deployments: [
    {
      id: "1",
      name: "Prod",
      createdOn: "2026-02-06",
      updatedOn: "2026-02-06",
      description: null,
      url: null,
      project: { id: "p1", label: "My Project" },
      type: { id: "6", label: "Primary Production" },
    },
  ],
  totalRecords: 1,
  offset: 0,
  limit: 10,
};

const mockAuthFetch = vi.fn().mockResolvedValue({
  ok: true,
  json: () => Promise.resolve(mockProjectDeploymentsResponse),
  status: 200,
} as Response);

const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
};
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
    isSignedIn: true,
    isLoading: false,
  }),
}));

vi.mock("@api/useAuthApiClient", () => ({
  useAuthApiClient: vi.fn(),
}));

describe("usePostProjectDeploymentsSearchAll", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockProjectDeploymentsResponse),
      status: 200,
    } as Response);
    vi.mocked(useAuthApiClient).mockReturnValue(mockAuthFetch);
    (
      window as unknown as {
        config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string };
      }
    ).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    };
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  it("should return deployments from API", async () => {
    const { result } = renderHook(
      () => usePostProjectDeploymentsSearchAll("project-123"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/projects/project-123/deployments/search",
      expect.objectContaining({ method: "POST" }),
    );
    expect(result.current.data).toEqual(
      mockProjectDeploymentsResponse.deployments,
    );
    expect(result.current.data?.[0].type.label).toBe("Primary Production");
  });

  it("should be disabled when projectId is empty", () => {
    const { result } = renderHook(
      () => usePostProjectDeploymentsSearchAll(""),
      {
        wrapper,
      },
    );

    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toBeUndefined();
  });
});
