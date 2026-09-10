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

import { type JSX, type ReactNode } from "react";
import { Outlet } from "react-router";
import { usePermissions } from "@hooks/usePermissions";
import RouteSuspenseFallback from "@components/route-fallback/RouteSuspenseFallback";
import Error403Page from "@components/error/Error403Page";

export interface RoleGuardProps {
  /**
   * The list of roles permitted to access the protected route.
   * If the user holds at least one of these roles, access is granted.
   */
  allowedRoles: string[];
  /**
   * Optional custom child components. When omitted, renders React Router's `<Outlet />`.
   */
  children?: ReactNode;
}

/**
 * Route guard that verifies the authenticated user possesses at least one of
 * the specified `allowedRoles`.
 *
 * If roles are still loading, renders `RouteSuspenseFallback`.
 * If unauthorized, renders `Error403Page`.
 * If authorized, renders child routes via `<Outlet />` (or provided children).
 */
export default function RoleGuard({
  allowedRoles,
  children,
}: RoleGuardProps): JSX.Element {
  const { isLoading, hasAnyRole } = usePermissions();

  if (isLoading) {
    return <RouteSuspenseFallback />;
  }

  if (!hasAnyRole(...allowedRoles)) {
    return <Error403Page message="You do not have permission to access this page." />;
  }

  return children ? <>{children}</> : <Outlet />;
}
