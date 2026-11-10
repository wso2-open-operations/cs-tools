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

import { type JSX, useEffect, useState } from "react";
import { Navigate, Outlet, useLocation } from "react-router";
import PreviewPageNotice from "@components/preview-notice/PreviewPageNotice";
import { navNodeForPath } from "@config/csmNavItems";
import {
  featureStateForPath,
  firstEnabledDestination,
  isPreviewedOnly,
} from "@config/featureFlags";
import { useFeatureOverridesVersion } from "@config/featureOverridesStore";
import { useFeaturePreviewOverrides } from "@config/featurePreview";
import { usePortalAccess, useSafeCurrentUserId } from "@context/current-user/usePortalAccess";
import RouteSuspenseFallback from "@components/route-fallback/RouteSuspenseFallback";
import CsmComingSoonPage from "@features/csm-coming-soon/pages/CsmComingSoonPage";
import { useFeatureOverridesQuery } from "@hooks/useFeatureOverrides";

/**
 * Layout guard honouring the shared per-page feature state (Settings > Feature
 * visibility, with `CSM_PORTAL_FEATURE_OVERRIDES` as the fallback), so a
 * direct, pinned or shared link can't reach a page that is restricted.
 *
 * A `preview` page is open to an admin, under a short Preview note. A
 * signed-in person who is not an admin is sent away, the same as `hidden`,
 * so the page never appears in their menu. When the caller is not known yet
 * the stored preview state is left as-is and this guard shows the shared
 * "coming soon" message. The exception is a page that already renders a more
 * specific unavailable message of its own (`rendersOwnPreviewPage`), which is
 * let through rather than downgraded to the generic one.
 *
 * A `hidden` page has no nav entry at all, so there is nothing to stay
 * consistent with and the link is bounced to the first destination this
 * deployment does offer. That target is never assumed to exist: a config that
 * hides everything, or that hides the target itself, falls through to `/404`,
 * which sits outside this guard and so cannot bounce back here.
 *
 * The path resolves to the most specific nav node, so restricting a section
 * does not restrict a finished tab inside it (and vice versa).
 */
/** How long a page waits for the shared per-page state before it renders from `config.js` anyway. */
const FIRST_LOAD_WAIT_MS = 2500;

/** True once `waiting` has been true for `ms`, so a slow endpoint never holds a page back for long. */
function useGaveUpWaiting(waiting: boolean, ms: number): boolean {
  const [gaveUp, setGaveUp] = useState(false);
  useEffect(() => {
    if (!waiting) return undefined;
    const timer = window.setTimeout(() => setGaveUp(true), ms);
    return () => window.clearTimeout(timer);
  }, [waiting, ms]);
  return gaveUp;
}

export default function FeatureRouteGuard(): JSX.Element {
  const { pathname } = useLocation();
  const access = usePortalAccess();
  const node = navNodeForPath(pathname);
  // An admin's own Feature Preview toggles (browser-local, see
  // featurePreview.ts) can promote a hidden/WIP page to enabled for them
  // alone -- never for any other caller, and never touching the deployment
  // config itself.
  const { overrides } = useFeaturePreviewOverrides(useSafeCurrentUserId());
  // Repaints when the shared per-page state loads or an admin saves a change.
  useFeatureOverridesVersion();
  // On the very first load, hold the page until the saved state is known, so a
  // page an admin put in Preview never opens for a moment from the config.
  // Any failure ends the wait (the page then follows `config.js`), and so does
  // a slow answer.
  const { isPending } = useFeatureOverridesQuery(true);
  const gaveUp = useGaveUpWaiting(isPending, FIRST_LOAD_WAIT_MS);
  // Per-user: a page the user's roles don't unlock (e.g. Operations for a
  // view-only role) is hidden the same way a deployment-hidden page is.
  const state = featureStateForPath(pathname, access, overrides);

  if (isPending && !gaveUp) return <RouteSuspenseFallback />;

  if (state === "hidden") {
    const fallback = firstEnabledDestination(access, overrides);
    const samePath = fallback !== undefined && fallback.split(/[?#]/)[0] === pathname;
    return <Navigate to={!fallback || samePath ? "/404" : fallback} replace />;
  }

  if (state === "preview" && !node?.rendersOwnPreviewPage) {
    const label = node?.label ?? "This section";
    return (
      <CsmComingSoonPage
        title={label}
        description={`${label} is in preview and isn't open to everyone yet.`}
      />
    );
  }

  // An admin only reaches a preview page because of its Preview state: say so.
  if (node && isPreviewedOnly(node.id, access, overrides)) {
    return (
      <>
        <PreviewPageNotice />
        <Outlet />
      </>
    );
  }

  return <Outlet />;
}
