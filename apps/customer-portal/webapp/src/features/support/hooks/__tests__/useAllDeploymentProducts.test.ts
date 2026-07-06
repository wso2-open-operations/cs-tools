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
import { describe, expect, it, vi, beforeEach } from "vitest";
import { useQueries } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import {
  createFetchWithMergedAuthHeaders,
  fetchDeploymentProductsAll,
} from "@features/project-details/api/usePostDeploymentProductsSearch";
import { useAllDeploymentProducts } from "@features/support/hooks/useAllDeploymentProducts";

vi.mock("@tanstack/react-query", () => ({
  useQueries: vi.fn(),
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: vi.fn(),
}));

vi.mock("@features/project-details/api/usePostDeploymentProductsSearch", () => ({
  createFetchWithMergedAuthHeaders: vi.fn(),
  fetchDeploymentProductsAll: vi.fn(),
}));

describe("useAllDeploymentProducts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns empty map when deployments are undefined", () => {
    vi.mocked(useAsgardeo).mockReturnValue({
      getIdToken: vi.fn(),
    } as unknown as ReturnType<typeof useAsgardeo>);
    vi.mocked(useQueries).mockReturnValue([]);

    const { result } = renderHook(() => useAllDeploymentProducts(undefined));

    expect(useQueries).toHaveBeenCalledWith({ queries: [] });
    expect(result.current.productsByDeploymentId).toEqual({});
    expect(result.current.isLoading).toBe(false);
  });

  it("builds deployment map and query function fetches with auth headers", async () => {
    const getIdToken = vi.fn().mockResolvedValue("token-1");
    const fetchFn = vi.fn();
    const deployments = [{ id: "dep-1" }, { id: "" }, { id: "dep-2" }];

    vi.mocked(useAsgardeo).mockReturnValue({
      getIdToken,
    } as unknown as ReturnType<typeof useAsgardeo>);
    vi.mocked(createFetchWithMergedAuthHeaders).mockReturnValue(fetchFn);
    vi.mocked(fetchDeploymentProductsAll).mockResolvedValue([
      { id: "p-1", name: "A" },
    ] as never);
    vi.mocked(useQueries).mockReturnValue([
      { data: [{ id: "p-1", name: "A" }], isLoading: false },
      { data: undefined, isLoading: true },
    ] as never);

    const { result } = renderHook(() => useAllDeploymentProducts(deployments));
    const queryOptions = vi.mocked(useQueries).mock.calls[0][0] as {
      queries: Array<{ queryFn: () => Promise<unknown> }>;
    };

    expect(queryOptions.queries).toHaveLength(2);
    expect(result.current.productsByDeploymentId).toEqual({
      "dep-1": [{ id: "p-1", name: "A" }],
      "dep-2": [],
    });
    expect(result.current.isLoading).toBe(true);

    await queryOptions.queries[0].queryFn();

    expect(getIdToken).toHaveBeenCalled();
    expect(createFetchWithMergedAuthHeaders).toHaveBeenCalledWith("token-1");
    expect(fetchDeploymentProductsAll).toHaveBeenCalledWith(
      expect.objectContaining({
        deploymentId: "dep-1",
        pageSize: 10,
        fetchFn,
      }),
    );
  });
});
