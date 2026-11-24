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
  CSM_NAV_ITEMS,
  flattenNavNodes,
  navNodeById,
  navNodeMatchForPath,
  navNodePath,
  navNodeRoutes,
  navSectionForPath,
} from "@config/csmNavItems";
import { enabledNavChildren, isFeatureVisible, visibleNavSections } from "@config/featureFlags";
import { getPortalAccess } from "@context/current-user/portalAccess";

describe("nav tree invariants", () => {
  it("gives every node a unique id", () => {
    const ids = flattenNavNodes().map((node) => node.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it("prefixes every tab id with its section id", () => {
    for (const section of CSM_NAV_ITEMS) {
      for (const child of section.children ?? []) {
        expect(child.id.startsWith(`${section.id}.`)).toBe(true);
      }
    }
  });

  it("keeps a query-param tab from claiming its section's landing route", () => {
    const incidents = navNodeById("operations.incidents");
    expect(navNodeRoutes(incidents!)).toEqual(["/operations/incidents", "/operations/incident-tasks"]);
    expect(navNodePath(incidents!)).toBe("/operations");
  });
});

describe("top-level nav order", () => {
  const ids = CSM_NAV_ITEMS.map((section) => section.id);
  const below = (upper: string, lower: string): void => {
    expect(ids.indexOf(lower)).toBe(ids.indexOf(upper) + 1);
  };

  it("puts Allocations directly below Engagements, and Announcements directly below Allocations", () => {
    below("engagements", "allocations");
    below("allocations", "announcements");
  });

  it("puts Customers directly below Security Center, and Knowledge directly below Customers", () => {
    below("security-center", "customers");
    below("customers", "kb-articles");
  });

  it("puts Team Schedule directly below Time cards", () => {
    below("time-cards", "team-schedule");
  });

  it("puts Skills directly below Team Schedule", () => {
    below("team-schedule", "skills");
  });

  it("keeps the remaining sections in their existing relative order", () => {
    expect(ids).toEqual([
      "dashboard",
      "support",
      "operations",
      "engagements",
      "allocations",
      "announcements",
      "security-center",
      "customers",
      "kb-articles",
      "plg",
      "updates",
      "time-cards",
      "team-schedule",
      "skills",
      "customer-health",
      "usage-metrics",
      "user-scan",
      "admin",
      "help",
    ]);
  });
});

describe("product catalogue in Settings", () => {
  it("is not a top-level section any more", () => {
    expect(CSM_NAV_ITEMS.map((section) => section.id)).not.toContain("products");
  });

  it("is the last Settings tab, after the existing ones (ids unchanged)", () => {
    expect(navNodeById("admin")?.children?.map((tab) => tab.id)).toEqual([
      "admin.user-management",
      "admin.dashboards",
      "admin.banners",
      "admin.sla-engine",
      "admin.product-catalog",
    ]);
  });

  it("has the Products and Release lines tabs", () => {
    expect(navNodeById("admin.product-catalog")?.children?.map((tab) => [tab.id, tab.label])).toEqual([
      ["admin.product-catalog.products", "Products"],
      ["admin.product-catalog.release-lines", "Release lines"],
    ]);
  });

  it("is shown, with its help topic, to catalogue managers only", () => {
    expect(navNodeById("admin.product-catalog")?.requires).toBe("canManageProductCatalog");
    expect(navNodeById("help.products")?.requires).toBe("canManageProductCatalog");
    const sees = (roles: string[]): boolean => getPortalAccess(roles).canManageProductCatalog;
    expect(sees(["cs_engineer"])).toBe(false);
    expect(sees(["cs_engineer", "product_catalog_manager"])).toBe(true);
    expect(sees(["admin"])).toBe(true);
  });

  it("shows Settings, with the catalogue as its only tab, to a manager who is not a CS engineer", () => {
    const access = getPortalAccess(["viewer", "product_catalog_manager"]);
    expect(visibleNavSections(access).map((section) => section.id)).toContain("admin");
    const settings = navNodeById("admin");
    expect(settings && enabledNavChildren(settings, access).map((tab) => tab.id)).toEqual(["admin.product-catalog"]);
    expect(isFeatureVisible("admin.user-management", access)).toBe(false);
    expect(isFeatureVisible("admin.user-management.users", access)).toBe(false);
    expect(isFeatureVisible("admin.dashboards", access)).toBe(false);
  });

  it("keeps Settings' other tabs, and hides the catalogue, for a CS engineer who is not a manager", () => {
    const access = getPortalAccess(["cs_engineer"]);
    expect(visibleNavSections(access).map((section) => section.id)).toContain("admin");
    expect(isFeatureVisible("admin.user-management", access)).toBe(true);
    expect(isFeatureVisible("admin.product-catalog", access)).toBe(false);
  });

  it("hides Settings from staff who hold neither role", () => {
    const access = getPortalAccess(["viewer", "escalator"]);
    expect(visibleNavSections(access).map((section) => section.id)).not.toContain("admin");
  });

  it("resolves each tab's route, and the catalogue's own path to it", () => {
    expect(navNodeMatchForPath("/admin/product-catalog/release-lines")).toMatchObject({
      node: { id: "admin.product-catalog.release-lines" },
    });
    expect(navNodeMatchForPath("/admin/product-catalog")).toMatchObject({
      node: { id: "admin.product-catalog" },
    });
  });
});

describe("Updates vs Time cards gating", () => {
  it("gates Updates (nav and help) on canViewUpdates", () => {
    expect(navNodeById("updates")?.requires).toBe("canViewUpdates");
    expect(navNodeById("help.updates")?.requires).toBe("canViewUpdates");
  });

  it("keeps Time cards (nav and help) on canUseTimeCardsAndUpdates", () => {
    expect(navNodeById("time-cards")?.requires).toBe("canUseTimeCardsAndUpdates");
    expect(navNodeById("help.time-cards")?.requires).toBe("canUseTimeCardsAndUpdates");
  });

  it("shows a viewer Updates but not Time cards, and leaves other roles unchanged", () => {
    const visible = (roles: string[], id: string): boolean => {
      const key = navNodeById(id)?.requires;
      return key ? getPortalAccess(roles)[key] : true;
    };
    expect(visible(["viewer"], "updates")).toBe(true);
    expect(visible(["viewer"], "help.updates")).toBe(true);
    expect(visible(["viewer"], "time-cards")).toBe(false);
    expect(visible(["viewer"], "help.time-cards")).toBe(false);
    for (const role of ["cs_engineer", "admin", "timecard_approver"]) {
      expect(visible([role], "updates")).toBe(true);
      expect(visible([role], "time-cards")).toBe(true);
    }
    expect(visible(["escalator"], "updates")).toBe(false);
  });
});

describe("help section", () => {
  it("sits immediately after Settings in the top-level nav order", () => {
    const ids = CSM_NAV_ITEMS.map((section) => section.id);
    const adminIndex = ids.indexOf("admin");
    expect(ids[adminIndex + 1]).toBe("help");
  });

  it("gives every topic an in-page anchor href on the single /help route", () => {
    const help = navNodeById("help");
    for (const topic of help?.children ?? []) {
      expect(topic.href.startsWith("/help#")).toBe(true);
      expect(topic.id.startsWith("help.")).toBe(true);
    }
  });

  it("resolves the whole /help route (and any of its topic anchors) to the help section, not a topic", () => {
    expect(navNodeMatchForPath("/help")).toMatchObject({
      node: { id: "help" },
      prefix: "/help",
    });
    // A topic anchor's own pathname is "/help" too — it must not out-compete
    // the section for that prefix (see navNodeRoutes's anchor-href handling).
    expect(navNodeRoutes(navNodeById("help.operations")!)).toEqual([]);
  });
});

describe("navNodeMatchForPath", () => {
  it("prefers the most specific node and reports the matched prefix", () => {
    expect(navNodeMatchForPath("/operations/incidents/INC0001")).toMatchObject({
      node: { id: "operations.incidents" },
      prefix: "/operations/incidents",
    });
  });

  it("falls back to the section for its own landing route", () => {
    expect(navNodeMatchForPath("/operations")).toMatchObject({
      node: { id: "operations" },
      prefix: "/operations",
    });
  });

  it("matches a route-backed tab on its own path", () => {
    expect(navNodeMatchForPath("/customers/accounts")).toMatchObject({
      node: { id: "customers.accounts" },
      prefix: "/customers/accounts",
    });
  });

  it("does not match a path that merely shares a prefix string", () => {
    expect(navNodeMatchForPath("/operations-archive")).toBeUndefined();
  });

  it("returns undefined for an unknown route", () => {
    expect(navNodeMatchForPath("/nothing-here")).toBeUndefined();
  });

  it("prefers a child over its parent on an equal-length prefix tie (plg's href aliases plg.dashboard's)", () => {
    expect(navNodeMatchForPath("/plg/dashboard")).toMatchObject({
      node: { id: "plg.dashboard" },
      prefix: "/plg/dashboard",
    });
  });
});

describe("navSectionForPath", () => {
  it("resolves a tab's deep link up to its owning section", () => {
    expect(navSectionForPath("/operations/incidents/INC0001")?.id).toBe(
      "operations",
    );
    expect(navSectionForPath("/customers/accounts/abc")?.id).toBe("customers");
  });
});

describe("rendersOwnWipPage", () => {
  it("is set exactly on the admin tabs that route to their own placeholder", () => {
    const flagged = flattenNavNodes()
      .filter((node) => node.rendersOwnWipPage)
      .map((node) => node.id);
    expect(flagged).toEqual(["admin.user-management.permissions"]);
  });
});
