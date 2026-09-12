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

import type { CaseRouteKind } from "@context/case-tabs/caseTabsTypes";

/**
 * The route base for every kind this tab mechanism covers. The five
 * case-like ones all render `CsmCaseDetailPage` (see that page's own doc
 * comment on `detailPath`/`canonicalDetailPath`) and mirror
 * `caseTypeDetailBasePath` in `features/csm-cases/utils/caseType.ts` (which
 * maps a *loaded case's* `caseType` to the same bases; this module instead
 * maps a *URL* to one of them, before any data has loaded). `incident` and
 * `change_request` each have their own dedicated page — see
 * `tabPageRegistry.tsx` for which component renders for which kind.
 */
const ROUTE_BASES: Record<CaseRouteKind, string> = {
  case: "/cases",
  service_request: "/operations/service-requests",
  engagement: "/engagements",
  announcement: "/announcements",
  security_report_analysis: "/security-center/security-reports",
  incident: "/operations/incidents",
  change_request: "/operations/change-requests",
};

// Longest/most specific base first so `/operations/service-requests/:id`
// isn't shadowed by a hypothetical shorter prefix — not currently a risk
// with these five literal bases, but keeps this order-independent.
const ROUTE_KIND_BY_BASE: [string, CaseRouteKind][] = (
  Object.entries(ROUTE_BASES) as [CaseRouteKind, string][]
)
  .map(([kind, base]) => [base, kind] as [string, CaseRouteKind])
  .sort((a, b) => b[0].length - a[0].length);

export interface CaseLocationMatch {
  kind: CaseRouteKind;
  caseId: string;
}

export function basePathForKind(kind: CaseRouteKind): string {
  return ROUTE_BASES[kind];
}

/**
 * Matches a pathname (no search/hash) against the five known case-detail
 * route bases, returning the route kind and the raw `:caseId` segment, or
 * `undefined` if the path isn't a case-detail route at all. Pure string
 * matching, deliberately independent of the app's real `<Routes>` tree so it
 * can also be used inside an isolated (non-real) router — see
 * `CaseTabIsolatedRouter`.
 *
 * `new` is never a real id here: every one of these bases also owns a
 * sibling `<base>/new` create route in `App.tsx` (`cases/new`,
 * `engagements/new`, `operations/incidents/new`, etc.). Without this
 * exclusion, navigating to one of those create routes from inside an
 * already-open case tab matched here first — `CaseTabIsolatedRouter`'s
 * in-tab `navigate()` override treats any match as "open/activate a tab for
 * this id" and never lets the real app router see the navigation at all, so
 * the create page never mounts. Instead it opened a bogus tab for a record
 * literally named "new", which fails to load (backend rejects "new" as an
 * id) — reported as "Create incident from case" failing, but the same
 * misroute hits every kind's create action once its originating case is
 * open as a tab.
 */
export function matchCaseLocation(pathname: string): CaseLocationMatch | undefined {
  for (const [base, kind] of ROUTE_KIND_BY_BASE) {
    if (pathname === base || pathname.startsWith(`${base}/`)) {
      const rest = pathname.slice(base.length + 1);
      const caseId = rest.split("/")[0];
      if (caseId && caseId !== "new") return { kind, caseId };
    }
  }
  return undefined;
}

/** Builds the concrete detail path for a case of the given kind. */
export function pathForTab(kind: CaseRouteKind, caseId: string): string {
  return `${ROUTE_BASES[kind]}/${caseId}`;
}
