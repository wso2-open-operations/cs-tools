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

vi.mock("@/constants/apiConstants", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    API_MOCK_DELAY: 0,
  };
});

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

let mockIsMockEnabled = true;
vi.mock("@/providers/MockConfigProvider", () => ({
  useMockConfig: () => ({
    isMockEnabled: mockIsMockEnabled,
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
    environments: ["Production"],
    productDetails: ["WSO2 Identity Server - v6.1.0"],
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
    mockIsMockEnabled = true;
    mockIsSignedIn = true;
    mockIsAuthLoading = false;
    vi.clearAllMocks();
  });

  afterEach(() => {
    window.config = originalConfig;
    vi.unstubAllGlobals();
  });

  it("returns mock data when isMockEnabled is true", async () => {
    const { result } = renderHook(() => usePostCaseClassifications(), {
      wrapper,
    });

    const data = await result.current.mutateAsync(requestBody);

    expect(data.issueType).toBe("Question");
    expect(data.severityLevel).toBe("S4");
    expect(data.case_info.productName).toBe("WSO2 Identity Server");
    expect(data.case_info.productVersion).toBe("v6.1.0");
  });

  it("posts to API when isMockEnabled is false", async () => {
    mockIsMockEnabled = false;
    const mockResponse = {
      issueType: "Question",
      severityLevel: "S4",
      case_info: {
        description: "desc",
        shortDescription: "short",
        productName: "WSO2 Identity Server",
        productVersion: "v6.1.0",
        environment: "Production",
        tier: "Enterprise",
        region: "",
      },
    };
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
      status: 200,
    } as Response);

    window.config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    } as typeof window.config;
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => usePostCaseClassifications(), {
      wrapper,
    });

    const data = await result.current.mutateAsync(requestBody);

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/cases/classify"),
      expect.objectContaining({
        method: "POST",
        headers: expect.any(Object),
      }),
    );
    expect(data).toEqual(mockResponse);
  });

  it("throws when CUSTOMER_PORTAL_BACKEND_BASE_URL is missing", async () => {
    mockIsMockEnabled = false;
    window.config = {} as typeof window.config;

    const { result } = renderHook(() => usePostCaseClassifications(), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestBody)).rejects.toThrow(
      "CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured",
    );
  });

  it("throws when API response is not ok", async () => {
    mockIsMockEnabled = false;
    const mockFetch = vi.fn().mockResolvedValue({
      ok: false,
      statusText: "Internal Server Error",
      status: 500,
    } as Response);

    window.config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    } as typeof window.config;
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => usePostCaseClassifications(), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestBody)).rejects.toThrow(
      "Error classifying case details: 500 Internal Server Error",
    );
  });

  it("throws when user is not signed in", async () => {
    mockIsMockEnabled = false;
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
