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

/** How often the hook compares the clock against the reload deadline. */
export const PAGE_RELOAD_CHECK_INTERVAL_MS = 30_000;

/**
 * Fully reloads the page once `intervalMs` has passed since this hook
 * mounted (i.e. since the page loaded) — the reload then starts the next
 * interval from scratch.
 *
 * Polls the clock every {@link PAGE_RELOAD_CHECK_INTERVAL_MS} rather than
 * arming one long `setTimeout`: a wallboard machine can sleep and a browser
 * can throttle background timers, either of which makes an hour-long
 * timeout fire late or not at all. Comparing `Date.now()` against a fixed
 * deadline reloads at the first check after it — within 30s normally, or
 * right after the machine wakes.
 */
export function usePeriodicPageReload(
  intervalMs: number,
  reload: () => void = () => window.location.reload(),
): void {
  useEffect(() => {
    const deadline = Date.now() + intervalMs;
    const id = window.setInterval(() => {
      if (Date.now() >= deadline) {
        window.clearInterval(id);
        reload();
      }
    }, PAGE_RELOAD_CHECK_INTERVAL_MS);
    return () => window.clearInterval(id);
    // Armed once per page load. `reload` is a default arrow (a new function
    // every render) at the real call site — re-arming on its identity would
    // keep pushing the deadline back and the page would never reload.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [intervalMs]);
}
