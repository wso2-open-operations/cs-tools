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

import { devBypassAccessCheck, devViewOverride } from "@config/devFlags";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";

/**
 * The two independent views this app renders behind one shared top bar:
 * "cs-abt" is CSM Portal's own nav (Dashboard, Support, Operations, ...) for
 * CS/ABT engineers; "sales-sa" is the ported Support Portal Lite nav (Cases,
 * Accounts, Projects, Team schedule, User scan, Customer health, Usage
 * metrics) for Sales/Solutions-Architecture staff. Unlike the earlier merge
 * (where SPL was just one more item inside the CS nav), the two are now
 * mutually exclusive: a user sees one full nav or the other, never both.
 */
export type PortalView = "cs-abt" | "sales-sa";

/**
 * Which view the signed-in user should see. Drives CsmSideBar's entire left
 * nav and RootLanding's default destination — see both for where this is
 * consumed.
 *
 * Detection reads a portal role off `GET /users/me` (the same
 * server-authoritative `roles` array `usePortalAccess` reads the other 8
 * portal roles from — see `internal/handler/access.go`'s `AccessConfig`),
 * not a client-side IdP-groups decode — so this can never disagree
 * with what the backend itself thinks the caller is.
 *
 * The routing rule: "cs_engineer" is CS/ABT staff's own portal-selector
 * role and always wins when present; "viewer" is Sales/SA staff's, and
 * only decides the view when cs_engineer is absent. This precedence is the
 * point, not a stopgap — CS engineers are expected to also hold Viewer
 * (it's the baseline read role composed into most staff role sets), so a
 * plain "does the caller hold viewer" check would misroute them into the
 * SPL nav the moment that happens. Checking cs_engineer first is what keeps
 * CS/ABT staff landing on "cs-abt" by default regardless of which other
 * read-capability roles (viewer, escalator, attachment-downloader,
 * usage-metrics-viewer, ...) they also carry — those are orthogonal
 * capability grants, not portal selectors, and apply the same way inside
 * whichever portal a user lands in (see `access.go`'s `PermEscalate`,
 * `PermDownloadAttachment`, `PermUsageMetricsViewer`, none of which are
 * gated by cs_engineer or viewer specifically).
 *
 * This governs the *default landing nav only* — it is not a hard audience
 * block. See `useAccess.ts` and `internal/handler/access.go`'s
 * `PermSPLAccess` for the actual SPL audience gate: it grants access on
 * "viewer" alone, unconditionally, with no cs_engineer exclusion, so a CS
 * engineer who also holds viewer still lands on "cs-abt" here but isn't
 * blocked from an SPL screen reached directly. There's no in-app nav
 * control yet for a CS engineer to deliberately switch into the SPL view —
 * worth adding once SPL itself is further along; Sales/SA staff getting a
 * working SPL is the current priority.
 *
 * `devViewOverride` (authConfig.ts) lets local testing force either view
 * regardless of the signed-in account's real roles — set
 * `CSM_PORTAL_DEV_VIEW_OVERRIDE` in a local config.js. Under
 * `devBypassAccessCheck` alone (no explicit override), defaults to "cs-abt".
 */
export function usePortalView(): PortalView {
  let roles: string[] | undefined;
  try {
    // useCurrentUser always runs its useContext before it can throw, so the
    // hook order is identical on every render — same pattern as
    // usePortalAccess, which this mirrors; the lint rule can't see that.
    // eslint-disable-next-line react-hooks/rules-of-hooks
    roles = useCurrentUser().user?.roles;
  } catch {
    roles = undefined;
  }
  if (devViewOverride) return devViewOverride;
  if (devBypassAccessCheck) return "cs-abt";
  if (roles?.includes("cs_engineer")) return "cs-abt";
  return roles?.includes("viewer") ? "sales-sa" : "cs-abt";
}
