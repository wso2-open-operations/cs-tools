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

import { useCallback, useState } from "react";

function storageKey(viewId: string, userKey: string): string {
  return `csm:${userKey}:${viewId}:filtersCollapsed`;
}

function loadPersisted(key: string): boolean | undefined {
  try {
    const raw = window.localStorage.getItem(key);
    if (raw === null) return undefined;
    const parsed = JSON.parse(raw) as unknown;
    return typeof parsed === "boolean" ? parsed : undefined;
  } catch {
    // Corrupt/inaccessible storage (private browsing, hand-edited value, …) —
    // fall back to the caller's own default rather than throwing.
    return undefined;
  }
}

function savePersisted(key: string, value: boolean): void {
  try {
    window.localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // Storage full/unavailable — the toggle still works for this session, it
    // just won't survive a reload. Not worth surfacing to the user.
  }
}

/**
 * Per-user, per-view "is the filter section open" state, persisted to
 * `localStorage` (no backend round trip — this is presentation-only and
 * never needs to sync across devices). Mirrors {@link useColumnPreferences}'s
 * storage/lazy-init/corrupted-value-guard pattern; see that hook's doc
 * comment for the full reasoning.
 *
 * `defaultOpen` only applies the first time this user has ever visited this
 * view (nothing yet stored under its key) — every filter bar's own
 * pre-persistence default keeps its meaning by passing that same value here.
 */
export function useFilterBarCollapsed(
  viewId: string,
  userKey: string,
  defaultOpen: boolean,
): [boolean, (next: boolean) => void] {
  const key = storageKey(viewId, userKey);

  const [state, setState] = useState<boolean>(() => loadPersisted(key) ?? defaultOpen);

  // `key` starts wrong on first render for any caller whose `userKey`
  // resolves asynchronously (e.g. `useCurrentUser()`/`useIdTokenClaims()` are
  // both still `undefined` on mount, so callers fall back to a shared
  // "anonymous" bucket) and then changes once the real id lands. `useState`'s
  // lazy initializer only runs once, so without this it keeps serving the
  // state it loaded under the stale key forever. Re-derive `state` from
  // `localStorage` whenever `key` actually changes, using React's "adjust
  // state during render" pattern (a second piece of state, not a ref — refs
  // must not be read/written during render) instead of an effect, so there's
  // no extra commit where the bar renders the wrong user's open/closed state
  // before catching up. See `useColumnPreferences`'s own `prevKey` handling.
  const [prevKey, setPrevKey] = useState(key);
  if (prevKey !== key) {
    setPrevKey(key);
    setState(loadPersisted(key) ?? defaultOpen);
  }

  const update = useCallback(
    (next: boolean) => {
      setState(next);
      savePersisted(key, next);
    },
    [key],
  );

  return [state, update];
}
