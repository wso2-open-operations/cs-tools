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

import {
  classifyQuickChangeRequestQuery,
  useQuickChangeRequestSearch,
} from "@features/csm-operations/api/useQuickChangeRequestSearch";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useQuickChangeRequestSearch", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ changeRequests: [] });
  });

  it("asks the server not to count every match: the palette never shows a total", async () => {
    const { result } = renderHook(() => useQuickChangeRequestSearch("database upgrade"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith(
      "/change-requests/search",
      expect.objectContaining({ skipTotal: true }),
    );
  });

  it("sends free text as searchQuery for a non-number-shaped query", async () => {
    const { result } = renderHook(
      () => useQuickChangeRequestSearch("upgrade gateway"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/change-requests/search",
      expect.objectContaining({
        filters: { searchQuery: "upgrade gateway" },
      }),
    );
  });

  it("does not fire a search until the query reaches the minimum length", () => {
    renderHook(() => useQuickChangeRequestSearch("a"), { wrapper });
    expect(postMock).not.toHaveBeenCalled();
  });

  it("routes a CHG number to an exact-match number filter, not searchQuery", async () => {
    const { result } = renderHook(
      () => useQuickChangeRequestSearch("CHG0038721"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const body = postMock.mock.calls[0][1];
    expect(body.filters).toEqual({ number: "CHG0038721" });
  });

  it("falls back to free-text search when forceFreeText is set, even for a matching query", async () => {
    const { result } = renderHook(
      () =>
        useQuickChangeRequestSearch("CHG0038721", { forceFreeText: true }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/change-requests/search",
      expect.objectContaining({
        filters: { searchQuery: "CHG0038721" },
      }),
    );
  });

  describe("classifyQuickChangeRequestQuery", () => {
    it("classifies a CHG number", () => {
      expect(classifyQuickChangeRequestQuery("CHG0038721")).toBe("number");
    });

    it("classifies free text as text", () => {
      expect(classifyQuickChangeRequestQuery("upgrade gateway")).toBe("text");
      // Wrong digit count for a change-request number.
      expect(classifyQuickChangeRequestQuery("CHG003872")).toBe("text");
      expect(classifyQuickChangeRequestQuery("CHG00387212")).toBe("text");
      // Lowercase "chg" prefix fails the strict, case-sensitive shape.
      expect(classifyQuickChangeRequestQuery("chg0038721")).toBe("text");
      // A different prefix entirely.
      expect(classifyQuickChangeRequestQuery("PRB0040192")).toBe("text");
    });
  });
});
