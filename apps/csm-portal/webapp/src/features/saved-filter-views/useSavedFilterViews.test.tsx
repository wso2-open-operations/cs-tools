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

// @vitest-environment jsdom
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  BeReorderSavedFilterViewPayload,
  BeSaveSavedFilterViewPayload,
  BeSavedFilterView,
} from "@api/backend/types";
import { LEGACY_SAVED_FILTER_STORAGE_KEYS } from "@features/saved-filter-views/legacyStorage";
import { useSavedFilterViews } from "@features/saved-filter-views/useSavedFilterViews";

type View = BeSavedFilterView;

const getMock = vi.fn();
const patchMock = vi.fn();
const delMock = vi.fn();
const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({
    get: getMock,
    patch: patchMock,
    del: delMock,
    post: postMock,
  }),
}));

function wrapper() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

function inMemory(initial: View[] = []) {
  let views = [...initial];
  getMock.mockImplementation(async () => ({ views: [...views] }));
  patchMock.mockImplementation(async (_path: string, body: BeSaveSavedFilterViewPayload) => {
    const name = body.name.trim();
    views = [{ name, qs: body.qs }, ...views.filter((v) => v.name.toLowerCase() !== name.toLowerCase())];
    return { views: [...views] };
  });
  delMock.mockImplementation(async (path: string) => {
    const name = new URL(path, "http://local").searchParams.get("name") ?? "";
    views = views.filter((v) => v.name.toLowerCase() !== name.toLowerCase());
    return { views: [...views] };
  });
  postMock.mockImplementation(async (_path: string, body: BeReorderSavedFilterViewPayload) => {
    const i = views.findIndex((v) => v.name.toLowerCase() === body.name.toLowerCase());
    const t =
      body.position !== undefined ? body.position : body.direction === "up" ? i - 1 : i + 1;
    if (i >= 0 && t >= 0 && t < views.length && t !== i) {
      const next = [...views];
      const [item] = next.splice(i, 1);
      next.splice(Math.min(t, next.length), 0, item);
      views = next;
    }
    return { views: [...views] };
  });
}

beforeEach(() => {
  if (typeof localStorage === "undefined") {
    const data = new Map<string, string>();
    Object.defineProperty(globalThis, "localStorage", {
      configurable: true,
      value: {
        getItem: (k: string) => data.get(k) ?? null,
        setItem: (k: string, v: string) => {
          data.set(k, v);
        },
        removeItem: (k: string) => {
          data.delete(k);
        },
        clear: () => data.clear(),
        key: () => null,
        get length() {
          return data.size;
        },
      },
    });
  }
  localStorage.clear();
  getMock.mockReset();
  patchMock.mockReset();
  delMock.mockReset();
  postMock.mockReset();
});

describe("useSavedFilterViews", () => {
  it("loads views from GET", async () => {
    inMemory([{ name: "Open", qs: "states=open" }]);
    const { result } = renderHook(() => useSavedFilterViews("cases"), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.views).toEqual([{ name: "Open", qs: "states=open" }]));
  });

  it("saves a view via PATCH and updates the list", async () => {
    inMemory([]);
    const { result } = renderHook(() => useSavedFilterViews("cases"), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    await act(async () => {
      await result.current.saveFilterView("  Spaced  ", "states=open");
    });
    await waitFor(() =>
      expect(result.current.views).toEqual([{ name: "Spaced", qs: "states=open" }]),
    );
    expect(patchMock).toHaveBeenCalledWith("/users/me/saved-filter-views", {
      listKey: "cases",
      name: "Spaced",
      qs: "states=open",
    });
  });

  it("ignores an empty name", async () => {
    inMemory([]);
    const { result } = renderHook(() => useSavedFilterViews("incidents"), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    await act(async () => {
      await result.current.saveFilterView("   ", "q=1");
    });
    expect(patchMock).not.toHaveBeenCalled();
  });

  it("deletes via DELETE", async () => {
    inMemory([
      { name: "Keep", qs: "a" },
      { name: "Drop", qs: "b" },
    ]);
    const { result } = renderHook(() => useSavedFilterViews("problems"), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.views).toHaveLength(2));
    await act(async () => {
      await result.current.deleteFilterView("DROP");
    });
    await waitFor(() => expect(result.current.views.map((v) => v.name)).toEqual(["Keep"]));
  });

  it("reorders via POST /reorder", async () => {
    inMemory([
      { name: "C", qs: "c" },
      { name: "B", qs: "b" },
      { name: "A", qs: "a" },
    ]);
    const { result } = renderHook(() => useSavedFilterViews("change_requests"), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.views.map((v) => v.name)).toEqual(["C", "B", "A"]));
    await act(async () => {
      await result.current.moveFilterView("B", "up");
    });
    await waitFor(() => expect(result.current.views.map((v) => v.name)).toEqual(["B", "C", "A"]));
    await act(async () => {
      await result.current.reorderFilterView("A", 0);
    });
    await waitFor(() => expect(result.current.views.map((v) => v.name)).toEqual(["A", "B", "C"]));
  });

  it("migrates localStorage when GET returns an empty list, preserving order", async () => {
    localStorage.setItem(
      LEGACY_SAVED_FILTER_STORAGE_KEYS.cases,
      JSON.stringify([
        { name: "First", qs: "q=1" },
        { name: "Second", qs: "q=2" },
      ]),
    );
    inMemory([]);
    const { result } = renderHook(() => useSavedFilterViews("cases"), { wrapper: wrapper() });
    await waitFor(() =>
      expect(result.current.views.map((v) => v.name)).toEqual(["First", "Second"]),
    );
    expect(patchMock).toHaveBeenCalledTimes(2);
    expect(localStorage.getItem(LEGACY_SAVED_FILTER_STORAGE_KEYS.cases)).toBeNull();
  });

  it("uploads leftover localStorage even when GET already has views", async () => {
    localStorage.setItem(
      LEGACY_SAVED_FILTER_STORAGE_KEYS.cases,
      JSON.stringify([{ name: "Legacy", qs: "q=1" }]),
    );
    inMemory([{ name: "Server", qs: "q=s" }]);
    const { result } = renderHook(() => useSavedFilterViews("cases"), { wrapper: wrapper() });
    await waitFor(() =>
      expect(result.current.views.map((v) => v.name)).toEqual(["Legacy", "Server"]),
    );
    expect(patchMock).toHaveBeenCalledTimes(1);
    expect(localStorage.getItem(LEGACY_SAVED_FILTER_STORAGE_KEYS.cases)).toBeNull();
  });

  it("keeps unuploaded localStorage entries when a migrate PATCH fails", async () => {
    localStorage.setItem(
      LEGACY_SAVED_FILTER_STORAGE_KEYS.cases,
      JSON.stringify([
        { name: "First", qs: "q=1" },
        { name: "Second", qs: "q=2" },
      ]),
    );
    let server: View[] = [];
    getMock.mockImplementation(async () => ({ views: [...server] }));
    patchMock.mockImplementation(async () => {
      throw new Error("upload failed");
    });
    patchMock.mockImplementationOnce(async (_path: string, body: BeSaveSavedFilterViewPayload) => {
      server = [{ name: body.name, qs: body.qs }, ...server];
      return { views: [...server] };
    });
    const { result } = renderHook(() => useSavedFilterViews("cases"), { wrapper: wrapper() });
    await waitFor(() => {
      const leftover = JSON.parse(
        localStorage.getItem(LEGACY_SAVED_FILTER_STORAGE_KEYS.cases) ?? "[]",
      ) as View[];
      expect(leftover.map((v) => v.name)).toEqual(["First"]);
    });
    await waitFor(() => expect(result.current.views.map((v) => v.name)).toEqual(["Second"]));
  });

  it("clears saveError when resetSaveError is called", async () => {
    inMemory([]);
    patchMock.mockRejectedValue(new Error("save failed"));
    const { result } = renderHook(() => useSavedFilterViews("cases"), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    await act(async () => {
      await expect(result.current.saveFilterView("Keep me", "q=1")).rejects.toThrow("save failed");
    });
    await waitFor(() => expect(result.current.saveError).toBeTruthy());
    act(() => result.current.resetSaveError());
    await waitFor(() => expect(result.current.saveError).toBeNull());
  });

  it("exposes saveError when PATCH fails and does not drop the name", async () => {
    inMemory([]);
    patchMock.mockRejectedValue(new Error("save failed"));
    const { result } = renderHook(() => useSavedFilterViews("cases"), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    await act(async () => {
      await expect(result.current.saveFilterView("Keep me", "q=1")).rejects.toThrow("save failed");
    });
    await waitFor(() => expect(result.current.saveError).toBeTruthy());
    expect(result.current.views).toEqual([]);
  });
});
