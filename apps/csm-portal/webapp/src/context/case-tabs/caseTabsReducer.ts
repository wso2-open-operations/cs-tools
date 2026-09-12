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

import {
  MAX_OPEN_CASE_TABS,
  type CaseRouteKind,
  type CaseTabState,
} from "@context/case-tabs/caseTabsTypes";

export interface CaseTabsState {
  tabs: CaseTabState[];
  activeTabId: string | null;
}

export const INITIAL_CASE_TABS_STATE: CaseTabsState = {
  tabs: [],
  activeTabId: null,
};

export type CaseTabsAction =
  | {
      type: "OPEN_OR_ACTIVATE";
      id: string;
      caseId: string;
      kind: CaseRouteKind;
      path: string;
      state?: unknown;
      /** Set by the caller (`useCaseTabsController.openTab`, based on the
       * current `CaseTabsCapMode`) when opening at the cap should evict
       * a tab to make room. `"oldest"` closes `tabs[0]` (first opened);
       * `"newest"` closes the last one. Ignored (irrelevant) when under the
       * cap or the case is already open. */
      evict?: "oldest" | "newest";
    }
  | { type: "CLOSE"; id: string }
  /** Closes every open tab. */
  | { type: "CLOSE_ALL" }
  /** Closes every open tab except `keepId`. */
  | { type: "CLOSE_OTHERS"; keepId: string }
  | { type: "SET_ACTIVE"; id: string }
  /** Same case, path (and/or kind) changed in place — see
   * `CaseTabIsolatedRouter`'s navigator. */
  | { type: "UPDATE_TAB_PATH"; id: string; kind: CaseRouteKind; path: string }
  /** Any subset of the display fields — an omitted field is left unchanged,
   * not reset. See `CaseTabsController.setTabMeta`'s own doc comment. */
  | {
      type: "SET_META";
      id: string;
      label?: string;
      internalId?: string;
      subject?: string;
    }
  | { type: "SET_DRAFT"; id: string; hasDraft: boolean }
  | { type: "HYDRATE"; state: CaseTabsState };

/** Picks the tab that should become active after the given one closes: the
 * next tab to its right, or failing that the previous one, matching the
 * common browser-tab-strip convention. `tabsBeforeClose` still contains the
 * closed tab at `closedIndex`. */
function nextActiveAfterClose(
  tabsBeforeClose: CaseTabState[],
  closedIndex: number,
): string | null {
  const rightNeighbor = tabsBeforeClose[closedIndex + 1];
  if (rightNeighbor) return rightNeighbor.id;
  const leftNeighbor = tabsBeforeClose[closedIndex - 1];
  return leftNeighbor?.id ?? null;
}

export function caseTabsReducer(
  state: CaseTabsState,
  action: CaseTabsAction,
): CaseTabsState {
  switch (action.type) {
    case "HYDRATE":
      return action.state;

    case "OPEN_OR_ACTIVATE": {
      const existing = state.tabs.find((t) => t.caseId === action.caseId);
      if (existing) {
        // Reactivating an already-open tab must still pick up whatever the
        // triggering navigation actually resolved to — `CaseDetailRouteSync`
        // dispatches this on every route match for the case, including a
        // bookmark/related-case link to the SAME case but a different
        // query string, `#hash`, or internal section. Without this, that
        // navigation silently discarded (path/kind/state stayed whatever
        // the tab had from when it was first opened) — same class of "stale
        // in-place update" `UPDATE_TAB_PATH` already exists to fix, just for
        // an outside (not in-tab) navigation instead.
        const pathChanged = existing.path !== action.path || existing.kind !== action.kind;
        const dataChanged = existing.state !== action.state;
        const activeChanged = existing.id !== state.activeTabId;
        if (!pathChanged && !dataChanged && !activeChanged) {
          return state;
        }
        const tabs =
          pathChanged || dataChanged
            ? state.tabs.map((t) =>
                t.id === existing.id
                  ? { ...t, path: action.path, kind: action.kind, state: action.state }
                  : t,
              )
            : state.tabs;
        return { tabs, activeTabId: existing.id };
      }
      let tabs = state.tabs;
      if (tabs.length >= MAX_OPEN_CASE_TABS) {
        // Caller (`useCaseTabsController.openTab`) is responsible for
        // skipping the dispatch entirely while the mechanism is disabled;
        // once `enabled`, every `CaseTabsCapMode` evicts an existing tab
        // rather than refusing the new one (see that type's own doc
        // comment — there is no longer a "refuse" mode), so `action.evict`
        // is always set here in practice. `"oldest"` is the only value that
        // means "oldest"; anything else (including a missing value, which
        // shouldn't happen but is treated as the current default mode
        // rather than silently dropping the open request) evicts newest.
        tabs = action.evict === "oldest" ? tabs.slice(1) : tabs.slice(0, -1);
      }
      const newTab: CaseTabState = {
        id: action.id,
        caseId: action.caseId,
        kind: action.kind,
        path: action.path,
        hasDraft: false,
        state: action.state,
      };
      return {
        tabs: [...tabs, newTab],
        activeTabId: newTab.id,
      };
    }

    case "CLOSE": {
      const index = state.tabs.findIndex((t) => t.id === action.id);
      if (index === -1) return state;
      const tabs = state.tabs.filter((t) => t.id !== action.id);
      const activeTabId =
        state.activeTabId === action.id
          ? nextActiveAfterClose(state.tabs, index)
          : state.activeTabId;
      return { tabs, activeTabId };
    }

    case "CLOSE_ALL": {
      if (state.tabs.length === 0) return state;
      return { tabs: [], activeTabId: null };
    }

    case "CLOSE_OTHERS": {
      const keep = state.tabs.find((t) => t.id === action.keepId);
      if (!keep) return state;
      if (state.tabs.length === 1) return state;
      return { tabs: [keep], activeTabId: keep.id };
    }

    case "SET_ACTIVE": {
      if (state.activeTabId === action.id) return state;
      if (!state.tabs.some((t) => t.id === action.id)) return state;
      return { ...state, activeTabId: action.id };
    }

    case "UPDATE_TAB_PATH": {
      const index = state.tabs.findIndex((t) => t.id === action.id);
      if (index === -1) return state;
      const current = state.tabs[index];
      if (current.path === action.path && current.kind === action.kind) {
        return state;
      }
      const tabs = state.tabs.slice();
      tabs[index] = { ...current, path: action.path, kind: action.kind };
      return { ...state, tabs };
    }

    case "SET_META": {
      const index = state.tabs.findIndex((t) => t.id === action.id);
      if (index === -1) return state;
      const current = state.tabs[index];
      const next = {
        ...current,
        ...(action.label !== undefined ? { label: action.label } : {}),
        ...(action.internalId !== undefined ? { internalId: action.internalId } : {}),
        ...(action.subject !== undefined ? { subject: action.subject } : {}),
      };
      if (
        next.label === current.label &&
        next.internalId === current.internalId &&
        next.subject === current.subject
      ) {
        return state;
      }
      const tabs = state.tabs.slice();
      tabs[index] = next;
      return { ...state, tabs };
    }

    case "SET_DRAFT": {
      const index = state.tabs.findIndex((t) => t.id === action.id);
      if (index === -1 || state.tabs[index].hasDraft === action.hasDraft) {
        return state;
      }
      const tabs = state.tabs.slice();
      tabs[index] = { ...tabs[index], hasDraft: action.hasDraft };
      return { ...state, tabs };
    }

    default:
      return state;
  }
}
