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
  | "super_admin"
  | "admin"
  | "agent"
  | "customer_admin"
  | "customer_user"
  | "partner_admin"
  | "partner_user"
  | "stakeholder"
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
  | "security_admin";

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
    case "sn_customerservice.super_admin":
    case "super_admin":
      return "super_admin";
    case "sn_customerservice.admin":
    case "admin":
      return "admin";
    case "wso2_agent":
    case "snc_internal":
    case "agent":
      return "agent";
    case "sn_customerservice.customer_admin":
    case "customer_admin":
      return "customer_admin";
    case "sn_customerservice.customer":
    case "customer":
    case "customer_user":
      return "customer_user";
    case "sn_customerservice.partner_admin":
    case "partner_admin":
      return "partner_admin";
    case "sn_customerservice.partner":
    case "partner":
    case "partner_user":
      return "partner_user";
    case "sn_customerservice.stakeholder":
    case "stakeholder":
      return "stakeholder";
    case "internal":
      return "internal";
    default:
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
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    read: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
      "stakeholder",
    ],
    update: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    delete: ["super_admin", "admin"],
  },
  time_cards: {
    create: ["super_admin", "admin", "agent"],
    read: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    update: ["super_admin", "admin", "agent"],
    delete: ["super_admin", "admin"],
  },
  projects: {
    create: ["super_admin", "admin"],
    read: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
      "stakeholder",
    ],
    update: ["super_admin", "admin", "agent"],
    delete: ["super_admin", "admin"],
  },
  change_requests: {
    create: ["super_admin", "admin", "agent"],
    read: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    update: ["super_admin", "admin", "agent"],
    delete: ["super_admin", "admin"],
  },
  deployments: {
    create: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    read: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
      "stakeholder",
    ],
    update: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    delete: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
  },
  deployment_products: {
    create: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    read: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
      "stakeholder",
    ],
    update: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    delete: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
  },
  deployment_resources: {
    create: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    read: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
      "stakeholder",
    ],
    update: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
    delete: [
      "super_admin",
      "admin",
      "agent",
      "customer_admin",
      "customer_user",
      "partner_admin",
      "partner_user",
    ],
  },
  security_admin: {
    create: ["super_admin"],
    read: ["super_admin"],
    update: ["super_admin"],
    delete: ["super_admin"],
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
  if (roles.includes("super_admin")) return true;
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
    const isSuperAdmin = roles.includes("super_admin");
    const isAdmin = isSuperAdmin || roles.includes("admin");
    const isAgent = roles.includes("agent");
    const isCustomerAdmin = roles.includes("customer_admin");
    const isCustomerUser = roles.includes("customer_user");
    const isPartnerAdmin = roles.includes("partner_admin");
    const isPartnerUser = roles.includes("partner_user");
    const isStakeholder = roles.includes("stakeholder");
    const isExternalUser =
      isCustomerAdmin || isCustomerUser || isPartnerAdmin || isPartnerUser;

    const can = (module: CustomerModule, action: CustomerAction): boolean =>
      hasCustomerPermission(roles, module, action);

    const hasRole = (role: CanonicalRole): boolean => roles.includes(role);
    const hasAnyRole = (checkRoles: CanonicalRole[]): boolean =>
      isSuperAdmin || checkRoles.some((r) => roles.includes(r));

    return {
      roles,
      isLoading,
      isError,
      isSuperAdmin,
      isAdmin,
      isAgent,
      isCustomerAdmin,
      isCustomerUser,
      isPartnerAdmin,
      isPartnerUser,
      isStakeholder,
      isExternalUser,
      can,
      hasRole,
      hasAnyRole,
      canCreateCase: can("cases", "create"),
      canUpdateCase: can("cases", "update"),
      canDeleteCase: can("cases", "delete"),
      canAccessTimeCards: can("time_cards", "read"),
      canAccessChangeRequests: can("change_requests", "read"),
      canAccessSecurityAdmin: can("security_admin", "read"),
      canManageContacts: hasAnyRole([
        "super_admin",
        "admin",
        "customer_admin",
        "partner_admin",
      ]),
    };
  }, [rawRoles, isLoading, isError]);
}

export default useCustomerPermissions;
