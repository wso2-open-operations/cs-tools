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

/** The record types this tab mechanism covers: the five case-like types that
 * share `CsmCaseDetailPage`, plus Incidents and Change Requests (their own
 * dedicated pages, `CsmIncidentDetailPage`/`CsmChangeRequestDetailPage`) —
 * see `tabPageRegistry.tsx` for which page each kind renders. The last two
 * values match the `RecentView` `kind` vocabulary already used by
 * `features/csm-recent` (`recordView({ kind: "incident" | "change_request",
 * ... })` in each page), not a coincidence — same records, same name. */
export type CaseRouteKind =
  | "case"
  | "service_request"
  | "engagement"
  | "announcement"
  | "security_report_analysis"
  | "incident"
  | "change_request";

export interface CaseTabState {
  /** Stable synthetic id for this open tab, assigned once at open time.
   * Deliberately NOT the same value as `caseId`: an in-tab navigation to a
   * different case (see `CaseTabIsolatedRouter`) always opens/activates a
   * *different* tab rather than mutating this one in place, so `id` never
   * needs to change for the lifetime of a tab — which keeps the tab a stable
   * React list key and avoids remounting `CsmCaseDetailPage` on every path
   * change inside it (the whole point of keeping tabs alive). */
  id: string;
  caseId: string;
  kind: CaseRouteKind;
  /** Current concrete path for this tab, e.g. "/cases/CS0001". Updated in
   * place (not via a tab-identity change) by in-tab navigation that resolves
   * to the SAME caseId — the misrouted-case redirect inside
   * `CsmCaseDetailPage`, or the dashless-id repair in `useNormalizedIdParam`. */
  path: string;
  /** Short display label (the record's number only, e.g. "CS0001"); undefined
   * until its own data has loaded once — see `useReportCaseTabMeta`. */
  label?: string;
  /** The internal/project-scoped id (`wso2CaseId` for cases, or the
   * equivalent field on incidents/change requests) and the subject, shown in
   * the tab chip's hover tooltip (`CaseTabStrip`) — a fuller identity than
   * the short `label`. Both undefined until the record's own data has
   * loaded once, same as `label`. */
  internalId?: string;
  subject?: string;
  /** Best-effort signal that this case's reply composer is open, used only
   * to decide whether closing this tab needs a confirm — see
   * `CaseTabsContext`'s `reportDraftState`. */
  hasDraft: boolean;
  /** Router `location.state` most recently associated with this tab — carries
   * the originating list's filtered URL (`{ from: string }`) so the page's
   * own Back button returns to it. Captured at open time, and refreshed on a
   * later OUTSIDE (not in-tab) navigation that reactivates this same
   * already-open tab — e.g. a bookmark or a related-case link to the same
   * case (see `caseTabsReducer`'s `OPEN_OR_ACTIVATE` case). NOT updated by
   * in-tab navigation (`CaseTabIsolatedRouter`'s own `navigate()` calls,
   * which dispatch `UPDATE_TAB_PATH` instead and leave this alone) — not
   * persisted to sessionStorage either way, so a reload falls back to the
   * page's own hardcoded backPath instead — see `CaseTabsContext`'s
   * persistence code. */
  state?: unknown;
}

export interface CaseTabsPersistedState {
  /** Deliberately just `caseId` + `kind` — not the full `CaseTabState`. The
   * internal `id` is a synthetic per-open-instance value (see `CaseTabState.id`
   * above) with no meaning across a reload, and `path` is always
   * reconstructible from `caseId` + `kind` (see `pathForTab`) — persisting
   * either would only be extra bytes that can drift from the value rehydrate
   * actually needs. See `CaseTabsContext`'s `readPersistedState`/
   * `writePersistedState`. */
  tabs: { caseId: string; kind: CaseRouteKind }[];
  /** Which tab was active, identified by `caseId` — not the old tab's `id`,
   * which no longer exists once rehydrate assigns every tab a fresh one. */
  activeCaseId: string | null;
}

/** Hard cap on simultaneously open in-app case tabs (browser-tab-strip
 * parity target — see `CaseTabsCapMode` for what happens when a new one
 * opens past it). Raised from the original 5 to 10 by explicit request once
 * the feature had settled. */
export const MAX_OPEN_CASE_TABS = 10;
