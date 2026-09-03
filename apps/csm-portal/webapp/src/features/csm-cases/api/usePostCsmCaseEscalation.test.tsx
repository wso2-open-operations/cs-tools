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
import { ApiQueryKeys } from "@constants/apiConstants";

const postMock = vi.fn();
const getMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock, get: getMock }),
}));

import { usePostCsmCaseEscalation } from "@features/csm-cases/api/usePostCsmCaseEscalation";

function makeWrapper(queryClient: QueryClient) {
  return function wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    );
  };
}

describe("usePostCsmCaseEscalation", () => {
  beforeEach(() => {
    postMock.mockReset();
    getMock.mockReset();
  });

  it("POSTs the action/reason body to /cases/{id}/escalations", async () => {
    postMock.mockResolvedValue({
      id: "esc-1",
      caseId: "case-1",
      currentLevel: "1",
      previousLevel: "0",
      createdBy: "jane.doe@example.com",
      createdOn: "2026-08-01T00:00:00Z",
      updatedOn: "2026-08-01T00:00:00Z",
      reason: "Customer needs urgent attention.",
    });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const { result } = renderHook(() => usePostCsmCaseEscalation("case-1"), {
      wrapper: makeWrapper(queryClient),
    });

    result.current.mutate({
      action: "ESCALATE",
      reason: "Customer needs urgent attention.",
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith("/cases/case-1/escalations", {
      action: "ESCALATE",
      reason: "Customer needs urgent attention.",
    });
  });

  it("invalidates the case detail, escalation history, activities, and list queries on success", async () => {
    postMock.mockResolvedValue({
      id: "esc-2",
      caseId: "case-1",
      currentLevel: "0",
      previousLevel: "1",
      createdBy: "jane.doe@example.com",
      createdOn: "2026-08-01T00:00:00Z",
      updatedOn: "2026-08-01T00:00:00Z",
      reason: null,
    });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => usePostCsmCaseEscalation("case-1"), {
      wrapper: makeWrapper(queryClient),
    });

    result.current.mutate({ action: "DEESCALATE" });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const invalidatedKeys = invalidateSpy.mock.calls.map(
      (call) => call[0]?.queryKey,
    );
    expect(invalidatedKeys).toContainEqual([
      ApiQueryKeys.CSM_CASE_DETAIL,
      "case-1",
    ]);
    expect(invalidatedKeys).toContainEqual([
      ApiQueryKeys.CSM_CASE_ESCALATIONS,
      "case-1",
    ]);
    expect(invalidatedKeys).toContainEqual([
      ApiQueryKeys.CSM_CASE_ACTIVITIES,
      "case-1",
    ]);
    expect(invalidatedKeys).toContainEqual([ApiQueryKeys.CSM_CASES]);
  });

  it("throws without calling the backend when no case id is set", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const { result } = renderHook(() => usePostCsmCaseEscalation(undefined), {
      wrapper: makeWrapper(queryClient),
    });

    result.current.mutate({ action: "ESCALATE", reason: "x" });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(postMock).not.toHaveBeenCalled();
  });
});
