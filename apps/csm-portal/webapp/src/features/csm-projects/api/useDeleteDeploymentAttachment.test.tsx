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

const deleteMock = vi.fn();

vi.mock("@api/backend/client", () => ({ useBackendApi: () => ({}) }));
vi.mock("@features/csm-cases/api/entityAttachments", () => ({
  deleteEntityAttachment: (...args: unknown[]) => deleteMock(...args),
  entityAttachmentErrorMessage: (err: unknown, fallback: string) =>
    err instanceof Error ? err.message : fallback,
}));

import { useDeleteDeploymentAttachment } from "@features/csm-projects/api/useDeleteDeploymentAttachment";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useDeleteDeploymentAttachment", () => {
  it("deletes the attachment by id as a deployment attachment", async () => {
    deleteMock.mockResolvedValue(undefined);
    const { result } = renderHook(() => useDeleteDeploymentAttachment(), { wrapper });

    result.current.mutate({ deploymentId: "dep-1", attachmentId: "att-1" });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(deleteMock).toHaveBeenCalledWith(expect.anything(), { type: "deployment", id: "att-1" });
  });

  it("fails with the backend's message", async () => {
    deleteMock.mockRejectedValue(new Error("Only the person who uploaded an attachment can delete it."));
    const { result } = renderHook(() => useDeleteDeploymentAttachment(), { wrapper });

    result.current.mutate({ deploymentId: "dep-1", attachmentId: "att-1" });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe("Only the person who uploaded an attachment can delete it.");
  });
});
