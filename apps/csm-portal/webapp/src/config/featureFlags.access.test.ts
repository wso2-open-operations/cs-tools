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
import { beforeEach, describe, expect, it } from "vitest";
import { navNodeById } from "@config/csmNavItems";
import {
  enabledNavChildren,
  featureState,
  featureStateForPath,
  firstEnabledDestination,
  navigableNavNodes,
  resetFeatureStatesForTests,
  visibleNavChildren,
  visibleNavSections,
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

  it("hides Operations, Security Center, Updates and Time cards from a view-only role, and keeps the other sections", () => {
    const ids = visibleNavSections(viewer).map((s) => s.id);
    expect(ids).not.toContain("operations");
    expect(ids).not.toContain("security-center");
    expect(ids).not.toContain("updates");
    expect(ids).not.toContain("time-cards");
    expect(ids.length).toBe(visibleNavSections().length - 4);
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
    for (const id of ["help.operations", "help.security-center", "help.updates", "help.time-cards"]) {
      expect(viewerTopics).not.toContain(id);
    }
    expect(viewerTopics).toContain("help.overview");
    expect(viewerTopics).toContain("help.support");

    const approverTopics = topics(getPortalAccess(["timecard_approver"]));
    expect(approverTopics).toEqual(expect.arrayContaining(["help.updates", "help.time-cards"]));
    expect(approverTopics).not.toContain("help.operations");
    expect(approverTopics).not.toContain("help.security-center");
    expect(topics(getPortalAccess(["cs_engineer"]))).toEqual(
      expect.arrayContaining(["help.operations", "help.security-center", "help.updates", "help.time-cards"]),
    );
  });
});
