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
import type { FeatureNodeInfo } from "@config/featureFlags";
import {
  allFeatureNodes,
  effectiveFeatureStates,
  filterFeatureTree,
} from "@features/csm-admin/feature-visibility/featureTree";

function node(
  id: string,
  label: string,
  depth: number,
  parentId: string | undefined,
  childIds: string[] = [],
): FeatureNodeInfo {
  return { id, label, depth, parentId, childIds };
}

// A small tree: one section with three pages (one of them has a page of its
// own), and a section with no pages.
const NODES: FeatureNodeInfo[] = [
  node("ops", "Operations", 0, undefined, ["ops.sr", "ops.inc", "ops.prob"]),
  node("ops.sr", "Service requests", 1, "ops"),
  node("ops.inc", "Incidents", 1, "ops"),
  node("ops.prob", "Problems", 1, "ops", ["ops.prob.deep"]),
  node("ops.prob.deep", "Problem tasks", 2, "ops.prob"),
  node("support", "Support", 0, undefined),
];

describe("effectiveFeatureStates", () => {
  it("is Available for a page with no entry anywhere", () => {
    const states = effectiveFeatureStates(NODES, {});
    expect([...states.values()].every((state) => state === "enabled")).toBe(true);
  });

  it("lets a page follow its section, and a page below it follow that page", () => {
    const states = effectiveFeatureStates(NODES, { ops: "preview" });
    expect(states.get("ops.sr")).toBe("preview");
    expect(states.get("ops.prob.deep")).toBe("preview");
    expect(states.get("support")).toBe("enabled");
  });

  it("keeps an Available entry under a Preview section", () => {
    const states = effectiveFeatureStates(NODES, { ops: "preview", "ops.inc": "enabled" });
    expect(states.get("ops.inc")).toBe("enabled");
    expect(states.get("ops.prob")).toBe("preview");
  });
});

describe("filterFeatureTree", () => {
  const none = new Set<string>();

  it("shows every section with all of its pages when nothing is narrowed", () => {
    const view = filterFeatureTree(NODES, {}, none, "", "all");
    expect(view.map((entry) => entry.section.id)).toEqual(["ops", "support"]);
    expect(view[0]!.rows.map((row) => row.id)).toEqual(["ops.sr", "ops.inc", "ops.prob", "ops.prob.deep"]);
  });

  it("keeps all of a section's pages when its own name matches the search", () => {
    const view = filterFeatureTree(NODES, {}, none, "operations", "all");
    expect(view).toHaveLength(1);
    expect(view[0]!.rows).toHaveLength(4);
  });

  it("keeps a section for a page that matches, showing only that page", () => {
    const view = filterFeatureTree(NODES, {}, none, "incid", "all");
    expect(view).toHaveLength(1);
    expect(view[0]!.rows.map((row) => row.id)).toEqual(["ops.inc"]);
  });

  it("returns nothing when nothing matches", () => {
    expect(filterFeatureTree(NODES, {}, none, "zzz", "all")).toEqual([]);
  });

  it("filters by the state a page shows, following its section", () => {
    const map = { ops: "preview", "ops.inc": "enabled" } as const;
    const preview = filterFeatureTree(NODES, map, none, "", "preview");
    expect(preview.map((entry) => entry.section.id)).toEqual(["ops"]);
    expect(preview[0]!.rows.map((row) => row.id)).toEqual(["ops.sr", "ops.prob", "ops.prob.deep"]);
    const available = filterFeatureTree(NODES, map, none, "", "enabled");
    expect(available.map((entry) => entry.section.id)).toEqual(["ops", "support"]);
    expect(available[0]!.rows.map((row) => row.id)).toEqual(["ops.inc"]);
  });

  it("filters to the pages with unsaved changes", () => {
    const view = filterFeatureTree(NODES, {}, new Set(["ops.sr", "support"]), "", "changed");
    expect(view.map((entry) => entry.section.id)).toEqual(["ops", "support"]);
    expect(view[0]!.rows.map((row) => row.id)).toEqual(["ops.sr"]);
  });
});

describe("allFeatureNodes", () => {
  it("lists the real nav tree, Knowledge included, with no shortcut", () => {
    const ids = allFeatureNodes().map((entry) => entry.id);
    expect(ids).toContain("kb-articles");
    expect(ids).toContain("kb-articles.admin");
    expect(ids).not.toContain("customers.customer-health");
  });
});
