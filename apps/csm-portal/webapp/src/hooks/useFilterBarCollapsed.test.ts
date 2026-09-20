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
import { useFilterBarCollapsed } from "@hooks/useFilterBarCollapsed";

beforeEach(() => {
  window.localStorage.clear();
});

describe("useFilterBarCollapsed", () => {
  it("starts at the caller's default when nothing is stored yet", () => {
    const { result } = renderHook(() => useFilterBarCollapsed("cases", "user-1", true));
    expect(result.current[0]).toBe(true);

    const { result: closedDefault } = renderHook(() =>
      useFilterBarCollapsed("conversations", "user-1", false),
    );
    expect(closedDefault.current[0]).toBe(false);
  });

  it("toggles and persists across a fresh hook instance for the same user + view", () => {
    const { result, rerender } = renderHook(
      ({ viewId, userKey, defaultOpen }) => useFilterBarCollapsed(viewId, userKey, defaultOpen),
      { initialProps: { viewId: "cases", userKey: "user-1", defaultOpen: true } },
    );

    act(() => result.current[1](false));
    expect(result.current[0]).toBe(false);

    rerender({ viewId: "cases", userKey: "user-1", defaultOpen: true });
    expect(result.current[0]).toBe(false);

    // A brand new hook instance for the same user + view picks up the saved state.
    const { result: reloaded } = renderHook(() =>
      useFilterBarCollapsed("cases", "user-1", true),
    );
    expect(reloaded.current[0]).toBe(false);
  });

  it("keys storage per user and per view, so neither leaks into the other", () => {
    const view1User1 = renderHook(() => useFilterBarCollapsed("cases", "user-1", true));
    act(() => view1User1.result.current[1](false));

    const view1User2 = renderHook(() => useFilterBarCollapsed("cases", "user-2", true));
    expect(view1User2.result.current[0]).toBe(true);

    const view2User1 = renderHook(() => useFilterBarCollapsed("incidents", "user-1", true));
    expect(view2User1.result.current[0]).toBe(true);
  });

  it("re-reconciles against the new key when userKey changes after mount", () => {
    // Mirrors the real call sites: on first render, the signed-in user's id
    // hasn't resolved yet (useCurrentUser()/useIdTokenClaims() are both
    // async), so the hook is first rendered under an "anonymous" userKey,
    // then rerendered once the real id lands.
    window.localStorage.setItem("csm:anonymous:cases:filtersCollapsed", JSON.stringify(false));
    window.localStorage.setItem("csm:user-1:cases:filtersCollapsed", JSON.stringify(true));

    const { result, rerender } = renderHook(
      ({ userKey }) => useFilterBarCollapsed("cases", userKey, true),
      { initialProps: { userKey: "anonymous" } },
    );

    expect(result.current[0]).toBe(false);

    rerender({ userKey: "user-1" });
    expect(result.current[0]).toBe(true);

    // A subsequent toggle must save under user-1's key, not clobber the
    // anonymous session's stale state.
    act(() => result.current[1](false));
    expect(window.localStorage.getItem("csm:user-1:cases:filtersCollapsed")).toBe("false");
    expect(window.localStorage.getItem("csm:anonymous:cases:filtersCollapsed")).toBe("false");
  });

  it("falls back to the caller's default when the stored value is corrupt", () => {
    window.localStorage.setItem("csm:user-1:cases:filtersCollapsed", "not json");
    const { result } = renderHook(() => useFilterBarCollapsed("cases", "user-1", true));
    expect(result.current[0]).toBe(true);
  });

  it("falls back to the caller's default when the stored value isn't a boolean", () => {
    window.localStorage.setItem("csm:user-1:cases:filtersCollapsed", JSON.stringify("open"));
    const { result } = renderHook(() => useFilterBarCollapsed("cases", "user-1", true));
    expect(result.current[0]).toBe(true);
  });
});
