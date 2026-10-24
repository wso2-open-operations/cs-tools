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

const patchMock = vi.fn();
const invalidateQueriesMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest; stub it.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ patch: patchMock }),
}));

import { usePatchProjectOnboarding } from "@features/csm-projects/api/usePatchProjectOnboarding";

const RESPONSE = {
  message: "Project updated",
  project: { id: "proj-1", onboardingStatus: "On-Hold", onboardingOwner: null },
};

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  queryClient.invalidateQueries = invalidateQueriesMock.mockImplementation(() =>
    Promise.resolve(),
  );
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("usePatchProjectOnboarding", () => {
  beforeEach(() => {
    patchMock.mockReset();
    invalidateQueriesMock.mockReset();
  });

  it("PATCHes /projects/{id} with exactly the given onboarding fields", async () => {
    patchMock.mockResolvedValue(RESPONSE);
    const { result } = renderHook(() => usePatchProjectOnboarding("proj-1"), { wrapper });

    act(() => {
      result.current.mutate({
        onboardingStatus: "On-Hold",
        onboardingOwner: "00000000-0000-0000-0000-000000000000",
      });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(patchMock).toHaveBeenCalledWith("/projects/proj-1", {
      onboardingStatus: "On-Hold",
      onboardingOwner: "00000000-0000-0000-0000-000000000000",
    });
  });

  it("sends the four onboarding dates as YYYY-MM-DD under their request names", async () => {
    patchMock.mockResolvedValue(RESPONSE);
    const { result } = renderHook(() => usePatchProjectOnboarding("proj-1"), { wrapper });

    act(() => {
      result.current.mutate({
        onboardingStartDate: "2026-03-01",
        onboardingGoLivePlanDate: "2026-04-01",
        onboardingGoLiveDate: "2026-05-01",
        onboardingExpiryDate: "2026-12-31",
      });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(patchMock).toHaveBeenCalledWith("/projects/proj-1", {
      onboardingStartDate: "2026-03-01",
      onboardingGoLivePlanDate: "2026-04-01",
      onboardingGoLiveDate: "2026-05-01",
      onboardingExpiryDate: "2026-12-31",
    });
  });

  it("invalidates the project detail query and the projects list cache on success", async () => {
    patchMock.mockResolvedValue(RESPONSE);
    const { result } = renderHook(() => usePatchProjectOnboarding("proj-1"), { wrapper });

    act(() => {
      result.current.mutate({ onboardingStatus: "Completed" });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["csm-project-detail", "proj-1"] }),
    );
    const withPredicate = invalidateQueriesMock.mock.calls
      .map((c) => c[0] as { predicate?: (q: { queryKey: unknown[] }) => boolean })
      .find((a) => a.predicate);
    expect(withPredicate?.predicate?.({ queryKey: ["csm-projects-search", {}] })).toBe(true);
    expect(withPredicate?.predicate?.({ queryKey: ["something-else"] })).toBe(false);
  });

  it("does not invalidate when the backend rejects, and surfaces the error", async () => {
    patchMock.mockRejectedValue(new Error("forbidden"));
    const { result } = renderHook(() => usePatchProjectOnboarding("proj-1"), { wrapper });

    act(() => {
      result.current.mutate({ onboardingStatus: "Completed" });
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe("forbidden");
    expect(invalidateQueriesMock).not.toHaveBeenCalled();
  });

  it("rejects without calling the backend when projectId is missing", async () => {
    const { result } = renderHook(() => usePatchProjectOnboarding(undefined), { wrapper });

    act(() => {
      result.current.mutate({ onboardingStatus: "Completed" });
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(patchMock).not.toHaveBeenCalled();
  });
});
