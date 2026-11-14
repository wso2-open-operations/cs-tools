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

// The global setup replaces this module with the defaults; this test needs the real one.
vi.unmock("@api/useGetAttachmentLimits");

const getMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock }),
}));

import { useAttachmentLimits, useGetAttachmentLimits } from "@api/useGetAttachmentLimits";

const MB = 1024 * 1024;

let queryClient: QueryClient;

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  getMock.mockReset();
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe("useGetAttachmentLimits", () => {
  it("reads GET /attachments/limits", async () => {
    getMock.mockResolvedValue({ maxFileBytes: 100 * MB, maxInlineImagesTotalBytes: 5 * MB, directTransferThresholdBytes: 16 * MB });
    const { result } = renderHook(() => useGetAttachmentLimits(), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(getMock).toHaveBeenCalledWith("/attachments/limits");
    expect(result.current.data).toEqual({ maxFileBytes: 100 * MB, maxInlineImagesTotalBytes: 5 * MB, directTransferThresholdBytes: 16 * MB });
  });

  it("makes no request while disabled", () => {
    renderHook(() => useGetAttachmentLimits(false), { wrapper });
    expect(getMock).not.toHaveBeenCalled();
  });
});

describe("useAttachmentLimits", () => {
  it("uses the defaults (10 MiB file, 1 MiB images) until the server answers", async () => {
    getMock.mockResolvedValue({ maxFileBytes: 1024 * MB, maxInlineImagesTotalBytes: 25 * MB, directTransferThresholdBytes: 16 * MB });
    const { result } = renderHook(() => useAttachmentLimits(), { wrapper });
    expect(result.current).toEqual({ maxFileBytes: 10 * MB, maxInlineImagesBytes: 1 * MB, directTransferThresholdBytes: 0 });
    await waitFor(() => expect(result.current.maxFileBytes).toBe(1024 * MB));
    expect(result.current.maxInlineImagesBytes).toBe(25 * MB);
    expect(result.current.directTransferThresholdBytes).toBe(16 * MB);
  });

  it("keeps the defaults when the request fails", async () => {
    getMock.mockImplementation(async () => {
      throw new Error("boom");
    });
    const { result } = renderHook(() => ({ q: useGetAttachmentLimits(), limits: useAttachmentLimits() }), { wrapper });
    await waitFor(() => expect(result.current.q.isError).toBe(true));
    expect(result.current.limits).toEqual({ maxFileBytes: 10 * MB, maxInlineImagesBytes: 1 * MB, directTransferThresholdBytes: 0 });
  });

  it("keeps the defaults for an unusable answer", async () => {
    getMock.mockResolvedValue({ maxFileBytes: 0, maxInlineImagesTotalBytes: "x", directTransferThresholdBytes: -1 });
    const { result } = renderHook(() => ({ q: useGetAttachmentLimits(), limits: useAttachmentLimits() }), { wrapper });
    await waitFor(() => expect(result.current.q.isSuccess).toBe(true));
    expect(result.current.limits).toEqual({ maxFileBytes: 10 * MB, maxInlineImagesBytes: 1 * MB, directTransferThresholdBytes: 0 });
  });
});
