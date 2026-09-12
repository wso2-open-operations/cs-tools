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
import { useUpdateDeploymentAttachment } from "@features/csm-projects/api/useUpdateDeploymentAttachment";
import type { BackendApi } from "@api/backend/client";

const patchMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: (): Partial<BackendApi> => ({ patch: patchMock }),
}));

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useUpdateDeploymentAttachment", () => {
  it("PATCHes only the provided fields, with referenceType=deployment", async () => {
    patchMock.mockResolvedValue({ message: "ok" });
    const { result } = renderHook(() => useUpdateDeploymentAttachment(), { wrapper });

    result.current.mutate({ deploymentId: "dep-1", attachmentId: "att-1", name: "new-name.pdf" });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(patchMock).toHaveBeenCalledWith("/attachments/att-1", {
      referenceId: "dep-1",
      referenceType: "deployment",
      name: "new-name.pdf",
    });
  });

  it("sends description: null to clear it", async () => {
    patchMock.mockResolvedValue({ message: "ok" });
    const { result } = renderHook(() => useUpdateDeploymentAttachment(), { wrapper });

    result.current.mutate({ deploymentId: "dep-1", attachmentId: "att-1", description: null });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(patchMock).toHaveBeenCalledWith("/attachments/att-1", {
      referenceId: "dep-1",
      referenceType: "deployment",
      description: null,
    });
  });
});
