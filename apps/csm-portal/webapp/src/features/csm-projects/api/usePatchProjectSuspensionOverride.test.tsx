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

import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const patchMock = vi.fn();
const invalidateQueriesMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest; stub it.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ patch: patchMock }),
}));

import { usePatchProjectSuspensionOverride } from "@features/csm-projects/api/usePatchProjectSuspensionOverride";

const RESPONSE = {
  message: "Project updated",
  project: { id: "proj-1", suspensionProcessOverridden: true },
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

describe("usePatchProjectSuspensionOverride", () => {
  beforeEach(() => {
    patchMock.mockReset();
    invalidateQueriesMock.mockReset();
  });

  it.each([true, false])(
    "PATCHes /projects/{id} with exactly { suspensionProcessOverridden: %s }",
    async (value) => {
      patchMock.mockResolvedValue(RESPONSE);
      const { result } = renderHook(() => usePatchProjectSuspensionOverride("proj-1"), {
        wrapper,
      });

      act(() => {
        result.current.mutate(value);
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(patchMock).toHaveBeenCalledWith("/projects/proj-1", {
        suspensionProcessOverridden: value,
      });
    },
  );

  it("url-encodes the project id", async () => {
    patchMock.mockResolvedValue(RESPONSE);
    const { result } = renderHook(() => usePatchProjectSuspensionOverride("a/b"), { wrapper });

    act(() => {
      result.current.mutate(true);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(patchMock.mock.calls[0]?.[0]).toBe("/projects/a%2Fb");
  });

  it("invalidates the project detail query on success", async () => {
    patchMock.mockResolvedValue(RESPONSE);
    const { result } = renderHook(() => usePatchProjectSuspensionOverride("proj-1"), { wrapper });

    act(() => {
      result.current.mutate(true);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["csm-project-detail", "proj-1"] }),
    );
  });

  it("does not invalidate when the backend rejects, and surfaces the error", async () => {
    patchMock.mockRejectedValue(new Error("forbidden"));
    const { result } = renderHook(() => usePatchProjectSuspensionOverride("proj-1"), { wrapper });

    act(() => {
      result.current.mutate(true);
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(invalidateQueriesMock).not.toHaveBeenCalled();
    expect(result.current.error?.message).toBe("forbidden");
  });

  it("rejects without calling the API when there is no project id", async () => {
    const { result } = renderHook(() => usePatchProjectSuspensionOverride(undefined), { wrapper });

    act(() => {
      result.current.mutate(true);
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(patchMock).not.toHaveBeenCalled();
  });
});
