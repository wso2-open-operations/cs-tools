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

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useCustomDashboards.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
  useBackendApi: () => ({ get: getMock, put: putMock, del: delMock }),
}));

import { BackendApiError } from "@api/backend/client";
import {
  CODE_DASHBOARD_DETAIL_KEY,
  CODE_DASHBOARD_LIST_KEY,
  savedCodeDashboardPath,
  useDeleteCodeDashboard,
  useGetCodeDashboard,
  useGetCodeDashboards,
  usePutCodeDashboard,
  useRenameCodeDashboard,
  type SavedCodeDashboard,
} from "@features/csm-custom-dashboards/api/useCodeDashboards";

const ID = "5e636a12-0000-4000-8000-000000000001";
const SOURCE = "async function render(portal, root) {}";

const saved: SavedCodeDashboard = {
  id: ID,
  name: "Open Cases",
  source: SOURCE,
  createdOn: "2026-11-13T10:00:00Z",
  updatedOn: "2026-11-13T10:05:00Z",
};

function setup() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { queryClient, wrapper };
}

describe("useCodeDashboards", () => {
  beforeEach(() => {
    getMock.mockReset();
    putMock.mockReset();
    delMock.mockReset();
  });

  describe("useGetCodeDashboards", () => {
    it("lists the caller's saved advanced dashboards", async () => {
      const list = [{ id: ID, name: "Open Cases", createdOn: "a", updatedOn: "b" }];
      getMock.mockResolvedValue({ dashboards: list });
      const { wrapper } = setup();

      const { result } = renderHook(() => useGetCodeDashboards(), { wrapper });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(getMock).toHaveBeenCalledWith("/dashboards/code");
      expect(result.current.data).toEqual(list);
    });

    it("is an error, not an empty list, when the route 404s", async () => {
      getMock.mockResolvedValue(null);
      const { wrapper } = setup();

      const { result } = renderHook(() => useGetCodeDashboards(), { wrapper });

      await waitFor(() => expect(result.current.isError).toBe(true));
    });

    it("makes no request when turned off", () => {
      const { wrapper } = setup();

      const { result } = renderHook(() => useGetCodeDashboards(false), { wrapper });

      expect(result.current.fetchStatus).toBe("idle");
      expect(getMock).not.toHaveBeenCalled();
    });
  });

  describe("useGetCodeDashboard", () => {
    it("fetches one dashboard by id, its id escaped into one path segment", async () => {
      getMock.mockResolvedValue(saved);
      const { wrapper } = setup();

      const { result } = renderHook(() => useGetCodeDashboard(ID), { wrapper });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(getMock).toHaveBeenCalledWith(`/dashboards/code/${ID}`);
      expect(result.current.data).toEqual(saved);

      renderHook(() => useGetCodeDashboard("a/../b"), { wrapper });
      await waitFor(() => expect(getMock).toHaveBeenCalledWith("/dashboards/code/a%2F..%2Fb"));
    });

    it("is null when the caller has no dashboard with that id", async () => {
      getMock.mockResolvedValue(null);
      const { wrapper } = setup();

      const { result } = renderHook(() => useGetCodeDashboard(ID), { wrapper });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toBeNull();
    });

    it("makes no request without an id", () => {
      const { wrapper } = setup();

      const { result } = renderHook(() => useGetCodeDashboard(undefined), { wrapper });

      expect(result.current.fetchStatus).toBe("idle");
      expect(getMock).not.toHaveBeenCalled();
    });
  });

  describe("useRenameCodeDashboard", () => {
    it("reads the dashboard and saves it back with the new name and its own source", async () => {
      getMock.mockResolvedValue(saved);
      const renamed = { ...saved, name: "Cases by Severity" };
      putMock.mockResolvedValue(renamed);
      const { queryClient, wrapper } = setup();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");

      const { result } = renderHook(() => useRenameCodeDashboard(), { wrapper });
      await act(async () => {
        await result.current.mutateAsync({ id: ID, name: "Cases by Severity" });
      });

      expect(getMock).toHaveBeenCalledWith(`/dashboards/code/${ID}`);
      expect(putMock).toHaveBeenCalledWith(`/dashboards/code/${ID}`, { name: "Cases by Severity", source: SOURCE });
      expect(queryClient.getQueryData([CODE_DASHBOARD_DETAIL_KEY, ID])).toEqual(renamed);
      expect(invalidate).toHaveBeenCalledWith({ queryKey: [CODE_DASHBOARD_LIST_KEY] });
    });

    it("throws a 404 and saves nothing when the dashboard is gone", async () => {
      getMock.mockResolvedValue(null);
      const { wrapper } = setup();

      const { result } = renderHook(() => useRenameCodeDashboard(), { wrapper });
      let thrown: unknown;
      await act(async () => {
        thrown = await result.current.mutateAsync({ id: ID, name: "Cases by Severity" }).catch((e: unknown) => e);
      });

      expect(thrown).toBeInstanceOf(BackendApiError);
      expect((thrown as BackendApiError).status).toBe(404);
      expect((thrown as BackendApiError).message).toBe("this dashboard no longer exists");
      expect(putMock).not.toHaveBeenCalled();
    });

    it("throws the save's error (a name already taken), caching nothing, and still refreshes the list", async () => {
      getMock.mockResolvedValue(saved);
      const err = new BackendApiError(409, "you already have an advanced dashboard named: Cases by Severity");
      putMock.mockRejectedValue(err);
      const { queryClient, wrapper } = setup();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");

      const { result } = renderHook(() => useRenameCodeDashboard(), { wrapper });
      await act(async () => {
        await expect(result.current.mutateAsync({ id: ID, name: "Cases by Severity" })).rejects.toBe(err);
      });

      expect(queryClient.getQueryData([CODE_DASHBOARD_DETAIL_KEY, ID])).toBeUndefined();
      expect(invalidate).toHaveBeenCalledWith({ queryKey: [CODE_DASHBOARD_LIST_KEY] });
    });
  });

  describe("usePutCodeDashboard", () => {
    it("saves the name and source under the draft's id, then caches it and refreshes the list", async () => {
      putMock.mockResolvedValue(saved);
      const { queryClient, wrapper } = setup();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");

      const { result } = renderHook(() => usePutCodeDashboard(), { wrapper });
      await act(async () => {
        await result.current.mutateAsync({ id: ID, name: "Open Cases", source: SOURCE });
      });

      expect(putMock).toHaveBeenCalledWith(`/dashboards/code/${ID}`, { name: "Open Cases", source: SOURCE });
      expect(queryClient.getQueryData([CODE_DASHBOARD_DETAIL_KEY, ID])).toEqual(saved);
      expect(invalidate).toHaveBeenCalledWith({ queryKey: [CODE_DASHBOARD_LIST_KEY] });
    });

    it("throws the backend's error, and still refreshes the list", async () => {
      const err = new Error("you already have an advanced dashboard named: Open Cases");
      putMock.mockRejectedValue(err);
      const { queryClient, wrapper } = setup();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");

      const { result } = renderHook(() => usePutCodeDashboard(), { wrapper });
      await act(async () => {
        await expect(result.current.mutateAsync({ id: ID, name: "Open Cases", source: SOURCE })).rejects.toBe(err);
      });

      expect(queryClient.getQueryData([CODE_DASHBOARD_DETAIL_KEY, ID])).toBeUndefined();
      expect(invalidate).toHaveBeenCalledWith({ queryKey: [CODE_DASHBOARD_LIST_KEY] });
    });
  });

  describe("useDeleteCodeDashboard", () => {
    it("deletes by id, drops the cached dashboard and refreshes the list", async () => {
      delMock.mockResolvedValue(null);
      const { queryClient, wrapper } = setup();
      queryClient.setQueryData([CODE_DASHBOARD_DETAIL_KEY, ID], saved);
      const other = { id: "other", name: "Other", createdOn: "a", updatedOn: "b" };
      queryClient.setQueryData([CODE_DASHBOARD_LIST_KEY], [{ id: ID, name: "Open Cases", createdOn: "a", updatedOn: "b" }, other]);
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");

      const { result } = renderHook(() => useDeleteCodeDashboard(), { wrapper });
      await act(async () => {
        await result.current.mutateAsync(ID);
      });

      expect(delMock).toHaveBeenCalledWith(`/dashboards/code/${ID}`);
      expect(queryClient.getQueryData([CODE_DASHBOARD_DETAIL_KEY, ID])).toBeUndefined();
      expect(queryClient.getQueryData([CODE_DASHBOARD_LIST_KEY])).toEqual([other]);
      expect(invalidate).toHaveBeenCalledWith({ queryKey: [CODE_DASHBOARD_LIST_KEY] });
    });
  });

  it("opens a saved dashboard at /dashboard/my/code/:id, its id one path segment", () => {
    expect(savedCodeDashboardPath(ID)).toBe(`/dashboard/my/code/${ID}`);
    expect(savedCodeDashboardPath("a/b")).toBe("/dashboard/my/code/a%2Fb");
  });

  it("keeps its cache apart from the JSON dashboards'", () => {
    expect(CODE_DASHBOARD_LIST_KEY).not.toBe("csm-custom-dashboard-list");
    expect(CODE_DASHBOARD_DETAIL_KEY).not.toBe("csm-custom-dashboard-detail");
  });
});
