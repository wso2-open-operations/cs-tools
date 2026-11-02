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
import type { CatalogVersionWire } from "@features/csm-products/types/productCatalog";
import { fromWireReleaseLine, fromWireVersion, wireDate } from "@features/csm-products/utils/catalogWire";

const wire = {
  id: "v1",
  productId: "p1",
  productName: "WSO2 API Manager",
  productCode: "wso2am",
  version: "4.2.0",
  deploymentProfile: "All In One",
  serialNumber: null,
  currentSupportStatus: "EXTENDED",
  releaseDate: "2023-05-04T00:00:00Z",
  earliestPossibleSupportEolDate: null,
  supportEolDate: "2027-05-04T00:00:00Z",
  supportEolAnnouncementDate: "2026-11-01T00:00:00Z",
  publicId: 1000,
  publicName: null,
  publicAbbreviation: null,
  description: null,
  isPublic: true,
  showInSupportMatrix: true,
  tags: [],
  releaseLinks: null,
  legacyStatus: null,
  releaseLineId: null,
  releaseLineName: null,
  carbon: null,
  retiredOn: null,
  createdOn: "2026-10-30T10:00:00Z",
  updatedOn: "2026-10-30T10:00:00Z",
} satisfies CatalogVersionWire;

describe("wireDate", () => {
  it("keeps the calendar date of a timestamp or a date", () => {
    expect(wireDate("2024-01-15T00:00:00Z")).toBe("2024-01-15");
    expect(wireDate("2024-01-15")).toBe("2024-01-15");
  });

  it("maps empty and unparseable values to null", () => {
    expect(wireDate(null)).toBeNull();
    expect(wireDate("")).toBeNull();
    expect(wireDate("15/01/2024")).toBeNull();
  });
});

describe("fromWireVersion", () => {
  it("lower-cases the status and turns the date fields into dates", () => {
    const v = fromWireVersion(wire);
    expect(v.currentSupportStatus).toBe("extended");
    expect(v.releaseDate).toBe("2023-05-04");
    expect(v.supportEolDate).toBe("2027-05-04");
    expect(v.supportEolAnnouncementDate).toBe("2026-11-01");
    expect(v.earliestPossibleSupportEolDate).toBeNull();
    expect(v.createdOn).toBe("2026-10-30T10:00:00Z");
  });

  it("drops an unknown status rather than passing it on", () => {
    const v = fromWireVersion({ ...wire, currentSupportStatus: "RETIRED" });
    expect(v.currentSupportStatus).toBeNull();
  });
});

describe("fromWireReleaseLine", () => {
  it("turns the start date into a date", () => {
    const line = fromWireReleaseLine({
      id: "l1",
      name: "APIM-4.x",
      startDate: "2024-04-10T00:00:00Z",
      status: "Active",
      legacyId: 1,
      versionCount: 3,
      retiredOn: null,
      createdOn: "2026-10-30T10:00:00Z",
      updatedOn: "2026-10-30T10:00:00Z",
    });
    expect(line.startDate).toBe("2024-04-10");
  });
});
