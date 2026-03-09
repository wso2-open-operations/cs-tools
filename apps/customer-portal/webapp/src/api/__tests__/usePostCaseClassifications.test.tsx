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
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import type { ReactNode } from "react";
import { usePostCaseClassifications } from "@api/usePostCaseClassifications";
import type { CaseClassificationRequest } from "@models/requests";

const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
};
vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

const mockAuthFetch = vi.fn();

const mockGetIdToken = vi.fn().mockResolvedValue("mock-token");
let mockIsSignedIn = true;
let mockIsAuthLoading = false;
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: mockGetIdToken,
    isSignedIn: mockIsSignedIn,
    isLoading: mockIsAuthLoading,
  }),
}));

describe("usePostCaseClassifications", () => {
  let queryClient: QueryClient;
  const originalConfig = window.config;

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  const requestBody: CaseClassificationRequest = {
    chatHistory: "User: Hello\nAssistant: Hi",
    envProducts: {
      Production: ["WSO2 Identity Server - v6.1.0"],
    },
    region: "",
    tier: "Enterprise",
  };

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        mutations: {
          retry: false,
        },
      },
    });
    mockIsSignedIn = true;
    mockIsAuthLoading = false;
    vi.clearAllMocks();
  });

  afterEach(() => {
    window.config = originalConfig;
    vi.unstubAllGlobals();
  });

  it("posts to API and returns classification", async () => {
    const mockResponse = {
      issueType: "Question",
      severityLevel: "S4",
      caseInfo: {
        description: "desc",
        shortDescription: "short",
        productName: "WSO2 Identity Server",
        productVersion: "v6.1.0",
        environment: "Production",
        tier: "Enterprise",
        region: "",
      },
    };
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockResponse),
      status: 200,
    } as Response);

    window.config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    } as typeof window.config;

    const { result } = renderHook(() => usePostCaseClassifications(), {
      wrapper,
    });

    const data = await result.current.mutateAsync(requestBody);

    expect(mockAuthFetch).toHaveBeenCalledWith(
      expect.stringContaining("/cases/classify"),
      expect.objectContaining({ method: "POST" }),
    );
    expect(data).toEqual(mockResponse);
  });

  it("throws when CUSTOMER_PORTAL_BACKEND_BASE_URL is missing", async () => {
    window.config = {} as typeof window.config;

    const { result } = renderHook(() => usePostCaseClassifications(), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestBody)).rejects.toThrow(
      "CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured",
    );
  });

  it("throws when API response is not ok", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: false,
      statusText: "Internal Server Error",
      status: 500,
    } as Response);
    window.config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    } as typeof window.config;

    const { result } = renderHook(() => usePostCaseClassifications(), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestBody)).rejects.toThrow(
      "Error classifying case details: 500 Internal Server Error",
    );
  });

  it("throws when user is not signed in", async () => {
    mockIsSignedIn = false;

    window.config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    } as typeof window.config;

    const { result } = renderHook(() => usePostCaseClassifications(), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestBody)).rejects.toThrow(
      "User must be signed in to classify case details",
    );
  });
});
