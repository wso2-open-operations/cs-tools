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
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import type { ReactNode } from "react";
import { useGetAffectedProductVulnerabilities } from "@features/security/api/useGetAffectedProductVulnerabilities";

const mockLogger = { debug: vi.fn(), error: vi.fn() };
vi.mock("@hooks/useLogger", () => ({ useLogger: () => mockLogger }));

let mockIsSignedIn = true;
let mockIsAuthLoading = false;
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: mockIsSignedIn, isLoading: mockIsAuthLoading }),
}));

const mockAuthFetch = vi.fn();
vi.mock("@/hooks/useAuthApiClient", () => ({ useAuthApiClient: () => mockAuthFetch }));

const record = (id: string) => ({ id, cveId: "CVE-2024-0001" });

/** Answers each search request with the slice of `total` records its offset and limit select. */
const serveRecords = (total: number, idFor: (index: number) => string = (i) => `r${i}`) => {
  mockAuthFetch.mockImplementation((_url: string, init: { body: string }) => {
    const { pagination } = JSON.parse(init.body);
    const count = Math.max(Math.min(pagination.limit, total - pagination.offset), 0);
    const productVulnerabilities = Array.from({ length: count }, (_, i) =>
      record(idFor(pagination.offset + i)),
    );
    return Promise.resolve({
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          productVulnerabilities,
          totalRecords: total,
          limit: pagination.limit,
          offset: pagination.offset,
        }),
    } as Response);
  });
};

const bodies = () =>
  mockAuthFetch.mock.calls.map(([, init]) => JSON.parse((init as { body: string }).body));

describe("useGetAffectedProductVulnerabilities", () => {
  let queryClient: QueryClient;

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  beforeEach(() => {
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockIsSignedIn = true;
    mockIsAuthLoading = false;
    vi.clearAllMocks();
    mockAuthFetch.mockReset();
    vi.stubGlobal("config", { CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test" });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("loads a CVE that fits in one page with a single request carrying the cveId filter", async () => {
    serveRecords(3);
    const { result } = renderHook(() => useGetAffectedProductVulnerabilities("CVE-2024-0001"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.productVulnerabilities.map((r) => r.id)).toEqual(["r0", "r1", "r2"]);
    expect(result.current.data?.totalRecords).toBe(3);
    expect(mockAuthFetch).toHaveBeenCalledTimes(1);
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/products/vulnerabilities/search",
      expect.objectContaining({ method: "POST" }),
    );
    expect(bodies()[0]).toEqual({
      filters: { cveId: "CVE-2024-0001" },
      pagination: { offset: 0, limit: 50 },
    });
  });

  it("requests the remaining pages at the right offsets and concatenates them in order", async () => {
    serveRecords(120);
    const { result } = renderHook(() => useGetAffectedProductVulnerabilities("CVE-2024-0001"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(bodies().map((b) => b.pagination)).toEqual([
      { offset: 0, limit: 50 },
      { offset: 50, limit: 50 },
      { offset: 100, limit: 50 },
    ]);
    expect(bodies().every((b) => b.filters.cveId === "CVE-2024-0001" && b.pagination.limit <= 50)).toBe(true);
    const ids = result.current.data?.productVulnerabilities.map((r) => r.id);
    expect(ids).toHaveLength(120);
    expect(ids?.[0]).toBe("r0");
    expect(ids?.[119]).toBe("r119");
    expect(result.current.data?.totalRecords).toBe(120);
  });

  it("does not request further pages when the first page holds every record", async () => {
    serveRecords(50);
    const { result } = renderHook(() => useGetAffectedProductVulnerabilities("CVE-2024-0001"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledTimes(1);
  });

  it("de-duplicates records that appear on more than one page", async () => {
    // A row shifting between pages while paging makes the same id show up twice.
    serveRecords(60, (i) => (i === 50 ? "r49" : `r${i}`));
    const { result } = renderHook(() => useGetAffectedProductVulnerabilities("CVE-2024-0001"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const ids = result.current.data?.productVulnerabilities.map((r) => r.id);
    expect(ids).toHaveLength(59);
    expect(new Set(ids).size).toBe(59);
    expect(result.current.data?.totalRecords).toBe(60);
  });

  it("stops at 10 pages and reports the larger total so the caller can tell it was truncated", async () => {
    serveRecords(900);
    const { result } = renderHook(() => useGetAffectedProductVulnerabilities("CVE-2024-0001"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledTimes(10);
    expect(Math.max(...bodies().map((b) => b.pagination.offset))).toBe(450);
    expect(result.current.data?.productVulnerabilities).toHaveLength(500);
    expect(result.current.data?.totalRecords).toBe(900);
  });

  it("trims the CVE id it sends", async () => {
    serveRecords(1);
    const { result } = renderHook(() => useGetAffectedProductVulnerabilities("  CVE-2024-0001 "), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(bodies()[0].filters.cveId).toBe("CVE-2024-0001");
  });

  it.each([
    ["undefined", undefined],
    ["empty", ""],
    ["blank", "   "],
  ])("stays idle while the CVE is %s", async (_label, cveId) => {
    const { result } = renderHook(() => useGetAffectedProductVulnerabilities(cveId), { wrapper });
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(mockAuthFetch).not.toHaveBeenCalled();
    expect(result.current.isPending).toBe(true);
    expect(result.current.fetchStatus).toBe("idle");
  });

  it("stays idle until the user is signed in", async () => {
    mockIsSignedIn = false;
    const { result } = renderHook(() => useGetAffectedProductVulnerabilities("CVE-2024-0001"), { wrapper });
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(mockAuthFetch).not.toHaveBeenCalled();
    expect(result.current.fetchStatus).toBe("idle");
  });

  it("keeps a separate cache entry per CVE", async () => {
    serveRecords(1);
    const { result, rerender } = renderHook(
      ({ cve }: { cve: string }) => useGetAffectedProductVulnerabilities(cve),
      { wrapper, initialProps: { cve: "CVE-2024-0001" } },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    rerender({ cve: "CVE-2024-0002" });
    await waitFor(() => expect(bodies().map((b) => b.filters.cveId)).toContain("CVE-2024-0002"));
    expect(
      queryClient.getQueryCache().findAll({ queryKey: ["product-vulnerabilities-affected"] }),
    ).toHaveLength(2);
  });

  it("propagates the error when the first page fails", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: false,
      status: 400,
      statusText: "Bad Request",
      text: () => Promise.resolve('{"message":"Invalid filter payload"}'),
    } as Response);
    const { result } = renderHook(() => useGetAffectedProductVulnerabilities("CVE-2024-0001"), { wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe("Invalid filter payload");
  });

  it("fails as a whole when a later page fails rather than returning a partial list", async () => {
    mockAuthFetch.mockImplementation((_url: string, init: { body: string }) => {
      const { pagination } = JSON.parse(init.body);
      if (pagination.offset === 50) {
        return Promise.resolve({
          ok: false,
          status: 500,
          statusText: "Internal Server Error",
          text: () => Promise.resolve(""),
        } as Response);
      }
      return Promise.resolve({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            productVulnerabilities: [record(`r${pagination.offset}`)],
            totalRecords: 120,
            limit: 50,
            offset: pagination.offset,
          }),
      } as Response);
    });
    const { result } = renderHook(() => useGetAffectedProductVulnerabilities("CVE-2024-0001"), { wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.data).toBeUndefined();
  });
});
