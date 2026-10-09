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
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useQuickCaseSearch.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useSearchGroups, useSearchSupportGroups } from "@api/useSearchGroups";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useSearchGroups", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ groups: [] });
  });

  it("fires with an empty query as soon as the caller enables it (dropdown opened, nothing typed)", async () => {
    const { result } = renderHook(() => useSearchGroups("", true), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/groups/search",
      expect.objectContaining({ filters: { searchQuery: "" } }),
    );
  });

  it("does not fire while the caller keeps it disabled (dropdown closed)", () => {
    renderHook(() => useSearchGroups("", false), { wrapper });
    expect(postMock).not.toHaveBeenCalled();
  });

  it("sends exactly the old body when no options are given", async () => {
    const { result } = renderHook(() => useSearchGroups("ops", true), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(postMock).toHaveBeenCalledWith("/groups/search", {
      filters: { searchQuery: "ops" },
      pagination: { offset: 0, limit: 20 },
    });
  });

  it("ignores a string third argument (AsyncEntitySelect's searchExtra)", async () => {
    const { result } = renderHook(() => useSearchGroups("ops", true, "extra"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(postMock).toHaveBeenCalledWith(
      "/groups/search",
      expect.objectContaining({ filters: { searchQuery: "ops" } }),
    );
  });

  it("adds supportGroupsOnly to the filters when asked", async () => {
    const { result } = renderHook(
      () => useSearchGroups("ops", true, { supportGroupsOnly: true }),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(postMock).toHaveBeenCalledWith(
      "/groups/search",
      expect.objectContaining({ filters: { searchQuery: "ops", supportGroupsOnly: true } }),
    );
  });

  it("caches the narrowed search separately from the plain one", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const shared = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const plain = renderHook(() => useSearchGroups("", true), { wrapper: shared });
    await waitFor(() => expect(plain.result.current.isSuccess).toBe(true));
    const narrowed = renderHook(() => useSearchSupportGroups("", true), { wrapper: shared });
    await waitFor(() => expect(narrowed.result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledTimes(2);
    expect(postMock).toHaveBeenLastCalledWith(
      "/groups/search",
      expect.objectContaining({ filters: { searchQuery: "", supportGroupsOnly: true } }),
    );
  });
});
