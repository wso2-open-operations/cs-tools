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

const postMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

// Imported after the mock above so the module picks it up.
import { usePostAnnouncementUpdateComments } from "@features/csm-announcements/api/usePostAnnouncementUpdateComments";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  postMock.mockReset();
});

describe("usePostAnnouncementUpdateComments", () => {
  it("posts a comment to every case id given", async () => {
    postMock.mockResolvedValue({ id: "comment-1", createdOn: "2026-07-01T00:00:00Z", createdBy: "x" });

    const { result } = renderHook(() => usePostAnnouncementUpdateComments(), { wrapper });
    await act(async () => {
      await result.current.handlePost(["case-1", "case-2"], "<p>update</p>", "Jane");
    });

    expect(postMock).toHaveBeenCalledTimes(2);
    expect(result.current.succeededCaseIds.sort()).toEqual(["case-1", "case-2"]);
    expect(result.current.failedCaseIds).toEqual([]);
    expect(result.current.done).toBe(true);
  });

  it("retrying only resends to cases that failed, not ones that already succeeded", async () => {
    postMock.mockImplementation((url: string) =>
      url.includes("case-2")
        ? Promise.reject(new Error("boom"))
        : Promise.resolve({ id: "comment-1", createdOn: "2026-07-01T00:00:00Z", createdBy: "x" }),
    );

    const { result } = renderHook(() => usePostAnnouncementUpdateComments(), { wrapper });
    await act(async () => {
      await result.current.handlePost(["case-1", "case-2"], "<p>update</p>", "Jane");
    });
    expect(result.current.failedCaseIds).toEqual(["case-2"]);
    expect(result.current.done).toBe(false);
    postMock.mockReset();
    postMock.mockResolvedValue({ id: "comment-2", createdOn: "2026-07-01T00:00:00Z", createdBy: "x" });

    await act(async () => {
      await result.current.handlePost(["case-1", "case-2"], "<p>update</p>", "Jane");
    });

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(result.current.succeededCaseIds.sort()).toEqual(["case-1", "case-2"]);
    expect(result.current.done).toBe(true);
  });

  // Regression test for a real bug CodeRabbit caught on review: this hook's
  // tracking used to never clear between distinct updates (only within
  // retries of the *same* one), since the dialog reuses one hook instance
  // for its whole lifetime. Without reset(), posting update #2 after
  // update #1 fully succeeded would see every case already in
  // succeededCaseIds and silently report done=true without ever posting
  // update #2's actual content.
  it("reset() clears prior tracking so a second, different update actually posts instead of being silently skipped", async () => {
    postMock.mockResolvedValue({ id: "comment-1", createdOn: "2026-07-01T00:00:00Z", createdBy: "x" });

    const { result } = renderHook(() => usePostAnnouncementUpdateComments(), { wrapper });
    await act(async () => {
      await result.current.handlePost(["case-1", "case-2"], "<p>first update</p>", "Jane");
    });
    expect(postMock).toHaveBeenCalledTimes(2);
    expect(result.current.done).toBe(true);
    postMock.mockClear();

    // Without reset(), this second call would see case-1/case-2 already in
    // succeededCaseIds (from the *first* update) and post nothing.
    act(() => {
      result.current.reset();
    });
    expect(result.current.succeededCaseIds).toEqual([]);
    expect(result.current.done).toBe(false);

    await act(async () => {
      await result.current.handlePost(["case-1", "case-2"], "<p>second update</p>", "Jane");
    });

    expect(postMock).toHaveBeenCalledTimes(2);
    expect(result.current.done).toBe(true);
  });
});
