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

import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { useUpdateTimeCardBillable } from "@features/csm-timecards/api/useTimeCards";
import type { BeTimeCardView } from "@api/backend/types";

// Same module-load constraints as useTimeSheets.test.tsx: the real client and
// config read window.config, so both are mocked and only `patch` is exercised.
const patchMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
  useBackendApi: () => ({ patch: patchMock, post: vi.fn() }),
}));
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));
vi.mock("@features/settings/api/useGetUsersMe", () => ({
  useGetUsersMe: () => ({ data: undefined }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({ useIdTokenClaims: () => undefined }));

const CARD_ID = "00000000-0000-0000-0000-000000000001";

function beCard(hasBillable: boolean): BeTimeCardView {
  return {
    id: CARD_ID,
    totalTime: 30,
    workDate: "2026-07-13",
    createdOn: "2026-07-13",
    hasBillable,
    state: "approved",
    user: { id: "00000000-0000-0000-0000-000000000002", name: "Jane Doe" },
    project: { id: "proj-1", name: "Acme" },
    case: { id: "case-1", number: "CS0000001", name: "Issue" },
  };
}

describe("useUpdateTimeCardBillable", () => {
  let queryClient: QueryClient;
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  beforeEach(() => {
    patchMock.mockReset();
    queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  });

  it("PATCHes the card with a body holding the single key isBillable", async () => {
    patchMock.mockResolvedValue({ timeCard: beCard(false) });
    const { result } = renderHook(() => useUpdateTimeCardBillable(), { wrapper });

    act(() => {
      result.current.mutate({ cardId: CARD_ID, billable: false });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(patchMock).toHaveBeenCalledTimes(1);
    const [path, body] = patchMock.mock.calls[0] as [string, Record<string, unknown>];
    expect(path).toBe(`/time-cards/${CARD_ID}`);
    expect(body).toEqual({ isBillable: false });
    expect(Object.keys(body)).toEqual(["isBillable"]);
    expect(result.current.data?.billable).toBe(false);
  });

  it("invalidates the time card queries and the time-log report on success", async () => {
    patchMock.mockResolvedValue({ timeCard: beCard(true) });
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUpdateTimeCardBillable(), { wrapper });

    act(() => {
      result.current.mutate({ cardId: CARD_ID, billable: true });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const keys = spy.mock.calls.map((c) => (c[0]?.queryKey as string[])[0]);
    expect(keys).toEqual(
      expect.arrayContaining([
        "time-cards-search",
        "case-time-cards-search",
        "time-sheets-search",
        "time-card-approval-queue",
        "time-card-all",
        "spl-timelogs-breakdown",
      ]),
    );
  });

  it("does not invalidate anything and surfaces the error when the PATCH is rejected", async () => {
    patchMock.mockRejectedValue(new Error("forbidden"));
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUpdateTimeCardBillable(), { wrapper });

    act(() => {
      result.current.mutate({ cardId: CARD_ID, billable: false });
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(spy).not.toHaveBeenCalled();
  });
});
