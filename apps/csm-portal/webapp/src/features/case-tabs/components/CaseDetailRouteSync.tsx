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

import { Suspense, useEffect, useMemo, type JSX } from "react";
import { useLocation } from "react-router";
import { pageComponentForKind } from "@features/case-tabs/tabPageRegistry";
import RouteSuspenseFallback from "@components/route-fallback/RouteSuspenseFallback";
import { useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import { useCaseTabsBehavior } from "@context/case-tabs/CaseTabsBehaviorContext";
import { useNormalizedIdParam } from "@hooks/useNormalizedIdParam";
import type { CaseRouteKind } from "@context/case-tabs/caseTabsTypes";

/**
 * The `element` for every detail route this tab mechanism covers in
 * `App.tsx` (the five case-like ones — `/cases/:caseId`,
 * `/engagements/:caseId`, `/operations/service-requests/:caseId`,
 * `/announcements/:caseId`, `/security-center/security-reports/:caseId` —
 * plus `/operations/incidents/:id` and `/operations/change-requests/:id`),
 * replacing a direct page-component mount.
 *
 * Its job is narrow: given the REAL matched route (real `useParams`/
 * `useLocation` — this component is not itself isolated), ask
 * `CaseTabsContext` to open/activate an in-app tab for this record. Actually
 * rendering the page happens elsewhere, in `CaseTabsWorkspace`'s keep-alive
 * host (each open tab gets its own isolated identity there — see
 * `CaseTabIsolatedRouter`/`CaseRouteOverrideContext`), so on success this
 * renders nothing, leaving the routed `<Outlet/>` slot empty while the
 * workspace's own content occupies the same visual area.
 *
 * Disabled (`enabled` false) is the only case this renders the record
 * directly, un-tabbed, via the real matched route — exactly how this route
 * worked before this feature existed. There is no longer a "the tab cap is
 * full, render this one un-tabbed instead" fallback: both cap-behavior modes
 * (`CaseTabsBehaviorContext`'s `CaseTabsCapMode`) always evict an existing
 * tab to make room rather than refusing the new one, so `openTab` never
 * fails for a capacity reason while `enabled` is true.
 */
export default function CaseDetailRouteSync({
  kind,
  // The five case-like routes all use `:caseId`; Incidents/Change Requests
  // use `:id` (see their own route definitions in App.tsx and their pages'
  // own `useNormalizedIdParam("id")` call) — same mechanism, different param
  // name on the route pattern.
  paramName = "caseId",
}: {
  kind: CaseRouteKind;
  paramName?: string;
}): JSX.Element | null {
  const caseId = useNormalizedIdParam(paramName);
  const location = useLocation();
  const { openTab } = useCaseTabsController();
  const { enabled } = useCaseTabsBehavior();

  useEffect(() => {
    // Disabled: never call openTab — see this component's own doc comment.
    if (!caseId || !enabled) return;
    const path = `${location.pathname}${location.search}${location.hash}`;
    openTab(caseId, kind, path, location.state);
  }, [caseId, kind, enabled, location.pathname, location.search, location.hash, location.state, openTab]);

  // Memoized so re-renders reuse the same reference — `pageComponentForKind`
  // always returns one of a small fixed set of stable, module-level `lazy()`
  // components, never a genuinely new one; this is a component REGISTRY
  // lookup, not the "defining a component inline during render" antipattern
  // `react-hooks/static-components` exists to catch, which can't be proven
  // statically here. Called before the early returns below so it's
  // unconditional, same as every other hook in this component.
  const Page = useMemo(() => pageComponentForKind(kind), [kind]);

  if (!caseId) return null;
  // Disabled always renders this fallback (there is never a tab to defer
  // to) — this is what makes the mechanism behave exactly like the
  // pre-feature app when off: every record renders directly, in place, via
  // the real matched route. See this component's own doc comment for why
  // there's no other reason left to fall back to this.
  if (enabled) return null;
  return (
    <Suspense fallback={<RouteSuspenseFallback />}>
      {/* eslint-disable-next-line react-hooks/static-components -- registry lookup among stable, pre-declared lazy() components, not a new component per render */}
      <Page />
    </Suspense>
  );
}
