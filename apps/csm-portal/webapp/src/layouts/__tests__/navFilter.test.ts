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
  filterNavItemsByRoles,
  isNavNodeAuthorized,
} from "@layouts/navFilter";
import type { CsmNavSection } from "@config/csmNavItems";

const dummyIcon = () => null;

const sampleNavItems: CsmNavSection[] = [
  {
    id: "dashboard",
    label: "Dashboard",
    href: "/dashboard",
    icon: dummyIcon,
  },
  {
    id: "cases",
    label: "Cases",
    href: "/cases",
    icon: dummyIcon,
  },
  {
    id: "admin",
    label: "Settings",
    href: "/admin",
    icon: dummyIcon,
    roles: ["admin"],
    children: [
      {
        id: "admin.users",
        label: "Users",
        href: "/admin/users",
      },
    ],
  },
];

describe("navFilter", () => {
  describe("isNavNodeAuthorized", () => {
    it("allows access when node has no roles requirement", () => {
      expect(isNavNodeAuthorized(sampleNavItems[0], [])).toBe(true);
      expect(isNavNodeAuthorized(sampleNavItems[0], ["agent"])).toBe(true);
    });

    it("denies access when node requires roles and user has none", () => {
      expect(isNavNodeAuthorized(sampleNavItems[2], [])).toBe(false);
      expect(isNavNodeAuthorized(sampleNavItems[2], ["agent"])).toBe(false);
    });

    it("allows access when user has matching role (case-insensitive)", () => {
      expect(isNavNodeAuthorized(sampleNavItems[2], ["admin"])).toBe(true);
      expect(isNavNodeAuthorized(sampleNavItems[2], ["ADMIN"])).toBe(true);
      expect(isNavNodeAuthorized(sampleNavItems[2], ["agent", "admin"])).toBe(true);
    });
  });

  describe("filterNavItemsByRoles", () => {
    it("omits admin section for an agent user", () => {
      const filtered = filterNavItemsByRoles(sampleNavItems, ["agent"]);
      expect(filtered.map((item) => item.id)).toEqual(["dashboard", "cases"]);
    });

    it("includes admin section for an admin user", () => {
      const filtered = filterNavItemsByRoles(sampleNavItems, ["admin"]);
      expect(filtered.map((item) => item.id)).toEqual(["dashboard", "cases", "admin"]);
      expect(filtered[2].children).toHaveLength(1);
    });

    it("handles empty user roles gracefully", () => {
      const filtered = filterNavItemsByRoles(sampleNavItems, []);
      expect(filtered.map((item) => item.id)).toEqual(["dashboard", "cases"]);
    });
  });
});
