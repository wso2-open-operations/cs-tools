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

import { describe, expect, it, vi } from "vitest";
import {
  changedFeatureIds,
  executeFeatureSave,
  planFeatureSave,
  type FeatureWrite,
} from "@features/csm-admin/feature-visibility/featureSave";

describe("changedFeatureIds", () => {
  it("lists only the pages whose edit differs from what is saved", () => {
    expect(
      changedFeatureIds(
        { operations: "preview", customers: "preview" },
        { operations: "preview", customers: "enabled", support: "enabled" },
      ),
    ).toEqual(["customers", "support"]);
  });
});

describe("planFeatureSave", () => {
  it("sends one write per changed page after the first save, and no mark", () => {
    const writes = planFeatureSave({
      saved: { operations: "preview" },
      edits: { operations: "enabled", customers: "preview" },
      rows: { operations: "preview" },
      savedOnce: true,
    });
    expect(writes).toEqual([
      { navNodeId: "operations", state: "enabled" },
      { navNodeId: "customers", state: "preview" },
    ]);
  });

  it("writes Available as a row, never as a missing one", () => {
    const writes = planFeatureSave({
      saved: { "operations.incidents": "preview" },
      edits: { "operations.incidents": "enabled" },
      rows: { "operations.incidents": "preview" },
      savedOnce: true,
    });
    expect(writes).toEqual([{ navNodeId: "operations.incidents", state: "enabled" }]);
  });

  it("writes a page set to Available that had no row before", () => {
    const writes = planFeatureSave({ saved: {}, edits: { support: "enabled" }, rows: {}, savedOnce: true });
    expect(writes).toEqual([{ navNodeId: "support", state: "enabled" }]);
  });

  it("sends nothing for an edit that matches what is saved", () => {
    expect(
      planFeatureSave({ saved: { operations: "preview" }, edits: { operations: "preview" }, rows: { operations: "preview" }, savedOnce: true }),
    ).toEqual([]);
  });

  describe("the first save", () => {
    const config = { operations: "preview", "operations.incidents": "enabled", customers: "preview" } as const;

    it("keeps the config lines as rows, then the edits, and marks the last write", () => {
      const writes = planFeatureSave({
        saved: { ...config },
        edits: { support: "preview" },
        rows: {},
        savedOnce: false,
      });
      expect(writes.map((write) => write.navNodeId)).toEqual([
        "operations",
        "operations.incidents",
        "customers",
        "support",
      ]);
      expect(writes.filter((write) => write.markSaved)).toEqual([
        { navNodeId: "support", state: "preview", markSaved: true },
      ]);
    });

    it("keeps an Available config line, since an enabled row is a setting", () => {
      const writes = planFeatureSave({ saved: { ...config }, edits: { support: "preview" }, rows: {}, savedOnce: false });
      expect(writes).toContainEqual({ navNodeId: "operations.incidents", state: "enabled" });
    });

    it("writes an edited config page once, with the admin's choice", () => {
      const writes = planFeatureSave({
        saved: { ...config },
        edits: { operations: "enabled" },
        rows: {},
        savedOnce: false,
      });
      // Seeded lines come first and the edit last, so the edit carries the mark.
      expect(writes.filter((write) => write.navNodeId === "operations")).toEqual([
        { navNodeId: "operations", state: "enabled", markSaved: true },
      ]);
      expect(writes).toHaveLength(3);
    });

    it("skips a config line that already has a row from an earlier save that stopped", () => {
      const writes = planFeatureSave({
        saved: { ...config },
        edits: { support: "preview" },
        rows: { operations: "preview" },
        savedOnce: false,
      });
      expect(writes.map((write) => write.navNodeId)).not.toContain("operations");
      expect(writes.map((write) => write.navNodeId)).toContain("customers");
    });

    it("puts the mark on one write only", () => {
      const writes = planFeatureSave({ saved: { ...config }, edits: {}, rows: {}, savedOnce: false });
      expect(writes.filter((write) => write.markSaved)).toHaveLength(1);
      expect(writes[writes.length - 1]!.markSaved).toBe(true);
    });
  });
});

describe("executeFeatureSave", () => {
  const writes: FeatureWrite[] = [
    { navNodeId: "operations", state: "preview" },
    { navNodeId: "customers", state: "preview" },
    { navNodeId: "support", state: "preview", markSaved: true },
  ];

  it("sends the mark last, after every other write went through", async () => {
    const order: string[] = [];
    const put = vi.fn(async (write: FeatureWrite) => {
      order.push(write.navNodeId);
    });
    const result = await executeFeatureSave(writes, put);
    expect(result).toEqual({ succeeded: ["operations", "customers", "support"], failed: [] });
    expect(order[order.length - 1]).toBe("support");
  });

  it("does not send the mark when another write failed, and reports it as not saved", async () => {
    const put = vi.fn(async (write: FeatureWrite) => {
      if (write.navNodeId === "customers") throw new Error("nope");
    });
    const result = await executeFeatureSave(writes, put);
    expect(result.succeeded).toEqual(["operations"]);
    expect(result.failed.sort()).toEqual(["customers", "support"]);
    expect(put).not.toHaveBeenCalledWith(expect.objectContaining({ markSaved: true }));
  });

  it("works through a long first save a few writes at a time", async () => {
    const many: FeatureWrite[] = Array.from({ length: 12 }, (_, index) => ({
      navNodeId: `page-${index}`,
      state: "preview" as const,
    }));
    let running = 0;
    let peak = 0;
    const put = vi.fn(async () => {
      running += 1;
      peak = Math.max(peak, running);
      await Promise.resolve();
      running -= 1;
    });
    const result = await executeFeatureSave(many, put);
    expect(result.succeeded).toHaveLength(12);
    expect(peak).toBeLessThanOrEqual(5);
  });

  it("returns an empty result for no writes", async () => {
    expect(await executeFeatureSave([], vi.fn())).toEqual({ succeeded: [], failed: [] });
  });
});
