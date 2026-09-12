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
import { useDeleteDeploymentAttachment } from "@features/csm-projects/api/useDeleteDeploymentAttachment";
import type { BackendApi } from "@api/backend/client";

const delMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: (): Partial<BackendApi> => ({ del: delMock }),
}));

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useDeleteDeploymentAttachment", () => {
  it("DELETEs by attachment id", async () => {
    delMock.mockResolvedValue({ message: "ok" });
    const { result } = renderHook(() => useDeleteDeploymentAttachment(), { wrapper });

    result.current.mutate({ deploymentId: "dep-1", attachmentId: "att-1" });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(delMock).toHaveBeenCalledWith("/attachments/att-1");
  });
});
