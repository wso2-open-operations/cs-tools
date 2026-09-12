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

import { useEffect } from "react";

/**
 * Name of the `window` `CustomEvent` fired once React's root component has
 * mounted for the first time. The plain inline boot script in `index.html`
 * (which cannot import this module — it runs before any bundled JS) listens
 * for the same literal string to remove its loading screen. Kept as a single
 * exported constant so the two sides can't drift silently; if this value
 * changes, `index.html`'s listener must be updated to match.
 */
export const APP_MOUNTED_EVENT = "csm:app-mounted";

/**
 * Dispatches {@link APP_MOUNTED_EVENT} once, on first render of whichever
 * component calls this. Exists as its own hook (rather than an inline
 * `useEffect` in `App`) so this specific piece of boot behaviour is testable
 * on its own, without mounting the app's full provider/router tree.
 */
export function useAppMountedSignal(): void {
  useEffect(() => {
    window.dispatchEvent(new CustomEvent(APP_MOUNTED_EVENT));
    // Intentionally fires only once, on mount — this marks "React has taken
    // over rendering", not a state that should re-fire on updates.
  }, []);
}
