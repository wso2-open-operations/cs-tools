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
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const authFetchMock = vi.fn();

vi.mock("@hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => authFetchMock,
}));

import { useGetAlert, useGetSmartAlert } from "@features/csm-cases/api/useSnLinkEntities";

function wrapper({ children }: { children: ReactNode }): ReactNode {
  // Mirrors the app's real behaviour of retrying 502/503 only, so a retry storm
  // on these hooks would show up as extra fetch calls below.
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: (count, error) =>
          count < 2 && [502, 503].includes((error as { status?: number }).status ?? 0),
        retryDelay: 1,
      },
    },
  });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe("alert / smart alert lookups", () => {
  beforeEach(() => {
    authFetchMock.mockReset();
    window.config = {
      ...window.config,
      CSM_PORTAL_BACKEND_BASE_URL: "http://localhost:9090",
    } as typeof window.config;
  });

  it.each([
    ["alert", "/alerts/a-1", () => useGetAlert("a-1")],
    ["smart alert", "/smart-alerts/s-1", () => useGetSmartAlert("s-1")],
  ])("%s: a plain-text 404 body resolves to null, not an error, with no retry", async (_n, path, useHook) => {
    authFetchMock.mockResolvedValue(new Response("Not Found", { status: 404 }));
    const { result } = renderHook(useHook, { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toBeNull();
    expect(result.current.isError).toBe(false);
    expect(authFetchMock).toHaveBeenCalledTimes(1);
    expect(authFetchMock.mock.calls[0][0]).toBe(`http://localhost:9090${path}`);
  });

  it.each([
    ["alert", () => useGetAlert("a-1")],
    ["smart alert", () => useGetSmartAlert("s-1")],
  ])("%s: a 503 is an error but is not retried", async (_n, useHook) => {
    authFetchMock.mockImplementation(() => Promise.resolve(new Response("unavailable", { status: 503 })));
    const { result } = renderHook(useHook, { wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(authFetchMock).toHaveBeenCalledTimes(1);
  });
});
