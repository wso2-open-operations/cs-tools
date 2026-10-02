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
import { PermissionProvider } from "@features/sales-sa/api/PermissionProvider";
import { useAccess } from "@features/sales-sa/api/useAccess";

/**
 * Route guard for every `/spl/*` route: hiding the nav section (see
 * CsmSideBar.tsx) stops a click, not a direct/bookmarked URL — this is the
 * actual enforcement for reaching the section at all. Same
 * "hold render open while resolving, fail closed on error/no-access"
 * shape as DashboardBuilderRouteGuard, this app's other client-side route
 * guard — see useAccess.ts for the underlying `roles` check.
 *
 * Also mounts PermissionProvider so every SPL screen below can call
 * usePermissions() for the four fine-grained action gates
 * (worknote/escalation/attachment/usage-metrics), independent of this
 * audience gate.
 */
export default function RouteGuard(): JSX.Element {
  const access = useAccess();

  if (!access.ready) {
    return <Skeleton variant="rounded" height={200} />;
  }

  if (!access.hasAccess) {
    return <Error403Page message="This section is only available to Sales / Solutions Architecture staff." />;
  }

  return (
    <PermissionProvider>
      <Outlet />
    </PermissionProvider>
  );
}
