// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
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
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useGetChatHistory } from "@features/support/api/useGetChatHistory";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

const mockChatResponse = {
  chatHistory: [
    {
      chatId: "1",
      title: "Test chat",
      startedTime: "1 hour ago",
      messages: 3,
      kbArticles: 1,
      status: "Resolved",
    },
  ],
};

const mockAuthFetch = vi.fn().mockResolvedValue({
  ok: true,
  json: () => Promise.resolve(mockChatResponse),
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

describe("useGetChatHistory", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    mockLogger.debug.mockClear();
    mockLogger.error.mockClear();
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockChatResponse),
      status: 200,
    } as Response);
    (
      window as unknown as {
        config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string };
      }
    ).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    };
    vi.clearAllMocks();
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  it("should return loading state initially", async () => {
    const { result } = renderHook(() => useGetChatHistory("project-1"), {
      wrapper,
    });

    expect(result.current.isLoading).toBe(true);
  });

  it("should return data from API", async () => {
    const { result } = renderHook(() => useGetChatHistory("project-1"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeDefined();
    expect(result.current.data?.chatHistory).toBeDefined();
    expect(Array.isArray(result.current.data?.chatHistory)).toBe(true);
    expect(result.current.data?.chatHistory.length).toBeGreaterThan(0);
    expect(result.current.data?.chatHistory[0]).toMatchObject({
      chatId: expect.any(String),
      title: expect.any(String),
      startedTime: expect.any(String),
      messages: expect.any(Number),
      kbArticles: expect.any(Number),
      status: expect.any(String),
    });
  });

  it("should have correct query options", () => {
    renderHook(() => useGetChatHistory("project-1"), { wrapper });

    const query = queryClient.getQueryCache().findAll({
      queryKey: ["chat-history", "project-1"],
    })[0];

    expect((query?.options as { staleTime?: number }).staleTime).toBe(0);
  });

  it("should handle API error", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: false,
      statusText: "Internal Server Error",
      status: 500,
    } as Response);

    const { result } = renderHook(() => useGetChatHistory("project-1"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toContain(
      "Error fetching chat history: Internal Server Error",
    );
    expect(mockLogger.error).toHaveBeenCalled();
  });

  it("should not fetch if projectId is missing", () => {
    const { result } = renderHook(() => useGetChatHistory(""), {
      wrapper,
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.fetchStatus).toBe("idle");
  });
});
