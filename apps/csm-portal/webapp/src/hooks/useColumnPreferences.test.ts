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

import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import {
  getColumnPreferencesUserKey,
  useColumnPreferences,
  type ColumnOption,
} from "@hooks/useColumnPreferences";

const COLUMNS: ColumnOption[] = [
  { id: "a", label: "Column A" },
  { id: "b", label: "Column B" },
  { id: "c", label: "Column C" },
];

beforeEach(() => {
  window.localStorage.clear();
});

describe("getColumnPreferencesUserKey", () => {
  it("prefers the platform user id", () => {
    expect(getColumnPreferencesUserKey({ id: "u-1", email: "jane.doe@example.com" })).toBe("u-1");
  });

  it("falls back to email when id is missing", () => {
    expect(getColumnPreferencesUserKey({ email: "jane.doe@example.com" })).toBe(
      "jane.doe@example.com",
    );
  });

  it("falls back to a shared anonymous bucket when neither is available", () => {
    expect(getColumnPreferencesUserKey(undefined)).toBe("anonymous");
    expect(getColumnPreferencesUserKey({})).toBe("anonymous");
  });
});

describe("useColumnPreferences", () => {
  it("starts with the default visible columns, in definition order", () => {
    const { result } = renderHook(() =>
      useColumnPreferences({
        viewId: "test-view",
        userKey: "user-1",
        columns: COLUMNS,
        defaultVisibleIds: ["a", "c"],
      }),
    );

    expect(result.current.allColumns.map((c) => c.id)).toEqual(["a", "b", "c"]);
    expect(result.current.visibleColumns.map((c) => c.id)).toEqual(["a", "c"]);
    expect(result.current.isVisible("b")).toBe(false);
  });

  it("toggles a column on and off, and persists across a fresh hook instance", () => {
    const args = {
      viewId: "test-view",
      userKey: "user-1",
      columns: COLUMNS,
      defaultVisibleIds: ["a"],
    };
    const { result, rerender } = renderHook(useColumnPreferences, { initialProps: args });

    act(() => result.current.toggleColumn("b"));
    expect(result.current.visibleColumns.map((c) => c.id)).toEqual(["a", "b"]);

    rerender(args);
    expect(result.current.visibleColumns.map((c) => c.id)).toEqual(["a", "b"]);

    // A brand new hook instance for the same user + view picks up the saved state.
    const { result: reloaded } = renderHook(() => useColumnPreferences(args));
    expect(reloaded.current.visibleColumns.map((c) => c.id)).toEqual(["a", "b"]);
  });

  it("refuses to hide the last visible column", () => {
    const { result } = renderHook(() =>
      useColumnPreferences({
        viewId: "test-view",
        userKey: "user-1",
        columns: COLUMNS,
        defaultVisibleIds: ["a"],
      }),
    );

    act(() => result.current.toggleColumn("a"));
    expect(result.current.visibleColumns.map((c) => c.id)).toEqual(["a"]);
  });

  it("reorders columns with moveColumn, both visible and hidden together", () => {
    const { result } = renderHook(() =>
      useColumnPreferences({
        viewId: "test-view",
        userKey: "user-1",
        columns: COLUMNS,
        defaultVisibleIds: ["a", "b", "c"],
      }),
    );

    act(() => result.current.moveColumn("c", "up"));
    expect(result.current.allColumns.map((c) => c.id)).toEqual(["a", "c", "b"]);
    expect(result.current.visibleColumns.map((c) => c.id)).toEqual(["a", "c", "b"]);

    // No-ops past either end.
    act(() => result.current.moveColumn("a", "up"));
    expect(result.current.allColumns.map((c) => c.id)).toEqual(["a", "c", "b"]);
    act(() => result.current.moveColumn("b", "down"));
    expect(result.current.allColumns.map((c) => c.id)).toEqual(["a", "c", "b"]);
  });

  it("reorders a column directly to a target index in one call, unlike a repeated moveColumn", () => {
    const { result } = renderHook(() =>
      useColumnPreferences({
        viewId: "test-view",
        userKey: "user-1",
        columns: COLUMNS,
        defaultVisibleIds: ["a", "b", "c"],
      }),
    );

    // Drag "a" (index 0) down to the last slot (index 2) in one gesture --
    // the scenario `reorderColumn` exists for, since two back-to-back
    // `moveColumn` calls in the same handler would both act on the same
    // pre-move `state` and only net a single-slot move.
    act(() => result.current.reorderColumn("a", 2));
    expect(result.current.allColumns.map((c) => c.id)).toEqual(["b", "c", "a"]);
  });

  it("clamps an out-of-range reorderColumn target to the list's bounds", () => {
    const { result } = renderHook(() =>
      useColumnPreferences({
        viewId: "test-view",
        userKey: "user-1",
        columns: COLUMNS,
        defaultVisibleIds: ["a", "b", "c"],
      }),
    );

    act(() => result.current.reorderColumn("a", 99));
    expect(result.current.allColumns.map((c) => c.id)).toEqual(["b", "c", "a"]);
  });

  it("no-ops reorderColumn for an unknown id or a target it already sits at", () => {
    const { result } = renderHook(() =>
      useColumnPreferences({
        viewId: "test-view",
        userKey: "user-1",
        columns: COLUMNS,
        defaultVisibleIds: ["a", "b", "c"],
      }),
    );

    act(() => result.current.reorderColumn("does-not-exist", 1));
    expect(result.current.allColumns.map((c) => c.id)).toEqual(["a", "b", "c"]);

    act(() => result.current.reorderColumn("b", 1));
    expect(result.current.allColumns.map((c) => c.id)).toEqual(["a", "b", "c"]);
  });

  it("resets to the table's built-in default order and visibility", () => {
    const { result } = renderHook(() =>
      useColumnPreferences({
        viewId: "test-view",
        userKey: "user-1",
        columns: COLUMNS,
        defaultVisibleIds: ["a"],
      }),
    );

    act(() => result.current.toggleColumn("b"));
    act(() => result.current.moveColumn("c", "up"));
    act(() => result.current.resetToDefault());

    expect(result.current.allColumns.map((c) => c.id)).toEqual(["a", "b", "c"]);
    expect(result.current.visibleColumns.map((c) => c.id)).toEqual(["a"]);
  });

  it("keys storage per user and per view, so neither leaks into the other", () => {
    const base = { columns: COLUMNS, defaultVisibleIds: ["a"] };
    const view1User1 = renderHook(() =>
      useColumnPreferences({ ...base, viewId: "view-1", userKey: "user-1" }),
    );
    act(() => view1User1.result.current.toggleColumn("b"));

    const view1User2 = renderHook(() =>
      useColumnPreferences({ ...base, viewId: "view-1", userKey: "user-2" }),
    );
    expect(view1User2.result.current.visibleColumns.map((c) => c.id)).toEqual(["a"]);

    const view2User1 = renderHook(() =>
      useColumnPreferences({ ...base, viewId: "view-2", userKey: "user-1" }),
    );
    expect(view2User1.result.current.visibleColumns.map((c) => c.id)).toEqual(["a"]);
  });

  it("drops columns the table no longer defines and appends new ones the saved state predates", () => {
    const key = "csm:user-1:test-view:columns";
    window.localStorage.setItem(
      key,
      JSON.stringify({ order: ["a", "removed"], visible: ["a", "removed"] }),
    );

    const { result } = renderHook(() =>
      useColumnPreferences({
        viewId: "test-view",
        userKey: "user-1",
        columns: COLUMNS,
        defaultVisibleIds: ["a", "b"],
      }),
    );

    // "removed" is gone; "b" and "c" (unknown to the saved state) are appended.
    expect(result.current.allColumns.map((c) => c.id)).toEqual(["a", "b", "c"]);
    // "b" is newly appended and its default says visible; "c" defaults hidden.
    expect(result.current.visibleColumns.map((c) => c.id)).toEqual(["a", "b"]);
  });

  it("re-reconciles against the new key when userKey changes after mount", () => {
    // Mirrors the real call sites: on first render, the signed-in user's id
    // hasn't resolved yet (useCurrentUser()/useIdTokenClaims() are both async),
    // so the hook is first rendered with userKey "anonymous", then rerendered
    // once the real id lands.
    window.localStorage.setItem(
      "csm:anonymous:test-view:columns",
      JSON.stringify({ order: ["c", "a", "b"], visible: ["c"] }),
    );
    window.localStorage.setItem(
      "csm:user-1:test-view:columns",
      JSON.stringify({ order: ["b", "a", "c"], visible: ["b", "a"] }),
    );

    const base = { viewId: "test-view", columns: COLUMNS, defaultVisibleIds: ["a"] };
    const { result, rerender } = renderHook(useColumnPreferences, {
      initialProps: { ...base, userKey: "anonymous" },
    });

    // First render picks up the anonymous bucket, as expected.
    expect(result.current.allColumns.map((c) => c.id)).toEqual(["c", "a", "b"]);
    expect(result.current.visibleColumns.map((c) => c.id)).toEqual(["c"]);

    // The real user id resolves a render or two later.
    rerender({ ...base, userKey: "user-1" });

    // The hook must now reflect user-1's saved layout, not the anonymous
    // state it happened to start with.
    expect(result.current.allColumns.map((c) => c.id)).toEqual(["b", "a", "c"]);
    expect(result.current.visibleColumns.map((c) => c.id)).toEqual(["b", "a"]);

    // And a subsequent toggle must save under user-1's key, not clobber it
    // with the anonymous session's stale state.
    act(() => result.current.toggleColumn("c"));
    const saved = JSON.parse(
      window.localStorage.getItem("csm:user-1:test-view:columns") ?? "null",
    );
    expect(saved.visible).toEqual(["b", "a", "c"]);
  });

  it("dedupes duplicate ids in a persisted order/visible array on reconcile", () => {
    // A hand-edited (or otherwise corrupted) localStorage value with a
    // duplicate id — every consumer keys off `allColumns`/`visibleColumns`
    // ids (e.g. `TableCell key={id}`), so a surviving duplicate is a React
    // duplicate-key rendering bug, not just untidy data.
    const key = "csm:user-1:test-view:columns";
    window.localStorage.setItem(
      key,
      JSON.stringify({ order: ["a", "a", "b", "c"], visible: ["a", "a", "c"] }),
    );

    const { result } = renderHook(() =>
      useColumnPreferences({
        viewId: "test-view",
        userKey: "user-1",
        columns: COLUMNS,
        defaultVisibleIds: ["a", "b"],
      }),
    );

    expect(result.current.allColumns.map((c) => c.id)).toEqual(["a", "b", "c"]);
    expect(result.current.visibleColumns.map((c) => c.id)).toEqual(["a", "c"]);
  });

  it("falls back to defaults when the stored value is corrupt", () => {
    window.localStorage.setItem("csm:user-1:test-view:columns", "not json");

    const { result } = renderHook(() =>
      useColumnPreferences({
        viewId: "test-view",
        userKey: "user-1",
        columns: COLUMNS,
        defaultVisibleIds: ["a"],
      }),
    );

    expect(result.current.visibleColumns.map((c) => c.id)).toEqual(["a"]);
  });
});
