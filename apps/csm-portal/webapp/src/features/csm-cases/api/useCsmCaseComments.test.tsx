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
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import type { BeComment } from "@api/backend/types";

const postMock = vi.fn();

vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useGetCsmCaseComments } from "@features/csm-cases/api/useCsmCaseComments";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

function beComment(id: string): BeComment {
  return {
    id,
    type: "comment",
    content: `content ${id}`,
    createdOn: "2026-07-01T00:00:00Z",
    createdBy: { id: null, email: "jane.doe@example.com", name: "Jane Doe" },
  };
}

describe("useGetCsmCaseComments", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("follows hasMore across pages instead of stopping at the first BE_MAX_PAGE_LIMIT page", async () => {
    // Page 1: a full page (50 rows) with hasMore true. Page 2: a partial page,
    // hasMore false — the loop must fetch both and return all 53 comments,
    // not silently drop the 3 past the first page.
    const page1 = Array.from({ length: 50 }, (_, i) => beComment(`c${i}`));
    const page2 = Array.from({ length: 3 }, (_, i) => beComment(`c${50 + i}`));
    postMock
      .mockResolvedValueOnce({ comments: page1, total: 53, limit: 50, offset: 0, hasMore: true })
      .mockResolvedValueOnce({ comments: page2, total: 53, limit: 50, offset: 50, hasMore: false });

    const { result } = renderHook(() => useGetCsmCaseComments("case-1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toHaveLength(53);
    expect(postMock).toHaveBeenCalledTimes(2);
    expect(postMock.mock.calls[0][1].pagination).toEqual({ offset: 0, limit: 50 });
    expect(postMock.mock.calls[1][1].pagination).toEqual({ offset: 50, limit: 50 });
  });

  it("stops after a single page when the BE reports no more", async () => {
    const rows = [beComment("only-one")];
    postMock.mockResolvedValueOnce({ comments: rows, total: 1, limit: 50, offset: 0, hasMore: false });

    const { result } = renderHook(() => useGetCsmCaseComments("case-2"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toHaveLength(1);
    expect(postMock).toHaveBeenCalledTimes(1);
  });
});
