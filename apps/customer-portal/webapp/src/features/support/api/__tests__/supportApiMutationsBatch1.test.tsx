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

import { renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDeleteAttachment } from "@features/support/api/useDeleteAttachment";
import { usePatchCallRequest } from "@features/support/api/usePatchCallRequest";
import { usePatchCase } from "@features/support/api/usePatchCase";
import { usePatchCaseAttachment } from "@features/support/api/usePatchCaseAttachment";
import { usePostAttachments } from "@features/support/api/usePostAttachments";
import { usePostCaseClassifications } from "@features/support/api/usePostCaseClassifications";
import { usePostComment } from "@features/support/api/usePostComment";
import { usePostConversationMessages } from "@features/support/api/usePostConversationMessages";
import { usePostConversations } from "@features/support/api/usePostConversations";
import { CommentType } from "@features/support/constants/supportConstants";

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

describe("support API mutation hooks - batch 1", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsSignedIn = true;
    mockIsAuthLoading = false;
    vi.stubGlobal("config", {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    });
    mockAuthFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ id: "ok" }),
      text: () => Promise.resolve(""),
    } as Response);
  });

  it("useDeleteAttachment should call DELETE attachment endpoint", async () => {
    const queryClient = new QueryClient();
    const { result } = renderHook(() => useDeleteAttachment(), {
      wrapper: getWrapper(queryClient),
    });

    await result.current.mutateAsync({ attachmentId: "att-1", caseId: "case-1" });
    expect(mockAuthFetch).toHaveBeenCalledWith("https://api.test/attachments/att-1", {
      method: "DELETE",
    });
  });

  it("usePatchCallRequest should send PATCH payload", async () => {
    const queryClient = new QueryClient();
    const { result } = renderHook(
      () => usePatchCallRequest("project-1", "case-1"),
      { wrapper: getWrapper(queryClient) },
    );

    await result.current.mutateAsync({
      callRequestId: "call-1",
      stateKey: 3,
      reason: "new reason",
    });
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/cases/case-1/call-requests/call-1",
      expect.objectContaining({
        method: "PATCH",
        body: JSON.stringify({ stateKey: 3, reason: "new reason" }),
      }),
    );
  });

  it("usePatchCase should PATCH case state", async () => {
    const queryClient = new QueryClient();
    const { result } = renderHook(() => usePatchCase("project-1", "case-1"), {
      wrapper: getWrapper(queryClient),
    });

    await result.current.mutateAsync({ stateKey: 2 });
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/cases/case-1",
      expect.objectContaining({ method: "PATCH" }),
    );
  });

  it("usePatchCaseAttachment should PATCH attachment details", async () => {
    const queryClient = new QueryClient();
    const { result } = renderHook(() => usePatchCaseAttachment(), {
      wrapper: getWrapper(queryClient),
    });

    await result.current.mutateAsync({
      caseId: "case-1",
      attachmentId: "att-1",
      body: { name: "new.txt" },
    });
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/cases/case-1/attachments/att-1",
      expect.objectContaining({ method: "PATCH" }),
    );
  });

  it("usePostAttachments should POST upload payload", async () => {
    const queryClient = new QueryClient();
    const { result } = renderHook(() => usePostAttachments(), {
      wrapper: getWrapper(queryClient),
    });

    await result.current.mutateAsync({
      caseId: "case-1",
      body: { name: "a.txt", type: "text/plain", content: "abc" },
    });
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/cases/case-1/attachments",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("usePostCaseClassifications should POST classification request", async () => {
    const queryClient = new QueryClient();
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ summary: "ok" }),
    } as Response);
    const { result } = renderHook(() => usePostCaseClassifications(), {
      wrapper: getWrapper(queryClient),
    });

    await result.current.mutateAsync({
      chatHistory: "h",
      projectTypeId: "project-type",
      envProducts: {},
      region: "us",
      tier: "prod",
    });
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/cases/classify",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("usePostComment should reject payloads larger than 10MB", async () => {
    const queryClient = new QueryClient();
    const { result } = renderHook(() => usePostComment(), {
      wrapper: getWrapper(queryClient),
    });

    await expect(
      result.current.mutateAsync({
        caseId: "case-1",
        body: {
          content: "x".repeat(10 * 1024 * 1024 + 200),
          type: CommentType.COMMENT,
        },
      }),
    ).rejects.toThrow("exceeds the 10 MB limit");
  });

  it("usePostConversationMessages should POST follow-up message", async () => {
    const queryClient = new QueryClient();
    const { result } = renderHook(() => usePostConversationMessages(), {
      wrapper: getWrapper(queryClient),
    });

    await result.current.mutateAsync({
      projectId: "project-1",
      conversationId: "conv-1",
      message: "hello",
      envProducts: {},
      region: "us",
      tier: "prod",
    });
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/projects/project-1/conversations/conv-1/messages",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("usePostConversations should POST initial conversation message", async () => {
    const queryClient = new QueryClient();
    const { result } = renderHook(() => usePostConversations(), {
      wrapper: getWrapper(queryClient),
    });

    await result.current.mutateAsync({
      projectId: "project-1",
      message: "hello",
      envProducts: {},
      region: "us",
      tier: "prod",
    });
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/projects/project-1/conversations",
      expect.objectContaining({ method: "POST" }),
    );
  });
});
