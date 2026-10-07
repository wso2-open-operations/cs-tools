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
// under vitest (same approach as useDecideChangeRequestApproval.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import {
  classifyQuickCaseQuery,
  useQuickCaseSearch,
} from "@features/csm-cases/api/useQuickCaseSearch";
import { ALL_CASE_TYPES } from "@features/csm-cases/utils/caseType";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useQuickCaseSearch", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ cases: [] });
  });

  it("asks the server not to count every match: the palette never shows a total", async () => {
    const { result } = renderHook(() => useQuickCaseSearch("printer jam"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      expect.objectContaining({ skipTotal: true }),
    );
  });

  it("requests every known case sub-type, not just the default 'case' type", async () => {
    // "free text" query — doesn't match the number/internalId patterns,
    // so this goes through the free-text `searchQuery` path.
    const { result } = renderHook(() => useQuickCaseSearch("printer jam"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      expect.objectContaining({
        filters: expect.objectContaining({
          searchQuery: "printer jam",
          filters: [{ field: "type", op: "in", values: ALL_CASE_TYPES }],
        }),
      }),
    );
    // Guards against the search silently regressing to only `case` hits again
    // (the SRA-missing-from-quick-nav bug) if `ALL_CASE_TYPES` ever shrinks.
    expect(ALL_CASE_TYPES).toEqual(
      expect.arrayContaining([
        "case",
        "service_request",
        "security_report_analysis",
        "announcement",
        "engagement",
      ]),
    );
  });

  it("does not fire a search until the query reaches the minimum length", () => {
    renderHook(() => useQuickCaseSearch("a"), { wrapper });
    expect(postMock).not.toHaveBeenCalled();
  });

  it("routes a CS case number to an exact-match number filter, not searchQuery", async () => {
    const { result } = renderHook(() => useQuickCaseSearch("CS0441174"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const body = postMock.mock.calls[0][1];
    expect(body.filters.searchQuery).toBeUndefined();
    expect(body.filters.filters).toEqual([
      { field: "type", op: "in", values: ALL_CASE_TYPES },
      { field: "number", op: "eq", values: ["CS0441174"] },
    ]);
  });

  it("routes a WSO2 case id to an exact-match internalId filter, not searchQuery", async () => {
    const { result } = renderHook(() => useQuickCaseSearch("SOMEID-4"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const body = postMock.mock.calls[0][1];
    expect(body.filters.searchQuery).toBeUndefined();
    expect(body.filters.filters).toEqual([
      { field: "type", op: "in", values: ALL_CASE_TYPES },
      { field: "internalId", op: "eq", values: ["SOMEID-4"] },
    ]);
  });

  it("falls back to free-text search when forceFreeText is set, even for a matching query", async () => {
    const { result } = renderHook(
      () => useQuickCaseSearch("CS0441174", { forceFreeText: true }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      expect.objectContaining({
        filters: expect.objectContaining({
          searchQuery: "CS0441174",
          filters: [{ field: "type", op: "in", values: ALL_CASE_TYPES }],
        }),
      }),
    );
  });

  describe("classifyQuickCaseQuery", () => {
    it("classifies a CS case number", () => {
      expect(classifyQuickCaseQuery("CS0441174")).toBe("number");
    });

    it("classifies a WSO2 case id", () => {
      expect(classifyQuickCaseQuery("SOMEID-4")).toBe("internalId");
      expect(classifyQuickCaseQuery("ABC-123")).toBe("internalId");
      expect(classifyQuickCaseQuery("wso2-1")).toBe("internalId");
    });

    it("classifies free text as text", () => {
      expect(classifyQuickCaseQuery("printer jam")).toBe("text");
      // Wrong digit count for a case number.
      expect(classifyQuickCaseQuery("CS044117")).toBe("text");
      expect(classifyQuickCaseQuery("CS04411745")).toBe("text");
      // Too many digits after the hyphen for a WSO2 case id.
      expect(classifyQuickCaseQuery("SOMEID-12345")).toBe("text");
      // No digits after the hyphen at all.
      expect(classifyQuickCaseQuery("SOMEID-")).toBe("text");
      // Lowercase "cs" prefix fails the strict, case-sensitive case-number
      // shape, and there's no hyphen for it to match the internalId shape.
      expect(classifyQuickCaseQuery("cs0441174")).toBe("text");
    });
  });
});
