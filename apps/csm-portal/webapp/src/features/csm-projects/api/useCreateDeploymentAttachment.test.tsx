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
import {
  MAX_DEPLOYMENT_ATTACHMENT_SIZE_BYTES,
  useCreateDeploymentAttachment,
} from "@features/csm-projects/api/useCreateDeploymentAttachment";
import type { BackendApi } from "@api/backend/client";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: (): Partial<BackendApi> => ({ post: postMock }),
}));

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

function makeFile(name: string, sizeBytes: number, type = "application/pdf"): File {
  return new File([new Uint8Array(sizeBytes)], name, { type });
}

describe("useCreateDeploymentAttachment", () => {
  beforeEach(() => {
    postMock.mockClear();
  });

  it("uploads with referenceType=deployment as a base64 data URI", async () => {
    postMock.mockResolvedValue({ message: "ok" });
    const { result } = renderHook(() => useCreateDeploymentAttachment(), { wrapper });

    result.current.mutate({ deploymentId: "dep-1", file: makeFile("a.txt", 4, "text/plain") });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(postMock).toHaveBeenCalledWith(
      "/attachments",
      expect.objectContaining({
        referenceId: "dep-1",
        referenceType: "deployment",
        name: "a.txt",
        type: "text/plain",
      }),
    );
    const body = postMock.mock.calls[0][1];
    expect(body.file).toMatch(/^data:text\/plain;base64,/);
  });

  it("rejects a file over the size cap without calling the API", async () => {
    const { result } = renderHook(() => useCreateDeploymentAttachment(), { wrapper });

    result.current.mutate({
      deploymentId: "dep-1",
      file: makeFile("big.bin", MAX_DEPLOYMENT_ATTACHMENT_SIZE_BYTES + 1),
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(postMock).not.toHaveBeenCalled();
  });
});
