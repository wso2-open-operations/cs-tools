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
import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const getMock = vi.fn();
// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useQuickIncidentSearch.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock }),
}));

const useSearchGroupsMock = vi.fn((query: string, enabled: boolean) => {
  void query;
  void enabled;
  return {
    data: [
      { id: "group-cab", name: "CAB Approval", active: true },
      // Also an SRE team's group: listed once, as the team.
      { id: "team-apollo", name: "Apollo Group", active: true },
    ],
    isFetching: false,
    isError: false,
  };
});
// Stable across renders, as React Query's own data is.
const { EXTRA_GROUPS, TEAMS } = vi.hoisted(() => ({
  EXTRA_GROUPS: { data: [{ id: "group-soc", name: "Security Operations", active: true }], isError: false },
  TEAMS: {
    data: [
      { id: "apollo", name: "Apollo", family: "sre-abt", sreGroupId: "team-apollo" },
      { id: "artemis", name: "Artemis", family: "sre-abt", sreGroupId: "team-artemis" },
      { id: "atlas", name: "Atlas", family: "cre-abt", creGroupId: "team-atlas" },
    ],
  },
}));
vi.mock("@api/useSearchGroups", () => ({
  useSearchGroups: (query: string, enabled: boolean) => useSearchGroupsMock(query, enabled),
  useSearchIncidentExtraGroups: () => EXTRA_GROUPS,
}));
vi.mock("@features/csm-dashboard/api/useTeams", () => ({
  useTeams: () => TEAMS,
}));

import {
  useIncidentFilterGroupLabels,
  useSearchIncidentFilterGroups,
} from "@features/csm-operations/api/useSearchIncidentFilterGroups";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useSearchIncidentFilterGroups", () => {
  it("lists the SRE teams and extra groups first, then the searched groups, each id once", () => {
    const { result } = renderHook(() => useSearchIncidentFilterGroups("", true));

    expect(result.current.data).toEqual([
      { id: "team-apollo", name: "Apollo" },
      { id: "team-artemis", name: "Artemis" },
      { id: "group-soc", name: "Security Operations" },
      { id: "group-cab", name: "CAB Approval" },
    ]);
  });

  it("narrows the teams and extra groups by what is typed and searches the rest with it", () => {
    const { result } = renderHook(() => useSearchIncidentFilterGroups(" ART ", true));

    expect(useSearchGroupsMock).toHaveBeenLastCalledWith(" ART ", true);
    // Artemis matches; Apollo and Security Operations don't. The search's own
    // results are whatever the server matched, except a pinned team's group:
    // Apollo's group is not listed again under the group's own name.
    expect(result.current.data.map((g) => g.name)).toEqual(["Artemis", "CAB Approval"]);
  });

  it("leaves out the previous query's results while the new search is in flight", () => {
    useSearchGroupsMock.mockReturnValueOnce({
      data: [{ id: "group-cab", name: "CAB Approval", active: true }],
      isFetching: true,
      isError: false,
      isPlaceholderData: true,
    } as ReturnType<typeof useSearchGroupsMock>);
    const { result } = renderHook(() => useSearchIncidentFilterGroups("sec", true));

    expect(result.current.isFetching).toBe(true);
    expect(result.current.data).toEqual([{ id: "group-soc", name: "Security Operations" }]);
  });

  it("keeps the teams and extra groups while the picker is closed, so a selection stays labelled", () => {
    const { result } = renderHook(() => useSearchIncidentFilterGroups("", false));

    expect(useSearchGroupsMock).toHaveBeenLastCalledWith("", false);
    expect(result.current.data.slice(0, 3).map((g) => g.id)).toEqual(["team-apollo", "team-artemis", "group-soc"]);
  });
});

describe("useIncidentFilterGroupLabels", () => {
  const CHOREO = "6a0e1b52-0000-4000-8000-0000000000c1";
  const GONE = "6a0e1b52-0000-4000-8000-0000000000c2";

  it("looks up each selected group by id and maps it to its name, leaving out one not found", async () => {
    getMock.mockReset();
    getMock.mockImplementation((path: string) =>
      Promise.resolve(path === `/groups/${CHOREO}` ? { id: CHOREO, name: "Choreo SRE Team", members: [] } : null),
    );
    const { result } = renderHook(() => useIncidentFilterGroupLabels([CHOREO, GONE]), { wrapper });

    await waitFor(() => expect(result.current).toEqual({ [CHOREO]: "Choreo SRE Team" }));
    expect(getMock).toHaveBeenCalledWith(`/groups/${CHOREO}`);
    expect(getMock).toHaveBeenCalledWith(`/groups/${GONE}`);
  });

  it("names a pinned team or extra group from the pinned list without looking it up", () => {
    getMock.mockReset();
    const { result } = renderHook(() => useIncidentFilterGroupLabels(["team-apollo", "group-soc"]), { wrapper });

    expect(result.current).toEqual({ "team-apollo": "Apollo", "group-soc": "Security Operations" });
    expect(getMock).not.toHaveBeenCalled();
  });

  it("never looks up an id that is not a UUID", () => {
    getMock.mockReset();
    const { result } = renderHook(() => useIncidentFilterGroupLabels(["foo"]), { wrapper });

    expect(result.current).toEqual({});
    expect(getMock).not.toHaveBeenCalled();
  });

  it("keeps the same map across renders while no name changes", async () => {
    getMock.mockReset();
    getMock.mockResolvedValue({ id: CHOREO, name: "Choreo SRE Team", members: [] });
    const { result, rerender } = renderHook(({ ids }) => useIncidentFilterGroupLabels(ids), {
      wrapper,
      initialProps: { ids: [CHOREO] },
    });
    await waitFor(() => expect(result.current).toEqual({ [CHOREO]: "Choreo SRE Team" }));

    const first = result.current;
    rerender({ ids: [CHOREO] });
    expect(result.current).toBe(first);
  });

  it("asks for nothing when no group is selected", () => {
    getMock.mockClear();
    const { result } = renderHook(() => useIncidentFilterGroupLabels([]), { wrapper });

    expect(result.current).toEqual({});
    expect(getMock).not.toHaveBeenCalled();
  });
});
