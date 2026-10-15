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
import { ApiQueryKeys } from "@constants/apiConstants";

const deleteMock = vi.fn();
vi.mock("@api/backend/client", () => ({ useBackendApi: () => ({ delete: deleteMock }) }));

import { useDeleteAnnouncementRequestAttachment } from "@features/csm-announcements/api/useDeleteAnnouncementRequestAttachment";

let queryClient: QueryClient;
function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  deleteMock.mockReset();
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
});

describe("useDeleteAnnouncementRequestAttachment", () => {
  it("deletes the file through its request", async () => {
    deleteMock.mockResolvedValue(undefined);
    const { result } = renderHook(() => useDeleteAnnouncementRequestAttachment(), { wrapper });
    await act(async () => {
      await result.current.mutateAsync({ id: "req-1", attachmentId: "att-1" });
    });
    expect(deleteMock).toHaveBeenCalledWith("/announcement-requests/req-1/attachments/att-1");
  });

  it("refreshes the file list, the request and the requests list", async () => {
    deleteMock.mockResolvedValue(undefined);
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useDeleteAnnouncementRequestAttachment(), { wrapper });
    await act(async () => {
      await result.current.mutateAsync({ id: "req-1", attachmentId: "att-1" });
    });

    const keys = invalidate.mock.calls.map((c) => (c[0] as { queryKey: unknown[] }).queryKey);
    expect(keys).toContainEqual([ApiQueryKeys.ANNOUNCEMENT_REQUEST_ATTACHMENTS, "req-1"]);
    expect(keys).toContainEqual([ApiQueryKeys.ANNOUNCEMENT_REQUEST_DETAIL, "req-1"]);
    expect(keys).toContainEqual([ApiQueryKeys.ANNOUNCEMENT_REQUESTS_SEARCH]);
  });

  it("surfaces a refusal", async () => {
    deleteMock.mockRejectedValue(new Error("announcement request attachment not found"));
    const { result } = renderHook(() => useDeleteAnnouncementRequestAttachment(), { wrapper });
    await act(async () => {
      await expect(result.current.mutateAsync({ id: "req-1", attachmentId: "gone" })).rejects.toThrow(/not found/);
    });
  });
});
