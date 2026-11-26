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

const getMock = vi.fn();
// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useQuickIncidentSearch.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock }),
}));

// Real group ids are UUIDs (lower case, as the server returns them), so the
// pinned fixtures are too: the label lookup only ever looks up a UUID.
const APOLLO = "6a0e1b52-0000-4000-8000-00000000a001";
const ARTEMIS = "6a0e1b52-0000-4000-8000-00000000a002";
const SOC = "6a0e1b52-0000-4000-8000-00000000a003";
const CAB = "6a0e1b52-0000-4000-8000-00000000a004";
const CHOREO = "6a0e1b52-0000-4000-8000-0000000000c1";
const GONE = "6a0e1b52-0000-4000-8000-0000000000c2";

const useSearchGroupsMock = vi.fn((query: string, enabled: boolean) => {
  void query;
  void enabled;
  return {
    data: [{ id: CAB, name: "CAB Approval", active: true }],
    isFetching: false,
    isError: false,
    isPlaceholderData: false,
  };
});

// The two pinned lists, as `useTeams` / `useSearchIncidentExtraGroups` return
// them. Each test sets what it needs; the objects are kept stable across
// renders, as React Query's own results are.
const pinnedState = vi.hoisted(() => ({
  teams: {} as { data?: unknown[]; isPending: boolean; isError: boolean },
  extra: {} as { data?: unknown[]; isPending: boolean; isError: boolean },
}));
const TEAMS = [
  { id: "apollo", name: "Apollo", family: "sre-abt", sreGroupId: APOLLO },
  { id: "artemis", name: "Artemis", family: "sre-abt", sreGroupId: ARTEMIS },
  { id: "atlas", name: "Atlas", family: "cre-abt", creGroupId: "6a0e1b52-0000-4000-8000-00000000a009" },
];
const EXTRA = [{ id: SOC, name: "Security Operations", active: true }];
function setPinned(
  teams: { data?: unknown[]; isPending?: boolean; isError?: boolean },
  extra: { data?: unknown[]; isPending?: boolean; isError?: boolean },
): void {
  pinnedState.teams = { isPending: false, isError: false, ...teams };
  pinnedState.extra = { isPending: false, isError: false, ...extra };
}

vi.mock("@api/useSearchGroups", () => ({
  useSearchGroups: (query: string, enabled: boolean) => useSearchGroupsMock(query, enabled),
  useSearchIncidentExtraGroups: () => pinnedState.extra,
}));
vi.mock("@features/csm-dashboard/api/useTeams", () => ({
  useTeams: () => pinnedState.teams,
}));

import {
  useIncidentFilterGroupLabels,
  useSearchIncidentFilterGroups,
} from "@features/csm-operations/api/useSearchIncidentFilterGroups";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  setPinned({ data: TEAMS }, { data: EXTRA });
  getMock.mockReset();
});

describe("useSearchIncidentFilterGroups", () => {
  it("lists the SRE teams and extra groups first, then the searched groups, each id once", () => {
    useSearchGroupsMock.mockReturnValueOnce({
      data: [
        { id: CAB, name: "CAB Approval", active: true },
        // Also an SRE team's group: listed once, as the team.
        { id: APOLLO, name: "Apollo Group", active: true },
      ],
      isFetching: false,
      isError: false,
      isPlaceholderData: false,
    });
    const { result } = renderHook(() => useSearchIncidentFilterGroups("", true));

    expect(result.current.data).toEqual([
      { id: APOLLO, name: "Apollo" },
      { id: ARTEMIS, name: "Artemis" },
      { id: SOC, name: "Security Operations" },
      { id: CAB, name: "CAB Approval" },
    ]);
  });

  it("narrows the teams and extra groups by what is typed and searches the rest with it", () => {
    const { result } = renderHook(() => useSearchIncidentFilterGroups(" ART ", true));

    expect(useSearchGroupsMock).toHaveBeenLastCalledWith(" ART ", true);
    // Artemis matches; Apollo and Security Operations don't. The search's own
    // results are whatever the server matched.
    expect(result.current.data.map((g) => g.name)).toEqual(["Artemis", "CAB Approval"]);
  });

  // Regression: a pinned group hidden by its pinned name was still marked as
  // seen, so the search's match for it under its own name was dropped too and
  // the group could not be picked from that search at all.
  it("lists a pinned group the search matches under its own name, once, by its pinned name", () => {
    useSearchGroupsMock.mockReturnValueOnce({
      data: [{ id: APOLLO, name: "Apollo Group", active: true }],
      isFetching: false,
      isError: false,
      isPlaceholderData: false,
    });
    const { result } = renderHook(() => useSearchIncidentFilterGroups("Apollo Group", true));

    expect(result.current.data).toEqual([{ id: APOLLO, name: "Apollo" }]);
  });

  it("leaves out the previous query's results while the new search is in flight", () => {
    useSearchGroupsMock.mockReturnValueOnce({
      data: [{ id: CAB, name: "CAB Approval", active: true }],
      isFetching: true,
      isError: false,
      isPlaceholderData: true,
    });
    const { result } = renderHook(() => useSearchIncidentFilterGroups("sec", true));

    expect(result.current.isFetching).toBe(true);
    expect(result.current.data).toEqual([{ id: SOC, name: "Security Operations" }]);
  });

  it("keeps the teams and extra groups while the picker is closed, so a selection stays labelled", () => {
    const { result } = renderHook(() => useSearchIncidentFilterGroups("", false));

    expect(useSearchGroupsMock).toHaveBeenLastCalledWith("", false);
    expect(result.current.data.slice(0, 3).map((g) => g.id)).toEqual([APOLLO, ARTEMIS, SOC]);
  });

  // Regression: a failed pinned list looked like "no matches".
  it("reports a failed pinned list as an error, still listing what the search found", () => {
    setPinned({ isError: true }, { data: EXTRA });
    const { result } = renderHook(() => useSearchIncidentFilterGroups("", true));

    expect(result.current.isError).toBe(true);
    expect(result.current.data.map((g) => g.id)).toEqual([SOC, CAB]);
  });
});

describe("useIncidentFilterGroupLabels", () => {
  it("looks up each selected group by id and maps it to its name, leaving out one not found", async () => {
    getMock.mockImplementation((path: string) =>
      Promise.resolve(path === `/groups/${CHOREO}` ? { id: CHOREO, name: "Choreo SRE Team", members: [] } : null),
    );
    const { result } = renderHook(() => useIncidentFilterGroupLabels([CHOREO, GONE]), { wrapper });

    await waitFor(() => expect(result.current).toEqual({ [CHOREO]: "Choreo SRE Team" }));
    expect(getMock).toHaveBeenCalledWith(`/groups/${CHOREO}`);
    expect(getMock).toHaveBeenCalledWith(`/groups/${GONE}`);
  });

  it("names a pinned team or extra group from the pinned list without looking it up", () => {
    const { result } = renderHook(() => useIncidentFilterGroupLabels([APOLLO, SOC]), { wrapper });

    expect(result.current).toEqual({ [APOLLO]: "Apollo", [SOC]: "Security Operations" });
    expect(getMock).not.toHaveBeenCalled();
  });

  // Regression: on a cold cache a restored pinned group was looked up before
  // the pinned lists arrived, fetching the group's full member list for a name
  // the pinned list was about to give.
  it("looks nothing up until the pinned lists have answered", () => {
    setPinned({ isPending: true }, { isPending: true });
    const { result, rerender } = renderHook(() => useIncidentFilterGroupLabels([APOLLO]), { wrapper });

    expect(result.current).toEqual({});
    expect(getMock).not.toHaveBeenCalled();

    setPinned({ data: TEAMS }, { data: EXTRA });
    rerender();

    expect(result.current).toEqual({ [APOLLO]: "Apollo" });
    expect(getMock).not.toHaveBeenCalled();
  });

  it("looks a group up after all when the pinned list that would name it failed", async () => {
    setPinned({ isError: true }, { data: EXTRA });
    getMock.mockResolvedValue({ id: APOLLO, name: "Apollo SRE", members: [] });
    const { result } = renderHook(() => useIncidentFilterGroupLabels([APOLLO]), { wrapper });

    await waitFor(() => expect(result.current).toEqual({ [APOLLO]: "Apollo SRE" }));
    expect(getMock).toHaveBeenCalledWith(`/groups/${APOLLO}`);
  });

  // Regression: names were stored under the server's lower-case id but read
  // with the id as selected, so an upper-case id from a link got no name.
  it("names a group selected by an upper-case id", async () => {
    const upper = CHOREO.toUpperCase();
    getMock.mockResolvedValue({ id: CHOREO, name: "Choreo SRE Team", members: [] });
    const { result } = renderHook(() => useIncidentFilterGroupLabels([upper, APOLLO.toUpperCase()]), { wrapper });

    await waitFor(() =>
      expect(result.current).toEqual({ [upper]: "Choreo SRE Team", [APOLLO.toUpperCase()]: "Apollo" }),
    );
    expect(getMock).toHaveBeenCalledTimes(1);
  });

  it("never looks up an id that is not a UUID", () => {
    const { result } = renderHook(() => useIncidentFilterGroupLabels(["foo"]), { wrapper });

    expect(result.current).toEqual({});
    expect(getMock).not.toHaveBeenCalled();
  });

  it("keeps the same map across renders while no name changes", async () => {
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
    const { result } = renderHook(() => useIncidentFilterGroupLabels([]), { wrapper });

    expect(result.current).toEqual({});
    expect(getMock).not.toHaveBeenCalled();
  });
});
