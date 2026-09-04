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

import { useMemo } from "react";
import { useCurrentUserOptional } from "@context/current-user/CurrentUserContext";

export const ROLE_ADMIN = "admin";
export const ROLE_AGENT = "agent";
export const ROLE_TIMECARD_APPROVER = "timecard_approver";

export interface UserPermissions {
  /** All normalized (lowercase, trimmed) roles assigned to the current user. */
  roles: string[];
  /** True when the user holds the platform `admin` role. */
  isAdmin: boolean;
  /** True when the user holds the operational `agent` role. */
  isAgent: boolean;
  /** True when the user holds the `timecard_approver` role. */
  isTimecardApprover: boolean;
  /** True while the user's profile is still loading. */
  isLoading: boolean;
  /** Check if the user possesses a specific role (case-insensitive). */
  hasRole: (role: string) => boolean;
  /** Check if the user possesses at least one of the specified roles (case-insensitive). */
  hasAnyRole: (...roles: string[]) => boolean;
}

/**
 * Access the signed-in user's roles and evaluate RBAC permissions.
 *
 * Central source of truth for UI feature gating, navigation filtering,
 * and route authorization across the CSM Portal.
 */
export function usePermissions(): UserPermissions {
  const currentUser = useCurrentUserOptional();
  const user = currentUser?.user;
  const isLoading = currentUser?.isLoading ?? false;

  return useMemo(() => {
    const rawRoles = user?.roles ?? [];
    const normalized = rawRoles
      .map((r) => r.trim().toLowerCase())
      .filter((r) => r.length > 0);

    const roleSet = new Set(normalized);

    const hasRole = (role: string): boolean => {
      if (!role) return false;
      return roleSet.has(role.trim().toLowerCase());
    };

    const hasAnyRole = (...roles: string[]): boolean => {
      return roles.some((role) => hasRole(role));
    };

    return {
      roles: normalized,
      isAdmin: hasRole(ROLE_ADMIN),
      isAgent: hasRole(ROLE_AGENT),
      isTimecardApprover: hasRole(ROLE_TIMECARD_APPROVER),
      isLoading,
      hasRole,
      hasAnyRole,
    };
  }, [user?.roles, isLoading]);
}
