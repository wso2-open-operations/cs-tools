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
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import useAttachmentLimits, {
  DEFAULT_ATTACHMENT_LIMITS,
} from "@api/useAttachmentLimits";

const MIB = 1024 * 1024;
const mockAuthFetch = vi.fn();

vi.unmock("@api/useAttachmentLimits");

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => mockAuthFetch,
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: true, isLoading: false }),
}));

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

describe("useAttachmentLimits", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (window as unknown as { config: unknown }).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    };
  });

  it("returns the defaults until the limits load, then the server's", async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        maxFileBytes: 1024 * MIB,
        maxInlineImagesTotalBytes: 25 * MIB,
      }),
    });
    const { result } = renderHook(() => useAttachmentLimits(), {
      wrapper: createWrapper(),
    });
    expect(result.current).toEqual({ maxFileBytes: 10 * MIB, maxInlineImagesTotalBytes: 1 * MIB });
    await waitFor(() => expect(result.current.maxFileBytes).toBe(1024 * MIB));
    expect(result.current.maxInlineImagesTotalBytes).toBe(25 * MIB);
    expect(mockAuthFetch).toHaveBeenCalledWith("https://api.test/attachments/limits", {
      method: "GET",
    });
  });

  it("keeps the defaults when the request fails", async () => {
    mockAuthFetch.mockResolvedValue({ ok: false, statusText: "Server Error" });
    const { result } = renderHook(() => useAttachmentLimits(), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(mockAuthFetch).toHaveBeenCalled());
    expect(result.current).toEqual(DEFAULT_ATTACHMENT_LIMITS);
  });

  it("uses the default for a value that is missing or not a positive number", async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: async () => ({ maxFileBytes: 50 * MIB, maxInlineImagesTotalBytes: 0 }),
    });
    const { result } = renderHook(() => useAttachmentLimits(), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.maxFileBytes).toBe(50 * MIB));
    expect(result.current.maxInlineImagesTotalBytes).toBe(1 * MIB);
  });
});
