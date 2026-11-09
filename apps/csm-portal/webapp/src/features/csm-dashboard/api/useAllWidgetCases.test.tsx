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
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useAllWidgetCases } from "@features/csm-dashboard/api/useAllWidgetCases";
import { resolveRelativeDatePlaceholder } from "@utils/resolveRelativeDatePlaceholder";

const filters = { filters: [{ field: "createdOn", op: "gte", values: ["__today__"] }] };

/** The `createdOn` value the hook's last request posted. */
function postedCreatedOn(): unknown {
  const body = postMock.mock.calls.at(-1)?.[1] as { filters: { filters: { values: string[] }[] } };
  return body.filters.filters[0].values[0];
}

describe("useAllWidgetCases", () => {
  afterEach(() => {
    vi.useRealTimers();
    postMock.mockReset();
  });

  it("asks again with the new day's dates after local midnight, not from the cache", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    const beforeMidnight = new Date(2026, 10, 3, 23, 59);
    const afterMidnight = new Date(2026, 10, 4, 0, 1);
    vi.setSystemTime(beforeMidnight);
    postMock.mockResolvedValue({ cases: [], total: 0 });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    const first = renderHook(() => useAllWidgetCases({ widgetId: "w", resourceType: "case", filters }), { wrapper });
    await waitFor(() => expect(first.result.current.isSuccess).toBe(true));
    expect(postedCreatedOn()).toBe(resolveRelativeDatePlaceholder("__today__", "gte", beforeMidnight));
    first.unmount();

    // Reopened within the query's stale time, but on the next local day.
    vi.setSystemTime(afterMidnight);
    const second = renderHook(() => useAllWidgetCases({ widgetId: "w", resourceType: "case", filters }), { wrapper });
    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(2));
    expect(postedCreatedOn()).toBe(resolveRelativeDatePlaceholder("__today__", "gte", afterMidnight));
    await waitFor(() => expect(second.result.current.isSuccess).toBe(true));
  });
});
