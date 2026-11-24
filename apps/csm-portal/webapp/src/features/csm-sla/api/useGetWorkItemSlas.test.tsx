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
import { ApiQueryKeys } from "@constants/apiConstants";

const postMock = vi.fn();

// The real client reads runtime config at module load, which is not present under vitest.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: postMock }),
}));

import { useGetWorkItemSlas } from "@features/csm-sla/api/useGetWorkItemSlas";

const WORK_ITEM_ID = "00000000-0000-0000-0000-000000000001";

function setup(id: string | undefined) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
  return { qc, ...renderHook(() => useGetWorkItemSlas(id), { wrapper }) };
}

describe("useGetWorkItemSlas", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("loads historical SLA pages so earlier breach instances remain available", async () => {
    const row = { id: "sla", slaDefinition: { name: "Response" }, stage: "completed", businessElapsedPercentage: 120 };
    postMock.mockResolvedValueOnce({ slas: Array.from({ length: 50 }, (_, i) => ({ ...row, id: `sla-${i}` })), total: 51 });
    postMock.mockResolvedValueOnce({ slas: [{ ...row, id: "last" }], total: 51 });
    const { result } = setup(WORK_ITEM_ID);
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.slas).toHaveLength(51);
    expect(result.current.data?.slas.at(-1)?.id).toBe("last");
    expect(postMock).toHaveBeenLastCalledWith("/slas/search", { filters: { taskIds: [WORK_ITEM_ID] }, pagination: { limit: 50, offset: 50 } });
  });

  it("searches by the work item id, maps rows, and caches under the work-item key", async () => {
    postMock.mockResolvedValue({
      slas: [
        {
          id: "sla-1",
          slaDefinition: { name: "S2 - Resolution", target: "8 Business Hours" },
          stage: "In progress",
          task: { id: WORK_ITEM_ID },
          businessTimeLeft: "2 hours",
          businessElapsedTime: "6 hours",
          businessElapsedPercentage: 75,
          startTime: "2026-07-01T10:00:00Z",
          endTime: null,
        },
      ],
      total: 3,
      limit: 50,
      offset: 0,
    });

    const { result, qc } = setup(WORK_ITEM_ID);
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith("/slas/search", {
      filters: { taskIds: [WORK_ITEM_ID] },
      pagination: { limit: 50, offset: 0 },
    });
    expect(result.current.data?.count).toBe(3);
    expect(result.current.data?.slas[0]).toMatchObject({
      id: "sla-1",
      definition: "S2 - Resolution",
      stage: "in_progress",
      businessElapsedPercent: 75,
    });
    expect(qc.getQueryData([ApiQueryKeys.WORK_ITEM_SLAS, WORK_ITEM_ID])).toBeDefined();
  });

  it("does not fetch while the id is undefined", () => {
    const { result } = setup(undefined);
    expect(result.current.fetchStatus).toBe("idle");
    expect(postMock).not.toHaveBeenCalled();
  });
});
