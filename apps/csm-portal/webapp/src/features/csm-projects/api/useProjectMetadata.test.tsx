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

const getMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useDashboard.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock }),
}));

import { useProjectMetadata } from "@features/csm-projects/api/useProjectMetadata";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useProjectMetadata", () => {
  beforeEach(() => {
    getMock.mockReset();
  });

  it("fetches a project's metadata via GET /projects/{id}/metadata", async () => {
    getMock.mockResolvedValue({
      features: { srProductCategories: ["ms", "pc"] },
    });

    const { result } = renderHook(() => useProjectMetadata("proj-1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getMock).toHaveBeenCalledTimes(1);
    expect(getMock).toHaveBeenCalledWith("/projects/proj-1/metadata");
    expect(result.current.data?.features?.srProductCategories).toEqual(["ms", "pc"]);
  });

  it("does not fetch while the project id is undefined", () => {
    const { result } = renderHook(() => useProjectMetadata(undefined), { wrapper });

    expect(getMock).not.toHaveBeenCalled();
    expect(result.current.isPending).toBe(true);
    expect(result.current.fetchStatus).toBe("idle");
  });

  it("resolves to null on a 404 (unknown project) rather than throwing", async () => {
    getMock.mockResolvedValue(null);

    const { result } = renderHook(() => useProjectMetadata("missing"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toBeNull();
  });

  it("surfaces a query error when the call fails", async () => {
    getMock.mockRejectedValue(new Error("boom"));

    const { result } = renderHook(() => useProjectMetadata("proj-1"), { wrapper });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe("boom");
  });
});
