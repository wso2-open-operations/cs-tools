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
import { type ReactNode } from "react";
import useGetConversationSummary from "@api/useGetConversationSummary";
import * as apiRequest from "../apiRequest";

vi.mock("@api/apiRequest", () => ({
  apiRequest: vi.fn(),
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

describe("useGetConversationSummary", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    const { useAuthApiClient } = require("@api/useAuthApiClient");
    useAuthApiClient.mockReturnValue(mockAuthFetch);
    (window as any).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "http://localhost:9090",
    };
  });

  it("should fetch conversation summary successfully", async () => {
    const mockSummary = {
      accountId: "test-account",
      conversationId: "test-conversation",
      messagesExchanged: 5,
      troubleshootingAttempts: 3,
      kbArticlesReviewed: 2,
    };

    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: async () => mockSummary,
    });

    const { result } = renderHook(
      () => useGetConversationSummary("project-1", "conversation-1"),
      { wrapper: createWrapper() },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toEqual(mockSummary);
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "http://localhost:9090/projects/project-1/conversations/conversation-1/summary",
      { method: "GET" },
    );
  });

  it("should not fetch when projectId is empty", () => {
    const { result } = renderHook(
      () => useGetConversationSummary("", "conversation-1"),
      { wrapper: createWrapper() },
    );

    expect(result.current.isFetching).toBe(false);
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("should not fetch when conversationId is empty", () => {
    const { result } = renderHook(
      () => useGetConversationSummary("project-1", ""),
      { wrapper: createWrapper() },
    );

    expect(result.current.isFetching).toBe(false);
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("should handle error when API request fails", async () => {
    mockAuthFetch.mockResolvedValue({
      ok: false,
      statusText: "Not Found",
    });

    const { result } = renderHook(
      () => useGetConversationSummary("project-1", "conversation-1"),
      { wrapper: createWrapper() },
    );

    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toContain("Error fetching conversation summary");
  });
});
