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

/**
 * The DOM `id`s a case tab's own chip (`CaseTabStrip`) and its own rendered
 * panel (`CaseTabIsolatedRouter`) point at each other with —
 * `aria-controls`/`aria-labelledby` — completing the standard ARIA
 * `tablist`/`tab`/`tabpanel` wiring those roles promise. Shared here (rather
 * than defined in either file directly) so both stay in sync without either
 * importing a non-component value out of the other — `CaseTabStrip.tsx` is a
 * component-only export (fast-refresh requires this, same reasoning as
 * `useCaseTabCloseConfirm`'s own split-out-of-`CaseTabStrip` doc comment).
 */

/** `id` of a case tab's own chip in the strip. */
export function tabElementId(tabId: string): string {
  return `case-tab-${tabId}`;
}

/** `id` of a case tab's own rendered panel (`CaseTabIsolatedRouter`) — also
 * used as that element's `data-testid`, so existing tests select by the
 * same identifier this wiring relies on rather than a second, parallel one. */
export function tabPanelElementId(tabId: string): string {
  return `case-tab-panel-${tabId}`;
}
