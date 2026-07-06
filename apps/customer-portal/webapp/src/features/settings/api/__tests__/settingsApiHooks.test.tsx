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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import useGetProjectContacts from "@features/settings/api/useGetProjectContacts";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import { useCreateRegistryToken } from "@features/settings/api/useCreateRegistryToken";
import { useDeleteProjectContact } from "@features/settings/api/useDeleteProjectContact";
import { useDeleteRegistryToken } from "@features/settings/api/useDeleteRegistryToken";
import { useGetIntegrationUsers } from "@features/settings/api/useGetIntegrationUsers";
import { usePatchProject } from "@features/settings/api/usePatchProject";
import { usePatchProjectContact } from "@features/settings/api/usePatchProjectContact";
import { usePatchUserMe } from "@features/settings/api/usePatchUserMe";
import { usePostProjectContact } from "@features/settings/api/usePostProjectContact";
import { useRegenerateRegistryToken } from "@features/settings/api/useRegenerateRegistryToken";
import { useSearchRegistryTokens } from "@features/settings/api/useSearchRegistryTokens";
import { useValidateProjectContact } from "@features/settings/api/useValidateProjectContact";

const { authFetchMock, setUserPreferredTimeZoneMock } = vi.hoisted(() => ({
  authFetchMock: vi.fn(),
  setUserPreferredTimeZoneMock: vi.fn(),
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: true, isLoading: false }),
}));

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => authFetchMock,
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn(), warn: vi.fn() }),
}));

vi.mock("@utils/dateTime", async () => {
  const actual = await vi.importActual("@utils/dateTime");
  return {
    ...actual,
    setUserPreferredTimeZone: setUserPreferredTimeZoneMock,
  };
});

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("settings API hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (
      window as unknown as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }
    ).config = { CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test" };
  });

  it("queries contacts for project", async () => {
    authFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => [{ id: "1", email: "u@test.dev" }],
    });
    const { result } = renderHook(() => useGetProjectContacts("p-1"), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetchMock).toHaveBeenCalledWith(
      "https://api.test/projects/p-1/contacts",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("queries user details and sets preferred timezone", async () => {
    authFetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ id: "u-1", timezone: "Asia/Colombo" }),
    });
    const { result } = renderHook(() => useGetUserDetails(), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(setUserPreferredTimeZoneMock).toHaveBeenCalledWith("Asia/Colombo");
  });

  it("queries integration users when enabled", async () => {
    authFetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => [{ id: "i-1", email: "bot@test.dev" }],
    });
    const { result } = renderHook(() => useGetIntegrationUsers("p-1"), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetchMock).toHaveBeenCalledWith(
      "https://api.test/projects/p-1/integration-users",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("posts a new project contact", async () => {
    authFetchMock.mockResolvedValueOnce({ ok: true, status: 200 });
    const { result } = renderHook(() => usePostProjectContact("p-1"), {
      wrapper: createWrapper(),
    });
    await result.current.mutateAsync({ contactEmail: "new@test.dev" } as never);
    expect(authFetchMock).toHaveBeenCalledWith(
      "https://api.test/projects/p-1/contacts",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("patches an existing project contact", async () => {
    authFetchMock.mockResolvedValueOnce({ ok: true, status: 200 });
    const { result } = renderHook(() => usePatchProjectContact("p-1"), {
      wrapper: createWrapper(),
    });
    await result.current.mutateAsync({
      email: "abc+1@test.dev",
      isCsAdmin: true,
      isLead: false,
      isPortalUser: true,
      isSecurityContact: false,
    });
    expect(authFetchMock).toHaveBeenCalledWith(
      "https://api.test/projects/p-1/contacts/abc%2B1%40test.dev",
      expect.objectContaining({ method: "PATCH" }),
    );
  });

  it("deletes a project contact", async () => {
    authFetchMock.mockResolvedValueOnce({ ok: true, status: 200 });
    const { result } = renderHook(() => useDeleteProjectContact("p-1"), {
      wrapper: createWrapper(),
    });
    await result.current.mutateAsync("delete@test.dev");
    expect(authFetchMock).toHaveBeenCalledWith(
      "https://api.test/projects/p-1/contacts/delete%40test.dev",
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("patches current user profile", async () => {
    authFetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ timeZone: "UTC" }),
    });
    const { result } = renderHook(() => usePatchUserMe(), { wrapper: createWrapper() });
    await result.current.mutateAsync({ timeZone: "UTC" });
    expect(authFetchMock).toHaveBeenCalledWith(
      "https://api.test/users/me",
      expect.objectContaining({ method: "PATCH" }),
    );
  });

  it("patches project settings", async () => {
    authFetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ id: "p-1" }),
    });
    const { result } = renderHook(() => usePatchProject("p-1"), {
      wrapper: createWrapper(),
    });
    await result.current.mutateAsync({ hasAgent: true });
    expect(authFetchMock).toHaveBeenCalledWith(
      "https://api.test/projects/p-1",
      expect.objectContaining({ method: "PATCH" }),
    );
  });

  it("searches registry tokens", async () => {
    authFetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => [],
    });
    const { result } = renderHook(() => useSearchRegistryTokens("p-1"), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetchMock).toHaveBeenCalledWith(
      "https://api.test/projects/p-1/registry-tokens/search",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("creates/regenerates/deletes registry tokens", async () => {
    authFetchMock
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ secret: "s1" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ secret: "s2" }),
      })
      .mockResolvedValueOnce({ ok: true, status: 200 });
    const wrapper = createWrapper();
    const create = renderHook(() => useCreateRegistryToken("p-1"), { wrapper });
    const regen = renderHook(() => useRegenerateRegistryToken("p-1"), { wrapper });
    const del = renderHook(() => useDeleteRegistryToken("p-1"), { wrapper });
    await create.result.current.mutateAsync({ robotName: "bot", tokenType: "UserToken" } as never);
    await regen.result.current.mutateAsync(42);
    await del.result.current.mutateAsync(42);
    expect(authFetchMock).toHaveBeenNthCalledWith(
      1,
      "https://api.test/projects/p-1/registry-tokens",
      expect.objectContaining({ method: "POST" }),
    );
    expect(authFetchMock).toHaveBeenNthCalledWith(
      2,
      "https://api.test/registry-tokens/42/regenerate",
      expect.objectContaining({ method: "POST" }),
    );
    expect(authFetchMock).toHaveBeenNthCalledWith(
      3,
      "https://api.test/registry-tokens/42",
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("validates a project contact", async () => {
    authFetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ isContactValid: true }),
    });
    const { result } = renderHook(() => useValidateProjectContact("p-1"), {
      wrapper: createWrapper(),
    });
    await result.current.mutateAsync({ contactEmail: "ok@test.dev" });
    expect(authFetchMock).toHaveBeenCalledWith(
      "https://api.test/projects/p-1/contacts/validate",
      expect.objectContaining({ method: "POST" }),
    );
  });
});

