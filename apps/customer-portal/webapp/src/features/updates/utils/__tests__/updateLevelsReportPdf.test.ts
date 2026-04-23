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

import { describe, it, expect } from "vitest";
import { getUpdateLevelsReportData } from "@features/updates/utils/updateLevelsReportPdf";

const mockData = {
  "2": {
    updateType: "regular",
    updateDescriptionLevels: [
      {
        updateLevel: 2,
        productName: "WSO2 API Manager",
        productVersion: "4.4.0",
        channel: "full",
        updateType: "regular",
        updateNumber: 100,
        description: "Fix.",
        instructions: "N/A",
        bugFixes: "[]",
        filesAdded: "[]",
        filesModified: "[]",
        filesRemoved: "[]",
        bundlesInfoChanges: null,
        dependantReleases: null,
        timestamp: 1705939200000,
        securityAdvisories: [],
      },
    ],
  },
  "3": {
    updateType: "security",
    updateDescriptionLevels: [
      {
        updateLevel: 3,
        productName: "WSO2 API Manager",
        productVersion: "4.4.0",
        channel: "full",
        updateType: "security",
        updateNumber: 101,
        description: "Security fix.",
        instructions: "N/A",
        bugFixes: "[]",
        filesAdded: "[]",
        filesModified: "[]",
        filesRemoved: "[]",
        bundlesInfoChanges: null,
        dependantReleases: null,
        timestamp: 1706025600000,
        securityAdvisories: [],
      },
    ],
  },
};

describe("getUpdateLevelsReportData", () => {
  it("returns structured report data with expected fields", () => {
    const result = getUpdateLevelsReportData({
      productName: "WSO2 API Manager",
      productVersion: "4.4.0",
      startLevel: 2,
      endLevel: 19,
      data: mockData,
    });

    expect(result.productName).toBe("WSO2 API Manager");
    expect(result.productVersion).toBe("4.4.0");
    expect(result.startLevel).toBe(2);
    expect(result.endLevel).toBe(19);
    expect(result.securityCount).toBe(1);
    expect(result.regularCount).toBe(1);
    expect(result.totalUpdates).toBe(2);
    expect(result.levelCount).toBe(2);
    expect(result.levelsRange).toBe("2 - 3");
    expect(result.tableRows).toHaveLength(2);
  });

  it("formats release dates correctly in UTC", () => {
    const result = getUpdateLevelsReportData({
      productName: "Product",
      productVersion: "1.0",
      startLevel: 2,
      endLevel: 3,
      data: mockData,
    });

    // mockData level 2 has timestamp 1705939200000 = Jan 22, 2024 UTC
    expect(result.tableRows[0].releaseDate).toBe("Jan 22, 2024");
    // mockData level 3 has timestamp 1706025600000 = Jan 23, 2024 UTC
    expect(result.tableRows[1].releaseDate).toBe("Jan 23, 2024");
  });

  it("throws when data is empty", () => {
    expect(() =>
      getUpdateLevelsReportData({
        productName: "Product",
        productVersion: "1.0",
        startLevel: 1,
        endLevel: 5,
        data: {},
      }),
    ).toThrow(/No update data/);
  });
});
