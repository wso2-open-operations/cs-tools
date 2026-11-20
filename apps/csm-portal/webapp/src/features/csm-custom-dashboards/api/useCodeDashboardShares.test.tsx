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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const getMock = vi.fn();
const putMock = vi.fn();
const delMock = vi.fn();
const postMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useCodeDashboards.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock, put: putMock, del: delMock, post: postMock }),
}));

import {
  CODE_DASHBOARD_SHARING,
  SHARED_CODE_DASHBOARD_LIST_KEY,
  sharedCodeDashboardPath,
  useDeleteCodeDashboardLink,
  useDeleteCodeDashboardShare,
  useDeleteCodeDashboardTeamShare,
  useGetCodeDashboardLink,
  useGetCodeDashboardShares,
  useGetCodeDashboardTeamShares,
  useGetSharedCodeDashboard,
  useGetSharedCodeDashboards,
  useOpenCodeDashboardLink,
  usePutCodeDashboardLink,
  usePutCodeDashboardShare,
  usePutCodeDashboardTeamShare,
} from "@features/csm-custom-dashboards/api/useCodeDashboardShares";
import { useGetCustomDashboardShares } from "@features/csm-custom-dashboards/api/useDashboardShares";

const ID = "5e636a12-0000-4000-8000-000000000001";
const USER = "5e636a12-0000-4000-8000-0000000000aa";
const alice = { user: { id: USER, email: "alice@example.com", name: "Alice" }, sharedOn: "2026-11-13T10:00:00Z" };
const bob = { user: { id: "b", email: "bob@example.com", name: "Bob" }, sharedOn: "2026-11-13T10:00:00Z" };
const rigel = { team: { key: "rigel", name: "Rigel", family: "cre-abt" }, sharedOn: "2026-11-13T10:00:00Z" };
const apollo = { team: { key: "apollo", name: "Apollo", family: "sre-abt" }, sharedOn: "2026-11-13T10:00:00Z" };

function setup() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { queryClient, wrapper };
}

describe("useCodeDashboardShares", () => {
  beforeEach(() => {
    getMock.mockReset();
    putMock.mockReset();
    delMock.mockReset();
    postMock.mockReset();
  });

  it("lists, adds and removes people on the code dashboard routes", async () => {
    getMock.mockResolvedValue({ shares: [bob] });
    const { wrapper } = setup();
    const { result } = renderHook(
      () => ({ list: useGetCodeDashboardShares(ID), add: usePutCodeDashboardShare(), remove: useDeleteCodeDashboardShare() }),
      { wrapper },
    );
    await waitFor(() => expect(result.current.list.data).toEqual([bob]));
    expect(getMock).toHaveBeenCalledWith(`/dashboards/code/${ID}/shares`);

    putMock.mockResolvedValue({ shares: [alice, bob] });
    getMock.mockReturnValue(new Promise(() => {})); // keep the refetch pending: the list is the answer's
    await act(async () => {
      await result.current.add.mutateAsync({ dashboardId: ID, userId: USER });
    });
    expect(putMock).toHaveBeenCalledWith(`/dashboards/code/${ID}/shares/${USER}`, {});
    await waitFor(() => expect(result.current.list.data).toEqual([alice, bob]));

    delMock.mockResolvedValue(null);
    getMock.mockReturnValue(new Promise(() => {})); // keep the refetch pending: the list is the cache's
    await act(async () => {
      await result.current.remove.mutateAsync({ dashboardId: ID, userId: USER });
    });
    expect(delMock).toHaveBeenCalledWith(`/dashboards/code/${ID}/shares/${USER}`);
    await waitFor(() => expect(result.current.list.data).toEqual([bob]));
  });

  it("lists, adds and removes teams by registry key", async () => {
    getMock.mockResolvedValue({ shares: [apollo] });
    const { wrapper } = setup();
    const { result } = renderHook(
      () => ({ list: useGetCodeDashboardTeamShares(ID), add: usePutCodeDashboardTeamShare(), remove: useDeleteCodeDashboardTeamShare() }),
      { wrapper },
    );
    await waitFor(() => expect(result.current.list.data).toEqual([apollo]));
    expect(getMock).toHaveBeenCalledWith(`/dashboards/code/${ID}/team-shares`);

    putMock.mockResolvedValue({ shares: [apollo, rigel] });
    getMock.mockReturnValue(new Promise(() => {}));
    await act(async () => {
      await result.current.add.mutateAsync({ dashboardId: ID, teamKey: "rigel" });
    });
    expect(putMock).toHaveBeenCalledWith(`/dashboards/code/${ID}/team-shares/rigel`, {});
    await waitFor(() => expect(result.current.list.data).toEqual([apollo, rigel]));

    delMock.mockResolvedValue(null);
    await act(async () => {
      await result.current.remove.mutateAsync({ dashboardId: ID, teamKey: "apollo" });
    });
    expect(delMock).toHaveBeenCalledWith(`/dashboards/code/${ID}/team-shares/apollo`);
    await waitFor(() => expect(result.current.list.data).toEqual([rigel]));
  });

  it("reads, turns on and turns off the link", async () => {
    getMock.mockResolvedValue({ enabled: false, openedBy: 0 });
    const { wrapper } = setup();
    const { result } = renderHook(
      () => ({ link: useGetCodeDashboardLink(ID), on: usePutCodeDashboardLink(), off: useDeleteCodeDashboardLink() }),
      { wrapper },
    );
    await waitFor(() => expect(result.current.link.data).toEqual({ enabled: false, openedBy: 0 }));
    expect(getMock).toHaveBeenCalledWith(`/dashboards/code/${ID}/link`);

    putMock.mockResolvedValue({ enabled: true, linkId: "l1", openedBy: 0 });
    getMock.mockReturnValue(new Promise(() => {}));
    await act(async () => {
      await result.current.on.mutateAsync(ID);
    });
    expect(putMock).toHaveBeenCalledWith(`/dashboards/code/${ID}/link`, {});
    await waitFor(() => expect(result.current.link.data).toEqual({ enabled: true, linkId: "l1", openedBy: 0 }));

    delMock.mockResolvedValue(null);
    await act(async () => {
      await result.current.off.mutateAsync(ID);
    });
    expect(delMock).toHaveBeenCalledWith(`/dashboards/code/${ID}/link`);
    await waitFor(() => expect(result.current.link.data).toEqual({ enabled: false, openedBy: 0 }));
  });

  it("opens a link with its id in the body and reads the shared list again", async () => {
    postMock.mockResolvedValue({ dashboardId: ID, own: false });
    const { queryClient, wrapper } = setup();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useOpenCodeDashboardLink(), { wrapper });
    await act(async () => {
      await result.current.mutateAsync("link-1");
    });
    expect(postMock).toHaveBeenCalledWith("/dashboards/shared/code/link", { linkId: "link-1" });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [SHARED_CODE_DASHBOARD_LIST_KEY] });
  });

  it("lists the advanced dashboards shared with the caller, only when enabled", async () => {
    const list = [{ id: ID, name: "Open Cases", owner: null, sharedViaTeam: null, sharedViaLink: true, sharedOn: "a", updatedOn: "b" }];
    getMock.mockResolvedValue({ dashboards: list });
    const { wrapper } = setup();
    const off = renderHook(() => useGetSharedCodeDashboards(false), { wrapper });
    expect(off.result.current.fetchStatus).toBe("idle");
    expect(getMock).not.toHaveBeenCalled();

    const { result } = renderHook(() => useGetSharedCodeDashboards(), { wrapper });
    await waitFor(() => expect(result.current.data).toEqual(list));
    expect(getMock).toHaveBeenCalledWith("/dashboards/shared/code");
  });

  it("treats a missing shared list route as an error, not as nothing shared", async () => {
    getMock.mockResolvedValue(null);
    const { wrapper } = setup();
    const { result } = renderHook(() => useGetSharedCodeDashboards(), { wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
  });

  it("reads one shared advanced dashboard with its source, null when not shared", async () => {
    const shared = { id: ID, name: "Open Cases", source: "async function render() {}", owner: null, sharedViaTeam: null, sharedViaLink: false, sharedOn: "a", updatedOn: "b" };
    getMock.mockResolvedValueOnce(shared).mockResolvedValueOnce(null);
    const { wrapper } = setup();
    const { result } = renderHook(() => useGetSharedCodeDashboard(ID), { wrapper });
    await waitFor(() => expect(result.current.data).toEqual(shared));
    expect(getMock).toHaveBeenCalledWith(`/dashboards/shared/code/${ID}`);

    const other = renderHook(() => useGetSharedCodeDashboard("a/b"), { wrapper });
    await waitFor(() => expect(other.result.current.data).toBeNull());
    expect(getMock).toHaveBeenLastCalledWith("/dashboards/shared/code/a%2Fb");

    const none = renderHook(() => useGetSharedCodeDashboard(undefined), { wrapper });
    expect(none.result.current.fetchStatus).toBe("idle");
  });

  it("escapes ids into one path segment", async () => {
    putMock.mockResolvedValue({ shares: [] });
    const { wrapper } = setup();
    const { result } = renderHook(() => usePutCodeDashboardTeamShare(), { wrapper });
    await act(async () => {
      await result.current.mutateAsync({ dashboardId: "a/b", teamKey: "c/d" });
    });
    expect(putMock).toHaveBeenCalledWith("/dashboards/code/a%2Fb/team-shares/c%2Fd", {});
  });

  it("keeps its cache apart from the JSON dashboards' shares", async () => {
    getMock.mockImplementation((path: string) =>
      Promise.resolve({ shares: path.startsWith("/dashboards/code/") ? [alice] : [bob] }),
    );
    const { wrapper } = setup();
    const { result } = renderHook(() => ({ code: useGetCodeDashboardShares(ID), json: useGetCustomDashboardShares(ID) }), {
      wrapper,
    });
    await waitFor(() => expect(result.current.code.data).toEqual([alice]));
    await waitFor(() => expect(result.current.json.data).toEqual([bob]));
  });

  it("gives the Share dialog these hooks, with the link at its own path", () => {
    expect(CODE_DASHBOARD_SHARING).toEqual({
      useGetShares: useGetCodeDashboardShares,
      usePutShare: usePutCodeDashboardShare,
      useDeleteShare: useDeleteCodeDashboardShare,
      useGetTeamShares: useGetCodeDashboardTeamShares,
      usePutTeamShare: usePutCodeDashboardTeamShare,
      useDeleteTeamShare: useDeleteCodeDashboardTeamShare,
      linkHooks: {
        useGetLink: useGetCodeDashboardLink,
        usePutLink: usePutCodeDashboardLink,
        useDeleteLink: useDeleteCodeDashboardLink,
        linkUrl: expect.any(Function),
      },
    });
    expect(CODE_DASHBOARD_SHARING.linkHooks?.linkUrl("a/b")).toBe(`${window.location.origin}/dashboard/shared/code-link/a%2Fb`);
  });

  it("opens a shared advanced dashboard under Shared Dashboards, its id one path segment", () => {
    expect(sharedCodeDashboardPath("a/b")).toBe("/dashboard/shared/code/a%2Fb");
  });
});
