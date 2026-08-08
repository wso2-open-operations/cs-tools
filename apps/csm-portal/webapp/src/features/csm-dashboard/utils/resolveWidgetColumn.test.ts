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
import { formatColumnValue, resolveColumnPath } from "./resolveWidgetColumn";

describe("resolveColumnPath", () => {
  it("resolves a top-level field", () => {
    expect(resolveColumnPath({ subject: "Disk full" }, "subject")).toBe("Disk full");
  });

  it("resolves a one-level nested field", () => {
    const item = { project: { id: "p-1", name: "Alpha", key: "ALPHA" } };
    expect(resolveColumnPath(item, "project.key")).toBe("ALPHA");
  });

  it("resolves an arbitrarily deep nested field", () => {
    const item = {
      project: {
        id: "p-1",
        name: "Alpha",
        account: { id: "a-1", name: "WSO2", tier: "Platinum" },
      },
    };
    expect(resolveColumnPath(item, "project.account.tier")).toBe("Platinum");
  });

  it("returns undefined for a missing top-level field", () => {
    expect(resolveColumnPath({ subject: "x" }, "bestCaseFixEta")).toBeUndefined();
  });

  it("returns undefined when an intermediate segment is missing", () => {
    expect(resolveColumnPath({ project: null }, "project.key")).toBeUndefined();
    expect(resolveColumnPath({}, "project.account.tier")).toBeUndefined();
  });

  it("returns undefined when an intermediate segment is not an object", () => {
    expect(resolveColumnPath({ project: "not-an-object" }, "project.key")).toBeUndefined();
  });
});

describe("formatColumnValue", () => {
  it("renders a plain string as-is", () => {
    expect(formatColumnValue("Disk full")).toBe("Disk full");
  });

  it("stringifies a number/boolean", () => {
    expect(formatColumnValue(42)).toBe("42");
    expect(formatColumnValue(true)).toBe("true");
  });

  it("renders the empty placeholder for null/undefined/empty string", () => {
    expect(formatColumnValue(null)).toBe("—");
    expect(formatColumnValue(undefined)).toBe("—");
    expect(formatColumnValue("")).toBe("—");
  });

  it("renders the empty placeholder for a nested object/array (not a scalar)", () => {
    expect(formatColumnValue({ id: "1" })).toBe("—");
    expect(formatColumnValue([1, 2])).toBe("—");
  });

  it("formats a date-only string with format 'date'", () => {
    expect(formatColumnValue("2026-08-01", "date")).toBe("Aug 1, 2026");
  });

  it("formats an ISO timestamp with format 'date'", () => {
    expect(formatColumnValue("2026-08-01T10:00:00Z", "date")).toMatch(/Aug 1, 2026|Jul 31, 2026/);
  });

  it("falls back to the empty placeholder for an unparseable date string", () => {
    expect(formatColumnValue("not-a-date", "date")).toBe("—");
  });

  it("falls back to the empty placeholder when format is 'date' but the value isn't a string", () => {
    expect(formatColumnValue(42, "date")).toBe("—");
  });
});
