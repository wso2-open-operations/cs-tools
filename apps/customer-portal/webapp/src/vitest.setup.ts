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

import { vi } from "vitest";
import "@testing-library/jest-dom";

// public/config.js is generated per environment and absent in CI/test runs, but
// modules such as src/config/apiConfig.ts read window.config at import time and
// throw when it is missing. Provide dummy loopback values; individual tests can
// still override window.config.
window.config = window.config ?? {
  CUSTOMER_PORTAL_AUTH_BASE_URL: "http://localhost:9000/auth",
  CUSTOMER_PORTAL_AUTH_CLIENT_ID: "test-client-id",
  CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL: "http://localhost:3000",
  CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL: "http://localhost:3000",
  CUSTOMER_PORTAL_BACKEND_BASE_URL: "http://localhost:9000/api",
  CUSTOMER_PORTAL_THEME: "default",
  CUSTOMER_PORTAL_LOG_LEVEL: "error",
  CUSTOMER_PORTAL_MAINTENANCE_BANNER_VISIBLE: false,
};

// The attachment limits come from the server through a query. Most tests render
// components that read them without a QueryClientProvider, so they get the
// built-in defaults here. A test that needs other limits mocks this module
// itself; the hook's own test calls `vi.unmock` to use the real one.
vi.mock("@api/useAttachmentLimits", () => ({
  default: () => ({ maxFileBytes: 10 * 1024 * 1024, maxInlineImagesTotalBytes: 1024 * 1024 }),
}));

vi.mock("dompurify", async (importOriginal) => {
  const actual = await importOriginal<typeof import("dompurify")>();
  return {
    default: {
      ...actual.default,
      sanitize: actual.default.sanitize.bind(actual.default),
      addHook: vi.fn(),
      removeHook: vi.fn(),
    },
  };
});

// Mock Asgardeo to avoid buffer resolution issues in tests
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isSignedIn: true,
    isLoading: false,
    state: {},
    signIn: vi.fn(),
    signOut: vi.fn(),
    getAccessToken: vi.fn().mockResolvedValue("mock-access-token-123"),
    getIdToken: vi.fn().mockResolvedValue("mock-token-123"),
  }),
  AsgardeoProvider: ({ children }: { children: unknown }) => children,
}));

// Globally mock useAuthApiClient to avoid unexpected native fetch executions inside UI component tests
export const mockAuthFetch = vi.fn().mockResolvedValue({
  ok: true,
  json: () => Promise.resolve({}),
});

vi.mock("@api/useAuthApiClient", () => ({
  useAuthApiClient: () => mockAuthFetch,
}));

vi.mock("@features/support/hooks/useResolvedInlineImageHtml", () => ({
  useResolvedInlineImageHtml: (html: string) => ({
    resolvedHtml: html,
    isLoading: false,
  }),
}));

vi.mock("@utils/useDarkMode", () => ({
  useDarkMode: () => false,
}));
