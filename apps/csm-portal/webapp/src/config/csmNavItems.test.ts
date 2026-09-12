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
    expect(navNodeRoutes(incidents!)).toEqual(["/operations/incidents"]);
    expect(navNodePath(incidents!)).toBe("/operations");
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
