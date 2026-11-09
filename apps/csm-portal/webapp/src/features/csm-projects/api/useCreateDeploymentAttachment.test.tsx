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
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const uploadMock = vi.fn();

vi.mock("@api/backend/client", () => ({ useBackendApi: () => ({}) }));
vi.mock("@features/csm-cases/api/entityAttachments", () => ({
  uploadEntityAttachment: (...args: unknown[]) => uploadMock(...args),
  entityAttachmentErrorMessage: (err: unknown, fallback: string) =>
    err instanceof Error ? err.message : fallback,
}));

import { useCreateDeploymentAttachment } from "@features/csm-projects/api/useCreateDeploymentAttachment";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

function makeFile(name: string, sizeBytes: number, type = "application/pdf"): File {
  return new File([new Uint8Array(sizeBytes)], name, { type });
}

describe("useCreateDeploymentAttachment", () => {
  beforeEach(() => {
    uploadMock.mockReset();
  });

  it("uploads the file as a deployment attachment through the chunked flow", async () => {
    uploadMock.mockResolvedValue({ id: "att-1" });
    const { result } = renderHook(() => useCreateDeploymentAttachment(), { wrapper });
    const file = makeFile("a.txt", 4, "text/plain");

    result.current.mutate({ deploymentId: "dep-1", file });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(uploadMock).toHaveBeenCalledWith(expect.anything(), {
      type: "deployment",
      parentId: "dep-1",
      file,
      name: "a.txt",
    });
  });

  it("uses the given display name", async () => {
    uploadMock.mockResolvedValue({ id: "att-1" });
    const { result } = renderHook(() => useCreateDeploymentAttachment(), { wrapper });

    result.current.mutate({ deploymentId: "dep-1", file: makeFile("a.txt", 4), name: " Runbook " });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(uploadMock.mock.calls[0][1].name).toBe("Runbook");
  });

  it("fails with the upload's message", async () => {
    uploadMock.mockRejectedValue(new Error("The upload is larger than allowed."));
    const { result } = renderHook(() => useCreateDeploymentAttachment(), { wrapper });

    result.current.mutate({ deploymentId: "dep-1", file: makeFile("big.bin", 4) });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe("The upload is larger than allowed.");
  });
});
