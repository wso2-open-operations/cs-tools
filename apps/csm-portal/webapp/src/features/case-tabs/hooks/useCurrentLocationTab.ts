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

import { useState } from "react";
import { useLocation } from "react-router";
import { navNodeForPath } from "@config/csmNavItems";
import { matchCaseLocation } from "@context/case-tabs/caseRoutePatterns";

export interface CurrentLocationTab {
  /** A short label for whatever non-case page the user is (or was last) on
   * — the owning nav section's label (e.g. "Dashboard", "Support",
   * "Operations"), or "Home" for a path outside the nav tree entirely. */
  label: string;
  /** The full path (pathname + search + hash) to navigate back to on click
   * — the hash matters for anchor-linked pages (e.g. `/help#topic`); losing
   * it on the way back through a case tab and back would silently drop the
   * user at the top of the page instead of the anchor they were at. */
  path: string;
  /** Whether the user is CURRENTLY on this page — false while a case tab is
   * the active view, in which case this still reflects the last non-case
   * page they were on, just not the live one. */
  active: boolean;
}

/**
 * Tracks "wherever the user currently is" outside of the case-tab mechanism
 * — the dashboard, a list view, admin, anything that isn't one of the
 * case/incident/change-request detail routes the tab strip itself manages.
 * Backs the pinned, non-closable first tab in `CaseTabStrip` (see
 * `CaseTabStripBar`).
 *
 * Deliberately does NOT reset when the user opens/switches case tabs: it
 * only updates on a navigation to a genuinely non-case route, so switching
 * between case tabs (or between a case tab and this pinned one) never loses
 * track of "the last non-case page" — that's the whole point of the pin.
 */
export function useCurrentLocationTab(): CurrentLocationTab {
  const location = useLocation();
  const isCaseRoute = matchCaseLocation(location.pathname) !== undefined;

  const [lastNonCaseLocation, setLastNonCaseLocation] = useState(() =>
    isCaseRoute
      ? { pathname: "/dashboard", search: "", hash: "" }
      : { pathname: location.pathname, search: location.search, hash: location.hash },
  );

  // Render-time state adjustment (not an effect — see React's own docs on
  // this pattern, and `RejectCallDialog`'s identical use of it elsewhere in
  // this codebase): updates synchronously within this render whenever the
  // current (non-case) location differs from what's stored, so there's no
  // extra render pass where the pinned tab's label/target is one navigation
  // behind.
  if (
    !isCaseRoute &&
    (lastNonCaseLocation.pathname !== location.pathname ||
      lastNonCaseLocation.search !== location.search ||
      lastNonCaseLocation.hash !== location.hash)
  ) {
    setLastNonCaseLocation({
      pathname: location.pathname,
      search: location.search,
      hash: location.hash,
    });
  }

  const label = navNodeForPath(lastNonCaseLocation.pathname)?.label ?? "Home";

  return {
    label,
    path: `${lastNonCaseLocation.pathname}${lastNonCaseLocation.search}${lastNonCaseLocation.hash}`,
    active: !isCaseRoute,
  };
}
