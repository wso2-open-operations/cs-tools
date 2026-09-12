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

import { describe, expect, it } from "vitest";
import {
  caseTabsReducer,
  INITIAL_CASE_TABS_STATE,
  type CaseTabsState,
} from "@context/case-tabs/caseTabsReducer";
import { MAX_OPEN_CASE_TABS } from "@context/case-tabs/caseTabsTypes";

function open(
  state: CaseTabsState,
  id: string,
  caseId: string,
  kind: "case" | "engagement" = "case",
): CaseTabsState {
  return caseTabsReducer(state, {
    type: "OPEN_OR_ACTIVATE",
    id,
    caseId,
    kind,
    path: `/cases/${caseId}`,
  });
}

function openWithEvict(
  state: CaseTabsState,
  id: string,
  caseId: string,
  evict: "oldest" | "newest",
): CaseTabsState {
  return caseTabsReducer(state, {
    type: "OPEN_OR_ACTIVATE",
    id,
    caseId,
    kind: "case",
    path: `/cases/${caseId}`,
    evict,
  });
}

function fillToCap(): CaseTabsState {
  let state = INITIAL_CASE_TABS_STATE;
  for (let i = 0; i < MAX_OPEN_CASE_TABS; i++) {
    state = open(state, `t${i}`, `CS${i}`);
  }
  return state;
}

describe("caseTabsReducer", () => {
  it("opens a new tab and makes it active", () => {
    const state = open(INITIAL_CASE_TABS_STATE, "t1", "CS1");
    expect(state.tabs).toHaveLength(1);
    expect(state.tabs[0]).toMatchObject({ id: "t1", caseId: "CS1", kind: "case" });
    expect(state.activeTabId).toBe("t1");
  });

  it("activates (not duplicates) an already-open case", () => {
    let state = open(INITIAL_CASE_TABS_STATE, "t1", "CS1");
    state = open(state, "t2", "CS2");
    // OPEN_OR_ACTIVATE for CS1 again, with a different synthetic id — the
    // reducer must find it by caseId and reuse the existing tab, not add one.
    state = open(state, "t1-again-ignored", "CS1");
    expect(state.tabs).toHaveLength(2);
    expect(state.activeTabId).toBe("t1");
  });

  // Regression test for bug: reactivating an already-open tab (a bookmark or
  // a related-case link to the SAME case but a different query
  // string/hash/section) silently discarded the new path/kind/state,
  // leaving the tab pinned to whatever it had from when it was first opened.
  it("reactivating an already-open tab updates its path/kind/state to the new navigation, not just activeTabId", () => {
    let state = caseTabsReducer(INITIAL_CASE_TABS_STATE, {
      type: "OPEN_OR_ACTIVATE",
      id: "t1",
      caseId: "CS1",
      kind: "case",
      path: "/cases/CS1?tab=details",
      state: { from: "/cases?tab=open" },
    });
    state = open(state, "t2", "CS2");

    state = caseTabsReducer(state, {
      type: "OPEN_OR_ACTIVATE",
      id: "ignored-existing-tab-keeps-its-own-id",
      caseId: "CS1",
      kind: "case",
      path: "/cases/CS1?tab=activities",
      state: { from: "/related-cases-widget" },
    });

    expect(state.tabs).toHaveLength(2);
    expect(state.activeTabId).toBe("t1");
    const reactivated = state.tabs.find((t) => t.id === "t1");
    expect(reactivated?.path).toBe("/cases/CS1?tab=activities");
    expect(reactivated?.state).toEqual({ from: "/related-cases-widget" });
  });

  it("reactivating an already-open tab with the exact same path/kind/state returns the same state object", () => {
    const navState = { from: "/cases?tab=open" };
    let state = caseTabsReducer(INITIAL_CASE_TABS_STATE, {
      type: "OPEN_OR_ACTIVATE",
      id: "t1",
      caseId: "CS1",
      kind: "case",
      path: "/cases/CS1",
      state: navState,
    });
    // Already active, and re-dispatched with the literal same values —
    // nothing to change, must return the identical state reference (same
    // no-op contract as every other reducer case here).
    const before = state;
    state = caseTabsReducer(state, {
      type: "OPEN_OR_ACTIVATE",
      id: "ignored",
      caseId: "CS1",
      kind: "case",
      path: "/cases/CS1",
      state: navState,
    });
    expect(state).toBe(before);
  });

  // There is no longer a "refuse the new tab" outcome at all — every
  // `CaseTabsCapMode` evicts an existing tab to make room instead (see that
  // type's own doc comment). An `OPEN_OR_ACTIVATE` past the cap with no
  // `evict` at all (the `open` helper here never sets one — only
  // `openWithEvict` does) still isn't refused: it defaults to evicting the
  // newest tab, same as an explicit `evict: "newest"` would (see this
  // reducer's own comment on why — the caller, `useCaseTabsController.
  // openTab`, always supplies one in practice; this is what happens if a
  // dispatch somehow doesn't).
  it("opening a new tab past the cap with no evict specified defaults to evicting the newest tab", () => {
    let state = INITIAL_CASE_TABS_STATE;
    for (let i = 0; i < MAX_OPEN_CASE_TABS; i++) {
      state = open(state, `t${i}`, `CS${i}`);
    }
    expect(state.tabs).toHaveLength(MAX_OPEN_CASE_TABS);
    const opened = open(state, "overflow", "CS-overflow");
    expect(opened.tabs).toHaveLength(MAX_OPEN_CASE_TABS);
    expect(opened.tabs.some((t) => t.caseId === "CS-overflow")).toBe(true);
    // The most-recently-opened tab (tMAX_OPEN_CASE_TABS-1, i.e. the last of
    // the fill loop) is the one evicted.
    expect(opened.tabs.some((t) => t.caseId === `CS${MAX_OPEN_CASE_TABS - 1}`)).toBe(
      false,
    );
    expect(opened.tabs.some((t) => t.caseId === "CS0")).toBe(true);
  });

  it("still activates an already-open tab even when at the cap", () => {
    let state = INITIAL_CASE_TABS_STATE;
    for (let i = 0; i < MAX_OPEN_CASE_TABS; i++) {
      state = open(state, `t${i}`, `CS${i}`);
    }
    state = open(state, "ignored", "CS2");
    expect(state.activeTabId).toBe("t2");
  });

  it("closes a tab and activates its right neighbor", () => {
    let state = INITIAL_CASE_TABS_STATE;
    state = open(state, "t1", "CS1");
    state = open(state, "t2", "CS2");
    state = open(state, "t3", "CS3");
    state = caseTabsReducer(state, { type: "SET_ACTIVE", id: "t2" });
    state = caseTabsReducer(state, { type: "CLOSE", id: "t2" });
    expect(state.tabs.map((t) => t.id)).toEqual(["t1", "t3"]);
    expect(state.activeTabId).toBe("t3");
  });

  it("closes the last tab and falls back to its left neighbor", () => {
    let state = INITIAL_CASE_TABS_STATE;
    state = open(state, "t1", "CS1");
    state = open(state, "t2", "CS2");
    state = caseTabsReducer(state, { type: "CLOSE", id: "t2" });
    expect(state.tabs.map((t) => t.id)).toEqual(["t1"]);
    expect(state.activeTabId).toBe("t1");
  });

  it("closing the only tab leaves no active tab", () => {
    let state = open(INITIAL_CASE_TABS_STATE, "t1", "CS1");
    state = caseTabsReducer(state, { type: "CLOSE", id: "t1" });
    expect(state.tabs).toHaveLength(0);
    expect(state.activeTabId).toBeNull();
  });

  it("closing a background (non-active) tab leaves the active tab untouched", () => {
    let state = INITIAL_CASE_TABS_STATE;
    state = open(state, "t1", "CS1");
    state = open(state, "t2", "CS2"); // t2 becomes active
    state = caseTabsReducer(state, { type: "CLOSE", id: "t1" });
    expect(state.activeTabId).toBe("t2");
    expect(state.tabs.map((t) => t.id)).toEqual(["t2"]);
  });

  it("updates a tab's path/kind in place without changing its id or position", () => {
    let state = INITIAL_CASE_TABS_STATE;
    state = open(state, "t1", "CS1");
    state = open(state, "t2", "CS2");
    state = caseTabsReducer(state, {
      type: "UPDATE_TAB_PATH",
      id: "t1",
      kind: "engagement",
      path: "/engagements/CS1",
    });
    expect(state.tabs.map((t) => t.id)).toEqual(["t1", "t2"]);
    expect(state.tabs[0]).toMatchObject({
      id: "t1",
      caseId: "CS1",
      kind: "engagement",
      path: "/engagements/CS1",
    });
  });

  it("sets a tab's label and draft flag independently", () => {
    let state = open(INITIAL_CASE_TABS_STATE, "t1", "CS1");
    state = caseTabsReducer(state, { type: "SET_META", id: "t1", label: "CS0001" });
    state = caseTabsReducer(state, { type: "SET_DRAFT", id: "t1", hasDraft: true });
    expect(state.tabs[0].label).toBe("CS0001");
    expect(state.tabs[0].hasDraft).toBe(true);
  });

  it("sets a tab's internalId/subject (tooltip fields) without touching its label", () => {
    let state = open(INITIAL_CASE_TABS_STATE, "t1", "CS1");
    state = caseTabsReducer(state, { type: "SET_META", id: "t1", label: "CS0001" });
    state = caseTabsReducer(state, {
      type: "SET_META",
      id: "t1",
      internalId: "CPASUB-8",
      subject: "Something broke",
    });
    expect(state.tabs[0]).toMatchObject({
      label: "CS0001",
      internalId: "CPASUB-8",
      subject: "Something broke",
    });
  });

  it("hydrates from a persisted state wholesale", () => {
    const persisted: CaseTabsState = {
      tabs: [{ id: "t1", caseId: "CS1", kind: "case", path: "/cases/CS1", hasDraft: false }],
      activeTabId: "t1",
    };
    const state = caseTabsReducer(INITIAL_CASE_TABS_STATE, {
      type: "HYDRATE",
      state: persisted,
    });
    expect(state).toEqual(persisted);
  });

  it("is a no-op for an action targeting an unknown tab id", () => {
    const state = open(INITIAL_CASE_TABS_STATE, "t1", "CS1");
    const unchanged = caseTabsReducer(state, { type: "SET_ACTIVE", id: "does-not-exist" });
    expect(unchanged).toBe(state);
    const unchanged2 = caseTabsReducer(state, { type: "CLOSE", id: "does-not-exist" });
    expect(unchanged2).toBe(state);
  });

  describe("eviction (mode 'evict-oldest' / 'evict-newest')", () => {
    it("evicts the first-opened tab (insertion order, not LRU) when evict is 'oldest'", () => {
      const full = fillToCap();
      const openOrder = full.tabs.map((t) => t.id);
      const state = openWithEvict(full, "new", "CS-new", "oldest");
      expect(state.tabs).toHaveLength(MAX_OPEN_CASE_TABS);
      // The very first tab opened (t0) is gone; every other original tab
      // survives in its original relative order, with the new one appended.
      expect(state.tabs.map((t) => t.id)).toEqual([...openOrder.slice(1), "new"]);
      expect(state.tabs.some((t) => t.caseId === "CS0")).toBe(false);
      expect(state.activeTabId).toBe("new");
    });

    it("evicts the last-opened tab when evict is 'newest'", () => {
      const full = fillToCap();
      const openOrder = full.tabs.map((t) => t.id);
      const state = openWithEvict(full, "new", "CS-new", "newest");
      expect(state.tabs).toHaveLength(MAX_OPEN_CASE_TABS);
      expect(state.tabs.map((t) => t.id)).toEqual([...openOrder.slice(0, -1), "new"]);
      expect(state.tabs.some((t) => t.caseId === `CS${MAX_OPEN_CASE_TABS - 1}`)).toBe(false);
      expect(state.activeTabId).toBe("new");
    });

    it("does not evict anything when the case is already open, even with evict set", () => {
      const full = fillToCap();
      const state = openWithEvict(full, "ignored-new-id", "CS0", "oldest");
      expect(state.tabs).toHaveLength(MAX_OPEN_CASE_TABS);
      expect(state.tabs.map((t) => t.id)).toEqual(full.tabs.map((t) => t.id));
      expect(state.activeTabId).toBe("t0");
    });

    it("does not evict anything when under the cap, even with evict set", () => {
      let state = INITIAL_CASE_TABS_STATE;
      state = open(state, "t1", "CS1");
      state = openWithEvict(state, "t2", "CS2", "oldest");
      expect(state.tabs.map((t) => t.id)).toEqual(["t1", "t2"]);
    });
  });
});
