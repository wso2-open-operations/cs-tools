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
import { describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useGetCaseCreationDetails } from "@/api/useGetCaseCreationDetails";
import { getCaseCreationMetadata } from "@/models/mockFunctions";
import type { ReactNode } from "react";

// Mock dependencies
vi.mock("@/models/mockFunctions", () => ({
  getCaseCreationMetadata: vi.fn(),
}));

vi.mock("@/constants/apiConstants", async (importOriginal) => {
  const actual = (await importOriginal()) as any;
  return {
    ...actual,
    API_MOCK_DELAY: 0,
  };
});

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

describe("useGetCaseCreationDetails", () => {
  const mockMetadata = {
    projects: ["Project A"],
    products: ["Product A"],
    deploymentTypes: ["Cloud"],
    issueTypes: ["Bug"],
    severityLevels: [
      { id: "S1", label: "Critical", description: "Critical issue" },
    ],
    conversationSummary: {
      messagesExchanged: 5,
      troubleshootingAttempts: "None",
      kbArticlesReviewed: "None",
    },
  };

  it("should return case creation metadata successfully", async () => {
    vi.mocked(getCaseCreationMetadata).mockReturnValue(mockMetadata);

    const { result } = renderHook(() => useGetCaseCreationDetails(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toEqual(mockMetadata);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.isError).toBe(false);
  });

  it("should handle errors when fetching metadata fails", async () => {
    const error = new Error("Failed to fetch");
    vi.mocked(getCaseCreationMetadata).mockImplementation(() => {
      throw error;
    });

    const { result } = renderHook(() => useGetCaseCreationDetails(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.isLoading).toBe(false);
  });
});
