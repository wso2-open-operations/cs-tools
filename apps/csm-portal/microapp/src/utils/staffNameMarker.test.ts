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
import { joinStaffName, stripStaffNameMarker } from "./staffNameMarker";

describe("stripStaffNameMarker", () => {
  it("removes the trailing circled W, lowercase or uppercase", () => {
    expect(stripStaffNameMarker("Rashmika Silva ⓦ")).toBe("Rashmika Silva");
    expect(stripStaffNameMarker("Rashmika Silva Ⓦ")).toBe("Rashmika Silva");
    expect(stripStaffNameMarker("Hesara Perera (Intern) Ⓦ")).toBe("Hesara Perera (Intern)");
  });

  it("leaves other names alone and tolerates nothing", () => {
    expect(stripStaffNameMarker("Rashmika Silva")).toBe("Rashmika Silva");
    expect(stripStaffNameMarker(undefined)).toBe("");
    expect(stripStaffNameMarker(null)).toBe("");
    expect(stripStaffNameMarker("ⓦ")).toBe("");
  });
});

describe("joinStaffName", () => {
  it("strips each part before joining, so a marker on the first name is caught too", () => {
    expect(joinStaffName("Jane Ⓦ", "Doe")).toBe("Jane Doe");
    expect(joinStaffName("Jane", "Doe ⓦ")).toBe("Jane Doe");
    expect(joinStaffName(undefined, "Doe")).toBe("Doe");
    expect(joinStaffName(null, null)).toBe("");
  });
});
