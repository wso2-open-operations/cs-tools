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
import { ApiQueryKeys } from "@constants/apiConstants";
import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { usePostCaseGithubIssue } from "@features/csm-cases/api/useCsmCaseGithubIssue";

const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

const input = {
  caseId: "case-1",
  reason: "default",
  title: "t",
  description: "d",
  issueTypeLabel: "Type/Discussion",
} as unknown as Parameters<ReturnType<typeof usePostCaseGithubIssue>["mutate"]>[0];

describe("usePostCaseGithubIssue", () => {
  it("sends the key as an Idempotency-Key header and keeps it out of the body", async () => {
    postMock.mockResolvedValue({});
    const { result } = renderHook(() => usePostCaseGithubIssue(), { wrapper });
    result.current.mutate({ ...input, idempotencyKey: "key-123" });
    await waitFor(() => expect(postMock).toHaveBeenCalled());
    const [path, body, options] = postMock.mock.calls[0];
    expect(path).toBe("/cases/case-1/github-issues");
    expect(body).not.toHaveProperty("idempotencyKey");
    expect(body).not.toHaveProperty("caseId");
    expect(options).toEqual({ headers: { "Idempotency-Key": "key-123" } });
  });

  it("invalidates the case's comments and product work items after success", async () => {
    postMock.mockResolvedValue({});
    const spy = vi.spyOn(qc, "invalidateQueries");
    const { result } = renderHook(() => usePostCaseGithubIssue(), { wrapper });
    result.current.mutate({ ...input, idempotencyKey: "k" });
    await waitFor(() =>
      expect(spy).toHaveBeenCalledWith({
        queryKey: [ApiQueryKeys.CSM_CASE_PRODUCT_WORK_ITEMS, "case-1"],
      }),
    );
    expect(spy).toHaveBeenCalledWith({ queryKey: [ApiQueryKeys.CSM_CASE_COMMENTS, "case-1"] });
  });
});
