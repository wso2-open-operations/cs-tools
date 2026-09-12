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
  useMemo,
  useState,
  type JSX,
  type ReactNode,
} from "react";

const ENABLED_STORAGE_KEY = "csm.caseTabs.enabled";
const CAP_MODE_STORAGE_KEY = "csm.caseTabs.capMode";
// The single 4-value setting this replaced ("off" | "block" | "evict-oldest"
// | "evict-newest") — read once, best-effort, so a browser that already
// picked a preference under the old shape doesn't silently revert to the
// default. Not written to anymore; safe to remove entirely in a later pass
// once this legacy shape has been unused for a while (it was short-lived). A
// legacy value of "block" (no longer a valid `CaseTabsCapMode` — see below)
// falls through `isCapMode` and resolves to `DEFAULT_CAP_MODE` instead, same
// as any other unrecognized value.
const LEGACY_STORAGE_KEY = "csm.caseTabs.behavior";

/**
 * What happens when a distinct new case/incident/change-request is opened
 * while `MAX_OPEN_CASE_TABS` are already open (only relevant while the
 * mechanism is `enabled` — see `useCaseTabsBehavior`). Always evicts an
 * existing tab to make room for the new one, which always opens and becomes
 * active — there is deliberately no "refuse the new one" mode (the feature
 * originally shipped with one, `block`, removed once the tab strip's own
 * "Close all tabs" / "Close other tabs" context menu made blocking feel
 * like unnecessary friction rather than a useful guardrail):
 *  - `evict-newest` — the most-recently-opened tab closes instead. The
 *    DEFAULT — closes whichever tab the user is least likely to still be
 *    mid-task on, since it was opened most recently.
 *  - `evict-oldest` — the longest-open tab (first opened, not
 *    least-recently-viewed) closes to make room.
 */
export type CaseTabsCapMode = "evict-oldest" | "evict-newest";

export const CASE_TABS_CAP_MODE_OPTIONS: { mode: CaseTabsCapMode; label: string }[] = [
  { mode: "evict-newest", label: "Replace the last tab" },
  { mode: "evict-oldest", label: "Replace the oldest tab" },
];

const DEFAULT_ENABLED = true;
const DEFAULT_CAP_MODE: CaseTabsCapMode = "evict-newest";

function isCapMode(value: unknown): value is CaseTabsCapMode {
  return value === "evict-oldest" || value === "evict-newest";
}

interface CaseTabsBehaviorContextValue {
  /** Whether the in-app tab mechanism (tab strip, keep-alive pages, the
   * pinned "current location" tab) is on at all. `true` — the DEFAULT as of
   * this flip — replaces the earlier "beta, off by default" launch decision
   * per explicit user instruction; `false` is now an explicit opt-out
   * (still available via `PreferencesDialog`), not the shipped default. See
   * this flag's own persistence code below for what a `false`-default user
   * who already opted in keeps seeing (their explicit choice, either way,
   * always wins over this default). */
  enabled: boolean;
  setEnabled: (next: boolean) => void;
  /** Only meaningful (and only shown as interactive in `PreferencesDialog`)
   * when `enabled` is true. */
  capMode: CaseTabsCapMode;
  setCapMode: (next: CaseTabsCapMode) => void;
  capModeOptions: typeof CASE_TABS_CAP_MODE_OPTIONS;
}

function readInitialEnabled(): boolean {
  try {
    const saved = window.localStorage.getItem(ENABLED_STORAGE_KEY);
    if (saved === "1") return true;
    if (saved === "0") return false;
    // No value under the current key yet — fall back to the legacy
    // single-setting shape, if any, so an existing user's choice survives
    // the split rather than silently reverting to "off".
    const legacy = window.localStorage.getItem(LEGACY_STORAGE_KEY);
    if (legacy) return legacy !== "off";
  } catch {
    /* localStorage may be unavailable — fall back to the default */
  }
  return DEFAULT_ENABLED;
}

function readInitialCapMode(): CaseTabsCapMode {
  try {
    const saved = window.localStorage.getItem(CAP_MODE_STORAGE_KEY);
    if (isCapMode(saved)) return saved;
    const legacy = window.localStorage.getItem(LEGACY_STORAGE_KEY);
    if (isCapMode(legacy)) return legacy;
  } catch {
    /* localStorage may be unavailable — fall back to the default */
  }
  return DEFAULT_CAP_MODE;
}

// Default value (not `null`) matches the defaults above — a component that
// reads this outside a `CaseTabsBehaviorProvider` (an isolated test render,
// e.g.) sees exactly what a fresh, provider-wrapped session would see by
// default: tabs on. Mirrors `CaseTabsContext`'s own no-op-default pattern,
// for the same reason (many existing tests render `CsmCaseDetailPage` et al.
// standalone) — though `CaseTabsContext`'s OWN no-op default stays a no-op
// regardless of this flag (its `openTab` et al. are always inert outside a
// real `CaseTabsProvider`), so a standalone page render still can't actually
// open a tab even though this flag alone now reads `true` there.
const DEFAULT_CONTEXT_VALUE: CaseTabsBehaviorContextValue = {
  enabled: DEFAULT_ENABLED,
  setEnabled: () => {},
  capMode: DEFAULT_CAP_MODE,
  setCapMode: () => {},
  capModeOptions: CASE_TABS_CAP_MODE_OPTIONS,
};

const CaseTabsBehaviorContext =
  createContext<CaseTabsBehaviorContextValue>(DEFAULT_CONTEXT_VALUE);

/**
 * Owns the two case-tabs preferences — same persistence shape as
 * `ThemePreferenceProvider` (localStorage only, no backend sync, read once
 * on mount): a plain on/off toggle for the mechanism as a whole, and a
 * separate cap-behavior mode that only matters while it's on. Split from a
 * single 4-value setting (`off | block | evict-oldest | evict-newest`) so
 * the "what happens at the cap" choice reads as its own, independent
 * question rather than being folded into the on/off switch itself.
 */
export function CaseTabsBehaviorProvider({ children }: { children: ReactNode }): JSX.Element {
  const [enabled, setEnabledState] = useState<boolean>(() => readInitialEnabled());
  const [capMode, setCapModeState] = useState<CaseTabsCapMode>(() => readInitialCapMode());

  const setEnabled = useCallback((next: boolean): void => {
    setEnabledState(next);
    try {
      window.localStorage.setItem(ENABLED_STORAGE_KEY, next ? "1" : "0");
    } catch {
      /* ignore — the in-memory choice still applies for this session */
    }
  }, []);

  const setCapMode = useCallback((next: CaseTabsCapMode): void => {
    setCapModeState(next);
    try {
      window.localStorage.setItem(CAP_MODE_STORAGE_KEY, next);
    } catch {
      /* ignore — the in-memory choice still applies for this session */
    }
  }, []);

  const value = useMemo<CaseTabsBehaviorContextValue>(
    () => ({
      enabled,
      setEnabled,
      capMode,
      setCapMode,
      capModeOptions: CASE_TABS_CAP_MODE_OPTIONS,
    }),
    [enabled, setEnabled, capMode, setCapMode],
  );

  return (
    <CaseTabsBehaviorContext.Provider value={value}>{children}</CaseTabsBehaviorContext.Provider>
  );
}

export function useCaseTabsBehavior(): CaseTabsBehaviorContextValue {
  return useContext(CaseTabsBehaviorContext);
}
