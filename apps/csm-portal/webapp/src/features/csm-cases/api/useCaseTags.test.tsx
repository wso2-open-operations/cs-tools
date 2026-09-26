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
import type { ReactNode } from "react";
import { describe, expect, it, vi, beforeEach } from "vitest";

const postMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

// Imported after the mock above so the module picks it up.
import { useAddTagToCase } from "@features/csm-cases/api/useCaseTags";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useAddTagToCase", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("posts the label to the given case's own tags endpoint, not a fixed one", async () => {
    postMock.mockResolvedValue({ id: "tag-1", label: "Security Announcement", color: null });

    const { result } = renderHook(() => useAddTagToCase(), { wrapper });
    await result.current.mutateAsync({ caseId: "case-a", label: "Security Announcement" });
    await result.current.mutateAsync({ caseId: "case-b", label: "Security Announcement" });

    expect(postMock).toHaveBeenNthCalledWith(1, "/cases/case-a/tags", {
      label: "Security Announcement",
    });
    expect(postMock).toHaveBeenNthCalledWith(2, "/cases/case-b/tags", {
      label: "Security Announcement",
    });
  });

  it("rejects when the backend call fails, so a caller can distinguish it from a successful create", async () => {
    postMock.mockRejectedValue(new Error("network down"));

    const { result } = renderHook(() => useAddTagToCase(), { wrapper });
    await expect(
      result.current.mutateAsync({ caseId: "case-a", label: "Security Announcement" }),
    ).rejects.toThrow("network down");
    await waitFor(() => expect(result.current.isError).toBe(true));
  });
});
