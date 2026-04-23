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

import { renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { ReactNode } from "react";
import { usePostCallRequest } from "@features/support/api/usePostCallRequest";
import type { CreateCallRequest } from "@features/support/types/calls";

const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
};
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

let mockIsSignedIn = true;
let mockIsAuthLoading = false;
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isSignedIn: mockIsSignedIn,
    isLoading: mockIsAuthLoading,
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
  }),
}));

describe("usePostCallRequest", () => {
  let queryClient: QueryClient;
  const originalConfig = window.config;
  const projectId = "proj-123";
  const caseId = "case-456";
  const requestBody: CreateCallRequest = {
    durationInMinutes: 30,
    reason: "Discuss configuration changes",
    utcTimes: ["2026-02-19T10:00:00Z"],
  };

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  beforeEach(() => {
    queryClient = new QueryClient();
    mockIsSignedIn = true;
    mockIsAuthLoading = false;
    vi.clearAllMocks();
    vi.stubGlobal("config", {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    });
  });

  afterEach(() => {
    window.config = originalConfig;
    vi.unstubAllGlobals();
  });

  it("should create a call request successfully", async () => {
    const mockResponse = { id: "call-123" };
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      json: () => Promise.resolve(mockResponse),
    } as Response);
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => usePostCallRequest(projectId, caseId), {
      wrapper,
    });

    const data = await result.current.mutateAsync(requestBody);

    expect(data).toEqual(mockResponse);
    expect(mockFetch).toHaveBeenCalledWith(
      `https://api.test/cases/${caseId}/call-requests`,
      expect.objectContaining({
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(requestBody),
      }),
    );
  });

  it("should invalidate CASE_CALL_REQUESTS query on successful mutation", async () => {
    const mockResponse = { id: "call-123" };
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      json: () => Promise.resolve(mockResponse),
    } as Response);
    vi.stubGlobal("fetch", mockFetch);

    const invalidateQueriesSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => usePostCallRequest(projectId, caseId), {
      wrapper,
    });

    await result.current.mutateAsync(requestBody);

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({
      queryKey: ["case-call-requests", projectId, caseId],
    });
  });

  it("should handle API errors", async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      statusText: "Bad Request",
      text: () => Promise.resolve("Invalid payload"),
    } as Response);
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => usePostCallRequest(projectId, caseId), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestBody)).rejects.toThrow(
      "Error creating call request: 400 Bad Request - Invalid payload",
    );
  });

  it("should throw when CUSTOMER_PORTAL_BACKEND_BASE_URL is missing", async () => {
    vi.stubGlobal("config", {});

    const { result } = renderHook(() => usePostCallRequest(projectId, caseId), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestBody)).rejects.toThrow(
      "CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured",
    );
  });

  it("should throw when user is not signed in", async () => {
    mockIsSignedIn = false;

    const { result } = renderHook(() => usePostCallRequest(projectId, caseId), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestBody)).rejects.toThrow(
      "User must be signed in to create a call request",
    );
  });
});
