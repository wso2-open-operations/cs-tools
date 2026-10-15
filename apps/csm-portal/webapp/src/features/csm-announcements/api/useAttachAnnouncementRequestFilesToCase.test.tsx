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
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const postEmptyMock = vi.fn();
vi.mock("@api/backend/client", () => ({ useBackendApi: () => ({ postEmpty: postEmptyMock }) }));

import { useAttachAnnouncementRequestFilesToCase } from "@features/csm-announcements/api/useAttachAnnouncementRequestFilesToCase";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  postEmptyMock.mockReset();
});

describe("useAttachAnnouncementRequestFilesToCase", () => {
  it("asks the server to copy the request's files onto one case, sending no bytes", async () => {
    postEmptyMock.mockResolvedValue({ attached: 2, alreadyAttached: 0 });
    const { result } = renderHook(() => useAttachAnnouncementRequestFilesToCase(), { wrapper });

    let outcome;
    await act(async () => {
      outcome = await result.current.mutateAsync({ id: "req-1", caseId: "case-9" });
    });

    expect(outcome).toEqual({ attached: 2, alreadyAttached: 0 });
    expect(postEmptyMock).toHaveBeenCalledWith("/announcement-requests/req-1/cases/case-9/attachments");
  });

  it("encodes both ids in the path", async () => {
    postEmptyMock.mockResolvedValue({ attached: 0, alreadyAttached: 1 });
    const { result } = renderHook(() => useAttachAnnouncementRequestFilesToCase(), { wrapper });
    await act(async () => {
      await result.current.mutateAsync({ id: "a/b", caseId: "c d" });
    });
    expect(postEmptyMock).toHaveBeenCalledWith("/announcement-requests/a%2Fb/cases/c%20d/attachments");
  });

  it("surfaces the server's reason when it refuses", async () => {
    postEmptyMock.mockRejectedValue(new Error("case does not belong to this request"));
    const { result } = renderHook(() => useAttachAnnouncementRequestFilesToCase(), { wrapper });
    await act(async () => {
      await expect(result.current.mutateAsync({ id: "req-1", caseId: "x" })).rejects.toThrow(/does not belong/);
    });
  });
});
