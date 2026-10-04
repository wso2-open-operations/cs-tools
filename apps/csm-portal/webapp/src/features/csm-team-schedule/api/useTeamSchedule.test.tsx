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
import { describe, expect, it, vi } from "vitest";

const postMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useScheduleAssignmentsByMonth } from "./useTeamSchedule";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useScheduleAssignmentsByMonth", () => {
  it("does not show another month's rows in a slot whose month is still loading", async () => {
    postMock.mockImplementation((_url: string, p: { from: string }) =>
      p.from === "2026-11-01"
        ? new Promise(() => undefined) // November never resolves
        : Promise.resolve({
            assignments: [{ id: `row-${p.from}`, rotaDate: p.from }],
            count: 1,
          }),
    );
    const sep = { from: "2026-09-01", to: "2026-09-30" };
    const oct = { from: "2026-10-01", to: "2026-10-31" };
    const nov = { from: "2026-11-01", to: "2026-11-30" };

    const { result, rerender } = renderHook(
      ({ months }) => useScheduleAssignmentsByMonth({ teamKey: "alpha" } as never, months),
      { wrapper, initialProps: { months: [sep, oct] } },
    );
    await waitFor(() => expect(result.current.data?.assignments).toHaveLength(2));

    // Move the window forward: slot 1 changes from October to November.
    rerender({ months: [oct, nov] });
    await waitFor(() => expect(result.current.isLoading).toBe(true));
    const ids = (result.current.data?.assignments ?? []).map((a: { id: string }) => a.id);
    // Only October's own row: slot 1 must not inherit October as November's placeholder.
    expect(ids).toEqual(["row-2026-10-01"]);
  });
});
