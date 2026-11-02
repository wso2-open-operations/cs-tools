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
import type { BeDashboard } from "@api/backend/types";
import { ApiQueryKeys } from "@constants/apiConstants";

const getMock = vi.fn();
const putMock = vi.fn();
const delMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest. BackendApiError is a stand-in the module under test and this
// test share (same approach as saveErrorMessage.test.ts).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock, put: putMock, del: delMock }),
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

import {
  useDeleteCustomDashboard,
  useGetCustomDashboard,
  useGetCustomDashboards,
  usePutCustomDashboard,
  useRenameCustomDashboard,
  type SavedCustomDashboard,
} from "@features/csm-custom-dashboards/api/useCustomDashboards";
import { BackendApiError } from "@api/backend/client";

const ID = "5e636a12-0000-4000-8000-000000000001";

const dashboard: BeDashboard = {
  id: ID,
  displayName: "My Cases",
  isDefault: false,
  targetTeam: "",
  isTeamBased: false,
  widgets: [],
};

const saved: SavedCustomDashboard = {
  dashboard,
  createdOn: "2026-10-26T10:00:00Z",
  updatedOn: "2026-10-26T10:05:00Z",
};

function setup() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { queryClient, wrapper };
}

describe("useCustomDashboards", () => {
  beforeEach(() => {
    getMock.mockReset();
    putMock.mockReset();
    delMock.mockReset();
  });

  describe("useGetCustomDashboards", () => {
    it("lists the caller's saved dashboards", async () => {
      const list = [{ id: ID, name: "My Cases", createdOn: "a", updatedOn: "b" }];
      getMock.mockResolvedValue({ dashboards: list });
      const { wrapper } = setup();

      const { result } = renderHook(() => useGetCustomDashboards(), { wrapper });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(getMock).toHaveBeenCalledWith("/dashboards/custom");
      expect(result.current.data).toEqual(list);
    });

    it("is an error, not an empty list, when the route 404s", async () => {
      getMock.mockResolvedValue(null);
      const { wrapper } = setup();

      const { result } = renderHook(() => useGetCustomDashboards(), { wrapper });

      await waitFor(() => expect(result.current.isError).toBe(true));
    });
  });

  describe("useGetCustomDashboard", () => {
    it("fetches one dashboard by id", async () => {
      getMock.mockResolvedValue(saved);
      const { wrapper } = setup();

      const { result } = renderHook(() => useGetCustomDashboard(ID), { wrapper });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(getMock).toHaveBeenCalledWith(`/dashboards/custom/${ID}`);
      expect(result.current.data).toEqual(saved);
    });

    it("escapes the id into one path segment", async () => {
      getMock.mockResolvedValue(null);
      const { wrapper } = setup();

      renderHook(() => useGetCustomDashboard("a/../b"), { wrapper });

      await waitFor(() => expect(getMock).toHaveBeenCalled());
      expect(getMock).toHaveBeenCalledWith("/dashboards/custom/a%2F..%2Fb");
    });

    it("is null when the caller has no dashboard with that id", async () => {
      getMock.mockResolvedValue(null);
      const { wrapper } = setup();

      const { result } = renderHook(() => useGetCustomDashboard(ID), { wrapper });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toBeNull();
    });

    it("makes no request without an id", () => {
      const { wrapper } = setup();

      const { result } = renderHook(() => useGetCustomDashboard(undefined), { wrapper });

      expect(result.current.fetchStatus).toBe("idle");
      expect(getMock).not.toHaveBeenCalled();
    });
  });

  describe("usePutCustomDashboard", () => {
    it("saves the dashboard under its own id, then caches it and refreshes the list", async () => {
      putMock.mockResolvedValue(saved);
      const { queryClient, wrapper } = setup();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");

      const { result } = renderHook(() => usePutCustomDashboard(), { wrapper });
      await act(async () => {
        await result.current.mutateAsync(dashboard);
      });

      expect(putMock).toHaveBeenCalledWith(`/dashboards/custom/${ID}`, { dashboard });
      expect(queryClient.getQueryData([ApiQueryKeys.CSM_CUSTOM_DASHBOARD_DETAIL, ID])).toEqual(saved);
      expect(invalidate).toHaveBeenCalledWith({ queryKey: [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_LIST] });
    });

    it("throws the backend's error, and still refreshes the list", async () => {
      const err = new Error("you already have a dashboard named: My Cases");
      putMock.mockRejectedValue(err);
      const { queryClient, wrapper } = setup();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");

      const { result } = renderHook(() => usePutCustomDashboard(), { wrapper });
      await act(async () => {
        await expect(result.current.mutateAsync(dashboard)).rejects.toBe(err);
      });

      expect(queryClient.getQueryData([ApiQueryKeys.CSM_CUSTOM_DASHBOARD_DETAIL, ID])).toBeUndefined();
      expect(invalidate).toHaveBeenCalledWith({ queryKey: [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_LIST] });
    });
  });

  describe("useRenameCustomDashboard", () => {
    it("reads the dashboard and saves it back under the same id with only the name changed", async () => {
      const withWidget = { ...dashboard, widgets: [{ widgetId: "open-cases" }] } as BeDashboard;
      getMock.mockResolvedValue({ ...saved, dashboard: withWidget });
      const renamed = { ...saved, dashboard: { ...withWidget, displayName: "Open Cases" } };
      putMock.mockResolvedValue(renamed);
      const { queryClient, wrapper } = setup();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");

      const { result } = renderHook(() => useRenameCustomDashboard(), { wrapper });
      await act(async () => {
        await result.current.mutateAsync({ id: ID, name: "Open Cases" });
      });

      expect(getMock).toHaveBeenCalledWith(`/dashboards/custom/${ID}`);
      expect(putMock).toHaveBeenCalledWith(`/dashboards/custom/${ID}`, {
        dashboard: { ...withWidget, displayName: "Open Cases" },
      });
      expect(queryClient.getQueryData([ApiQueryKeys.CSM_CUSTOM_DASHBOARD_DETAIL, ID])).toEqual(renamed);
      expect(invalidate).toHaveBeenCalledWith({ queryKey: [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_LIST] });
    });

    it("throws a 404 and saves nothing when the dashboard is gone", async () => {
      getMock.mockResolvedValue(null);
      const { wrapper } = setup();

      const { result } = renderHook(() => useRenameCustomDashboard(), { wrapper });
      await act(async () => {
        await expect(result.current.mutateAsync({ id: ID, name: "New" })).rejects.toMatchObject({ status: 404 });
      });

      expect(putMock).not.toHaveBeenCalled();
    });

    it("throws the read's error and saves nothing when the dashboard can't be read", async () => {
      const err = new BackendApiError(422, "This saved dashboard can't be shown any more. Generate it again.");
      getMock.mockRejectedValue(err);
      const { wrapper } = setup();

      const { result } = renderHook(() => useRenameCustomDashboard(), { wrapper });
      await act(async () => {
        await expect(result.current.mutateAsync({ id: ID, name: "New" })).rejects.toBe(err);
      });

      expect(putMock).not.toHaveBeenCalled();
    });

    it("throws the save's error, and still refreshes the list", async () => {
      getMock.mockResolvedValue(saved);
      const err = new BackendApiError(409, "you already have a dashboard named: Taken");
      putMock.mockRejectedValue(err);
      const { queryClient, wrapper } = setup();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");

      const { result } = renderHook(() => useRenameCustomDashboard(), { wrapper });
      await act(async () => {
        await expect(result.current.mutateAsync({ id: ID, name: "Taken" })).rejects.toBe(err);
      });

      expect(invalidate).toHaveBeenCalledWith({ queryKey: [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_LIST] });
    });
  });

  describe("useDeleteCustomDashboard", () => {
    it("deletes by id, drops the cached dashboard and refreshes the list", async () => {
      delMock.mockResolvedValue(null);
      const { queryClient, wrapper } = setup();
      queryClient.setQueryData([ApiQueryKeys.CSM_CUSTOM_DASHBOARD_DETAIL, ID], saved);
      const other = { id: "other", name: "Other", createdOn: "a", updatedOn: "b" };
      queryClient.setQueryData([ApiQueryKeys.CSM_CUSTOM_DASHBOARD_LIST], [
        { id: ID, name: "My Cases", createdOn: "a", updatedOn: "b" },
        other,
      ]);
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");

      const { result } = renderHook(() => useDeleteCustomDashboard(), { wrapper });
      await act(async () => {
        await result.current.mutateAsync(ID);
      });

      expect(delMock).toHaveBeenCalledWith(`/dashboards/custom/${ID}`);
      expect(queryClient.getQueryData([ApiQueryKeys.CSM_CUSTOM_DASHBOARD_DETAIL, ID])).toBeUndefined();
      expect(queryClient.getQueryData([ApiQueryKeys.CSM_CUSTOM_DASHBOARD_LIST])).toEqual([other]);
      expect(invalidate).toHaveBeenCalledWith({ queryKey: [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_LIST] });
    });
  });
});
