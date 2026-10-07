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

import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();
const invalidateQueriesMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest; stub it (same approach as ChangeRequestApprovals.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status = 500, message = "") {
      super(message);
      this.status = status;
    }
  },
  useBackendApi: () => ({ post: postMock }),
}));

import { BackendApiError } from "@api/backend/client";
import { useDecideChangeRequestApproval } from "@features/csm-operations/api/useDecideChangeRequestApproval";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  queryClient.invalidateQueries = invalidateQueriesMock.mockImplementation(
    () => Promise.resolve(),
  );
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useDecideChangeRequestApproval", () => {
  beforeEach(() => {
    postMock.mockReset();
    invalidateQueriesMock.mockReset();
  });

  it("POSTs the decision to /change-requests/{id}/approvals/decision", async () => {
    postMock.mockResolvedValue({ id: "approval-1", state: "approved" });
    const { result } = renderHook(() => useDecideChangeRequestApproval(), {
      wrapper,
    });

    act(() => {
      result.current.mutate({ id: "cr-1", decision: "approved" });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/change-requests/cr-1/approvals/decision",
      { decision: "approved" },
    );
    expect(result.current.data).toEqual({ id: "approval-1", state: "approved" });
  });

  it("invalidates the approvals, detail, and list queries for the CR on success", async () => {
    postMock.mockResolvedValue({ id: "approval-1", state: "rejected" });
    const { result } = renderHook(() => useDecideChangeRequestApproval(), {
      wrapper,
    });

    act(() => {
      result.current.mutate({ id: "cr-1", decision: "rejected" });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["change-request-approvals", "cr-1"] }),
    );
    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["change-request-details", "cr-1"] }),
    );
    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["change-requests"] }),
    );
  });

  it("also invalidates the approvals, detail, and list queries after an approval (an approval can move the CR on server-side, e.g. the CAB approval that schedules it)", async () => {
    postMock.mockResolvedValue({ id: "approval-1", state: "approved" });
    const { result } = renderHook(() => useDecideChangeRequestApproval(), {
      wrapper,
    });

    act(() => {
      result.current.mutate({ id: "cr-1", decision: "approved" });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const keys = invalidateQueriesMock.mock.calls.map(
      ([arg]) => (arg as { queryKey: unknown[] }).queryKey,
    );
    expect(keys).toEqual(
      expect.arrayContaining([
        ["change-request-approvals", "cr-1"],
        ["change-request-details", "cr-1"],
        ["change-requests"],
      ]),
    );
  });

  // A 409 means the change moved on while the page was open (the approval is no
  // longer pending): refresh, so the stale row stops being offered.
  it("refreshes the approvals, detail and list queries when the decision is refused with a 409", async () => {
    const stale = new (BackendApiError as unknown as new (s: number, m: string) => Error)(
      409,
      "this approval is no longer pending: the change request is in Closed, but the Review stage can only be decided while it is in Review",
    );
    postMock.mockRejectedValue(stale);
    const { result } = renderHook(() => useDecideChangeRequestApproval(), {
      wrapper,
    });

    act(() => {
      result.current.mutate({ id: "cr-1", decision: "approved" });
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBe(stale);
    const keys = invalidateQueriesMock.mock.calls.map(
      ([arg]) => (arg as { queryKey: unknown[] }).queryKey,
    );
    expect(keys).toEqual(
      expect.arrayContaining([
        ["change-request-approvals", "cr-1"],
        ["change-request-details", "cr-1"],
        ["change-requests"],
      ]),
    );
  });

  it("does not refresh anything for any other failure (a 403, a 5xx, a non-backend error)", async () => {
    for (const err of [
      new (BackendApiError as unknown as new (s: number, m: string) => Error)(403, "the creator of a change request cannot approve it"),
      new (BackendApiError as unknown as new (s: number, m: string) => Error)(500, "boom"),
      new Error("network"),
    ]) {
      invalidateQueriesMock.mockClear();
      postMock.mockRejectedValue(err);
      const { result } = renderHook(() => useDecideChangeRequestApproval(), {
        wrapper,
      });
      act(() => {
        result.current.mutate({ id: "cr-1", decision: "approved" });
      });
      await waitFor(() => expect(result.current.isError).toBe(true));
      expect(invalidateQueriesMock).not.toHaveBeenCalled();
    }
  });

  it("surfaces upstream errors via mutation error state", async () => {
    const upstreamError = new Error("Only the caller's own pending approval can be decided.");
    postMock.mockRejectedValue(upstreamError);
    const { result } = renderHook(() => useDecideChangeRequestApproval(), {
      wrapper,
    });

    act(() => {
      result.current.mutate({ id: "cr-1", decision: "approved" });
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBe(upstreamError);
  });
});
