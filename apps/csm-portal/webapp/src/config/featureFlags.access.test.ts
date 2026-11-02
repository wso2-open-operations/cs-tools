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
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { navNodeById, navNodeForPath, navNodeRoutes } from "@config/csmNavItems";
import {
  enabledNavChildren,
  featureState,
  featureStateForPath,
  firstEnabledDestination,
  listPreviewableFeatures,
  navigableNavNodes,
  resetFeatureStatesForTests,
  visibleNavChildren,
  visibleNavSections,
  visibleRailChildren,
  visibleShortcutTo,
} from "@config/featureFlags";
import { getPortalAccess } from "@context/current-user/portalAccess";

const viewer = getPortalAccess(["viewer"]);
const csEngineer = getPortalAccess(["cs_engineer"]);

describe("feature visibility by portal access", () => {
  beforeEach(() => {
    resetFeatureStatesForTests();
  });

  it("shows Operations to a CS engineer and to callers with no access filter", () => {
    expect(visibleNavSections().map((s) => s.id)).toContain("operations");
    expect(visibleNavSections(csEngineer).map((s) => s.id)).toContain("operations");
  });

  // A plain `viewer` role set is exactly {viewer}, so this also exercises
  // canViewStaffSections's one false case (see that flag's own doc comment)
  // -- Knowledge and Settings are hidden here alongside PLG (which a
  // view-only role has never had access to). Team Schedule is hidden too,
  // via the separate canViewTeamSchedule allow-list (cs_engineer/admin/
  // comment_updater), which a plain viewer holds none of. Usage Metrics is
  // also hidden: canViewUsageMetrics needs full access or
  // usage_metrics_viewer, which a plain viewer holds neither of (reported
  // live). Customer Health is not a
  // top-level section for a plain viewer: it moves under the Customers
  // dropdown (showCustomerHealthUnderCustomers), see the describe block below.
  //
  // Asserted as an explicit allowlist rather than
  // `visibleNavSections().length - N`: that form silently drifted stale
  // (by 1, then by 3 more) every time an unrelated section was added to the
  // nav tree without this test being updated alongside it -- exactly the
  // kind of break a plain content comparison can't hide.
  it("hides Operations, Security Center, PLG, Updates, Time cards, Team Schedule, Knowledge, Usage Metrics and Settings from a view-only role, and keeps the other sections", () => {
    const ids = visibleNavSections(viewer).map((s) => s.id);
    for (const hidden of [
      "operations",
      "security-center",
      "plg",
      "updates",
      "time-cards",
      "team-schedule",
      "kb-articles",
      "usage-metrics",
      "admin",
    ]) {
      expect(ids).not.toContain(hidden);
    }
    expect(ids).toEqual([
      "dashboard",
      "support",
      "engagements",
      "allocations",
      "announcements",
      "customers",
      "help",
    ]);
  });

  // Customer Health is for the viewer role AND for cs_engineer/admin
  // (canViewCustomerHealth); Usage Metrics for cs_engineer/admin and
  // usage_metrics_viewer. Both sit together at the end of the sidebar, above
  // Settings and Help.
  it("shows Customer Health and Usage Metrics to a CS engineer, at the end of the sidebar above Settings and Help", () => {
    expect(visibleNavSections(csEngineer).map((s) => s.id).slice(-4)).toEqual([
      "customer-health",
      "usage-metrics",
      "admin",
      "help",
    ]);
    expect(featureStateForPath("/customer-health", csEngineer)).toBe("enabled");
    expect(featureStateForPath("/customer-health/account/acct-1", csEngineer)).toBe("enabled");
    expect(featureStateForPath("/usage-metrics", csEngineer)).toBe("enabled");
  });

  it("puts Customer Health and Usage Metrics above Settings and Help for a CS engineer who also holds viewer", () => {
    const both = getPortalAccess(["cs_engineer", "viewer"]);
    expect(visibleNavSections(both).map((s) => s.id).slice(-4)).toEqual([
      "customer-health",
      "usage-metrics",
      "admin",
      "help",
    ]);
  });

  it("opens Customer Health to a CS engineer without the viewer role", () => {
    // Roles that hold neither viewer nor full access still do not see it.
    for (const role of ["escalator", "attachment_downloader", "usage_metrics_viewer", "timecard_approver", "dashboard_designer", "security_team"]) {
      const access = getPortalAccess([role]);
      expect(featureStateForPath("/customer-health", access)).toBe("hidden");
    }
  });

  it("hides every Operations tab and route along with the section", () => {
    const operations = navNodeById("operations");
    expect(operations).toBeDefined();
    expect(visibleNavChildren(operations!, viewer)).toEqual([]);
    expect(featureState("operations.incidents", viewer)).toBe("hidden");
    expect(featureStateForPath("/operations/incidents", viewer)).toBe("hidden");
    expect(featureStateForPath("/operations/incidents/INC0001", viewer)).toBe("hidden");
    expect(featureStateForPath("/operations/incidents", csEngineer)).toBe("enabled");
  });

  it("hides every Security Center tab and route along with the section, for every role but cs_engineer/admin", () => {
    const securityCenter = navNodeById("security-center");
    expect(securityCenter).toBeDefined();
    for (const role of ["viewer", "escalator", "attachment_downloader", "usage_metrics_viewer", "timecard_approver", "dashboard_designer"]) {
      const access = getPortalAccess([role]);
      expect(visibleNavChildren(securityCenter!, access)).toEqual([]);
      expect(featureState("security-center.reports", access)).toBe("hidden");
      expect(featureState("security-center.vulnerabilities", access)).toBe("hidden");
      expect(featureStateForPath("/security-center", access)).toBe("hidden");
      expect(featureStateForPath("/security-center/reports", access)).toBe("hidden");
      expect(featureStateForPath("/security-center/vulnerabilities", access)).toBe("hidden");
    }
    for (const role of ["cs_engineer", "admin"]) {
      const access = getPortalAccess([role]);
      expect(featureStateForPath("/security-center", access)).toBe("enabled");
      expect(featureStateForPath("/security-center/vulnerabilities", access)).toBe("enabled");
    }
  });

  // canViewSettings is a dedicated, narrower allow-list than canViewStaffSections
  // (which also gates Settings' nav entry but is shared with Knowledge): a caller
  // holding viewer plus one unrelated role is not caught by canViewStaffSections'
  // exactly-{viewer} check, so it used to still reach Settings -- reported as too
  // wide for a section that holds user/role/group/team administration.
  it("hides Settings for every role but cs_engineer/admin, even combined with an unrelated role", () => {
    const admin = navNodeById("admin");
    expect(admin).toBeDefined();
    for (const roles of [
      ["viewer"],
      ["escalator"],
      ["attachment_downloader"],
      ["usage_metrics_viewer"],
      ["timecard_approver"],
      ["dashboard_designer"],
      ["viewer", "attachment_downloader"],
    ]) {
      const access = getPortalAccess(roles);
      expect(visibleNavChildren(admin!, access)).toEqual([]);
      expect(featureStateForPath("/admin", access)).toBe("hidden");
      expect(featureStateForPath("/admin/user-management/users", access)).toBe("hidden");
    }
    for (const role of ["cs_engineer", "admin"]) {
      const access = getPortalAccess([role]);
      expect(featureStateForPath("/admin", access)).toBe("enabled");
      expect(featureStateForPath("/admin/user-management/users", access)).toBe("enabled");
    }
  });

  it("drops Operations from quick-nav for a viewer and redirects somewhere else", () => {
    expect(navigableNavNodes(viewer).some((n) => n.id.startsWith("operations"))).toBe(false);
    expect(navigableNavNodes(csEngineer).some((n) => n.id.startsWith("operations"))).toBe(true);
    const fallback = firstEnabledDestination(viewer);
    expect(fallback).toBeDefined();
    expect(fallback).not.toMatch(/^\/operations/);
  });

  it("every role that is not full access is treated as a viewer for Operations", () => {
    for (const role of ["escalator", "attachment_downloader", "usage_metrics_viewer"]) {
      expect(featureState("operations", getPortalAccess([role]))).toBe("hidden");
    }
    expect(featureState("operations", getPortalAccess(["admin"]))).toBe("enabled");
  });

  it("hides Updates and Time cards from roles without time-card access, and shows them otherwise", () => {
    const ids = (a: ReturnType<typeof getPortalAccess>) => visibleNavSections(a).map((s) => s.id);
    for (const role of ["viewer", "escalator", "attachment_downloader", "usage_metrics_viewer", "dashboard_designer"]) {
      const access = getPortalAccess([role]);
      expect(ids(access)).not.toContain("updates");
      expect(ids(access)).not.toContain("time-cards");
      expect(featureStateForPath("/time-cards", access)).toBe("hidden");
      expect(featureStateForPath("/updates", access)).toBe("hidden");
    }
    for (const role of ["timecard_approver", "cs_engineer", "admin"]) {
      const access = getPortalAccess([role]);
      expect(ids(access)).toContain("updates");
      expect(ids(access)).toContain("time-cards");
      expect(featureStateForPath("/time-cards", access)).toBe("enabled");
    }
  });

  it("a time-card approver sees Time cards and Updates but not Operations", () => {
    const access = getPortalAccess(["timecard_approver"]);
    const ids = visibleNavSections(access).map((s) => s.id);
    expect(ids).toContain("time-cards");
    expect(ids).not.toContain("operations");
  });

  it("Help topics follow the page they document", () => {
    const help = navNodeById("help");
    expect(help).toBeDefined();
    const topics = (a?: ReturnType<typeof getPortalAccess>) =>
      enabledNavChildren(help!, a).map((t) => t.id);

    expect(topics()).toEqual(
      expect.arrayContaining(["help.operations", "help.security-center", "help.updates", "help.time-cards"]),
    );
    const viewerTopics = topics(getPortalAccess(["viewer"]));
    for (const id of ["help.operations", "help.security-center", "help.updates", "help.time-cards", "help.settings"]) {
      expect(viewerTopics).not.toContain(id);
    }
    expect(viewerTopics).toContain("help.overview");
    expect(viewerTopics).toContain("help.support");

    const approverTopics = topics(getPortalAccess(["timecard_approver"]));
    expect(approverTopics).toEqual(expect.arrayContaining(["help.updates", "help.time-cards"]));
    expect(approverTopics).not.toContain("help.operations");
    expect(approverTopics).not.toContain("help.security-center");
    expect(topics(getPortalAccess(["cs_engineer"]))).toEqual(
      expect.arrayContaining([
        "help.operations",
        "help.security-center",
        "help.updates",
        "help.time-cards",
        "help.settings",
      ]),
    );
  });
});

// A viewer (who is not also a CS engineer/admin) finds Customer Health under
// the Customers dropdown instead of as its own top-level entry; the page and
// its route still belong to the top-level "customer-health" node for everyone.
describe("Customer Health under Customers for a viewer", () => {
  const customers = navNodeById("customers")!;
  const ids = (nodes: { id: string }[]): string[] => nodes.map((n) => n.id);

  function setOverrides(value: unknown): void {
    window.config = {
      ...window.config,
      CSM_PORTAL_FEATURE_OVERRIDES: value,
    } as Window["config"];
    resetFeatureStatesForTests();
  }

  beforeEach(() => setOverrides(undefined));
  afterEach(() => setOverrides(undefined));

  it("puts Customer Health in the viewer's Customers dropdown and drops the top-level entry", () => {
    expect(ids(visibleRailChildren(customers, viewer))).toEqual([
      "customers.accounts",
      "customers.projects",
      "customers.customer-health",
    ]);
    expect(ids(visibleNavSections(viewer))).not.toContain("customer-health");
    expect(visibleShortcutTo("customer-health", viewer)?.id).toBe("customers.customer-health");
  });

  it("keeps the Customers page's own tab strip to Accounts and Projects", () => {
    expect(ids(visibleNavChildren(customers, viewer))).toEqual(["customers.accounts", "customers.projects"]);
    expect(ids(enabledNavChildren(customers, viewer))).toEqual(["customers.accounts", "customers.projects"]);
    // The strip is built without a role filter (useRouteTabs); the shortcut
    // must still never be one of its tabs.
    expect(ids(visibleNavChildren(customers))).toEqual(["customers.accounts", "customers.projects"]);
  });

  it("still lets a viewer open the page and its account detail, owned by the top-level node", () => {
    expect(navNodeRoutes(navNodeById("customers.customer-health")!)).toEqual([]);
    expect(navNodeForPath("/customer-health")?.id).toBe("customer-health");
    expect(navNodeForPath("/customer-health/account/acct-1")?.id).toBe("customer-health");
    expect(featureStateForPath("/customer-health", viewer)).toBe("enabled");
    expect(featureStateForPath("/customer-health/account/acct-1", viewer)).toBe("enabled");
  });

  it("offers Customer Health once in Quick-nav for a viewer, at the dashboard's own route", () => {
    const entries = navigableNavNodes(viewer).filter((n) => n.label === "Customer Health");
    expect(entries).toHaveLength(1);
    expect(entries[0]).toMatchObject({ id: "customer-health", href: "/customer-health" });
  });

  it("leaves CS engineers and admins on the top-level entry, even when they also hold viewer", () => {
    for (const roles of [["cs_engineer"], ["admin"], ["cs_engineer", "viewer"], ["admin", "viewer"]]) {
      const access = getPortalAccess(roles);
      expect(ids(visibleRailChildren(customers, access))).toEqual(["customers.accounts", "customers.projects"]);
      expect(ids(visibleNavSections(access))).toContain("customer-health");
      expect(visibleShortcutTo("customer-health", access)).toBeUndefined();
      expect(featureState("customers.customer-health", access)).toBe("hidden");
    }
  });

  it("shows neither entry to a role that cannot see Customer Health", () => {
    for (const role of ["escalator", "usage_metrics_viewer", "timecard_approver", "security_team"]) {
      const access = getPortalAccess([role]);
      expect(featureState("customers.customer-health", access)).toBe("hidden");
      expect(ids(visibleNavSections(access))).not.toContain("customer-health");
    }
  });

  it("hides the shortcut too when a deployment hides Customer Health, and the top-level entry stays hidden", () => {
    setOverrides({ "customer-health": "hidden" });
    expect(featureState("customers.customer-health", viewer)).toBe("hidden");
    expect(ids(visibleRailChildren(customers, viewer))).toEqual(["customers.accounts", "customers.projects"]);
    expect(ids(visibleNavSections(viewer))).not.toContain("customer-health");
  });

  it("marks the shortcut WIP when a deployment marks Customer Health WIP", () => {
    setOverrides({ "customer-health": "wip" });
    expect(featureState("customers.customer-health", viewer)).toBe("wip");
  });

  it("hides only the shortcut when a deployment hides it, bringing back the top-level entry", () => {
    setOverrides({ "customers.customer-health": "hidden" });
    expect(visibleShortcutTo("customer-health", viewer)).toBeUndefined();
    expect(ids(visibleNavSections(viewer))).toContain("customer-health");
  });

  // The shipped config.js.example marks Customers WIP. A WIP section renders
  // as one disabled entry with no dropdown, so the viewer must keep the
  // working top-level Customer Health entry instead of losing it.
  it("keeps a viewer's top-level entry when a deployment marks Customers WIP", () => {
    setOverrides({ customers: "wip" });
    expect(featureState("customers", viewer)).toBe("wip");
    expect(visibleShortcutTo("customer-health", viewer)).toBeUndefined();
    expect(ids(visibleNavSections(viewer))).toContain("customer-health");
    expect(featureState("customer-health", viewer)).toBe("enabled");
  });

  it("keeps a viewer's top-level entry when a deployment hides Customers", () => {
    setOverrides({ customers: "hidden" });
    expect(visibleShortcutTo("customer-health", viewer)).toBeUndefined();
    expect(ids(visibleNavSections(viewer))).toContain("customer-health");
  });

  it("offers a working shortcut when a WIP Customers is promoted by an enabled tab", () => {
    setOverrides({ customers: "wip", "customers.accounts": "enabled" });
    expect(featureState("customers", viewer)).toBe("enabled");
    // Not the section's inherited WIP: the shortcut is not one of its tabs.
    expect(featureState("customers.customer-health", viewer)).toBe("enabled");
    expect(visibleShortcutTo("customer-health", viewer)?.id).toBe("customers.customer-health");
    expect(ids(visibleRailChildren(customers, viewer))).toContain("customers.customer-health");
    expect(ids(visibleNavSections(viewer))).not.toContain("customer-health");
  });

  it("does not let the shortcut promote a WIP Customers section for anyone", () => {
    setOverrides({ customers: "wip", "customers.customer-health": "enabled" });
    expect(featureState("customers")).toBe("wip");
    expect(featureState("customers", getPortalAccess(["cs_engineer"]))).toBe("wip");
  });

  // An admin's Feature Preview toggles (featurePreview.ts) only ever promote
  // for that admin; the shortcut is for viewers, so it has no toggle of its
  // own and a viewer never benefits from an admin's preview.
  it("leaves the shortcut out of the Feature Preview list, even when the deployment names it", () => {
    const admin = getPortalAccess(["admin"]);
    setOverrides({ "customers.customer-health": "hidden" });
    expect(ids(listPreviewableFeatures(admin))).not.toContain("customers.customer-health");
    expect(ids(listPreviewableFeatures())).not.toContain("customers.customer-health");
    setOverrides({ "customer-health": "hidden", "customers.customer-health": "wip" });
    expect(ids(listPreviewableFeatures(admin))).toContain("customer-health");
    expect(ids(listPreviewableFeatures(admin))).not.toContain("customers.customer-health");
  });

  it("lets an admin preview a hidden Customer Health without showing it to viewers", () => {
    setOverrides({ "customer-health": "hidden" });
    const preview = { "customer-health": true };
    expect(ids(visibleNavSections(getPortalAccess(["admin"]), preview))).toContain("customer-health");
    expect(ids(visibleNavSections(viewer, preview))).not.toContain("customer-health");
    expect(visibleShortcutTo("customer-health", viewer, preview)).toBeUndefined();
    expect(ids(visibleRailChildren(customers, viewer, preview))).toEqual(["customers.accounts", "customers.projects"]);
  });

  it("never offers the shortcut to an admin, whatever they preview", () => {
    const admin = getPortalAccess(["admin"]);
    setOverrides({ customers: "wip", "customer-health": "hidden", "customers.customer-health": "hidden" });
    const preview = { customers: true, "customer-health": true, "customers.customer-health": true };
    expect(featureState("customers.customer-health", admin, preview)).toBe("hidden");
    expect(visibleShortcutTo("customer-health", admin, preview)).toBeUndefined();
    expect(ids(visibleRailChildren(customers, admin, preview))).not.toContain("customers.customer-health");
    expect(ids(visibleNavSections(admin, preview))).toContain("customer-health");
  });

  // Main's Feature Preview promotes a restricted section when one of its tabs
  // is previewed. The shortcut is not a tab, so a stored value for it (it has
  // no toggle, so only a stale or hand-edited one) must not surface Customers.
  it("does not let a preview value for the shortcut promote its section", () => {
    const admin = getPortalAccess(["admin"]);
    setOverrides({ customers: "hidden" });
    expect(featureState("customers", admin, { "customers.customer-health": true })).toBe("hidden");
    expect(ids(visibleNavSections(admin, { "customers.customer-health": true }))).not.toContain("customers");
    // A real tab still promotes it.
    expect(featureState("customers", admin, { "customers.accounts": true })).toBe("enabled");
    setOverrides({ customers: "wip" });
    expect(featureState("customers", admin, { "customers.customer-health": true })).toBe("wip");
  });

  it("does not change anything a viewer sees, whatever preview values are passed for them", () => {
    const preview = {
      customers: true,
      "customers.accounts": true,
      "customer-health": true,
      "customers.customer-health": true,
    };
    for (const overrides of [
      undefined,
      { customers: "wip" },
      { customers: "hidden" },
      { "customer-health": "hidden" },
      { "customer-health": "wip" },
      { "customers.customer-health": "hidden" },
    ]) {
      setOverrides(overrides);
      expect(ids(visibleNavSections(viewer, preview))).toEqual(ids(visibleNavSections(viewer)));
      expect(ids(visibleRailChildren(customers, viewer, preview))).toEqual(ids(visibleRailChildren(customers, viewer)));
      expect(visibleShortcutTo("customer-health", viewer, preview)?.id).toBe(
        visibleShortcutTo("customer-health", viewer)?.id,
      );
      expect(featureState("customers.customer-health", viewer, preview)).toBe(
        featureState("customers.customer-health", viewer),
      );
    }
  });

  // The narrowing in featureState: the owner's preview reaches the shortcut,
  // the shortcut's own preview value does not. Only an access object that is
  // both an admin and placed under Customers can show it, which no real role
  // set is today (the placement flag excludes admins) -- this pins the
  // threading in case the two ever overlap.
  it("lets the owner's preview reach the shortcut, but not the shortcut's own preview value", () => {
    const adminUnderCustomers = { ...viewer, isAdmin: true };
    setOverrides({ "customer-health": "hidden" });
    expect(featureState("customers.customer-health", adminUnderCustomers)).toBe("hidden");
    expect(featureState("customers.customer-health", adminUnderCustomers, { "customer-health": true })).toBe(
      "enabled",
    );
    expect(visibleShortcutTo("customer-health", adminUnderCustomers, { "customer-health": true })?.id).toBe(
      "customers.customer-health",
    );
    expect(ids(visibleRailChildren(customers, adminUnderCustomers, { "customer-health": true }))).toContain(
      "customers.customer-health",
    );
    expect(ids(visibleNavSections(adminUnderCustomers, { "customer-health": true }))).not.toContain(
      "customer-health",
    );

    setOverrides({ "customers.customer-health": "hidden" });
    expect(
      featureState("customers.customer-health", adminUnderCustomers, { "customers.customer-health": true }),
    ).toBe("hidden");
    expect(
      visibleShortcutTo("customer-health", adminUnderCustomers, { "customers.customer-health": true }),
    ).toBeUndefined();

    // The section's own preview decides whether it opens as a dropdown at all.
    setOverrides({ customers: "wip" });
    expect(visibleShortcutTo("customer-health", adminUnderCustomers)).toBeUndefined();
    expect(visibleShortcutTo("customer-health", adminUnderCustomers, { customers: true })?.id).toBe(
      "customers.customer-health",
    );
  });

  it("hides the shortcut from a role that cannot reach the Customer Health page itself", () => {
    // Today the placement flag implies the page flag; this pins that the
    // shortcut follows the owner even if the two ever drift apart.
    const access = { ...viewer, canViewCustomerHealth: false };
    expect(featureState("customers.customer-health", access)).toBe("hidden");
    expect(visibleShortcutTo("customer-health", access)).toBeUndefined();
  });
});
