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
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();

vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
// The ID-token claims decode asynchronously in the app, so the email is
// undefined on the first render and present a tick later.
let mockEmail: string | undefined;
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => (mockEmail ? { email: mockEmail } : undefined),
}));
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" } }),
}));

import { useGetCsmCases } from "@features/csm-cases/api/useGetCsmCases";
import { DEFAULT_CASES_FILTERS } from "@features/csm-cases/utils/casesFiltersUrl";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useGetCsmCases — signed-in email arriving after the first render", () => {
  beforeEach(() => {
    postMock.mockReset();
    mockEmail = undefined;
  });

  it("re-derives assigneeIsMe without refetching the search", async () => {
    postMock.mockResolvedValue({
      cases: [
        {
          id: "c1",
          number: "C1",
          subject: "s",
          state: "new",
          assignedEngineer: { email: "Jane.Doe@example.com", name: "Jane Doe" },
        },
      ],
      total: 1,
      limit: 20,
      offset: 0,
    });

    const { result, rerender } = renderHook(
      () => useGetCsmCases({ ...DEFAULT_CASES_FILTERS }, 0, 20),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.cases[0].assigneeIsMe).toBe(false);

    mockEmail = "jane.doe@example.com";
    rerender();

    await waitFor(() => expect(result.current.data?.cases[0].assigneeIsMe).toBe(true));
    expect(postMock).toHaveBeenCalledTimes(1);
  });
});

describe("useGetCsmCases — list request body", () => {
  beforeEach(() => {
    postMock.mockReset();
    mockEmail = undefined;
  });

  it("asks the backend to leave the description out of each row", async () => {
    postMock.mockResolvedValue({ cases: [], total: 0, limit: 20, offset: 0 });
    const { result } = renderHook(
      () => useGetCsmCases({ ...DEFAULT_CASES_FILTERS }, 0, 20),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(postMock.mock.calls[0][1].excludeFields).toEqual(["description"]);
  });
});
