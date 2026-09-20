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

import { type JSX } from "react";
import { Outlet } from "react-router";
import { Skeleton } from "@wso2/oxygen-ui";
import Error403Page from "@components/error/Error403Page";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { hasDashboardBuilderAccess } from "@features/csm-admin/dashboards/utils/dashboardBuilderAccess";

/**
 * Route guard for every `/admin/dashboards*` route: hiding the nav tab (see
 * `CsmAdminLayout`) stops a click, not a direct/bookmarked URL — this is
 * the actual enforcement. Still frontend-only (see
 * `dashboardBuilderAccess.ts`): there is no backend endpoint behind this
 * feature to fall back on, everything it does is local `localStorage`.
 */
export default function DashboardBuilderRouteGuard(): JSX.Element {
  const { user, isLoading, isError } = useCurrentUser();

  // Hold the render open (rather than flash a 403 then swap to the real
  // page) while the profile — the only place the admin role lives — is
  // still in flight. A failed profile fetch (isError) must not hang this
  // forever, so it falls through to the (denying) check below.
  if (isLoading && !isError) {
    return <Skeleton variant="rounded" height={200} />;
  }

  if (!hasDashboardBuilderAccess(user?.roles)) {
    return (
      <Error403Page message="You need the admin role, or dashboard_designer access, to open the dashboard builder." />
    );
  }

  return <Outlet />;
}
