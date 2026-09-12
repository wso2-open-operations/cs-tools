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

/* eslint-disable react-refresh/only-export-components -- Provider component and its useXxx hook are colocated per the repo's context idiom (fast-refresh DX only) */

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  type JSX,
  type ReactNode,
} from "react";
import {
  caseTabsReducer,
  INITIAL_CASE_TABS_STATE,
  type CaseTabsState,
} from "@context/case-tabs/caseTabsReducer";
import {
  MAX_OPEN_CASE_TABS,
  type CaseRouteKind,
  type CaseTabState,
  type CaseTabsPersistedState,
} from "@context/case-tabs/caseTabsTypes";
import { useCaseTabsBehavior } from "@context/case-tabs/CaseTabsBehaviorContext";
import { pathForTab } from "@context/case-tabs/caseRoutePatterns";

// Deliberately sessionStorage, not localStorage: an open-tabs list is
// per-browser-session working state, not something that should survive
// across logins/users on a shared machine (see this feature's design notes).
const STORAGE_KEY = "csm.caseTabs.v1";

function readPersistedState(): CaseTabsState | undefined {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return undefined;
    const parsed = JSON.parse(raw) as CaseTabsPersistedState;
    if (!Array.isArray(parsed.tabs)) return undefined;
    // Every rehydrated tab gets a fresh synthetic `id` — the persisted shape
    // only carries `caseId` + `kind`, not the prior session's internal ids —
    // and its concrete path is reconstructed from those two, the same way
    // `openTab` builds it for a freshly-opened tab.
    const tabs: CaseTabState[] = parsed.tabs.map(({ caseId, kind }) => ({
      id: nextTabId(),
      caseId,
      kind,
      path: pathForTab(kind, caseId),
      hasDraft: false,
      label: undefined,
    }));
    const activeTab = tabs.find((t) => t.caseId === parsed.activeCaseId);
    const activeTabId = activeTab ? activeTab.id : (tabs[tabs.length - 1]?.id ?? null);
    return { tabs, activeTabId };
  } catch {
    return undefined;
  }
}

function writePersistedState(state: CaseTabsState): void {
  try {
    const activeCaseId = state.tabs.find((t) => t.id === state.activeTabId)?.caseId ?? null;
    const persisted: CaseTabsPersistedState = {
      tabs: state.tabs.map(({ caseId, kind }) => ({ caseId, kind })),
      activeCaseId,
    };
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(persisted));
  } catch {
    /* sessionStorage unavailable (private mode, quota, ...) — tabs still
     * work for the current page-load, they just won't survive a reload. */
  }
}

let tabIdCounter = 0;
function nextTabId(): string {
  tabIdCounter += 1;
  return `case-tab-${Date.now().toString(36)}-${tabIdCounter}`;
}

export interface CaseTabsController {
  tabs: CaseTabState[];
  activeTabId: string | null;
  activeTab: CaseTabState | undefined;
  /** Opens a tab for `caseId` (or activates it if already open). Returns
   * `false` without changing state when the cap is already reached and this
   * would be a genuinely new tab — the caller (list row click, route sync)
   * decides how to surface that (a toast, or simply rendering the record
   * without a tab). `state` (router `location.state`, e.g. the originating
   * list's `{ from }`) is only captured on a genuinely new tab, ignored when
   * activating an already-open one. */
  openTab: (
    caseId: string,
    kind: CaseRouteKind,
    path: string,
    state?: unknown,
  ) => boolean;
  closeTab: (id: string) => void;
  /** Closes every open tab except the pinned "current location" one (which
   * is never part of this array to begin with — see `useCurrentLocationTab`
   * — so there's nothing to special-case here). Backs the tab strip's
   * right-click "Close all tabs". */
  closeAllTabs: () => void;
  /** Closes every open tab except `keepId`. Backs "Close other tabs". */
  closeOtherTabs: (keepId: string) => void;
  setActiveTab: (id: string) => void;
  updateTabPath: (id: string, kind: CaseRouteKind, path: string) => void;
  /** Reports a tab's display label plus its tooltip identity (internal id +
   * subject) — see `useReportCaseTabMeta`, the sole caller. Any omitted
   * field is left unchanged (not reset to undefined), so the label and the
   * tooltip fields — which typically resolve from the same data at slightly
   * different times — don't stomp each other. */
  setTabMeta: (
    id: string,
    meta: { label?: string; internalId?: string; subject?: string },
  ) => void;
  setTabDraft: (id: string, hasDraft: boolean) => void;
  isAtCapacity: boolean;
}

// A no-op default (not `undefined`) so `CsmCaseDetailPage` — which now
// unconditionally calls `useCaseTabsController` (via
// `useReportCaseTabDraft`) — keeps working exactly as it did before this
// feature existed when rendered outside a `CaseTabsProvider`: its many
// existing tests, Storybook-style isolated renders, etc. None of them
// exercise tab behavior, so a silent no-op is correct here, not just
// convenient — there is no "tab" for a page rendered standalone to report
// into.
const NOOP_CASE_TABS_CONTROLLER: CaseTabsController = {
  tabs: [],
  activeTabId: null,
  activeTab: undefined,
  openTab: () => true,
  closeTab: () => {},
  closeAllTabs: () => {},
  closeOtherTabs: () => {},
  setActiveTab: () => {},
  updateTabPath: () => {},
  setTabMeta: () => {},
  setTabDraft: () => {},
  isAtCapacity: false,
};

const CaseTabsContext = createContext<CaseTabsController>(NOOP_CASE_TABS_CONTROLLER);

export function CaseTabsProvider({ children }: { children: ReactNode }): JSX.Element {
  const { enabled, capMode } = useCaseTabsBehavior();
  const [state, dispatch] = useReducer(
    caseTabsReducer,
    undefined,
    // Disabled never restores a prior session's open tabs, even if some are
    // still sitting in sessionStorage from before the user (or a previous
    // session on this browser) turned the mechanism off — without this,
    // `CaseTabsContentHost`/`CaseTabStripBar` would render stale tabs while
    // disabled, since they only read `tabs` off this state and don't
    // separately re-check `enabled`.
    () =>
      enabled ? (readPersistedState() ?? INITIAL_CASE_TABS_STATE) : INITIAL_CASE_TABS_STATE,
  );

  useEffect(() => {
    writePersistedState(state);
  }, [state]);

  // Same reasoning as the lazy-init check above, but for a LIVE toggle (the
  // user turns the mechanism off via the preferences menu while tabs are
  // already open) — clears them immediately rather than leaving them to
  // linger, hidden behind `CaseTabStripBar`'s own `enabled` early return,
  // until the next full reload.
  useEffect(() => {
    if (!enabled) dispatch({ type: "HYDRATE", state: INITIAL_CASE_TABS_STATE });
  }, [enabled]);

  // Read inside `openTab` via a ref, not a `[state.tabs]` dependency: a
  // dependency there gives `openTab` a new identity on every single tab
  // change (label resolved, draft flag toggled, ...), and `openTab` sits in
  // `CaseDetailRouteSync`'s own effect dependency array — that combination
  // was re-running (and re-dispatching) that effect far more often than the
  // route actually changed. Harmless on its own (`OPEN_OR_ACTIVATE` on an
  // already-open, already-active tab is a no-op), but needless churn this
  // callback doesn't need to cause.
  const tabsRef = useRef(state.tabs);
  useEffect(() => {
    tabsRef.current = state.tabs;
  });

  // `enabled`/`capMode` are read directly from this render's closure below
  // (real `useCallback` DEPENDENCIES, not a ref) — deliberately NOT the same
  // ref-not-dependency treatment as `tabsRef` above. A ref here needs a
  // `useEffect` (or a render-time ref mutation, which this codebase's lint
  // config forbids outright — refs are for values read outside render) to
  // stay synced, and `useEffect`s fire child-before-parent within a commit:
  // a descendant (`CaseDetailRouteSync`) whose own effect calls `openTab` in
  // the SAME commit that flips `enabled` false -> true (e.g. the user
  // toggles the preference on while already viewing a case) would run
  // before this provider's own effect had synced a ref from `false` to
  // `true` — so `openTab` saw a stale `false` and silently refused the very
  // first tab (that case kept rendering un-tabbed for the rest of the
  // session, since nothing ever retried). Depending on them directly has no
  // such staleness — a closure captured during a render always sees THAT
  // render's values —
  // at the cost of `openTab` getting a new identity on a preference change,
  // which is rare (a user toggle, not per-tab churn) and, since `openTab`
  // sits in `CaseDetailRouteSync`'s own effect deps, is exactly what makes
  // that effect re-evaluate against the fresh preference in the first place.
  const openTab = useCallback(
    (caseId: string, kind: CaseRouteKind, path: string, tabState?: unknown): boolean => {
      // Disabled means the mechanism is off entirely — never opens a tab,
      // for any case. Callers (`CaseDetailRouteSync`) already skip calling
      // this while disabled, but this is the authoritative check
      // other/future callers should be able to rely on too.
      if (!enabled) return false;
      const tabs = tabsRef.current;
      const alreadyOpen = tabs.some((t) => t.caseId === caseId);
      const atCap = !alreadyOpen && tabs.length >= MAX_OPEN_CASE_TABS;
      // Both `CaseTabsCapMode` values evict an existing tab to make room —
      // there is no "refuse the new one" mode (see that type's own doc
      // comment), so a genuinely new tab always succeeds once `enabled`.
      dispatch({
        type: "OPEN_OR_ACTIVATE",
        id: nextTabId(),
        caseId,
        kind,
        path,
        state: tabState,
        evict: atCap ? (capMode === "evict-oldest" ? "oldest" : "newest") : undefined,
      });
      return true;
    },
    [enabled, capMode],
  );

  const closeTab = useCallback((id: string) => {
    dispatch({ type: "CLOSE", id });
  }, []);

  const closeAllTabs = useCallback(() => {
    dispatch({ type: "CLOSE_ALL" });
  }, []);

  const closeOtherTabs = useCallback((keepId: string) => {
    dispatch({ type: "CLOSE_OTHERS", keepId });
  }, []);

  const setActiveTab = useCallback((id: string) => {
    dispatch({ type: "SET_ACTIVE", id });
  }, []);

  const updateTabPath = useCallback((id: string, kind: CaseRouteKind, path: string) => {
    dispatch({ type: "UPDATE_TAB_PATH", id, kind, path });
  }, []);

  const setTabMeta = useCallback(
    (id: string, meta: { label?: string; internalId?: string; subject?: string }) => {
      dispatch({ type: "SET_META", id, ...meta });
    },
    [],
  );

  const setTabDraft = useCallback((id: string, hasDraft: boolean) => {
    dispatch({ type: "SET_DRAFT", id, hasDraft });
  }, []);

  const activeTab = state.tabs.find((t) => t.id === state.activeTabId);

  const value = useMemo<CaseTabsController>(
    () => ({
      tabs: state.tabs,
      activeTabId: state.activeTabId,
      activeTab,
      openTab,
      closeTab,
      closeAllTabs,
      closeOtherTabs,
      setActiveTab,
      updateTabPath,
      setTabMeta,
      setTabDraft,
      isAtCapacity: state.tabs.length >= MAX_OPEN_CASE_TABS,
    }),
    [
      state.tabs,
      state.activeTabId,
      activeTab,
      openTab,
      closeTab,
      closeAllTabs,
      closeOtherTabs,
      setActiveTab,
      updateTabPath,
      setTabMeta,
      setTabDraft,
    ],
  );

  return <CaseTabsContext.Provider value={value}>{children}</CaseTabsContext.Provider>;
}

export function useCaseTabsController(): CaseTabsController {
  return useContext(CaseTabsContext);
}

/** Ref-based escape hatch for call sites that need the latest controller
 * inside a callback without adding it to a dependency array (e.g. the
 * isolated router's navigator, which is memoized once per tab instance). */
export function useCaseTabsControllerRef() {
  const controller = useCaseTabsController();
  const ref = useRef(controller);
  // Kept fresh in an effect (not written during render) — the ref is only
  // ever read later, from event/navigation callbacks, never during render.
  useEffect(() => {
    ref.current = controller;
  });
  return ref;
}
