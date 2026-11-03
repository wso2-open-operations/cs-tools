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

import { renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { usePostComment } from "../usePostComment";

const mockAuthFetch = vi.fn();
const mockRefreshCaseComments = vi.hoisted(() => vi.fn());

vi.mock("@/hooks/useAuthApiClient", () => ({ useAuthApiClient: () => mockAuthFetch }));
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: true, isLoading: false }),
}));
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn(), warn: vi.fn() }),
}));
vi.mock("@features/support/api/caseCommentsRefresh", () => ({
  refreshCaseComments: mockRefreshCaseComments,
}));

function setup() {
  const queryClient = new QueryClient();
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  const { result } = renderHook(() => usePostComment(), { wrapper });
  return { result, invalidate, queryClient };
}

describe("usePostComment", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("config", { CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test" });
    mockAuthFetch.mockResolvedValue({ ok: true, status: 200 } as Response);
  });

  it("exports a hook function", () => {
    expect(typeof usePostComment).toBe("function");
  });

  it("refreshes only the newest comments on success, without invalidating anything", async () => {
    const { result, invalidate, queryClient } = setup();

    await result.current.mutateAsync({
      caseId: "case-1",
      body: { content: "hello", type: "comments" },
    });

    expect(mockRefreshCaseComments).toHaveBeenCalledTimes(1);
    expect(mockRefreshCaseComments.mock.calls[0][0]).toBe(queryClient);
    expect(mockRefreshCaseComments.mock.calls[0][2]).toBe("case-1");
    expect(invalidate).not.toHaveBeenCalled();
  });

  it("does not refresh when the post fails", async () => {
    const { result, invalidate } = setup();
    mockAuthFetch.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: "Server Error",
      text: () => Promise.resolve(""),
    } as Response);

    await expect(
      result.current.mutateAsync({ caseId: "case-1", body: { content: "hello", type: "comments" } }),
    ).rejects.toThrow();

    expect(mockRefreshCaseComments).not.toHaveBeenCalled();
    expect(invalidate).not.toHaveBeenCalled();
  });
});
