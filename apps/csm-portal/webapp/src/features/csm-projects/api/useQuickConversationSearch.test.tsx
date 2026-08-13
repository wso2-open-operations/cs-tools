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
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useQuickCaseSearch.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import {
  classifyQuickConversationQuery,
  useQuickConversationSearch,
} from "@features/csm-projects/api/useQuickConversationSearch";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useQuickConversationSearch", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ conversations: [] });
  });

  it("sends free text as searchQuery for a non-number-shaped query", async () => {
    const { result } = renderHook(
      () => useQuickConversationSearch("billing question"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/conversations/search",
      expect.objectContaining({
        filters: { searchQuery: "billing question" },
      }),
    );
    // No `projectIds` at all — this is a global, unscoped search.
    const body = postMock.mock.calls[0][1];
    expect(body.filters.projectIds).toBeUndefined();
  });

  it("does not fire a search until the query reaches the minimum length", () => {
    renderHook(() => useQuickConversationSearch("a"), { wrapper });
    expect(postMock).not.toHaveBeenCalled();
  });

  it("routes a CHAT number to an exact-match number filter, not searchQuery", async () => {
    const { result } = renderHook(
      () => useQuickConversationSearch("CHAT0000012345"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const body = postMock.mock.calls[0][1];
    expect(body.filters).toEqual({ number: "CHAT0000012345" });
  });

  it("falls back to free-text search when forceFreeText is set, even for a matching query", async () => {
    const { result } = renderHook(
      () =>
        useQuickConversationSearch("CHAT0000012345", { forceFreeText: true }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/conversations/search",
      expect.objectContaining({
        filters: { searchQuery: "CHAT0000012345" },
      }),
    );
  });

  it("maps a conversation hit's initiator name/email and initial message", async () => {
    postMock.mockResolvedValue({
      conversations: [
        {
          id: "conv-1",
          number: "CHAT0000012345",
          initialMessage: "Need help resetting my password",
          messageCount: 3,
          project: null,
          case: null,
          state: "ACTIVE",
          createdOn: "2026-08-01T00:00:00Z",
          createdBy: { id: null, email: "jane@example.com", name: "Jane Doe" },
        },
      ],
    });

    const { result } = renderHook(
      () => useQuickConversationSearch("CHAT0000012345"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toEqual([
      {
        id: "conv-1",
        number: "CHAT0000012345",
        initiatorName: "Jane Doe",
        initialMessage: "Need help resetting my password",
      },
    ]);
  });

  describe("classifyQuickConversationQuery", () => {
    it("classifies a CHAT conversation number", () => {
      expect(classifyQuickConversationQuery("CHAT0000012345")).toBe("number");
      // The two real examples found in this codebase disagree on digit
      // padding (10 digits vs. 9) — both must classify as a number.
      expect(classifyQuickConversationQuery("CHAT000002755")).toBe("number");
    });

    it("classifies free text as text", () => {
      expect(classifyQuickConversationQuery("billing question")).toBe("text");
      // Lowercase "chat" prefix fails the strict, case-sensitive shape.
      expect(classifyQuickConversationQuery("chat0000012345")).toBe("text");
      // No digits after the prefix at all.
      expect(classifyQuickConversationQuery("CHAT")).toBe("text");
      // A different prefix entirely.
      expect(classifyQuickConversationQuery("CS0441174")).toBe("text");
    });
  });
});
