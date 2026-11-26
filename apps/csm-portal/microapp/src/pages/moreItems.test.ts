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
import { getPortalAccess } from "@src/access/portalAccess";
import { SETTINGS_MORE_PATH, isMoreItemVisible } from "./moreItems";

describe("isMoreItemVisible", () => {
  it("hides Settings from a viewer and from a catalogue manager", () => {
    expect(isMoreItemVisible(SETTINGS_MORE_PATH, getPortalAccess(["viewer"]))).toBe(false);
    expect(isMoreItemVisible(SETTINGS_MORE_PATH, getPortalAccess(["viewer", "product_catalog_manager"]))).toBe(false);
  });

  it("shows Settings to a CS engineer and leaves other More links alone", () => {
    expect(isMoreItemVisible(SETTINGS_MORE_PATH, getPortalAccess(["cs_engineer"]))).toBe(true);
    expect(isMoreItemVisible("/profile", getPortalAccess(["viewer"]))).toBe(true);
  });
});
