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

import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { ApiQueryKeys } from "@constants/apiConstants";

const patchMock = vi.fn();

// The real client reads runtime config at module load, which is not present under vitest.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: vi.fn(), patch: patchMock }),
}));

import { usePatchCsmCaseCallRequest } from "@features/csm-cases/api/useCsmCaseCallRequests";

const LIST = [ApiQueryKeys.CASE_CALL_REQUESTS, "case-1", []];
const OTHER_CASE_LIST = [ApiQueryKeys.CASE_CALL_REQUESTS, "case-2", []];
const WIDGET = ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA;
const CALL_WIDGETS = [
  [WIDGET, "my-calls", "call_request", {}, 5, 0, undefined],
  [WIDGET, "pie-slice", "calls-by-state", "call_request", { state: "scheduled" }],
];
const OTHER_WIDGETS = [[WIDGET, "my-cases", "case", {}, 5, 0, undefined]];

const stale = (qc: QueryClient, key: readonly unknown[]): boolean =>
  qc.getQueryState(key)?.isInvalidated === true;

// The mutation hook plus an ACTIVE observer of the case's list, so "the list was
// refetched" is something a test can count rather than assume.
function setup() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const listFetches = vi.fn().mockResolvedValue([]);
  for (const key of [...CALL_WIDGETS, ...OTHER_WIDGETS, OTHER_CASE_LIST]) qc.setQueryData(key, { seeded: true });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
  const hook = renderHook(
    () => ({
      list: useQuery({ queryKey: LIST, queryFn: listFetches, staleTime: Infinity }),
      patch: usePatchCsmCaseCallRequest(),
    }),
    { wrapper },
  );
  return { qc, listFetches, ...hook };
}

describe("usePatchCsmCaseCallRequest", () => {
  // Braces matter: a function returned from beforeEach is run as the test's cleanup, and
  // mockReset() returns the mock itself, so it would be called once more after the test.
  beforeEach(() => {
    patchMock.mockReset();
  });

  it("PATCHes the call request under its case", async () => {
    patchMock.mockResolvedValue({});
    const { result } = setup();
    await waitFor(() => expect(result.current.list.isSuccess).toBe(true));

    await act(async () => {
      await result.current.patch.mutateAsync({ caseId: "case 1", callRequestId: "cr/1", patch: { state: "concluded" } });
    });

    expect(patchMock).toHaveBeenCalledWith("/cases/case%201/call-requests/cr%2F1", { state: "concluded" });
  });

  it("holds the action open until the case's list has been refetched, and marks only call-request dashboard widgets stale", async () => {
    patchMock.mockResolvedValue({});
    const { qc, listFetches, result } = setup();
    await waitFor(() => expect(result.current.list.isSuccess).toBe(true));
    expect(listFetches).toHaveBeenCalledTimes(1);

    // The refetch stays in flight until released, so "resolved too early" is observable.
    let release: () => void = () => undefined;
    listFetches.mockImplementationOnce(
      () =>
        new Promise<never[]>((resolve) => {
          release = () => resolve([]);
        }),
    );
    let settled = false;
    let done: Promise<void> = Promise.resolve();
    act(() => {
      done = result.current.patch
        .mutateAsync({ caseId: "case-1", callRequestId: "cr-1", patch: { state: "concluded" } })
        .then(() => {
          settled = true;
        });
    });

    await waitFor(() => expect(listFetches).toHaveBeenCalledTimes(2));
    await new Promise((resolve) => setTimeout(resolve, 25));
    // The row the engineer just changed must be up to date before the action ends,
    // so its buttons cannot be clicked again on stale data.
    expect(settled).toBe(false);

    release();
    await act(async () => {
      await done;
    });
    expect(settled).toBe(true);

    for (const key of CALL_WIDGETS) expect(stale(qc, key), JSON.stringify(key)).toBe(true);
    for (const key of OTHER_WIDGETS) expect(stale(qc, key), JSON.stringify(key)).toBe(false);
    expect(stale(qc, OTHER_CASE_LIST)).toBe(false);
  });

  it("refetches the list after a refusal too, since the row on screen is probably out of date", async () => {
    patchMock.mockRejectedValue(new Error("a call request can only be marked completed while it is scheduled or notes pending"));
    const { qc, listFetches, result } = setup();
    await waitFor(() => expect(result.current.list.isSuccess).toBe(true));

    act(() => {
      result.current.patch.mutate({ caseId: "case-1", callRequestId: "cr-1", patch: { state: "concluded" } });
    });

    await waitFor(() => expect(result.current.patch.isError).toBe(true));
    expect(result.current.patch.error?.message).toMatch(/only be marked completed/);
    await waitFor(() => expect(listFetches).toHaveBeenCalledTimes(2));
    // Nothing changed on the server, so the dashboards are left alone.
    for (const key of [...CALL_WIDGETS, ...OTHER_WIDGETS]) expect(stale(qc, key), JSON.stringify(key)).toBe(false);
  });
});
