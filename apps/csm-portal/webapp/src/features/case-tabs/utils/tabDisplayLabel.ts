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

import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";

// Shown while a tab's own page hasn't reported a label yet (see
// `useReportCaseTabMeta`) — deliberately not the raw caseId/UUID, which reads
// as a rendering glitch rather than "still loading". Shared by every place
// that renders a tab's display label (`CaseTabStrip`'s chips,
// `useCaseTabCloseConfirm`'s dialog text) — a previous version of this
// module had its own copy of this fallback per call site, and one of them
// (the close-confirm dialog) fell back to the raw `caseId` instead, showing
// a UUID before its data resolved.
export const LOADING_LABEL = "Loading…";

/** Short display label for a tab: its own resolved `label` (a record
 * number, e.g. "CS0001"), or `LOADING_LABEL` while that hasn't resolved yet. */
export function tabDisplayLabel(tab: CaseTabState): string {
  return tab.label ?? LOADING_LABEL;
}
