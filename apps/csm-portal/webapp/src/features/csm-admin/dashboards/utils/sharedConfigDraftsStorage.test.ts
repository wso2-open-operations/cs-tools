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
import type { BeDashboardWidget } from "@api/backend/types";
import {
  deployableDashboardFromDraft,
  getSharedConfigDraft,
  presetsFileFromDraft,
  saveSharedConfigDraft,
  sectionsFileFromDraft,
  seedSharedConfigDraft,
  widgetToDefinition,
} from "@features/csm-admin/dashboards/utils/sharedConfigDraftsStorage";

const widget = (overrides: Partial<BeDashboardWidget> = {}): BeDashboardWidget => ({
  widgetId: "my_open",
  displayName: "My Open",
  resourceType: "case",
  shape: "list",
  gridWidth: 12,
  query: { filters: [{ preset: "activeCaseStates" }] },
  ...overrides,
});

describe("sharedConfigDraftsStorage", () => {
  beforeEach(() => localStorage.clear());

  describe("presetsFileFromDraft", () => {
    it("emits the loader's name-keyed shape, not the endpoint's array shape", () => {
      expect(
        presetsFileFromDraft([
          { name: "b", filter: { field: "state", op: "in", values: ["open"] } },
          { name: "a", filter: { field: "tag", op: "in", values: ["x"] } },
        ]),
      ).toEqual({
        a: { field: "tag", op: "in", values: ["x"] },
        b: { field: "state", op: "in", values: ["open"] },
      });
    });

    it("emits keys in sorted order so re-exporting an unchanged draft diffs cleanly", () => {
      const out = presetsFileFromDraft([
        { name: "zeta", filter: {} },
        { name: "alpha", filter: {} },
      ]);
      expect(Object.keys(out)).toEqual(["alpha", "zeta"]);
    });

    it("skips an unnamed preset rather than emitting an empty key", () => {
      expect(presetsFileFromDraft([{ name: "   ", filter: { a: 1 } }])).toEqual({});
    });
  });

  describe("sectionsFileFromDraft", () => {
    it("renames widgetId to id, which is what the definition file requires", () => {
      const out = sectionsFileFromDraft([
        { name: "my-work", displayName: "My Work", widgets: [widget()] },
      ]);
      expect(out["my-work"].widgets[0].id).toBe("my_open");
      expect(out["my-work"].widgets[0].widgetId).toBeUndefined();
    });

    it("keeps an unexpanded preset reference in a section widget's query", () => {
      // A section file stores the authored form; expanding here would defeat
      // the entire point of referencing a preset.
      const out = sectionsFileFromDraft([
        { name: "my-work", displayName: "My Work", widgets: [widget()] },
      ]);
      expect(out["my-work"].widgets[0].query).toEqual({
        filters: [{ preset: "activeCaseStates" }],
      });
    });

    it("omits each widget's own section, which the loader overwrites anyway", () => {
      const out = sectionsFileFromDraft([
        {
          name: "my-work",
          displayName: "My Work",
          widgets: [widget({ section: "Something Else" })],
        },
      ]);
      expect(out["my-work"].widgets[0].section).toBeUndefined();
    });
  });

  describe("widgetToDefinition", () => {
    it("drops absent optional keys instead of emitting nulls", () => {
      const out = widgetToDefinition(widget());
      expect(out).not.toHaveProperty("groupBy");
      expect(out).not.toHaveProperty("columns");
      expect(out).not.toHaveProperty("description");
    });

    it("carries the optional keys that are present", () => {
      const out = widgetToDefinition(
        widget({
          description: "why",
          listLimit: 5,
          sortBy: { field: "updatedOn", order: "asc" },
          columns: [{ path: "number", label: "Number" }],
        }),
      );
      expect(out.description).toBe("why");
      expect(out.listLimit).toBe(5);
      expect(out.sortBy).toEqual({ field: "updatedOn", order: "asc" });
      expect(out.columns).toEqual([{ path: "number", label: "Number" }]);
    });
  });

  describe("seedSharedConfigDraft", () => {
    it("folds the deployed catalogues in on a fresh draft", () => {
      const got = seedSharedConfigDraft(
        [{ name: "activeCaseStates", filter: { field: "state", op: "in", values: ["open"] } }],
        [{ name: "my-work", displayName: "My Work", widgets: [widget()] }],
      );
      expect(got.presets).toHaveLength(1);
      expect(got.sections).toHaveLength(1);
      expect(got.seeded).toBe(true);
    });

    it("does not re-seed once seeded, so a deleted entry stays deleted", () => {
      seedSharedConfigDraft([{ name: "gone", filter: {} }], []);
      // The admin deletes it.
      saveSharedConfigDraft({ presets: [], sections: [], seeded: true });
      // A later visit re-runs the seed with the same deployed catalogue.
      const got = seedSharedConfigDraft([{ name: "gone", filter: {} }], []);
      expect(got.presets).toEqual([]);
    });

    it("does not clobber an in-progress edit of a deployed entry", () => {
      saveSharedConfigDraft({
        presets: [{ name: "activeCaseStates", filter: { field: "state", op: "in", values: ["mine"] } }],
        sections: [],
        seeded: false,
      });
      const got = seedSharedConfigDraft(
        [{ name: "activeCaseStates", filter: { field: "state", op: "in", values: ["deployed"] } }],
        [],
      );
      expect(got.presets).toHaveLength(1);
      expect(got.presets[0].filter).toEqual({ field: "state", op: "in", values: ["mine"] });
    });
  });

  it("survives corrupt storage rather than throwing", () => {
    localStorage.setItem("csm.dashboardSharedConfigDraft", "{not json");
    expect(getSharedConfigDraft()).toEqual({
      presets: [],
      sections: [],
      updatedAt: "",
      seeded: false,
    });
  });

  it("survives storage whose fields are the wrong types", () => {
    localStorage.setItem(
      "csm.dashboardSharedConfigDraft",
      JSON.stringify({ presets: "nope", sections: 3, seeded: "yes" }),
    );
    const got = getSharedConfigDraft();
    expect(got.presets).toEqual([]);
    expect(got.sections).toEqual([]);
    expect(got.seeded).toBe(false);
  });
});

describe("deployableDashboardFromDraft", () => {
  const draft = {
    id: "draft-abc",
    displayName: "My Dashboard",
    type: "cs" as const,
    isDefault: true,
    isTeamBased: false,
    widgets: [widget()],
  };

  it("emits an id, which the loader requires and the old export omitted", () => {
    expect(deployableDashboardFromDraft(draft).id).toBe("draft-abc");
  });

  it("keeps the deployed dashboard's own id when the draft came from one", () => {
    // Otherwise re-deploying an edit would create a SECOND dashboard rather
    // than replacing the one being edited.
    expect(
      deployableDashboardFromDraft({ ...draft, sourceDashboardId: "abt-engineer" }).id,
    ).toBe("abt-engineer");
  });

  it("renames each widget's widgetId to id", () => {
    const out = deployableDashboardFromDraft(draft);
    const widgets = out.widgets as Record<string, unknown>[];
    expect(widgets[0].id).toBe("my_open");
    expect(widgets[0].widgetId).toBeUndefined();
  });

  it("omits includeSections entirely when there are none", () => {
    expect(deployableDashboardFromDraft(draft)).not.toHaveProperty("includeSections");
  });

  it("carries includeSections when present", () => {
    const out = deployableDashboardFromDraft({
      ...draft,
      includeSections: [{ section: "my-work", position: "start" }],
    });
    expect(out.includeSections).toEqual([{ section: "my-work", position: "start" }]);
  });

  it("drops the builder's own bookkeeping fields", () => {
    const out = deployableDashboardFromDraft({
      ...draft,
      sourceDashboardId: "abt-engineer",
    });
    expect(out).not.toHaveProperty("sourceDashboardId");
    expect(out).not.toHaveProperty("emptySections");
    expect(out).not.toHaveProperty("updatedAt");
  });
});

describe("read() defends the contents of the persisted arrays", () => {
  beforeEach(() => localStorage.clear());

  it("drops a null preset entry instead of throwing on it later", () => {
    // Previously survived read() (Array.isArray passes) and then threw on
    // `p.name` during seeding, leaving the designer unopenable until the
    // user cleared localStorage by hand.
    localStorage.setItem(
      "csm.dashboardSharedConfigDraft",
      JSON.stringify({ presets: [null, { name: "ok", filter: { a: 1 } }], sections: [] }),
    );
    const got = getSharedConfigDraft();
    expect(got.presets).toEqual([{ name: "ok", filter: { a: 1 } }]);
  });

  it("drops a preset with no filter body", () => {
    localStorage.setItem(
      "csm.dashboardSharedConfigDraft",
      JSON.stringify({ presets: [{ name: "bad" }], sections: [] }),
    );
    expect(getSharedConfigDraft().presets).toEqual([]);
  });

  it("drops a malformed section and malformed widgets inside a good one", () => {
    localStorage.setItem(
      "csm.dashboardSharedConfigDraft",
      JSON.stringify({
        presets: [],
        sections: [
          null,
          { name: "no-widgets-array", displayName: "X" },
          { name: "ok", displayName: "Ok", widgets: [null, { widgetId: "w" }] },
        ],
      }),
    );
    const got = getSharedConfigDraft();
    expect(got.sections).toHaveLength(1);
    expect(got.sections[0].name).toBe("ok");
    expect(got.sections[0].widgets).toEqual([{ widgetId: "w" }]);
  });

  it("seeding still works over a draft that had malformed entries", () => {
    localStorage.setItem(
      "csm.dashboardSharedConfigDraft",
      JSON.stringify({ presets: [null], sections: [null] }),
    );
    const got = seedSharedConfigDraft([{ name: "p", filter: {} }], []);
    expect(got.presets).toEqual([{ name: "p", filter: {} }]);
  });
});

describe("export-time preset collapse", () => {
  const presets = [
    {
      name: "activeCaseStates",
      filter: { field: "state", op: "in", values: ["open", "work_in_progress"] },
    },
  ];
  const expanded = {
    filters: [{ field: "state", op: "in", values: ["open", "work_in_progress"] }],
  };

  it("rewrites a literal filter back to its preset reference on export", () => {
    // Covers the widget an admin never opened in the editor: the draft holds
    // the API's expanded form, so without this the reference is lost purely
    // because nobody happened to click that widget.
    const out = deployableDashboardFromDraft(
      {
        id: "d",
        displayName: "D",
        isDefault: false,
        isTeamBased: false,
        widgets: [widget({ query: expanded })],
      },
      presets,
    );
    const widgets = out.widgets as Record<string, unknown>[];
    expect(widgets[0].query).toEqual({ filters: [{ preset: "activeCaseStates" }] });
  });

  it("rewrites slice queries too", () => {
    const out = deployableDashboardFromDraft(
      {
        id: "d",
        displayName: "D",
        isDefault: false,
        isTeamBased: false,
        widgets: [
          widget({
            shape: "pie",
            query: null,
            slices: [{ label: "A", query: expanded }],
          }),
        ],
      },
      presets,
    );
    const widgets = out.widgets as Record<string, unknown>[];
    const slices = widgets[0].slices as { query: unknown }[];
    expect(slices[0].query).toEqual({ filters: [{ preset: "activeCaseStates" }] });
  });

  it("leaves a filter no preset accounts for alone", () => {
    const other = { filters: [{ field: "severity", op: "in", values: ["critical"] }] };
    const out = deployableDashboardFromDraft(
      {
        id: "d",
        displayName: "D",
        isDefault: false,
        isTeamBased: false,
        widgets: [widget({ query: other })],
      },
      presets,
    );
    const widgets = out.widgets as Record<string, unknown>[];
    expect(widgets[0].query).toEqual(other);
  });

  it("emits literals unchanged when there is no catalogue", () => {
    const out = deployableDashboardFromDraft({
      id: "d",
      displayName: "D",
      isDefault: false,
      isTeamBased: false,
      widgets: [widget({ query: expanded })],
    });
    const widgets = out.widgets as Record<string, unknown>[];
    expect(widgets[0].query).toEqual(expanded);
  });

  it("collapses inside a designed section's widgets too", () => {
    const out = sectionsFileFromDraft(
      [{ name: "s", displayName: "S", widgets: [widget({ query: expanded })] }],
      presets,
    );
    expect(out.s.widgets[0].query).toEqual({ filters: [{ preset: "activeCaseStates" }] });
  });
});
