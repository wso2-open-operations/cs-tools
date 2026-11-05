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

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  configOverridesAsStored,
  featureState,
  isPreviewedOnly,
  listFeatureNodes,
  resetFeatureStatesForTests,
  visibleNavChildren,
} from "@config/featureFlags";
import {
  getSharedFeatureOverridesVersion,
  resetSharedFeatureOverridesForTests,
  setSharedFeatureOverrides,
} from "@config/featureOverridesStore";
import { navNodeById } from "@config/csmNavItems";
import { getPortalAccess } from "@context/current-user/portalAccess";

function setConfig(value: unknown): void {
  window.config = {
    ...window.config,
    CSM_PORTAL_FEATURE_OVERRIDES: value,
  } as Window["config"];
  resetFeatureStatesForTests();
}

const admin = getPortalAccess(["admin"]);
const csEngineer = getPortalAccess(["cs_engineer"]);

beforeEach(() => {
  vi.spyOn(console, "warn").mockImplementation(() => undefined);
  resetSharedFeatureOverridesForTests();
  setConfig(undefined);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("the config file, until an admin has saved", () => {
  it("reads an older wip line as preview", () => {
    setConfig({ operations: "wip" });
    expect(featureState("operations")).toBe("preview");
    expect(featureState("operations.problems")).toBe("preview");
  });

  it("still honours a hidden line, exactly as before", () => {
    setConfig({ updates: "hidden" });
    expect(featureState("updates")).toBe("hidden");
  });

  it("reads a JSON string, and keeps reading it while the table is empty", () => {
    setConfig('{"operations":"preview"}');
    setSharedFeatureOverrides({ rows: {}, savedOnce: false });
    expect(featureState("operations")).toBe("preview");
  });

  it("lays rows from a first save that stopped part-way over the config", () => {
    setConfig({ operations: "preview", customers: "preview" });
    setSharedFeatureOverrides({ rows: { customers: "enabled" }, savedOnce: false });
    expect(featureState("operations")).toBe("preview");
    expect(featureState("customers")).toBe("enabled");
  });

  it("falls back to the config when the shared map could not be loaded", () => {
    setConfig({ operations: "preview" });
    setSharedFeatureOverrides(undefined);
    expect(featureState("operations")).toBe("preview");
  });
});

describe("the saved map, once an admin has saved", () => {
  it("ignores the config file", () => {
    setConfig({ operations: "preview", updates: "hidden" });
    setSharedFeatureOverrides({ rows: { customers: "preview" }, savedOnce: true });
    expect(featureState("operations")).toBe("enabled");
    expect(featureState("updates")).toBe("enabled");
    expect(featureState("customers")).toBe("preview");
  });

  it("keeps an enabled row under a preview section, so Incidents stays open", () => {
    setSharedFeatureOverrides({
      rows: { operations: "preview", "operations.incidents": "enabled" },
      savedOnce: true,
    });
    expect(featureState("operations.incidents", csEngineer)).toBe("enabled");
    expect(featureState("operations.problems", csEngineer)).toBe("preview");
    // The section is promoted for the menu so the open page can be reached.
    expect(featureState("operations", csEngineer)).toBe("enabled");
  });

  it("lets a page with no row follow its section", () => {
    setSharedFeatureOverrides({ rows: { operations: "preview" }, savedOnce: true });
    expect(featureState("operations.service-requests", csEngineer)).toBe("preview");
  });

  it("ignores a stored value it does not know, and a page that does not exist", () => {
    setSharedFeatureOverrides({
      rows: { operations: "hidden", customers: "wip", "no.such.page": "preview" },
      savedOnce: true,
    });
    expect(featureState("operations")).toBe("enabled");
    expect(featureState("customers")).toBe("enabled");
  });

  it("picks up a change without anyone resetting the cache", () => {
    setSharedFeatureOverrides({ rows: {}, savedOnce: true });
    expect(featureState("customers")).toBe("enabled");
    const before = getSharedFeatureOverridesVersion();
    setSharedFeatureOverrides({ rows: { customers: "preview" }, savedOnce: true });
    expect(getSharedFeatureOverridesVersion()).toBeGreaterThan(before);
    expect(featureState("customers")).toBe("preview");
  });
});

describe("a Preview page for an admin and for everyone else", () => {
  beforeEach(() => {
    setSharedFeatureOverrides({
      rows: { operations: "preview", "operations.incidents": "enabled" },
      savedOnce: true,
    });
  });

  it("opens for an admin and stays greyed out for everyone else", () => {
    expect(featureState("operations.problems", admin)).toBe("enabled");
    expect(featureState("operations.problems", csEngineer)).toBe("preview");
    expect(featureState("operations.problems")).toBe("preview");
  });

  it("marks only the preview nodes for an admin, never an enabled one", () => {
    expect(isPreviewedOnly("operations.problems", admin)).toBe(true);
    // The section is Preview as saved, even though it is promoted for the menu.
    expect(isPreviewedOnly("operations", admin)).toBe(true);
    expect(isPreviewedOnly("operations.incidents", admin)).toBe(false);
    expect(isPreviewedOnly("dashboard", admin)).toBe(false);
  });

  it("never marks anything for someone who is not an admin", () => {
    expect(isPreviewedOnly("operations.problems", csEngineer)).toBe(false);
    expect(isPreviewedOnly("operations.problems")).toBe(false);
  });

  it("keeps a Preview tab in the strip for everyone, so it still advertises", () => {
    const operations = navNodeById("operations")!;
    const ids = (access: ReturnType<typeof getPortalAccess>) =>
      visibleNavChildren(operations, access).map((node) => node.id);
    expect(ids(admin)).toContain("operations.problems");
    expect(ids(csEngineer)).toContain("operations.problems");
  });

  it("does not open a page whose role capability the admin lacks", () => {
    const adminWithoutOperations = { ...admin, canUseOperations: false };
    expect(featureState("operations.problems", adminWithoutOperations)).toBe("hidden");
  });
});

describe("listFeatureNodes", () => {
  const nodes = listFeatureNodes();
  const byId = new Map(nodes.map((node) => [node.id, node]));

  it("lists every page including Knowledge, with sections at depth 0", () => {
    expect(byId.get("kb-articles")?.depth).toBe(0);
    expect(byId.get("kb-articles.to-review")?.parentId).toBe("kb-articles");
    expect(byId.get("operations")?.childIds).toContain("operations.incidents");
  });

  it("indents a page that sits a level deeper", () => {
    expect(byId.get("admin.user-management.users")?.depth).toBe(2);
    expect(byId.get("admin.user-management.users")?.parentId).toBe("admin.user-management");
  });

  it("leaves shortcuts out, since they follow the page they point to", () => {
    expect(byId.has("customers.customer-health")).toBe(false);
    expect(byId.get("customers")?.childIds).not.toContain("customers.customer-health");
  });

  it("carries a page's capability, or its section's", () => {
    expect(byId.get("operations.incidents")?.requires).toBe("canUseOperations");
    expect(byId.get("admin.banners")?.requires).toBe("isAdmin");
  });
});

describe("configOverridesAsStored", () => {
  it("writes wip and hidden as preview, keeps enabled, drops unknown pages", () => {
    setConfig({
      operations: "wip",
      updates: "hidden",
      "operations.incidents": "enabled",
      "no.such.page": "preview",
      customers: "nonsense",
    });
    expect(configOverridesAsStored()).toEqual({
      operations: "preview",
      updates: "preview",
      "operations.incidents": "enabled",
    });
  });

  it("is empty when the config names nothing", () => {
    expect(configOverridesAsStored()).toEqual({});
  });
});
