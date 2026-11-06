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

import { describe, expect, it } from "vitest";
import {
  hasCustomerAdminRole,
  getAvatarColor,
  getInitials,
  getRoleBadges,
  getRoleChipSx,
  getRoleLabel,
} from "@features/settings/utils/settings";

describe("settings utils", () => {
  it("derives expected role badges", () => {
    const badges = getRoleBadges({
      isCsAdmin: true,
      isCsIntegrationUser: false,
      isPortalUser: false,
      isSecurityContact: true,
      account: { classification: "Partner" },
    } as never);
    expect(badges.map((badge) => badge.label)).toEqual([
      "Admin",
      "Security User",
      "Partner",
    ]);
  });

  it("returns default portal badge when no explicit end-user role exists", () => {
    const badges = getRoleBadges({
      isCsIntegrationUser: false,
      isPortalUser: false,
      isSecurityContact: false,
    } as never);
    expect(badges[0]?.label).toBe("Portal User");
  });

  describe("Admin badge", () => {
    const base = { isCsIntegrationUser: false, isPortalUser: true, isSecurityContact: false };
    const labels = (c: object) => getRoleBadges({ ...base, ...c } as never).map((b) => b.label);

    it("shows Admin for isAccountAdmin only", () => {
      expect(labels({ isCsAdmin: false, isAccountAdmin: true })).toEqual(["Admin", "Portal User"]);
    });
    it("shows Admin for isCsAdmin only", () => {
      expect(labels({ isCsAdmin: true })).toEqual(["Admin", "Portal User"]);
    });
    it("shows a single Admin badge when both are set", () => {
      expect(labels({ isCsAdmin: true, isAccountAdmin: true }).filter((l) => l === "Admin")).toHaveLength(1);
    });
    it("shows no Admin badge when neither is set", () => {
      expect(labels({ isCsAdmin: false })).not.toContain("Admin");
    });
    it("keeps Admin first", () => {
      expect(labels({ isAccountAdmin: true, isLead: true, isSecurityContact: true })[0]).toBe("Admin");
    });
  });

  it("resolves initials from names and email fallback", () => {
    expect(getInitials("Jane", "Doe")).toBe("JD");
    expect(getInitials(undefined, undefined, "ab@example.com")).toBe("AB");
  });

  it("creates stable avatar colors and role chip sx", () => {
    expect(getAvatarColor("user-1")).toBe(getAvatarColor("user-1"));
    expect(getRoleChipSx("primary")).toMatchObject({ borderColor: expect.any(String) });
  });
});


describe("hasCustomerAdminRole", () => {
  it("accepts the ServiceNow and the CSM database spellings", () => {
    expect(hasCustomerAdminRole(["external", "sn_customerservice.customer_admin"])).toBe(true);
    expect(hasCustomerAdminRole(["external", "customer", "customer_admin"])).toBe(true);
    expect(hasCustomerAdminRole([" Customer_Admin "])).toBe(true);
  });

  it("refuses everything else", () => {
    expect(hasCustomerAdminRole(undefined)).toBe(false);
    expect(hasCustomerAdminRole([])).toBe(false);
    expect(hasCustomerAdminRole(["customer", "partner_admin", "sn_customerservice.customer"])).toBe(false);
  });
});

describe("getRoleLabel", () => {
  it("maps roles correctly across dev and staging variants", () => {
    expect(getRoleLabel(undefined)).toBe("Not Available");
    expect(getRoleLabel([])).toBe("Not Available");
    expect(getRoleLabel(["customer_admin"])).toBe("Admin");
    expect(getRoleLabel(["admin"])).toBe("Admin");
    expect(getRoleLabel(["sn_customerservice.customer_admin"])).toBe("Admin");
    expect(getRoleLabel(["sn_customerservice.admin"])).toBe("Admin");
    expect(getRoleLabel(["partner_admin"])).toBe("Partner Admin");
    expect(getRoleLabel(["sn_customerservice.partner_admin"])).toBe("Partner Admin");
    expect(getRoleLabel(["lead"])).toBe("Lead");
    expect(getRoleLabel(["security_user"])).toBe("Security User");
    expect(getRoleLabel(["partner"])).toBe("Partner");
    expect(getRoleLabel(["agent"])).toBe("Internal User");
    expect(getRoleLabel(["internal"])).toBe("Internal User");
    expect(getRoleLabel(["system_user"])).toBe("System User");
    expect(getRoleLabel(["customer"])).toBe("Portal User");
    expect(getRoleLabel(["customer_user"])).toBe("Portal User");
    expect(getRoleLabel(["unknown_role"])).toBe("Portal User");
  });
});
