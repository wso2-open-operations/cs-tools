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

import { renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import type { ReactNode } from "react";
import { usePostConversationMessages } from "@api/usePostConversationMessages";

const mockLogger = { debug: vi.fn() };
vi.mock("@hooks/useLogger", () => ({ useLogger: () => mockLogger }));

const mockAuthFetch = vi.fn();

const mockUseAsgardeo = vi.fn(() => ({
  isSignedIn: true,
  isLoading: false,
  getIdToken: vi.fn().mockResolvedValue("mock-token"),
}));
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => mockUseAsgardeo(),
}));

describe("usePostConversationMessages", () => {
  let queryClient: QueryClient;
  const originalConfig = window.config;

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  const requestParams = {
    projectId: "proj-1",
    conversationId: "conv-1",
    message: "Follow-up question",
    envProducts: { QA: ["WSO2 IS 6.1"], Production: ["WSO2 AM 4.2"] },
    region: "EU",
    tier: "Tier 1",
  };

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    });
    vi.clearAllMocks();
    window.config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.example.com",
    } as typeof window.config;
    mockAuthFetch.mockResolvedValue(
      new Response(
        JSON.stringify({ message: "AI response", conversationId: "conv-1" }),
        {
          status: 200,
        },
      ),
    );
  });

  afterEach(() => {
    window.config = originalConfig;
  });

  it("should POST to messages endpoint and return response", async () => {
    const { result } = renderHook(() => usePostConversationMessages(), {
      wrapper,
    });

    const response = await result.current.mutateAsync(requestParams);

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.example.com/projects/proj-1/conversations/conv-1/messages",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          message: requestParams.message,
          envProducts: requestParams.envProducts,
          region: requestParams.region,
          tier: requestParams.tier,
        }),
      }),
    );
    expect(response).toEqual({
      message: "AI response",
      conversationId: "conv-1",
    });
  });

  it("should throw when base URL is not configured", async () => {
    window.config = {} as typeof window.config;
    const { result } = renderHook(() => usePostConversationMessages(), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestParams)).rejects.toThrow(
      "CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured",
    );
  });

  it("should throw when user is not signed in", async () => {
    mockUseAsgardeo.mockReturnValueOnce({
      isSignedIn: false,
      isLoading: false,
      getIdToken: vi.fn(),
    });
    const { result } = renderHook(() => usePostConversationMessages(), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestParams)).rejects.toThrow(
      "User must be signed in to send messages",
    );
  });

  it("should throw when auth is loading", async () => {
    mockUseAsgardeo.mockReturnValueOnce({
      isSignedIn: false,
      isLoading: true,
      getIdToken: vi.fn(),
    });
    const { result } = renderHook(() => usePostConversationMessages(), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestParams)).rejects.toThrow(
      "User must be signed in to send messages",
    );
  });

  it("should throw when API returns error response", async () => {
    mockAuthFetch.mockResolvedValueOnce(
      new Response("Internal Server Error", {
        status: 500,
        statusText: "Internal Server Error",
      }),
    );
    const { result } = renderHook(() => usePostConversationMessages(), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestParams)).rejects.toThrow(
      "Conversation messages API error: 500 Internal Server Error",
    );
  });
});
