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
import useGetUserDetails from "@features/settings/api/useGetUserDetails";

/**
 * Canonical customer portal roles.
 */
export type CanonicalRole =
  | "admin"
  | "agent"
  | "customer_admin"
  | "customer_user"
  | "partner_admin"
  | "partner_user"
  | "internal";

/**
 * Functional modules across the Customer Portal.
 */
export type CustomerModule =
  | "cases"
  | "time_cards"
  | "projects"
  | "change_requests"
  | "deployments"
  | "deployment_products"
  | "deployment_resources"
;

/**
 * CRUD actions applicable to portal modules.
 */
export type CustomerAction = "create" | "read" | "update" | "delete";

/**
 * Maps raw role strings (e.g. from ServiceNow or Asgardeo) into canonical roles.
 */
export function normalizeCustomerRole(roleStr: string): CanonicalRole {
  const trimmed = roleStr.trim();
  switch (trimmed) {
    case "sn_customerservice.admin":
    case "admin":
      return "admin";
    case "wso2_agent":
    case "agent":
      return "agent";
    case "sn_customerservice.customer_admin":
    case "customer_admin":
      return "customer_admin";
    // snc_external is ServiceNow's customer-side marker, so it resolves to the
    // customer persona rather than granting nothing.
    case "sn_customerservice.customer":
    case "customer":
    case "customer_user":
    case "snc_external":
    case "external":
      return "customer_user";
    case "sn_customerservice.partner_admin":
    case "partner_admin":
      return "partner_admin";
    case "sn_customerservice.partner":
    case "partner":
    case "partner_user":
      return "partner_user";
    // snc_internal is the ServiceNow wire form of internal; both resolve to the
    // same persona so access no longer depends on which data source
    // entity-service runs. "internal" carries the grants snc_internal already
    // had via agent.
    case "snc_internal":
    case "internal":
      return "internal";
    default:
      // Deliberate passthrough: an unrecognised role keeps its raw name, which
      // matches no entry in CUSTOMER_PERMISSION_MATRIX and therefore grants
      // nothing. Failing closed is the point, but note the cast means adding a
      // new CanonicalRole to the union does NOT make the compiler flag a
      // missing case here, so a new role must be added to this switch by hand.
      return trimmed as CanonicalRole;
  }
}

/**
 * Normalizes an array of role strings, discarding empty values and deduplicating.
 */
export function normalizeCustomerRoles(rawRoles?: string[]): CanonicalRole[] {
  if (!rawRoles || rawRoles.length === 0) return [];
  const set = new Set<CanonicalRole>();
  for (const r of rawRoles) {
    if (r) set.add(normalizeCustomerRole(r));
  }
  return Array.from(set);
}

/**
 * Strict permission matrix implementing the 8-module x 5-role specification.
 */
export const CUSTOMER_PERMISSION_MATRIX: Record<
  CustomerModule,
  Record<CustomerAction, readonly CanonicalRole[]>
> = {
  cases: {
    create: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    read: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    update: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    delete: ["admin"],
  },
  time_cards: {
    create: ["admin", "agent", "internal"],
    read: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    update: ["admin", "agent", "internal"],
    delete: ["admin"],
  },
  projects: {
    create: ["admin"],
    read: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    update: ["admin", "agent", "internal"],
    delete: ["admin"],
  },
  change_requests: {
    create: ["admin", "agent", "internal"],
    read: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    update: ["admin", "agent", "internal"],
    delete: ["admin"],
  },
  deployments: {
    create: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    read: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    update: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    delete: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
  },
  deployment_products: {
    create: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    read: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    update: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    delete: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
  },
  deployment_resources: {
    create: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    read: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    update: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    delete: [
      "admin",
      "agent",
      "internal",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
  },
};

/**
 * Checks whether any of the user roles grant permission on the module and action.
 */
export function hasCustomerPermission(
  roles: CanonicalRole[],
  module: CustomerModule,
  action: CustomerAction,
): boolean {
  // No blanket-grant role: every decision comes from the matrix, mirroring
  // middleware.HasPermission on the backend.
  const allowed = CUSTOMER_PERMISSION_MATRIX[module]?.[action];
  if (!allowed) return false;
  return roles.some((r) => allowed.includes(r));
}

/**
 * Hook providing normalized roles, capability checkers, and persona flags.
 */
export function useCustomerPermissions() {
  const { data: userDetails, isLoading, isError } = useGetUserDetails();
  const rawRoles = userDetails?.roles;

  return useMemo(() => {
    const roles = normalizeCustomerRoles(rawRoles);
    const isAdmin = roles.includes("admin");
    const isAgent = roles.includes("agent");
    const isCustomerAdmin = roles.includes("customer_admin");
    const isCustomerUser = roles.includes("customer_user");
    const isPartnerAdmin = roles.includes("partner_admin");
    const isPartnerUser = roles.includes("partner_user");
    const isExternalUser =
      isCustomerAdmin || isCustomerUser || isPartnerAdmin || isPartnerUser;

    const can = (module: CustomerModule, action: CustomerAction): boolean =>
      hasCustomerPermission(roles, module, action);

    const hasRole = (role: CanonicalRole): boolean => roles.includes(role);
    const hasAnyRole = (checkRoles: CanonicalRole[]): boolean =>
      checkRoles.some((r) => roles.includes(r));

    return {
      roles,
      isLoading,
      isError,
      isAdmin,
      isAgent,
      isCustomerAdmin,
      isCustomerUser,
      isPartnerAdmin,
      isPartnerUser,
      isExternalUser,
      can,
      hasRole,
      hasAnyRole,
      canCreateCase: can("cases", "create"),
      canUpdateCase: can("cases", "update"),
      canDeleteCase: can("cases", "delete"),
      canAccessTimeCards: can("time_cards", "read"),
      canAccessChangeRequests: can("change_requests", "read"),
      canManageContacts: hasAnyRole([
        "admin",
        "customer_admin",
        "partner_admin",
      ]),
    };
  }, [rawRoles, isLoading, isError]);
}

export default useCustomerPermissions;
