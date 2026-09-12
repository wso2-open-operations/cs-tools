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
import { useSearchDeploymentAttachments } from "@features/csm-projects/api/useSearchDeploymentAttachments";
import type { BackendApi } from "@api/backend/client";
import type { BeAttachmentSearchResponse } from "@api/backend/types";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: (): Partial<BackendApi> => ({ post: postMock }),
}));

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useSearchDeploymentAttachments", () => {
  it("is disabled until a deploymentId is provided", () => {
    const { result } = renderHook(() => useSearchDeploymentAttachments(undefined), { wrapper });
    expect(result.current.fetchStatus).toBe("idle");
    expect(postMock).not.toHaveBeenCalled();
  });

  it("searches referenceType=deployment and maps the response", async () => {
    const response: BeAttachmentSearchResponse = {
      attachments: [
        {
          id: "att-1",
          referenceId: "dep-1",
          referenceType: "deployment",
          name: "runbook.pdf",
          type: "application/pdf",
          sizeBytes: 2048,
          description: "Runbook",
          createdBy: { id: "u-1", email: "jane.doe@example.com", name: "Jane Doe" },
          createdOn: "2026-01-01T00:00:00Z",
          downloadUrl: null,
        },
      ],
      total: 1,
      limit: 50,
      offset: 0,
      hasMore: false,
    };
    postMock.mockResolvedValue(response);

    const { result } = renderHook(() => useSearchDeploymentAttachments("dep-1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith("/attachments/search", {
      referenceId: "dep-1",
      referenceType: "deployment",
      pagination: { offset: 0, limit: 50 },
    });
    expect(result.current.data).toEqual([
      {
        id: "att-1",
        name: "runbook.pdf",
        contentType: "application/pdf",
        sizeBytes: 2048,
        description: "Runbook",
        uploadedBy: "Jane Doe",
        uploadedOn: "2026-01-01T00:00:00Z",
        downloadUrl: null,
      },
    ]);
  });
});
