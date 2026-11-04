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
import { ApiQueryKeys } from "@constants/apiConstants";
import { usePostAttachments } from "../usePostAttachments";

const mockAuthFetch = vi.fn();
const mockRefreshCaseComments = vi.hoisted(() => vi.fn());
vi.mock("@features/support/api/caseCommentsRefresh", () => ({
  refreshCaseComments: mockRefreshCaseComments,
}));

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => mockAuthFetch,
}));
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: true, isLoading: false }),
}));
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn(), warn: vi.fn() }),
}));

describe("usePostAttachments", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("config", {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    });
    mockAuthFetch.mockResolvedValue({ ok: true, status: 200 } as Response);
  });

  it("exports a hook function", () => {
    expect(typeof usePostAttachments).toBe("function");
  });

  it("invalidates attachments and refreshes only the newest comments on success", async () => {
    const queryClient = new QueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => usePostAttachments(), { wrapper });

    await result.current.mutateAsync({
      caseId: "case-1",
      body: { name: "a.txt", type: "text/plain", content: "abc" },
    });

    await waitFor(() => expect(mockRefreshCaseComments).toHaveBeenCalledTimes(1));
    expect(mockRefreshCaseComments.mock.calls[0][2]).toBe("case-1");
    const keys = invalidate.mock.calls.map((c) => c[0]?.queryKey);
    expect(keys).toEqual([[ApiQueryKeys.CASE_ATTACHMENTS, "case-1"]]);
  });

  it("does not invalidate when the upload fails", async () => {
    const queryClient = new QueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    mockAuthFetch.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: "Server Error",
      text: () => Promise.resolve(""),
    } as Response);
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => usePostAttachments(), { wrapper });

    await expect(
      result.current.mutateAsync({
        caseId: "case-1",
        body: { name: "a.txt", type: "text/plain", content: "abc" },
      }),
    ).rejects.toThrow();
    expect(invalidate).not.toHaveBeenCalled();
    expect(mockRefreshCaseComments).not.toHaveBeenCalled();
  });
});
