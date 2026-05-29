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
import { getUpdateLevelsForProductVersion } from "@features/updates/utils/allUpdatesTab";

describe("getUpdateLevelsForProductVersion", () => {
  it("always includes 0 in returned level options", () => {
    const data = [
      {
        productName: "WSO2 API Manager",
        productUpdateLevels: [
          {
            productBaseVersion: "4.2.0",
            channel: "full",
            updateLevels: [3, 5, 7],
          },
        ],
      },
    ];

    expect(getUpdateLevelsForProductVersion(data, "WSO2 API Manager", "4.2.0")).toEqual([
      0, 3, 5, 7,
    ]);
  });

  it("returns only 0 when update levels are unavailable", () => {
    expect(getUpdateLevelsForProductVersion(undefined, "WSO2 API Manager", "4.2.0")).toEqual([0]);
  });
});

