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

import { useSyncExternalStore } from "react";

/**
 * The shared per-page feature state the signed-in shell loaded from
 * `GET /features` (see `useSharedFeatureOverrides`). `rows` is the
 * saved `nav-node id -> state` map, `savedOnce` says an admin has finished a
 * first save in Settings. `featureFlags.ts` reads it instead of
 * `CSM_PORTAL_FEATURE_OVERRIDES` once `savedOnce` is true, and layers it over
 * the config until then.
 *
 * A plain module store (the same shape as `featurePreview.ts`) because
 * `featureState` and its callers are plain functions: a component that needs
 * to repaint when the map changes subscribes with {@link useFeatureOverridesVersion}.
 */
export interface SharedFeatureOverrides {
  rows: Record<string, string>;
  savedOnce: boolean;
}

let shared: SharedFeatureOverrides | undefined;
let version = 0;
const listeners = new Set<() => void>();

/** The loaded map, or `undefined` until the first response (or after a failure). */
export function getSharedFeatureOverrides(): SharedFeatureOverrides | undefined {
  return shared;
}

/** Bumps on every change, so `featureFlags.ts` knows when its memoised resolution is stale. */
export function getSharedFeatureOverridesVersion(): number {
  return version;
}

/** Stores a freshly loaded (or saved) map and tells subscribers. `undefined` clears it. */
export function setSharedFeatureOverrides(next: SharedFeatureOverrides | undefined): void {
  shared = next;
  version += 1;
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/**
 * Re-renders the caller whenever the shared map changes. Call it next to
 * anything that renders from `featureState` so the sidebar, tab chips, route
 * guards and quick nav follow a save or a refresh. The returned number has no
 * meaning of its own.
 */
export function useFeatureOverridesVersion(): number {
  return useSyncExternalStore(subscribe, getSharedFeatureOverridesVersion, getSharedFeatureOverridesVersion);
}

/** Clears the store between tests. Not for application code. */
export function resetSharedFeatureOverridesForTests(): void {
  shared = undefined;
  version += 1;
}
