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
 * The platform's own admin role key, as returned in `GET /users/me`'s
 * `roles` (see `UsersMeResponse.roles`) — the same catalogue `GET /roles`
 * offers (see `CsmRolesPage`). Matched case-insensitively, same as
 * `useTimecardRole` does for its own group keys.
 */
const ADMIN_ROLE_KEY = "admin";

/**
 * A narrower, dashboard-builder-only role key. The backend appends this to
 * `GET /users/me`'s `roles` for individual users on a configurable email
 * allowlist (`DASHBOARD_DESIGNER_EMAILS`), alongside whatever other roles
 * (possibly none) they already hold — this lets specific non-admin users
 * design dashboards without granting them the full `admin` role. Matched
 * case-insensitively, same as `ADMIN_ROLE_KEY`.
 */
const DASHBOARD_DESIGNER_ROLE_KEY = "dashboard_designer";

/**
 * True when the given `GET /users/me` roles grant dashboard-builder access:
 * either the full platform `admin` role, or the narrower `dashboard_designer`
 * role (backend-allowlisted per email for dashboard-only access). Frontend
 * -only gate for the dashboard builder (see `csmNavItems.ts`'s own comment
 * on `admin.dashboards`): unlike every other `/admin` tab, this one has no
 * privileged backend action to fall back on for enforcement — everything
 * the builder does is local to the browser (`localStorage` only), so hiding
 * it here IS the whole gate. Every other `/admin` page deliberately does
 * NOT do this (see App.tsx), so don't reuse this helper to gate anything
 * else without re-checking that reasoning.
 */
export function hasDashboardBuilderAccess(roles: string[] | undefined): boolean {
  return (roles ?? []).some((r) => {
    const lower = r.toLowerCase();
    return lower === ADMIN_ROLE_KEY || lower === DASHBOARD_DESIGNER_ROLE_KEY;
  });
}
