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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useSetDeploymentAirGapped } from "@features/csm-projects/api/useSetDeploymentAirGapped";
import type { BackendApi } from "@api/backend/client";

const putMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: (): Partial<BackendApi> => ({ put: putMock }),
}));

function setup() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  function wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  }
  return { wrapper, invalidate };
}

describe("useSetDeploymentAirGapped", () => {
  it("PUTs the flag and refreshes the project's deployments", async () => {
    putMock.mockResolvedValue({ id: "dep 1", airGapped: true, updatedOn: "", updatedBy: "" });
    const { wrapper, invalidate } = setup();
    const { result } = renderHook(() => useSetDeploymentAirGapped("proj-1"), { wrapper });

    result.current.mutate({ deploymentId: "dep 1", airGapped: true });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(putMock).toHaveBeenCalledWith("/deployments/dep%201/air-gapped", { airGapped: true });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [ApiQueryKeys.DEPLOYMENTS, "proj-1"] });
  });

  it("sends false to clear the mark", async () => {
    putMock.mockResolvedValue({ id: "dep-1", airGapped: false, updatedOn: "", updatedBy: "" });
    const { wrapper } = setup();
    const { result } = renderHook(() => useSetDeploymentAirGapped("proj-1"), { wrapper });

    result.current.mutate({ deploymentId: "dep-1", airGapped: false });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(putMock).toHaveBeenCalledWith("/deployments/dep-1/air-gapped", { airGapped: false });
  });
});
