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

const postMock = vi.fn();
vi.mock("@api/backend/client", () => ({ useBackendApi: () => ({ post: postMock }) }));

import { useUploadAnnouncementRequestAttachment } from "@features/csm-announcements/api/useUploadAnnouncementRequestAttachment";

let queryClient: QueryClient;
function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

const STORED = {
  id: "att-1",
  announcementRequestId: "req-1",
  name: "advisory.pdf",
  type: "application/pdf",
  sizeBytes: 8,
  createdBy: "jane@example.com",
  createdOn: "2026-07-01T10:00:00Z",
};

beforeEach(() => {
  postMock.mockReset();
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
});

describe("useUploadAnnouncementRequestAttachment", () => {
  it("posts the file as a base64 data URI under its own name", async () => {
    postMock.mockResolvedValue(STORED);
    const { result } = renderHook(() => useUploadAnnouncementRequestAttachment(), { wrapper });

    let stored;
    await act(async () => {
      stored = await result.current.mutateAsync({
        id: "req-1",
        file: new File(["%PDF-1.4"], "advisory.pdf", { type: "application/pdf" }),
      });
    });

    expect(stored).toEqual(STORED);
    expect(postMock).toHaveBeenCalledTimes(1);
    const [path, body] = postMock.mock.calls[0] as [string, { name: string; type?: string; file: string }];
    expect(path).toBe("/announcement-requests/req-1/attachments");
    expect(body.name).toBe("advisory.pdf");
    expect(body.type).toBe("application/pdf");
    expect(body.file.startsWith("data:application/pdf;base64,")).toBe(true);
    expect(atob(body.file.split(",")[1])).toBe("%PDF-1.4");
  });

  it("sends no declared type when the browser reports none (the server derives it from the name)", async () => {
    postMock.mockResolvedValue(STORED);
    const { result } = renderHook(() => useUploadAnnouncementRequestAttachment(), { wrapper });
    await act(async () => {
      await result.current.mutateAsync({ id: "req-1", file: new File(["x"], "advisory.pdf") });
    });
    expect((postMock.mock.calls[0][1] as { type?: string }).type).toBeUndefined();
  });

  it("refreshes the file list, the request and the requests list", async () => {
    postMock.mockResolvedValue(STORED);
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUploadAnnouncementRequestAttachment(), { wrapper });
    await act(async () => {
      await result.current.mutateAsync({ id: "req-1", file: new File(["x"], "advisory.pdf") });
    });

    const keys = invalidate.mock.calls.map((c) => (c[0] as { queryKey: unknown[] }).queryKey);
    expect(keys).toContainEqual([ApiQueryKeys.ANNOUNCEMENT_REQUEST_ATTACHMENTS, "req-1"]);
    expect(keys).toContainEqual([ApiQueryKeys.ANNOUNCEMENT_REQUEST_DETAIL, "req-1"]);
    expect(keys).toContainEqual([ApiQueryKeys.ANNOUNCEMENT_REQUESTS_SEARCH]);
  });

  it("surfaces the server's reason when it refuses the file, and still refreshes (the request may have moved)", async () => {
    postMock.mockRejectedValue(new Error("some projects have already received this announcement, so its files can no longer change"));
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUploadAnnouncementRequestAttachment(), { wrapper });

    await act(async () => {
      await expect(
        result.current.mutateAsync({ id: "req-1", file: new File(["x"], "advisory.pdf") }),
      ).rejects.toThrow(/already received this announcement/);
    });
    expect(invalidate).toHaveBeenCalled();
  });
});
