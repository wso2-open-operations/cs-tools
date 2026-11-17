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
import { nameInitials, stripStaffNameMarker } from "./staffNameMarker";

describe("stripStaffNameMarker", () => {
  it("removes the trailing circled W, lowercase or uppercase", () => {
    expect(stripStaffNameMarker("Rashmika Silva ⓦ")).toBe("Rashmika Silva");
    expect(stripStaffNameMarker("Rashmika Silva Ⓦ")).toBe("Rashmika Silva");
    expect(stripStaffNameMarker("Rashmika Silvaⓦ")).toBe("Rashmika Silva");
  });

  it("leaves a name without the marker alone, and tolerates nothing", () => {
    expect(stripStaffNameMarker("Rashmika Silva")).toBe("Rashmika Silva");
    expect(stripStaffNameMarker("  Rashmika  ")).toBe("Rashmika");
    expect(stripStaffNameMarker(undefined)).toBe("");
    expect(stripStaffNameMarker(null)).toBe("");
  });

  it("empties a string that was only the marker", () => {
    expect(stripStaffNameMarker("ⓦ")).toBe("");
  });

  it("does not touch a circled W that is not at the end", () => {
    expect(stripStaffNameMarker("Jane ⓦ Doe")).toBe("Jane ⓦ Doe");
  });
});

describe("nameInitials", () => {
  it("uses the first two words only", () => {
    expect(nameInitials("Rashmika Silva")).toBe("RS");
    expect(nameInitials("Rashmika Silva ⓦ")).toBe("RS");
    expect(nameInitials("Mary Jane Watson")).toBe("MJ");
    expect(nameInitials("Rashmika")).toBe("R");
    expect(nameInitials("ⓦ")).toBe("");
    expect(nameInitials("")).toBe("");
  });
});
