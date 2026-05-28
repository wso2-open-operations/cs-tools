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
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useConversationRecommendationsSearch } from "@features/support/api/useConversationRecommendationsSearch";
import useGetAIChatHistory from "@features/support/api/useGetAIChatHistory";
import { useGetCaseAttachments } from "@features/support/api/useGetCaseAttachments";
import useGetCaseCommentsInfinite from "@features/support/api/useGetCaseCommentsInfinite";
import useGetCaseDetails from "@features/support/api/useGetCaseDetails";
import { useGetConversationMessages } from "@features/support/api/useGetConversationMessages";
import { useGetConversationStats } from "@features/support/api/useGetConversationStats";
import useGetConversationSummary from "@features/support/api/useGetConversationSummary";
import { useSearchConversations } from "@features/support/api/useSearchConversations";

const mockAuthFetch = vi.fn();
let mockIsSignedIn = true;
let mockIsAuthLoading = false;

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => mockAuthFetch,
}));
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isSignedIn: mockIsSignedIn,
    isLoading: mockIsAuthLoading,
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
  }),
}));
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn(), warn: vi.fn() }),
}));

const getWrapper = (queryClient: QueryClient) =>
  ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

describe("support API query hooks - batch 1", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsSignedIn = true;
    mockIsAuthLoading = false;
    vi.stubGlobal("config", {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    });
  });

  it("useConversationRecommendationsSearch should POST request body", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ recommendations: [] }),
    } as Response);

    const payload = { chatHistory: [{ timestamp: "2026-05-28T10:00:00Z", message: "m1" }] };
    const { result } = renderHook(
      () => useConversationRecommendationsSearch(payload as never, true),
      { wrapper: getWrapper(queryClient) },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/conversations/recommendations/search",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(payload),
      }),
    );
  });

  it("useGetAIChatHistory should map activities into comments", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          activities: [{ id: "a1", content: "hello", createdOn: "now", type: "COMMENT" }],
          totalRecords: 1,
          offset: 0,
          limit: 10,
        }),
    } as Response);

    const { result } = renderHook(() => useGetAIChatHistory("project-1", "case-1"), {
      wrapper: getWrapper(queryClient),
    });
    await waitFor(() => expect(result.current.comments.length).toBe(1));
    expect(result.current.comments[0]).toMatchObject({ id: "a1", content: "hello" });
  });

  it("useGetCaseAttachments should call encoded attachments URL", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ attachments: [], totalRecords: 0, offset: 0, limit: 10 }),
    } as Response);

    const { result } = renderHook(() => useGetCaseAttachments("case with spaces"), {
      wrapper: getWrapper(queryClient),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      expect.stringContaining("/cases/case%20with%20spaces/attachments?"),
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("useGetCaseCommentsInfinite should POST activities search request", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ activities: [], totalRecords: 0, offset: 0, limit: 10 }),
    } as Response);

    const { result } = renderHook(
      () => useGetCaseCommentsInfinite("project-1", "case-99"),
      { wrapper: getWrapper(queryClient) },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/cases/case-99/activities/search",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("useGetCaseDetails should GET case details endpoint", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ id: "case-1", project: { id: "project-1" } }),
    } as Response);

    const { result } = renderHook(() => useGetCaseDetails("project-1", "case-1"), {
      wrapper: getWrapper(queryClient),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith("https://api.test/cases/case-1", {
      method: "GET",
    });
  });

  it("useGetConversationMessages should include limit and offset params", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ messages: [], totalRecords: 0, offset: 0, limit: 25 }),
    } as Response);

    const { result } = renderHook(
      () => useGetConversationMessages("conv-1", { pageSize: 25 }),
      { wrapper: getWrapper(queryClient) },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/conversations/conv-1/messages?limit=25&offset=0",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("useGetConversationStats should append createdBy me filter", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({}),
      status: 200,
    } as Response);

    const { result } = renderHook(
      () => useGetConversationStats("project-1", { createdByMe: true }),
      { wrapper: getWrapper(queryClient) },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/projects/project-1/stats/conversations?createdBy=me",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("useGetConversationSummary should call summary endpoint", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ summary: "hello" }),
    } as Response);

    const { result } = renderHook(
      () => useGetConversationSummary("project-1", "conv-1"),
      { wrapper: getWrapper(queryClient) },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/projects/project-1/conversations/conv-1/summary",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("useSearchConversations should POST search body", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ conversations: [] }),
      status: 200,
    } as Response);
    const request = { pagination: { limit: 10, offset: 0 }, filters: {} };

    const { result } = renderHook(
      () => useSearchConversations("project-1", request as never),
      { wrapper: getWrapper(queryClient) },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/projects/project-1/conversations/search",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(request),
      }),
    );
  });
});
