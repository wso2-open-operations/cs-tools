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

import type { QueryClient } from "@tanstack/react-query";

/** Window event dispatched just before an explicit or idle sign-out starts. */
export const SIGNING_OUT_EVENT = "app:signing-out";

/**
 * Key prefixes this app owns in `localStorage`/`sessionStorage`: per-user
 * view state (recent views, case tabs, column/filter preferences, saved
 * filters), dashboard/list filters, change-request drafts, recent approvers.
 */
const APP_OWNED_KEY_PREFIXES = ["csm", "spl.", "dashboard_"] as const;

/**
 * App-owned keys that carry no user data and are deliberately kept across a
 * sign-out so the next sign-in does not flash the wrong theme.
 */
const KEPT_KEYS: ReadonlySet<string> = new Set(["csm.theme"]);

function clearStore(store: Storage): void {
  const doomed: string[] = [];
  for (let i = 0; i < store.length; i++) {
    const key = store.key(i);
    if (!key || KEPT_KEYS.has(key)) continue;
    if (APP_OWNED_KEY_PREFIXES.some((prefix) => key.startsWith(prefix))) {
      doomed.push(key);
    }
  }
  for (const key of doomed) store.removeItem(key);
}

/** Removes every app-owned key from both browser storages. Best-effort, never throws. */
export function clearAppOwnedStorage(): void {
  for (const getStore of [() => window.localStorage, () => window.sessionStorage]) {
    try {
      clearStore(getStore());
    } catch {
      /* storage unavailable: nothing persisted to clear */
    }
  }
}

/**
 * Registers the single sign-out listener: clears every app-owned storage key
 * and the whole query cache so no signed-out user's data survives in the tab
 * or in the browser profile. Returns the unregister function.
 */
export function registerSignOutCleanup(queryClient: QueryClient): () => void {
  const handler = (): void => {
    clearAppOwnedStorage();
    queryClient.clear();
  };
  window.addEventListener(SIGNING_OUT_EVENT, handler);
  return () => window.removeEventListener(SIGNING_OUT_EVENT, handler);
}
