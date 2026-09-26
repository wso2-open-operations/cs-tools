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
import { Navigate } from "react-router";
import { usePortalAccess } from "@context/current-user/usePortalAccess";

interface RequireWriteAccessProps {
  /** Where a caller without canWrite is sent instead — the list page this
   * create route's own nav/toolbar button already lives on and is already
   * hidden for them, so a direct URL visit lands back on familiar ground
   * rather than a bare 404. */
  to: string;
  children: ReactNode;
}

/**
 * Route-level guard for a create/edit page whose only normal entry point
 * (a "Create …" button) is already hidden for a caller without `canWrite`
 * (CS engineer/admin only — see `usePortalAccess`). A direct URL visit
 * would otherwise still reach a live form: the backend's `PermWrite` gate
 * would reject the submit with a 403, but the caller could fill out the
 * whole form first with no indication anything was wrong. Used at the
 * `<Route>` level in `App.tsx` rather than inside each page component so
 * every create route gets the same treatment from one place.
 */
export default function RequireWriteAccess({ to, children }: RequireWriteAccessProps): JSX.Element {
  const { canWrite } = usePortalAccess();
  if (!canWrite) {
    return <Navigate to={to} replace />;
  }
  return <>{children}</>;
}
