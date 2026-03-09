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

// Mock Asgardeo to avoid buffer resolution issues in tests
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isSignedIn: true,
    isLoading: false,
    state: {},
    signIn: vi.fn(),
    signOut: vi.fn(),
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
