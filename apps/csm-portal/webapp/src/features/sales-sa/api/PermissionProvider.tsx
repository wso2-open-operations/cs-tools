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

import { useMemo, type ReactNode } from "react";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { PORTAL_ROLE } from "@context/current-user/portalAccess";
import { PermissionContext, type Permissions } from "./permissionsContext";

// Ported from apps/support-portal-lite/webapp's own PermissionProvider,
// now reading the same backend-`roles` array (`GET /users/me`) as every
// other permission check in this app instead of Asgardeo groups — see
// useAccess.ts for why the earlier client-side-groups exception was
// removed. Three independent booleans, each a role-membership check — UI
// guidance only: the Go backend enforces its own copies of these same
// checks server-side (internal/handler/auth.go's requireViewerPermission).
export function PermissionProvider({ children }: { children: ReactNode }) {
  let roles: string[] | undefined;
  try {
    // eslint-disable-next-line react-hooks/rules-of-hooks
    roles = useCurrentUser().user?.roles;
  } catch {
    roles = undefined;
  }

  const value = useMemo<Permissions>(() => {
    const held = new Set(roles ?? []);
    const full = held.has(PORTAL_ROLE.csEngineer) || held.has(PORTAL_ROLE.admin);
    return {
      // No backend permission grants this — the /spl/cases/:id/work-notes
      // route it once gated was removed from the Go backend before this
      // migration, so this stays permanently false rather than being wired
      // to a role that doesn't correspond to anything server-side.
      canAddWorkNotes: false,
      canAddEscalations: full || held.has(PORTAL_ROLE.escalator),
      canDownloadAttachments: full || held.has(PORTAL_ROLE.attachmentDownloader),
      canViewUsageMetrics: full || held.has(PORTAL_ROLE.usageMetricsViewer),
    };
  }, [roles]);

  return <PermissionContext.Provider value={value}>{children}</PermissionContext.Provider>;
}
