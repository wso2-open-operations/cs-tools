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
import { useGetProjectStat } from "@/api/useGetProjectStat";
import { mockProjects } from "@/models/mockData";
import type { ReactNode } from "react";

vi.mock("@/constants/apiConstants", async (importOriginal) => {
  const actual = (await importOriginal()) as any;
  return {
    ...actual,
    API_MOCK_DELAY: 0,
  };
});

// Mock @asgardeo/react
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: vi.fn(),
    isSignedIn: true,
    isLoading: false,
  }),
}));

// Mock MockConfigProvider
vi.mock("@/providers/MockConfigProvider", () => ({
  useMockConfig: () => ({
    isMockEnabled: true,
  }),
}));

// Mock useLogger
vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
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

describe("useGetProjectStat", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should return project stats for a valid project ID", async () => {
    const validProjectId = mockProjects[0].id;

    const { result } = renderHook(() => useGetProjectStat(validProjectId), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const response = result.current.data;
    expect(response).toBeDefined();
    expect(response?.projectStats).toBeDefined();
    expect(response?.recentActivity).toBeDefined();
  });

  it("should error for an invalid project ID", async () => {
    const invalidProjectId = "invalid-id";

    const { result } = renderHook(() => useGetProjectStat(invalidProjectId), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(result.current.error).toBeDefined();
    expect(result.current.error?.message).toContain(
      "Project stats not found for ID: invalid-id",
    );
  });
});
