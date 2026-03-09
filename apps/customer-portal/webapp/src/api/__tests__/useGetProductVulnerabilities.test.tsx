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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useGetProductVulnerabilities } from "@api/useGetProductVulnerabilities";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

const mockVulnerabilityResponse = {
  vulnerabilityId: "XRAY-999003",
  cveId: "CVE-2099-3333",
  componentType: "library",
  updateLevel: "2.6.1",
  severity: { id: 1, label: "Low" },
};

const mockAuthFetch = vi.fn().mockResolvedValue({
  ok: true,
  json: () => Promise.resolve(mockVulnerabilityResponse),
  status: 200,
} as Response);

const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
};
vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
    isSignedIn: true,
    isLoading: false,
  }),
}));

describe("useGetProductVulnerabilities", () => {
  let queryClient: QueryClient;
  const originalConfig = window.config;

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockVulnerabilityResponse),
      status: 200,
    } as Response);
    (
      window as unknown as {
        config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string };
      }
    ).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    };
    vi.clearAllMocks();
  });

  afterEach(() => {
    window.config = originalConfig;
    vi.unstubAllGlobals();
  });

  it("returns vulnerability from API", async () => {
    const { result } = renderHook(
      () => useGetProductVulnerabilities("XRAY-999003"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeDefined();
    expect(result.current.data).toEqual(mockVulnerabilityResponse);
    expect(result.current.data?.vulnerabilityId).toBe("XRAY-999003");
    expect(result.current.data?.cveId).toBe("CVE-2099-3333");
    expect(result.current.data?.componentType).toBe("library");
    expect(result.current.data?.updateLevel).toBe("2.6.1");
    expect(result.current.data?.severity.label).toBe("Low");
    expect(mockAuthFetch).toHaveBeenCalledWith(
      expect.stringContaining("/products/vulnerabilities/XRAY-999003"),
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("throws when CUSTOMER_PORTAL_BACKEND_BASE_URL is missing", async () => {
    (window as unknown as { config?: unknown }).config = {};

    const { result } = renderHook(
      () => useGetProductVulnerabilities("XRAY-999003"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe(
      "CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured",
    );
  });

  it("throws when API response is not ok", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: false,
      statusText: "Not Found",
      status: 404,
    } as Response);

    const { result } = renderHook(
      () => useGetProductVulnerabilities("XRAY-999003"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toContain(
      "Error fetching product vulnerability",
    );
  });

  it("does not fetch when vulnerabilityId is empty", () => {
    const { result } = renderHook(() => useGetProductVulnerabilities(""), {
      wrapper,
    });

    expect(result.current.isPending).toBe(true);
    expect(result.current.fetchStatus).toBe("idle");
  });

  it("uses correct query key", () => {
    renderHook(() => useGetProductVulnerabilities("XRAY-999003"), {
      wrapper,
    });
    const query = queryClient.getQueryCache().findAll({
      queryKey: ["product-vulnerability", "XRAY-999003"],
    })[0];
    expect(query).toBeDefined();
  });
});
