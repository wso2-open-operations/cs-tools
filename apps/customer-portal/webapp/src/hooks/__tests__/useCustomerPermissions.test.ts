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

import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import {
  useCustomerPermissions,
  normalizeCustomerRoles,
  hasCustomerPermission,
  type CanonicalRole,
  type CustomerModule,
  type CustomerAction,
} from "../useCustomerPermissions";

vi.mock("@features/settings/api/useGetUserDetails");

const ALL_MODULES: CustomerModule[] = [
  "cases",
  "time_cards",
  "projects",
  "change_requests",
  "deployments",
  "deployment_products",
  "deployment_resources",
  "security_admin",
];

const ALL_ACTIONS: CustomerAction[] = ["create", "read", "update", "delete"];

describe("useCustomerPermissions & Permission Matrix", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("Role Normalization", () => {
    it("normalizes ServiceNow role strings to canonical names", () => {
      const roles = normalizeCustomerRoles([
        "sn_customerservice.super_admin",
        "sn_customerservice.admin",
        "wso2_agent",
        "sn_customerservice.customer_admin",
        "sn_customerservice.customer",
        "sn_customerservice.partner_admin",
        "sn_customerservice.partner",
        "sn_customerservice.stakeholder",
      ]);

      expect(roles).toEqual([
        "super_admin",
        "admin",
        "agent",
        "customer_admin",
        "customer_user",
        "partner_admin",
        "partner_user",
        "stakeholder",
      ]);
    });

    it("handles empty and duplicate roles", () => {
      expect(normalizeCustomerRoles([])).toEqual([]);
      expect(normalizeCustomerRoles(undefined)).toEqual([]);
      expect(
        normalizeCustomerRoles(["admin", "sn_customerservice.admin"]),
      ).toEqual(["admin"]);
    });
  });

  describe("Permission Matrix Rules", () => {
    it("Super Admin has full CRUD across all 8 modules", () => {
      const roles: CanonicalRole[] = ["super_admin"];
      for (const mod of ALL_MODULES) {
        for (const act of ALL_ACTIONS) {
          expect(hasCustomerPermission(roles, mod, act)).toBe(true);
        }
      }
    });

    it("Admin has full CRUD on 7 modules and zero access to Security Admin", () => {
      const roles: CanonicalRole[] = ["admin"];
      for (const mod of ALL_MODULES) {
        if (mod === "security_admin") {
          for (const act of ALL_ACTIONS) {
            expect(hasCustomerPermission(roles, mod, act)).toBe(false);
          }
        } else {
          for (const act of ALL_ACTIONS) {
            expect(hasCustomerPermission(roles, mod, act)).toBe(true);
          }
        }
      }
    });

    it("Agent has CRU on Cases/Timecards/CRs, RU on Projects, CRUD on Deployments, zero on Security Admin", () => {
      const roles: CanonicalRole[] = ["agent"];

      // Cases: CRU, no D
      expect(hasCustomerPermission(roles, "cases", "create")).toBe(true);
      expect(hasCustomerPermission(roles, "cases", "read")).toBe(true);
      expect(hasCustomerPermission(roles, "cases", "update")).toBe(true);
      expect(hasCustomerPermission(roles, "cases", "delete")).toBe(false);

      // Time Cards: CRU, no D
      expect(hasCustomerPermission(roles, "time_cards", "create")).toBe(true);
      expect(hasCustomerPermission(roles, "time_cards", "read")).toBe(true);
      expect(hasCustomerPermission(roles, "time_cards", "update")).toBe(true);
      expect(hasCustomerPermission(roles, "time_cards", "delete")).toBe(false);

      // Projects: RU, no C, no D
      expect(hasCustomerPermission(roles, "projects", "create")).toBe(false);
      expect(hasCustomerPermission(roles, "projects", "read")).toBe(true);
      expect(hasCustomerPermission(roles, "projects", "update")).toBe(true);
      expect(hasCustomerPermission(roles, "projects", "delete")).toBe(false);

      // Change Requests: CRU, no D
      expect(hasCustomerPermission(roles, "change_requests", "create")).toBe(
        true,
      );
      expect(hasCustomerPermission(roles, "change_requests", "read")).toBe(true);
      expect(hasCustomerPermission(roles, "change_requests", "update")).toBe(
        true,
      );
      expect(hasCustomerPermission(roles, "change_requests", "delete")).toBe(
        false,
      );

      // Deployments, Products, Resources: CRUD
      for (const mod of [
        "deployments",
        "deployment_products",
        "deployment_resources",
      ] as const) {
        for (const act of ALL_ACTIONS) {
          expect(hasCustomerPermission(roles, mod, act)).toBe(true);
        }
      }

      // Security Admin: Zero access
      for (const act of ALL_ACTIONS) {
        expect(hasCustomerPermission(roles, "security_admin", act)).toBe(false);
      }
    });

    it("External roles have CRU on Cases, Read on Timecards/Projects/CRs, CRUD on Deployments, zero on Security Admin", () => {
      const externalRoles: CanonicalRole[][] = [
        ["customer_user"],
        ["customer_admin"],
        ["partner_user"],
        ["partner_admin"],
      ];

      for (const roles of externalRoles) {
        // Cases: CRU, no D
        expect(hasCustomerPermission(roles, "cases", "create")).toBe(true);
        expect(hasCustomerPermission(roles, "cases", "read")).toBe(true);
        expect(hasCustomerPermission(roles, "cases", "update")).toBe(true);
        expect(hasCustomerPermission(roles, "cases", "delete")).toBe(false);

        // Timecards: Read Only
        expect(hasCustomerPermission(roles, "time_cards", "read")).toBe(true);
        expect(hasCustomerPermission(roles, "time_cards", "create")).toBe(false);
        expect(hasCustomerPermission(roles, "time_cards", "update")).toBe(false);
        expect(hasCustomerPermission(roles, "time_cards", "delete")).toBe(false);

        // Projects: Read Only
        expect(hasCustomerPermission(roles, "projects", "read")).toBe(true);
        expect(hasCustomerPermission(roles, "projects", "create")).toBe(false);
        expect(hasCustomerPermission(roles, "projects", "update")).toBe(false);
        expect(hasCustomerPermission(roles, "projects", "delete")).toBe(false);

        // Change Requests: Read Only
        expect(hasCustomerPermission(roles, "change_requests", "read")).toBe(
          true,
        );
        expect(hasCustomerPermission(roles, "change_requests", "create")).toBe(
          false,
        );
        expect(hasCustomerPermission(roles, "change_requests", "update")).toBe(
          false,
        );
        expect(hasCustomerPermission(roles, "change_requests", "delete")).toBe(
          false,
        );

        // Deployments, Products, Resources: CRUD
        for (const mod of [
          "deployments",
          "deployment_products",
          "deployment_resources",
        ] as const) {
          for (const act of ALL_ACTIONS) {
            expect(hasCustomerPermission(roles, mod, act)).toBe(true);
          }
        }

        // Security Admin: Zero access
        for (const act of ALL_ACTIONS) {
          expect(hasCustomerPermission(roles, "security_admin", act)).toBe(
            false,
          );
        }
      }
    });

    it("Stakeholder has strictly Read on Cases/Projects/Deployments/Products/Resources, zero on others", () => {
      const roles: CanonicalRole[] = ["stakeholder"];

      // Read only on Cases, Projects, Deployments, Products, Resources
      for (const mod of [
        "cases",
        "projects",
        "deployments",
        "deployment_products",
        "deployment_resources",
      ] as const) {
        expect(hasCustomerPermission(roles, mod, "read")).toBe(true);
        expect(hasCustomerPermission(roles, mod, "create")).toBe(false);
        expect(hasCustomerPermission(roles, mod, "update")).toBe(false);
        expect(hasCustomerPermission(roles, mod, "delete")).toBe(false);
      }

      // Zero access on Timecards, Change Requests, Security Admin
      for (const mod of [
        "time_cards",
        "change_requests",
        "security_admin",
      ] as const) {
        for (const act of ALL_ACTIONS) {
          expect(hasCustomerPermission(roles, mod, act)).toBe(false);
        }
      }
    });
  });

  describe("Hook Functionality", () => {
    it("returns correct flags and getters for SuperAdmin", () => {
      vi.mocked(useGetUserDetails).mockReturnValue({
        data: {
          id: "usr-1",
          email: "super@wso2.com",
          firstName: "Super",
          lastName: "Admin",
          timeZone: "UTC",
          roles: ["sn_customerservice.super_admin"],
        },
        isLoading: false,
        isError: false,
      } as ReturnType<typeof useGetUserDetails>);

      const { result } = renderHook(() => useCustomerPermissions());

      expect(result.current.isSuperAdmin).toBe(true);
      expect(result.current.isAdmin).toBe(true);
      expect(result.current.isAgent).toBe(false);
      expect(result.current.isStakeholder).toBe(false);
      expect(result.current.canAccessSecurityAdmin).toBe(true);
      expect(result.current.canCreateCase).toBe(true);
      expect(result.current.canDeleteCase).toBe(true);
      expect(result.current.canManageContacts).toBe(true);
    });

    it("returns correct flags and getters for Stakeholder", () => {
      vi.mocked(useGetUserDetails).mockReturnValue({
        data: {
          id: "usr-2",
          email: "stakeholder@example.com",
          firstName: "Stake",
          lastName: "Holder",
          timeZone: "UTC",
          roles: ["sn_customerservice.stakeholder"],
        },
        isLoading: false,
        isError: false,
      } as ReturnType<typeof useGetUserDetails>);

      const { result } = renderHook(() => useCustomerPermissions());

      expect(result.current.isStakeholder).toBe(true);
      expect(result.current.isAdmin).toBe(false);
      expect(result.current.canAccessSecurityAdmin).toBe(false);
      expect(result.current.canAccessTimeCards).toBe(false);
      expect(result.current.canAccessChangeRequests).toBe(false);
      expect(result.current.canCreateCase).toBe(false);
      expect(result.current.canDeleteCase).toBe(false);
      expect(result.current.canManageContacts).toBe(false);
    });
  });
});
