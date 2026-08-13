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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { useSearchConversations } from "@features/csm-projects/api/useSearchConversations";
import type { BackendApi } from "@api/backend/client";
import type { BeSearchConversationsResponse } from "@api/backend/types";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: (): Partial<BackendApi> => ({ post: postMock }),
}));

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useSearchConversations", () => {
  it("is disabled until a projectId is provided", () => {
    const { result } = renderHook(
      () => useSearchConversations(undefined, { page: 0, rowsPerPage: 20 }),
      { wrapper },
    );

    expect(result.current.fetchStatus).toBe("idle");
    expect(postMock).not.toHaveBeenCalled();
  });

  it("searches by projectId, sorted most-recently-active first, and returns conversations + total", async () => {
    const response: BeSearchConversationsResponse = {
      conversations: [
        {
          id: "conv-1",
          number: "CONV0000001",
          initialMessage: "Hi, need help",
          messageCount: 4,
          project: { id: "proj-1", name: "Acme" },
          case: null,
          state: "ACTIVE",
          createdOn: "2026-07-01T10:00:00Z",
          createdBy: { id: null, email: "jane@example.com", name: "" },
        },
      ],
      total: 1,
      limit: 20,
      offset: 0,
    };
    postMock.mockResolvedValue(response);

    const { result } = renderHook(
      () => useSearchConversations("proj-1", { page: 0, rowsPerPage: 20 }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith("/conversations/search", {
      filters: { projectIds: ["proj-1"] },
      sortBy: { field: "updatedOn", order: "desc" },
      pagination: { limit: 20, offset: 0 },
    });
    expect(result.current.data?.total).toBe(1);
    expect(result.current.data?.conversations[0].id).toBe("conv-1");
  });

  it("caps rowsPerPage at BE_MAX_PAGE_LIMIT and computes offset from the requested page", async () => {
    postMock.mockResolvedValue({ conversations: [], total: 0, limit: 50, offset: 100 });

    const { result } = renderHook(
      () => useSearchConversations("proj-1", { page: 2, rowsPerPage: 200 }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/conversations/search",
      expect.objectContaining({ pagination: { limit: 50, offset: 100 } }),
    );
  });

  it("includes states/searchQuery/createdByMe in the request only when set", async () => {
    postMock.mockResolvedValue({ conversations: [], total: 0, limit: 20, offset: 0 });

    const { result } = renderHook(
      () =>
        useSearchConversations(
          "proj-1",
          { page: 0, rowsPerPage: 20 },
          { states: ["ACTIVE", "CONVERTED"], searchQuery: "  billing  ", createdByMe: true },
        ),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/conversations/search",
      expect.objectContaining({
        filters: {
          projectIds: ["proj-1"],
          states: ["ACTIVE", "CONVERTED"],
          searchQuery: "billing",
          createdByMe: true,
        },
      }),
    );
  });

  it("routes a CHAT-number-shaped search to filters.number, not searchQuery", async () => {
    postMock.mockResolvedValue({ conversations: [], total: 0, limit: 20, offset: 0 });

    const { result } = renderHook(
      () =>
        useSearchConversations(
          "proj-1",
          { page: 0, rowsPerPage: 20 },
          { searchQuery: "CHAT0000012345" },
        ),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/conversations/search",
      expect.objectContaining({
        filters: { projectIds: ["proj-1"], number: "CHAT0000012345" },
      }),
    );
  });

  it("omits empty filter fields from the request payload", async () => {
    postMock.mockResolvedValue({ conversations: [], total: 0, limit: 20, offset: 0 });

    const { result } = renderHook(
      () =>
        useSearchConversations(
          "proj-1",
          { page: 0, rowsPerPage: 20 },
          { states: [], searchQuery: "   ", createdByMe: false, number: "  ", createdBy: [] },
        ),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/conversations/search",
      expect.objectContaining({
        filters: { projectIds: ["proj-1"] },
      }),
    );
  });

  it("includes the explicit number and createdBy filters when set", async () => {
    postMock.mockResolvedValue({ conversations: [], total: 0, limit: 20, offset: 0 });

    const { result } = renderHook(
      () =>
        useSearchConversations(
          "proj-1",
          { page: 0, rowsPerPage: 20 },
          { number: "CHAT0000012345", createdBy: ["jane.doe@example.com"] },
        ),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/conversations/search",
      expect.objectContaining({
        filters: {
          projectIds: ["proj-1"],
          number: "CHAT0000012345",
          createdBy: ["jane.doe@example.com"],
        },
      }),
    );
  });

  it("prefers the explicit number filter over a CHAT-number-shaped search box value", async () => {
    postMock.mockResolvedValue({ conversations: [], total: 0, limit: 20, offset: 0 });

    const { result } = renderHook(
      () =>
        useSearchConversations(
          "proj-1",
          { page: 0, rowsPerPage: 20 },
          { searchQuery: "CHAT0000099999", number: "CHAT0000012345" },
        ),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/conversations/search",
      expect.objectContaining({
        filters: { projectIds: ["proj-1"], number: "CHAT0000012345" },
      }),
    );
  });
});
