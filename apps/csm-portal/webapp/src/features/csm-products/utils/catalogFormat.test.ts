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
  EMPTY_VALUE,
  catalogFieldLabel,
  formatCatalogDate,
  formatCatalogValue,
} from "@features/csm-products/utils/catalogFormat";

describe("formatCatalogDate", () => {
  it("formats a catalogue date and keeps an unparseable one as stored", () => {
    expect(formatCatalogDate("2026-08-01")).toBe("Aug 1, 2026");
    expect(formatCatalogDate("8/1/2026")).toBe("8/1/2026");
    expect(formatCatalogDate(null)).toBe(EMPTY_VALUE);
  });
});

describe("formatCatalogValue", () => {
  it("renders empties, booleans and arrays", () => {
    expect(formatCatalogValue("description", null)).toBe(EMPTY_VALUE);
    expect(formatCatalogValue("isPublic", true)).toBe("Yes");
    expect(formatCatalogValue("retired", true)).toBe("Retired");
    expect(formatCatalogValue("tags", ["OB", "X"])).toBe("OB, X");
    expect(formatCatalogValue("tags", [])).toBe(EMPTY_VALUE);
  });

  it("labels enums, resolves release line ids and formats dates", () => {
    expect(formatCatalogValue("currentSupportStatus", "extended")).toBe("Extended");
    expect(formatCatalogValue("releaseLineId", "id-1", new Map([["id-1", "Line A"]]))).toBe("Line A");
    expect(formatCatalogValue("supportEolDate", "2026-08-01")).toBe("Aug 1, 2026");
    expect(formatCatalogValue("startDate", "2026-08-01")).toBe("Aug 1, 2026");
  });

  it("lists a Carbon object's set parts", () => {
    expect(
      formatCatalogValue("carbon", { platformRelease: "4.4", platformVersion: null, carbonKernelVersion: "4.4.9" }),
    ).toBe("Platform release: 4.4; Kernel version: 4.4.9");
  });

  it("falls back to the key for an unknown field label", () => {
    expect(catalogFieldLabel("supportEolDate")).toBe("EOL date");
    expect(catalogFieldLabel("mystery")).toBe("mystery");
  });
});
