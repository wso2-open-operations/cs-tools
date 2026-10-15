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
import { beforeEach, describe, expect, it, vi } from "vitest";

const getMock = vi.fn();

vi.mock("@api/backend/client", () => {
  class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  }
  return { BackendApiError, useBackendApi: () => ({ get: getMock }) };
});

// Imported after the mock above so the module picks it up.
import { BackendApiError } from "@api/backend/client";
import { useListAnnouncementRequestAttachments } from "@features/csm-announcements/api/useListAnnouncementRequestAttachments";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

const FILE = {
  id: "att-1",
  announcementRequestId: "req-1",
  name: "advisory.pdf",
  type: "application/pdf",
  sizeBytes: 10,
  createdBy: "jane@example.com",
  createdOn: "2026-07-01T10:00:00Z",
};

beforeEach(() => {
  getMock.mockReset();
});

describe("useListAnnouncementRequestAttachments", () => {
  it("reads the request's files from its own endpoint", async () => {
    getMock.mockResolvedValue({ attachments: [FILE] });
    const { result } = renderHook(() => useListAnnouncementRequestAttachments("req-1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([FILE]);
    expect(getMock).toHaveBeenCalledWith("/announcement-requests/req-1/attachments");
  });

  it("encodes the id in the path", async () => {
    getMock.mockResolvedValue({ attachments: [] });
    const { result } = renderHook(() => useListAnnouncementRequestAttachments("a/b c"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(getMock).toHaveBeenCalledWith("/announcement-requests/a%2Fb%20c/attachments");
  });

  it("treats a response with no files as an empty list, never undefined", async () => {
    getMock.mockResolvedValue({});
    const { result } = renderHook(() => useListAnnouncementRequestAttachments("req-1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
  });

  it("reads a 404 as no files: a backend that predates attachments cannot hold any", async () => {
    getMock.mockRejectedValue(new BackendApiError(404, "not found"));
    const { result } = renderHook(() => useListAnnouncementRequestAttachments("req-1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
  });

  it.each([500, 503, 403, 401])("reports a %i as a failed load, not as no files", async (status) => {
    getMock.mockRejectedValue(new BackendApiError(status, "nope"));
    const { result } = renderHook(() => useListAnnouncementRequestAttachments("req-1"), { wrapper });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.data).toBeUndefined();
  });

  it("reports a network failure as a failed load", async () => {
    getMock.mockRejectedValue(new Error("network down"));
    const { result } = renderHook(() => useListAnnouncementRequestAttachments("req-1"), { wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
  });

  it("asks for nothing without an id", () => {
    renderHook(() => useListAnnouncementRequestAttachments(undefined), { wrapper });
    expect(getMock).not.toHaveBeenCalled();
  });

  it("asks for nothing when the caller has no use for the list", () => {
    renderHook(() => useListAnnouncementRequestAttachments("req-1", false), { wrapper });
    expect(getMock).not.toHaveBeenCalled();
  });
});
