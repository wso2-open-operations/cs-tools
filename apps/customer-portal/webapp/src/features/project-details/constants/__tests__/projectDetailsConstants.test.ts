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
  PROJECT_DETAILS_INVALID_PROJECT_ID_MESSAGE,
  PROJECT_DETAILS_NOT_AVAILABLE_DISPLAY,
  PROJECT_DETAILS_TABS,
} from "@features/project-details/constants/projectDetailsConstants";
import { PRODUCT_CLASS } from "@features/project-details/constants/productConstants";
import { PROJECT_METADATA_CHIP_SX } from "@features/project-details/constants/projectInformationConstants";
import { ProjectDetailsTabId } from "@features/project-details/types/projectDetails";

describe("projectDetailsConstants", () => {
  it("exports tabs and placeholders", () => {
    expect(PROJECT_DETAILS_TABS.map((t) => t.id)).toEqual([
      ProjectDetailsTabId.OVERVIEW,
      ProjectDetailsTabId.DEPLOYMENTS,
      ProjectDetailsTabId.TIME_TRACKING,
    ]);
    expect(PROJECT_DETAILS_NOT_AVAILABLE_DISPLAY).toBe("Not Available");
    expect(PROJECT_DETAILS_INVALID_PROJECT_ID_MESSAGE).toContain("Invalid Project ID");
  });
});

describe("productConstants", () => {
  it("exports product class", () => {
    expect(PRODUCT_CLASS).toBeTruthy();
  });
});

describe("projectInformationConstants", () => {
  it("exports metadata chip styles", () => {
    expect(PROJECT_METADATA_CHIP_SX).toBeDefined();
  });
});
