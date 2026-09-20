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
// under vitest (same approach as useSearchTags.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useDeployedProductOptions } from "@features/csm-cases/api/useDeployedProductOptions";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useDeployedProductOptions", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ deployedProducts: [] });
  });

  it("omits productCategories from the request body when not given", async () => {
    const { result } = renderHook(() => useDeployedProductOptions("dep-1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledTimes(1);
    const [path, payload] = postMock.mock.calls[0];
    expect(path).toBe("/deployments/dep-1/products/search");
    expect(payload).toEqual({ pagination: { offset: 0, limit: expect.any(Number) } });
  });

  it("omits productCategories from the request body when given an empty array", async () => {
    const { result } = renderHook(() => useDeployedProductOptions("dep-1", []), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const payload = postMock.mock.calls[0][1];
    expect(payload).not.toHaveProperty("productCategories");
  });

  it("forwards a non-empty productCategories list in the request body", async () => {
    const { result } = renderHook(
      () => useDeployedProductOptions("dep-1", ["pdp"]),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const payload = postMock.mock.calls[0][1];
    expect(payload).toMatchObject({ productCategories: ["pdp"] });
  });

  it("does not call the backend while no deployment id is given", () => {
    renderHook(() => useDeployedProductOptions(undefined), { wrapper });
    expect(postMock).not.toHaveBeenCalled();
  });
});
